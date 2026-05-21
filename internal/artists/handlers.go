package artists

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bogdan/kirpichmusic/internal/auth"
	"github.com/bogdan/kirpichmusic/internal/httpx"
	"github.com/bogdan/kirpichmusic/internal/storage"
)

type Handler struct {
	repo    *Repository
	pool    *pgxpool.Pool
	storage *storage.Storage
}

func NewHandler(repo *Repository, pool *pgxpool.Pool, st *storage.Storage) *Handler {
	return &Handler{repo: repo, pool: pool, storage: st}
}

// ─── public lookup ─────────────────────────────────────────────────────

// GetByID — GET /api/artists/{id}
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	a, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "Артист не найден", "not_found")
			return
		}
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.withMyFlag(r, a))
}

// GetBySlug — GET /api/artists/by-slug/{slug}
func (h *Handler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(chi.URLParam(r, "slug"))
	if slug == "" {
		httpx.WriteBadRequest(w, "slug required")
		return
	}
	a, err := h.repo.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "Артист не найден", "not_found")
			return
		}
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.withMyFlag(r, a))
}

// withMyFlag добавляет к артисту флаги can_edit/is_my_profile.
func (h *Handler) withMyFlag(r *http.Request, a *Artist) map[string]any {
	viewer := auth.UserIDFromContext(r.Context())
	canEdit := false
	if viewer > 0 && a.ClaimedByUserID != nil && *a.ClaimedByUserID == viewer {
		canEdit = true
	}
	if viewer > 0 && auth.IsAdminFromContext(r.Context()) {
		canEdit = true
	}
	return map[string]any{
		"artist":   a,
		"can_edit": canEdit,
	}
}

// List — GET /api/artists?q=&limit=&offset=
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	out, err := h.repo.List(r.Context(), ListParams{
		Query:        strings.TrimSpace(q.Get("q")),
		OnlyClaimed:  q.Get("claimed") == "true",
		OnlyVerified: q.Get("verified") == "true",
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		log.Printf("artists list: %v", err)
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"artists": out})
}

// Suggest — GET /api/artists/suggest?q=&limit=
func (h *Handler) Suggest(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.repo.Suggest(r.Context(), q, limit)
	if err != nil {
		log.Printf("artists suggest: %v", err)
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"suggestions": out, "query": q})
}

// ─── avatar / banner serving ───────────────────────────────────────────

// ServeAvatar — GET /api/artists/{id}/avatar
func (h *Handler) ServeAvatar(w http.ResponseWriter, r *http.Request) {
	h.serveImage(w, r, "avatar")
}

// ServeBanner — GET /api/artists/{id}/banner
func (h *Handler) ServeBanner(w http.ResponseWriter, r *http.Request) {
	h.serveImage(w, r, "banner")
}

func (h *Handler) serveImage(w http.ResponseWriter, r *http.Request, kind string) {
	if h.storage == nil {
		httpx.WriteError(w, http.StatusNotFound, "not configured", "not_found")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	col := "avatar_path"
	mimeCol := "avatar_mime"
	if kind == "banner" {
		col = "banner_path"
		mimeCol = "banner_mime"
	}
	var path, mime *string
	q := "SELECT " + col + ", " + mimeCol + " FROM artists WHERE id = $1"
	if err := h.pool.QueryRow(r.Context(), q, id).Scan(&path, &mime); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "Not found", "not_found")
		return
	}
	if path == nil || *path == "" {
		httpx.WriteError(w, http.StatusNotFound, "Not set", "not_found")
		return
	}
	abs, err := h.storage.SafePath(*path)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "Not found", "not_found")
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "Not found", "not_found")
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		httpx.WriteInternal(w)
		return
	}
	if mime != nil && *mime != "" {
		w.Header().Set("Content-Type", *mime)
	} else {
		w.Header().Set("Content-Type", "image/jpeg")
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	http.ServeContent(w, r, "", st.ModTime(), f)
}

// ─── upload (claimed user OR admin) ────────────────────────────────────

// UploadAvatar — POST /api/artists/{id}/avatar (multipart "avatar")
//
// Доступ: claimed_by_user_id == viewer ИЛИ viewer is admin.
// Иначе → 403, юзер должен использовать proposal-flow.
func (h *Handler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	h.uploadImage(w, r, "avatar")
}

// UploadBanner — POST /api/artists/{id}/banner (multipart "banner")
func (h *Handler) UploadBanner(w http.ResponseWriter, r *http.Request) {
	h.uploadImage(w, r, "banner")
}

func (h *Handler) uploadImage(w http.ResponseWriter, r *http.Request, kind string) {
	uid := auth.UserIDFromContext(r.Context())
	if uid == 0 {
		httpx.WriteUnauthorized(w)
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	if !h.canEdit(r.Context(), id, uid) {
		httpx.WriteError(w, http.StatusForbidden, "Только владелец артиста или админ", "forbidden")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		httpx.WriteBadRequest(w, "Файл слишком большой или некорректен")
		return
	}
	file, header, err := r.FormFile(kind)
	if err != nil {
		httpx.WriteBadRequest(w, "Файл не получен")
		return
	}
	defer file.Close()

	var saved *storage.Stored
	if kind == "avatar" {
		saved, err = h.storage.SaveArtistAvatar(file, header)
	} else {
		saved, err = h.storage.SaveArtistBanner(file, header)
	}
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrUnsupportedMIME):
			httpx.WriteBadRequest(w, "Поддерживаются JPG, PNG, WEBP")
		case errors.Is(err, storage.ErrTooLarge):
			httpx.WriteBadRequest(w, "Файл слишком большой")
		default:
			log.Printf("save artist %s: %v", kind, err)
			httpx.WriteInternal(w)
		}
		return
	}

	colP, colM, colB := "avatar_path", "avatar_mime", "avatar_bytes"
	if kind == "banner" {
		colP, colM, colB = "banner_path", "banner_mime", "banner_bytes"
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		_ = h.storage.Remove(saved.RelPath)
		httpx.WriteInternal(w)
		return
	}
	defer tx.Rollback(r.Context())

	var oldPath *string
	if err := tx.QueryRow(r.Context(),
		"SELECT "+colP+" FROM artists WHERE id = $1 FOR UPDATE", id,
	).Scan(&oldPath); err != nil {
		_ = h.storage.Remove(saved.RelPath)
		httpx.WriteInternal(w)
		return
	}
	if _, err := tx.Exec(r.Context(),
		"UPDATE artists SET "+colP+" = $1, "+colM+" = $2, "+colB+" = $3 WHERE id = $4",
		saved.RelPath, saved.MIME, saved.Bytes, id,
	); err != nil {
		_ = h.storage.Remove(saved.RelPath)
		httpx.WriteInternal(w)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		_ = h.storage.Remove(saved.RelPath)
		httpx.WriteInternal(w)
		return
	}
	if oldPath != nil && *oldPath != "" && *oldPath != saved.RelPath {
		_ = h.storage.Remove(*oldPath)
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"kind": kind,
		"url":  "/api/artists/" + chi.URLParam(r, "id") + "/" + kind,
	})
}

// DeleteAvatar — DELETE /api/artists/{id}/avatar
func (h *Handler) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	h.deleteImage(w, r, "avatar")
}

// DeleteBanner — DELETE /api/artists/{id}/banner
func (h *Handler) DeleteBanner(w http.ResponseWriter, r *http.Request) {
	h.deleteImage(w, r, "banner")
}

func (h *Handler) deleteImage(w http.ResponseWriter, r *http.Request, kind string) {
	uid := auth.UserIDFromContext(r.Context())
	if uid == 0 {
		httpx.WriteUnauthorized(w)
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	if !h.canEdit(r.Context(), id, uid) {
		httpx.WriteError(w, http.StatusForbidden, "Forbidden", "forbidden")
		return
	}
	colP, colM, colB := "avatar_path", "avatar_mime", "avatar_bytes"
	if kind == "banner" {
		colP, colM, colB = "banner_path", "banner_mime", "banner_bytes"
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.WriteInternal(w)
		return
	}
	defer tx.Rollback(r.Context())
	var oldPath *string
	if err := tx.QueryRow(r.Context(),
		"SELECT "+colP+" FROM artists WHERE id = $1 FOR UPDATE", id,
	).Scan(&oldPath); err != nil {
		httpx.WriteInternal(w)
		return
	}
	if _, err := tx.Exec(r.Context(),
		"UPDATE artists SET "+colP+" = NULL, "+colM+" = NULL, "+colB+" = NULL WHERE id = $1",
		id,
	); err != nil {
		httpx.WriteInternal(w)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteInternal(w)
		return
	}
	if oldPath != nil && *oldPath != "" {
		_ = h.storage.Remove(*oldPath)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ─── update description / claim ─────────────────────────────────────────

type updateInput struct {
	Description *string `json:"description,omitempty"`
}

// Patch — PATCH /api/artists/{id}  (claimed user OR admin)
func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	uid := auth.UserIDFromContext(r.Context())
	if uid == 0 {
		httpx.WriteUnauthorized(w)
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	if !h.canEdit(r.Context(), id, uid) {
		httpx.WriteError(w, http.StatusForbidden, "Forbidden", "forbidden")
		return
	}
	var in updateInput
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		httpx.WriteBadRequest(w, "invalid body")
		return
	}
	if in.Description != nil {
		desc := strings.TrimSpace(*in.Description)
		if len(desc) > 5000 {
			httpx.WriteBadRequest(w, "description слишком длинное")
			return
		}
		if _, err := h.pool.Exec(r.Context(),
			`UPDATE artists SET description = $1 WHERE id = $2`,
			desc, id,
		); err != nil {
			log.Printf("artist update: %v", err)
			httpx.WriteInternal(w)
			return
		}
	}
	a, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.withMyFlag(r, a))
}

// ─── admin: claim / unclaim ─────────────────────────────────────────────

type claimInput struct {
	UserID *int64 `json:"user_id"` // null = unclaim
}

// AdminClaim — POST /api/admin/artists/{id}/claim  body: {"user_id": 42}
// Только админ. Линкует профиль артиста к user'у. is_verified = TRUE.
// Если user_id == null → отвязать.
func (h *Handler) AdminClaim(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	var in claimInput
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		httpx.WriteBadRequest(w, "invalid body")
		return
	}
	if in.UserID != nil && *in.UserID > 0 {
		// Сделать claimed.
		var exists bool
		if err := h.pool.QueryRow(r.Context(),
			`SELECT TRUE FROM users WHERE id = $1 AND is_active = TRUE`, *in.UserID,
		).Scan(&exists); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				httpx.WriteBadRequest(w, "Пользователь не найден")
				return
			}
			httpx.WriteInternal(w)
			return
		}
		if _, err := h.pool.Exec(r.Context(),
			`UPDATE artists SET claimed_by_user_id = $1, is_verified = TRUE WHERE id = $2`,
			*in.UserID, id,
		); err != nil {
			log.Printf("artist claim: %v", err)
			httpx.WriteInternal(w)
			return
		}
	} else {
		// Unclaim.
		if _, err := h.pool.Exec(r.Context(),
			`UPDATE artists SET claimed_by_user_id = NULL, is_verified = FALSE WHERE id = $1`, id,
		); err != nil {
			httpx.WriteInternal(w)
			return
		}
	}
	a, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.withMyFlag(r, a))
}

// canEdit — true если viewer == claimed_by_user_id ИЛИ admin.
func (h *Handler) canEdit(ctx context.Context, artistID, viewerID int64) bool {
	if auth.IsAdminFromContext(ctx) {
		return true
	}
	cid, err := h.repo.ClaimedBy(ctx, artistID)
	if err != nil || cid == nil {
		return false
	}
	return *cid == viewerID
}

// ─── proposal flow (any user → propose avatar; admin → review) ─────────

// Proposal — DTO в выдаче админу.
type Proposal struct {
	ID             int64     `json:"id"`
	ArtistID       int64     `json:"artist_id"`
	ArtistName     string    `json:"artist_name"`
	ArtistSlug     string    `json:"artist_slug"`
	ProposedBy     *int64    `json:"proposed_by_user_id,omitempty"`
	ProposedByName string    `json:"proposed_by_name,omitempty"`
	URL            string    `json:"url"` // /api/artists/proposals/{id}/image
	Status         string    `json:"status"`
	Note           string    `json:"note,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// SubmitProposal — POST /api/artists/{id}/proposals (multipart "image", auth required)
func (h *Handler) SubmitProposal(w http.ResponseWriter, r *http.Request) {
	uid := auth.UserIDFromContext(r.Context())
	if uid == 0 {
		httpx.WriteUnauthorized(w)
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	// Проверяем что артист существует.
	if _, err := h.repo.GetByID(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "Артист не найден", "not_found")
			return
		}
		httpx.WriteInternal(w)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		httpx.WriteBadRequest(w, "Файл слишком большой")
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		httpx.WriteBadRequest(w, "Файл не получен")
		return
	}
	defer file.Close()
	saved, err := h.storage.SaveArtistProposal(file, header)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrUnsupportedMIME):
			httpx.WriteBadRequest(w, "JPG/PNG/WEBP")
		case errors.Is(err, storage.ErrTooLarge):
			httpx.WriteBadRequest(w, "Слишком большой файл")
		default:
			log.Printf("save proposal: %v", err)
			httpx.WriteInternal(w)
		}
		return
	}
	var pid int64
	if err := h.pool.QueryRow(r.Context(), `
		INSERT INTO artist_avatar_proposals (artist_id, proposed_by, image_path, image_mime, image_bytes)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, id, uid, saved.RelPath, saved.MIME, saved.Bytes).Scan(&pid); err != nil {
		_ = h.storage.Remove(saved.RelPath)
		log.Printf("insert proposal: %v", err)
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"ok":          true,
		"proposal_id": pid,
		"url":         "/api/artists/proposals/" + strconv.FormatInt(pid, 10) + "/image",
	})
}

// ServeProposalImage — GET /api/artists/proposals/{pid}/image  (admin only — для UI очереди)
func (h *Handler) ServeProposalImage(w http.ResponseWriter, r *http.Request) {
	pid, err := strconv.ParseInt(chi.URLParam(r, "pid"), 10, 64)
	if err != nil || pid <= 0 {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	var path, mime string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT image_path, image_mime FROM artist_avatar_proposals WHERE id = $1`, pid,
	).Scan(&path, &mime); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "Not found", "not_found")
		return
	}
	abs, err := h.storage.SafePath(path)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "Not found", "not_found")
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "Not found", "not_found")
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=60")
	http.ServeContent(w, r, "", st.ModTime(), f)
}

// AdminListProposals — GET /api/admin/artists/proposals?status=pending
func (h *Handler) AdminListProposals(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status == "" {
		status = "pending"
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT p.id, p.artist_id, a.name, a.slug,
		       p.proposed_by, COALESCE(u.username, ''),
		       p.status, COALESCE(p.note, ''), p.created_at
		  FROM artist_avatar_proposals p
		  JOIN artists a ON a.id = p.artist_id
		  LEFT JOIN users u ON u.id = p.proposed_by
		 WHERE p.status = $1
		 ORDER BY p.created_at DESC
		 LIMIT 200
	`, status)
	if err != nil {
		log.Printf("list proposals: %v", err)
		httpx.WriteInternal(w)
		return
	}
	defer rows.Close()
	out := []Proposal{}
	for rows.Next() {
		var p Proposal
		if err := rows.Scan(&p.ID, &p.ArtistID, &p.ArtistName, &p.ArtistSlug,
			&p.ProposedBy, &p.ProposedByName, &p.Status, &p.Note, &p.CreatedAt,
		); err != nil {
			log.Printf("scan: %v", err)
			httpx.WriteInternal(w)
			return
		}
		p.URL = "/api/artists/proposals/" + strconv.FormatInt(p.ID, 10) + "/image"
		out = append(out, p)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

type reviewInput struct {
	Note string `json:"note,omitempty"`
}

// AdminApproveProposal — POST /api/admin/artists/proposals/{pid}/approve
//
// Принимаем предложенную аватарку: копируем её путь в artists.avatar_path,
// статус → approved, остальные pending по этому артисту автоотклоняются (мусор не копится).
// Транзакционно. Старая artists.avatar_path (если была) удаляется с диска.
func (h *Handler) AdminApproveProposal(w http.ResponseWriter, r *http.Request) {
	pid, err := strconv.ParseInt(chi.URLParam(r, "pid"), 10, 64)
	if err != nil || pid <= 0 {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	var in reviewInput
	if r.ContentLength > 0 {
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		dec.DisallowUnknownFields()
		_ = dec.Decode(&in) // body опционален
	}
	uid := auth.UserIDFromContext(r.Context())

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.WriteInternal(w)
		return
	}
	defer tx.Rollback(r.Context())

	var (
		artistID                       int64
		propPath, propMime             string
		propBytes                      int64
	)
	if err := tx.QueryRow(r.Context(), `
		SELECT artist_id, image_path, image_mime, image_bytes
		  FROM artist_avatar_proposals
		 WHERE id = $1 AND status = 'pending'
		 FOR UPDATE
	`, pid).Scan(&artistID, &propPath, &propMime, &propBytes); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "Заявка не найдена или уже обработана", "not_found")
			return
		}
		httpx.WriteInternal(w)
		return
	}
	var oldAvatar *string
	if err := tx.QueryRow(r.Context(),
		`SELECT avatar_path FROM artists WHERE id = $1 FOR UPDATE`, artistID,
	).Scan(&oldAvatar); err != nil {
		httpx.WriteInternal(w)
		return
	}
	// Копируем путь в artists и помечаем proposal approved.
	if _, err := tx.Exec(r.Context(),
		`UPDATE artists SET avatar_path = $1, avatar_mime = $2, avatar_bytes = $3 WHERE id = $4`,
		propPath, propMime, propBytes, artistID,
	); err != nil {
		httpx.WriteInternal(w)
		return
	}
	if _, err := tx.Exec(r.Context(),
		`UPDATE artist_avatar_proposals
		    SET status = 'approved', note = $1, reviewed_by = $2, reviewed_at = now()
		  WHERE id = $3`, in.Note, uid, pid,
	); err != nil {
		httpx.WriteInternal(w)
		return
	}
	// Все остальные pending по этому артисту → rejected.
	if _, err := tx.Exec(r.Context(),
		`UPDATE artist_avatar_proposals
		    SET status = 'rejected', reviewed_by = $1, reviewed_at = now(),
		        note = COALESCE(NULLIF(note, ''), 'Auto-rejected: another approved')
		  WHERE artist_id = $2 AND status = 'pending'`, uid, artistID,
	); err != nil {
		httpx.WriteInternal(w)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteInternal(w)
		return
	}
	if oldAvatar != nil && *oldAvatar != "" && *oldAvatar != propPath {
		_ = h.storage.Remove(*oldAvatar)
	}
	// Auto-rejected proposal images удаляем с диска отдельной транзакцией —
	// файлы proposal'ов не нужны после отклонения.
	rows, _ := h.pool.Query(r.Context(),
		`SELECT image_path FROM artist_avatar_proposals
		  WHERE artist_id = $1 AND status = 'rejected' AND id != $2`, artistID, pid)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var p string
			if rows.Scan(&p) == nil && p != "" && p != propPath {
				_ = h.storage.Remove(p)
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// AdminRejectProposal — POST /api/admin/artists/proposals/{pid}/reject
func (h *Handler) AdminRejectProposal(w http.ResponseWriter, r *http.Request) {
	pid, err := strconv.ParseInt(chi.URLParam(r, "pid"), 10, 64)
	if err != nil || pid <= 0 {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	var in reviewInput
	if r.ContentLength > 0 {
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		dec.DisallowUnknownFields()
		_ = dec.Decode(&in)
	}
	uid := auth.UserIDFromContext(r.Context())

	var path string
	err = h.pool.QueryRow(r.Context(), `
		UPDATE artist_avatar_proposals
		   SET status = 'rejected', note = $1, reviewed_by = $2, reviewed_at = now()
		 WHERE id = $3 AND status = 'pending'
		 RETURNING image_path
	`, in.Note, uid, pid).Scan(&path)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "Заявка не найдена или уже обработана", "not_found")
			return
		}
		httpx.WriteInternal(w)
		return
	}
	if path != "" {
		_ = h.storage.Remove(path)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}
