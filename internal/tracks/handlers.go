package tracks

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/bogdan/kirpichmusic/internal/artists"
	"github.com/bogdan/kirpichmusic/internal/auth"
	"github.com/bogdan/kirpichmusic/internal/httpx"
	"github.com/bogdan/kirpichmusic/internal/storage"
)

type Handler struct {
	repo    *Repository
	storage *storage.Storage
	artists *artists.Repository // опционально; если nil — старое поведение (без artist_id)
}

func NewHandler(repo *Repository, st *storage.Storage) *Handler {
	return &Handler{repo: repo, storage: st}
}

// SetArtistsRepo — поздняя инжекция (избегаем циклической зависимости пакетов).
func (h *Handler) SetArtistsRepo(r *artists.Repository) {
	h.artists = r
}

// allowedGenres — те же значения, что в DB CHECK.
var allowedGenres = map[string]struct{}{
	"Electronic": {}, "Hip-Hop": {}, "Rock": {}, "Pop": {}, "Jazz": {},
	"Classical": {}, "Ambient": {}, "Lo-fi": {}, "Другое": {},
}

// Upload — POST /api/tracks (multipart/form-data, требует auth)
//
// Поля формы:
//   audio       (file, required)
//   cover       (file, optional)
//   title       (string, required)
//   artist      (string, required)
//   genre       (string, optional, из whitelist)
//   duration    (int seconds, optional, иначе 0)
//   cover_color (string, optional, fallback)
//   cover_emoji (string, optional, fallback)
//
// Безопасность:
//   • MaxBytesReader на R.Body (предотвращение memory-DoS)
//   • MIME-detection из magic bytes (не trust-by-extension)
//   • DB CHECK constraints — третий рубеж
//   • path-traversal невозможен: имя файла генерируется на сервере
//
// Производительность:
//   • Файлы стримятся на диск, не буферизируются в RAM
//   • Post-processing (инкремент plays и т.п.) уйдёт в горутины
//   • Голый INSERT через параметризованный запрос
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	uid := auth.UserIDFromContext(r.Context())
	if uid == 0 {
		httpx.WriteUnauthorized(w)
		return
	}

	// Лимит на размер всего multipart-тела. ParseMultipartForm уважает MaxBytesReader.
	r.Body = http.MaxBytesReader(w, r.Body, 110*1024*1024) // 110 MiB запас сверху MAX_UPLOAD_MB
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "Файл слишком большой", "too_large")
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	artist := strings.TrimSpace(r.FormValue("artist"))
	artistIDStr := strings.TrimSpace(r.FormValue("artist_id"))
	genre := strings.TrimSpace(r.FormValue("genre"))
	durStr := strings.TrimSpace(r.FormValue("duration"))
	coverColor := strings.TrimSpace(r.FormValue("cover_color"))
	coverEmoji := strings.TrimSpace(r.FormValue("cover_emoji"))

	if title == "" || len(title) > 200 {
		httpx.WriteBadRequest(w, "title должен быть 1..200 символов")
		return
	}
	if artist == "" || len(artist) > 120 {
		httpx.WriteBadRequest(w, "artist должен быть 1..120 символов")
		return
	}

	// Резолв artist_id ДО загрузки файлов: если артист claimed на другого
	// юзера, мы хотим знать это сразу — но создание артиста делаем уже после
	// успешной записи аудио (чтобы не плодить артистов на каждую отвалившуюся
	// загрузку).
	var (
		artistID         int64
		moderationStatus = "approved"
	)
	if h.artists != nil {
		if artistIDStr != "" {
			// Юзер выбрал существующего артиста через autocomplete-suggestion.
			id, err := strconv.ParseInt(artistIDStr, 10, 64)
			if err != nil || id <= 0 {
				httpx.WriteBadRequest(w, "invalid artist_id")
				return
			}
			a, err := h.artists.GetByID(r.Context(), id)
			if err != nil {
				httpx.WriteBadRequest(w, "artist not found")
				return
			}
			artistID = a.ID
			// Имя берём из БД (а не из формы), чтобы CITEXT-канон сохранялся.
			artist = a.Name
		}
		// Проверяем claim ДО загрузки файла, чтобы можно было сразу понять статус.
		if artistID > 0 {
			cb, err := h.artists.ClaimedBy(r.Context(), artistID)
			if err == nil && cb != nil && *cb != uid {
				if !auth.IsAdminFromContext(r.Context()) {
					moderationStatus = "pending"
				}
			}
		}
	}
	var genrePtr *string
	if genre != "" {
		if _, ok := allowedGenres[genre]; !ok {
			httpx.WriteBadRequest(w, "Недопустимый жанр")
			return
		}
		genrePtr = &genre
	}
	dur := 0
	if durStr != "" {
		v, err := strconv.Atoi(durStr)
		if err != nil || v < 0 || v > 24*3600 {
			httpx.WriteBadRequest(w, "duration должен быть int 0..86400")
			return
		}
		dur = v
	}

	// 1) Аудио — обязателен.
	af, ahdr, err := r.FormFile("audio")
	if err != nil {
		httpx.WriteBadRequest(w, "Не приложен файл audio")
		return
	}
	defer af.Close()

	stored, err := h.storage.SaveAudio(af, ahdr)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrUnsupportedMIME):
			httpx.WriteBadRequest(w, "Неподдерживаемый формат аудио")
		case errors.Is(err, storage.ErrTooLarge):
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, "Аудиофайл слишком большой", "too_large")
		case errors.Is(err, storage.ErrEmpty):
			httpx.WriteBadRequest(w, "Пустой аудиофайл")
		default:
			log.Printf("save audio: %v", err)
			httpx.WriteInternal(w)
		}
		return
	}

	// 2) Обложка — опциональна.
	var coverStored *storage.Stored
	if cf, chdr, err := r.FormFile("cover"); err == nil {
		defer cf.Close()
		cs, cerr := h.storage.SaveCover(cf, chdr)
		if cerr != nil {
			// Если обложка плохая, удаляем уже сохранённое аудио и отклоняем целиком.
			_ = h.storage.Remove(stored.RelPath)
			switch {
			case errors.Is(cerr, storage.ErrUnsupportedMIME):
				httpx.WriteBadRequest(w, "Неподдерживаемый формат обложки (только JPG/PNG/WEBP)")
			case errors.Is(cerr, storage.ErrTooLarge):
				httpx.WriteError(w, http.StatusRequestEntityTooLarge, "Обложка слишком большая", "too_large")
			default:
				log.Printf("save cover: %v", cerr)
				httpx.WriteInternal(w)
			}
			return
		}
		coverStored = cs
	}

	// Если ещё не резолвили artist_id, делаем это сейчас (по имени).
	// Анти-дубль: FindOrCreateByName использует UNIQUE(name) с CITEXT.
	if h.artists != nil && artistID == 0 {
		aid, _, err := h.artists.FindOrCreateByName(r.Context(), artist)
		if err != nil {
			_ = h.storage.Remove(stored.RelPath)
			if coverStored != nil {
				_ = h.storage.Remove(coverStored.RelPath)
			}
			log.Printf("find/create artist: %v", err)
			httpx.WriteInternal(w)
			return
		}
		artistID = aid
		// Перепроверяем claim после авто-создания (на случай, если артиста
		// уже привязали под другим username case'ом).
		cb, err := h.artists.ClaimedBy(r.Context(), artistID)
		if err == nil && cb != nil && *cb != uid && !auth.IsAdminFromContext(r.Context()) {
			moderationStatus = "pending"
		}
	}

	in := CreateInput{
		UploaderID:       uid,
		Title:            title,
		ArtistName:       artist,
		Genre:            genrePtr,
		DurationSec:      dur,
		AudioPath:        stored.RelPath,
		AudioMIME:        stored.MIME,
		AudioBytes:       stored.Bytes,
		ModerationStatus: moderationStatus,
	}
	if artistID > 0 {
		in.ArtistID = &artistID
	}
	if coverStored != nil {
		in.CoverPath = &coverStored.RelPath
		in.CoverMIME = &coverStored.MIME
		in.CoverBytes = &coverStored.Bytes
	}
	if coverColor != "" {
		in.CoverColor = &coverColor
	}
	if coverEmoji != "" {
		in.CoverEmoji = &coverEmoji
	}

	id, _, err := h.repo.Create(r.Context(), in)
	if err != nil {
		// Откатываем файлы если БД отказала — иначе будут осиротевшие.
		_ = h.storage.Remove(stored.RelPath)
		if coverStored != nil {
			_ = h.storage.Remove(coverStored.RelPath)
		}
		log.Printf("insert track: %v", err)
		httpx.WriteInternal(w)
		return
	}

	t, err := h.repo.GetByID(r.Context(), id, uid)
	if err != nil {
		httpx.WriteInternal(w)
		return
	}

	// Постпроцессинг: пики формы волны.
	// Запускаем в горутине, чтобы Upload-handler ответил клиенту немедленно.
	// Если ffmpeg отсутствует или не справился — фронт нарисует случайный
	// фолбэк-waveform, как было в исходнике.
	go func(trackID int64, audioRel string) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		abs, perr := h.storage.SafePath(audioRel)
		if perr != nil {
			log.Printf("waveform: bad path %v", perr)
			return
		}
		peaks, ferr := ExtractPeaks(ctx, abs, 120)
		if ferr != nil {
			log.Printf("waveform: extract %v", ferr)
			return
		}
		if uerr := h.repo.SetWaveform(ctx, trackID, peaks); uerr != nil {
			log.Printf("waveform: set %v", uerr)
		}
	}(id, stored.RelPath)

	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"track": t})
}

// List — GET /api/tracks?sort=recent|popular&genre=Rock&q=miku&limit=30&offset=0
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	uploaderID, _ := strconv.ParseInt(q.Get("uploader_id"), 10, 64)
	artistID, _ := strconv.ParseInt(q.Get("artist_id"), 10, 64)

	viewer := auth.UserIDFromContext(r.Context())
	// На "Моих треках" (?own=1) показываем pending тоже.
	includeOwnPending := q.Get("own") == "1" && viewer > 0

	tracks, err := h.repo.List(r.Context(), ListParams{
		Sort:              q.Get("sort"),
		Genre:             q.Get("genre"),
		Query:             strings.TrimSpace(q.Get("q")),
		UploaderID:        uploaderID,
		ArtistID:          artistID,
		Limit:             limit,
		Offset:            offset,
		ViewerID:          viewer,
		IncludeOwnPending: includeOwnPending,
	})
	if err != nil {
		log.Printf("list tracks: %v", err)
		httpx.WriteInternal(w)
		return
	}
	if tracks == nil {
		tracks = []Track{}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"tracks": tracks})
}

// GetOne — GET /api/tracks/{id}
func (h *Handler) GetOne(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	viewer := auth.UserIDFromContext(r.Context())
	t, err := h.repo.GetByID(r.Context(), id, viewer)
	if err != nil {
		if errors.Is(err, ErrTrackNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "Track not found", "not_found")
			return
		}
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"track": t})
}

// Like — POST /api/tracks/{id}/like (cookie required)
func (h *Handler) Like(w http.ResponseWriter, r *http.Request) {
	uid := auth.UserIDFromContext(r.Context())
	if uid == 0 {
		httpx.WriteUnauthorized(w)
		return
	}
	id, ok := parseID(r)
	if !ok {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	exists, err := h.repo.TrackExists(r.Context(), id)
	if err != nil {
		httpx.WriteInternal(w)
		return
	}
	if !exists {
		httpx.WriteError(w, http.StatusNotFound, "Track not found", "not_found")
		return
	}
	if _, err := h.repo.AddLike(r.Context(), uid, id); err != nil {
		log.Printf("add like: %v", err)
		httpx.WriteInternal(w)
		return
	}
	t, err := h.repo.GetByID(r.Context(), id, uid)
	if err != nil {
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"track": t})
}

// Unlike — DELETE /api/tracks/{id}/like (cookie required)
func (h *Handler) Unlike(w http.ResponseWriter, r *http.Request) {
	uid := auth.UserIDFromContext(r.Context())
	if uid == 0 {
		httpx.WriteUnauthorized(w)
		return
	}
	id, ok := parseID(r)
	if !ok {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	if _, err := h.repo.RemoveLike(r.Context(), uid, id); err != nil {
		log.Printf("remove like: %v", err)
		httpx.WriteInternal(w)
		return
	}
	t, err := h.repo.GetByID(r.Context(), id, uid)
	if err != nil {
		if errors.Is(err, ErrTrackNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "Track not found", "not_found")
			return
		}
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"track": t})
}

// Edit — PATCH /api/tracks/{id}  (multipart/form-data, cookie required)
//
// Поля формы — все опциональны (передаём только то, что меняем):
//   title       (string)
//   artist      (string)
//   genre       (string)
//   cover       (file) — если приложен, заменяет обложку, старая удаляется
//   cover_color, cover_emoji (string) — фолбэк-обложка
//
// Право редактировать — только у автора трека (uploader_id).
// Любой посторонний получит 404 (не раскрываем существование).
func (h *Handler) Edit(w http.ResponseWriter, r *http.Request) {
	uid := auth.UserIDFromContext(r.Context())
	if uid == 0 {
		httpx.WriteUnauthorized(w)
		return
	}
	id, ok := parseID(r)
	if !ok {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 12*1024*1024)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "Слишком большой файл", "too_large")
		return
	}

	in := UpdateInput{}

	// Form у нас уже распарсен; чтобы отличить "пришло пустое значение" от
	// "поле вообще не отправлено", смотрим в r.MultipartForm.Value.
	form := r.MultipartForm
	hasField := func(key string) bool {
		_, ok := form.Value[key]
		return ok
	}

	if hasField("title") {
		v := strings.TrimSpace(r.FormValue("title"))
		if v == "" || len(v) > 200 {
			httpx.WriteBadRequest(w, "title должен быть 1..200 символов")
			return
		}
		in.Title = &v
	}
	if hasField("artist") {
		v := strings.TrimSpace(r.FormValue("artist"))
		if v == "" || len(v) > 120 {
			httpx.WriteBadRequest(w, "artist должен быть 1..120 символов")
			return
		}
		in.ArtistName = &v
	}
	if hasField("genre") {
		v := strings.TrimSpace(r.FormValue("genre"))
		if v != "" {
			if _, ok := allowedGenres[v]; !ok {
				httpx.WriteBadRequest(w, "Недопустимый жанр")
				return
			}
		}
		in.Genre = &v
	}
	if hasField("cover_color") {
		v := strings.TrimSpace(r.FormValue("cover_color"))
		if v != "" {
			in.CoverColor = &v
		}
	}
	if hasField("cover_emoji") {
		v := strings.TrimSpace(r.FormValue("cover_emoji"))
		if v != "" {
			in.CoverEmoji = &v
		}
	}

	// Новая обложка (опционально).
	var newCover *storage.Stored
	if cf, chdr, err := r.FormFile("cover"); err == nil {
		defer cf.Close()
		cs, cerr := h.storage.SaveCover(cf, chdr)
		if cerr != nil {
			switch {
			case errors.Is(cerr, storage.ErrUnsupportedMIME):
				httpx.WriteBadRequest(w, "Неподдерживаемый формат обложки (JPG/PNG/WEBP)")
			case errors.Is(cerr, storage.ErrTooLarge):
				httpx.WriteError(w, http.StatusRequestEntityTooLarge, "Обложка слишком большая", "too_large")
			default:
				log.Printf("save cover (edit): %v", cerr)
				httpx.WriteInternal(w)
			}
			return
		}
		newCover = cs
		in.CoverPath = &cs.RelPath
		in.CoverMIME = &cs.MIME
		in.CoverBytes = &cs.Bytes
	}

	oldCover, err := h.repo.Update(r.Context(), id, uid, in)
	if err != nil {
		// Если апдейт не прошёл — удаляем уже сохранённую новую обложку.
		if newCover != nil {
			_ = h.storage.Remove(newCover.RelPath)
		}
		switch {
		case errors.Is(err, ErrTrackNotFound), errors.Is(err, ErrNotOwner):
			httpx.WriteError(w, http.StatusNotFound, "Track not found", "not_found")
		default:
			log.Printf("update track: %v", err)
			httpx.WriteInternal(w)
		}
		return
	}

	// Удаляем старую обложку с диска (если её заменили).
	if oldCover != nil && *oldCover != "" && newCover != nil {
		if err := h.storage.Remove(*oldCover); err != nil {
			log.Printf("remove old cover %q: %v", *oldCover, err)
		}
	}

	t, err := h.repo.GetByID(r.Context(), id, uid)
	if err != nil {
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"track": t})
}

// Delete — DELETE /api/tracks/{id}  (cookie required, только владелец)
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	uid := auth.UserIDFromContext(r.Context())
	if uid == 0 {
		httpx.WriteUnauthorized(w)
		return
	}
	id, ok := parseID(r)
	if !ok {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	audioP, coverP, err := h.repo.DeleteByOwner(r.Context(), id, uid)
	if err != nil {
		switch {
		case errors.Is(err, ErrTrackNotFound), errors.Is(err, ErrNotOwner):
			httpx.WriteError(w, http.StatusNotFound, "Track not found", "not_found")
		default:
			log.Printf("delete track: %v", err)
			httpx.WriteInternal(w)
		}
		return
	}
	// Файлы удаляем оппортунистически — БД уже отпустила, ошибка тут не критична.
	if audioP != "" {
		if err := h.storage.Remove(audioP); err != nil {
			log.Printf("remove audio %q: %v", audioP, err)
		}
	}
	if coverP != nil && *coverP != "" {
		if err := h.storage.Remove(*coverP); err != nil {
			log.Printf("remove cover %q: %v", *coverP, err)
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ServeAudio — GET /api/tracks/{id}/audio
//
// Используем http.ServeContent: автоматически:
//   • Range Requests (HTTP 206 Partial Content) → перемотка в плеере
//   • Conditional GET (Last-Modified / If-Modified-Since)
//   • правильный Content-Type / Content-Length
//
// + параллельно в горутине инкрементим plays_count (не блокируя стриминг).
func (h *Handler) ServeAudio(w http.ResponseWriter, r *http.Request) {
	h.serveMedia(w, r, true)
}

// ServeCover — GET /api/tracks/{id}/cover
func (h *Handler) ServeCover(w http.ResponseWriter, r *http.Request) {
	h.serveMedia(w, r, false)
}

func (h *Handler) serveMedia(w http.ResponseWriter, r *http.Request, isAudio bool) {
	id, ok := parseID(r)
	if !ok {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}

	var (
		info *MediaInfo
		err  error
	)
	if isAudio {
		info, err = h.repo.Audio(r.Context(), id)
	} else {
		info, err = h.repo.Cover(r.Context(), id)
	}
	if err != nil {
		switch {
		case errors.Is(err, ErrTrackNotFound):
			httpx.WriteError(w, http.StatusNotFound, "Track not found", "not_found")
		case errors.Is(err, ErrCoverNotSet):
			httpx.WriteError(w, http.StatusNotFound, "Cover not set", "not_found")
		default:
			log.Printf("media lookup: %v", err)
			httpx.WriteInternal(w)
		}
		return
	}

	abs, err := h.storage.SafePath(info.Path)
	if err != nil {
		log.Printf("path traversal attempt: %v (path=%q)", err, info.Path)
		httpx.WriteError(w, http.StatusNotFound, "Not found", "not_found")
		return
	}

	f, err := os.Open(abs)
	if err != nil {
		log.Printf("open media: %v", err)
		httpx.WriteError(w, http.StatusNotFound, "Not found", "not_found")
		return
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		httpx.WriteInternal(w)
		return
	}

	w.Header().Set("Content-Type", info.MIME)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Accept-Ranges", "bytes")

	// http.ServeContent сам разрулит Range, ETag, If-Modified-Since.
	http.ServeContent(w, r, "", st.ModTime(), f)

	// Инкремент plays — в фоне, чтобы не задерживать стрим.
	if isAudio && r.Method == http.MethodGet && !isRangeContinuation(r) {
		go func(trackID int64) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := h.repo.IncrementPlays(ctx, trackID); err != nil {
				log.Printf("plays++ failed: %v", err)
			}
		}(id)
	}
}

// isRangeContinuation — true если запрос с Range и не первый сегмент.
// Чтобы не считать каждый chunk прослушивания как новый "play".
func isRangeContinuation(r *http.Request) bool {
	rng := r.Header.Get("Range")
	if rng == "" {
		return false
	}
	// "bytes=0-" — самое начало, считаем как новое прослушивание;
	// "bytes=N-" / "bytes=N-M" с N>0 — продолжение.
	if strings.HasPrefix(rng, "bytes=0-") {
		return false
	}
	return true
}

func parseID(r *http.Request) (int64, bool) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
