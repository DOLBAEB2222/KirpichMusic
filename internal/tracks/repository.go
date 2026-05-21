package tracks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Track — DTO для отдачи клиенту.
type Track struct {
	ID               int64     `json:"id"`
	Title            string    `json:"title"`
	ArtistName       string    `json:"artist_name"`
	ArtistID         *int64    `json:"artist_id,omitempty"`
	ArtistSlug       string    `json:"artist_slug,omitempty"`
	ArtistVerified   bool      `json:"artist_verified,omitempty"`
	Genre            *string   `json:"genre,omitempty"`
	DurationSec      int       `json:"duration_sec"`
	AudioMIME        string    `json:"audio_mime"`
	HasCover         bool      `json:"has_cover"`
	CoverColor       *string   `json:"cover_color,omitempty"`
	CoverEmoji       *string   `json:"cover_emoji,omitempty"`
	PlaysCount       int64     `json:"plays_count"`
	LikesCount       int64     `json:"likes_count"`
	CommentsCount    int64     `json:"comments_count"`
	WaveformPeaks    []float32 `json:"waveform_peaks,omitempty"`
	ModerationStatus string    `json:"moderation_status,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	Uploader         *Uploader `json:"uploader,omitempty"`
	LikedByMe        bool      `json:"liked_by_me"`
}

type Uploader struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

// CreateInput — параметры INSERT'а.
type CreateInput struct {
	UploaderID  int64
	Title       string
	ArtistName  string
	ArtistID    *int64 // ссылка на artists.id (опц., но сильно желательно)
	Genre       *string
	DurationSec int

	AudioPath  string
	AudioMIME  string
	AudioBytes int64

	CoverPath  *string
	CoverMIME  *string
	CoverBytes *int64

	CoverColor *string
	CoverEmoji *string

	// approved (по умолчанию) | pending | rejected.
	// Если pending — трек не виден в публичных листингах (фильтр в List/GetByID).
	ModerationStatus string
}

// Create — добавляет трек, возвращает ID и created_at.
// Полностью параметризованный запрос → SQL-инъекций быть не может.
func (r *Repository) Create(ctx context.Context, in CreateInput) (int64, time.Time, error) {
	if in.ModerationStatus == "" {
		in.ModerationStatus = "approved"
	}
	// Если трек pending — он "не опубликован" в смысле публичных листингов.
	// is_published остаётся TRUE, но фильтр идёт через moderation_status.
	const q = `
		INSERT INTO tracks (
			uploader_id, title, artist_name, artist_id, genre, duration_sec,
			audio_path, audio_mime, audio_bytes,
			cover_path, cover_mime, cover_bytes,
			cover_color, cover_emoji,
			moderation_status
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9,
			$10, $11, $12,
			$13, $14,
			$15
		)
		RETURNING id, created_at
	`
	var (
		id        int64
		createdAt time.Time
	)
	err := r.pool.QueryRow(ctx, q,
		in.UploaderID, in.Title, in.ArtistName, in.ArtistID, in.Genre, in.DurationSec,
		in.AudioPath, in.AudioMIME, in.AudioBytes,
		in.CoverPath, in.CoverMIME, in.CoverBytes,
		in.CoverColor, in.CoverEmoji,
		in.ModerationStatus,
	).Scan(&id, &createdAt)
	if err != nil {
		return 0, time.Time{}, err
	}
	return id, createdAt, nil
}

// ListParams — параметры выборки списка треков.
type ListParams struct {
	Sort         string // recent | popular
	Genre        string // фильтр (опц.)
	Query        string // поиск по title/artist (опц.)
	UploaderID   int64  // если >0 — только треки этого юзера (для профиля)
	ArtistID     int64  // если >0 — только треки этого артиста
	Limit        int
	Offset       int
	ViewerID     int64 // если >0 — заполняем liked_by_me
	IncludeOwnPending bool // если ViewerID>0, показывать его собственные pending-треки
}

// List — выборка с правильными индексами:
//   recent  → idx_tracks_created_at_desc
//   popular → idx_tracks_plays_desc
//   q       → idx_tracks_title_trgm / idx_tracks_artist_trgm (pg_trgm GIN)
func (r *Repository) List(ctx context.Context, p ListParams) ([]Track, error) {
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 30
	}
	if p.Offset < 0 {
		p.Offset = 0
	}

	args := []any{}
	where := []string{"t.is_published = TRUE"}

	// Модерация: по умолчанию показываем только approved.
	// Если viewer хочет видеть свои pending (на странице "Мои треки") —
	// ослабляем фильтр через OR.
	if p.IncludeOwnPending && p.ViewerID > 0 {
		args = append(args, p.ViewerID)
		where = append(where, fmt.Sprintf(
			"(t.moderation_status = 'approved' OR t.uploader_id = $%d)", len(args)))
	} else {
		where = append(where, "t.moderation_status = 'approved'")
	}

	if p.Genre != "" {
		args = append(args, p.Genre)
		where = append(where, fmt.Sprintf("t.genre = $%d", len(args)))
	}
	if p.Query != "" {
		args = append(args, "%"+p.Query+"%")
		where = append(where, fmt.Sprintf(
			"(t.title ILIKE $%d OR t.artist_name ILIKE $%d)", len(args), len(args)))
	}
	if p.UploaderID > 0 {
		args = append(args, p.UploaderID)
		where = append(where, fmt.Sprintf("t.uploader_id = $%d", len(args)))
	}
	if p.ArtistID > 0 {
		args = append(args, p.ArtistID)
		where = append(where, fmt.Sprintf("t.artist_id = $%d", len(args)))
	}

	orderBy := "t.created_at DESC"
	if p.Sort == "popular" {
		orderBy = "t.plays_count DESC, t.created_at DESC"
	}

	// liked_by_me через скаляр-плейсхолдер — пусто, если viewer не авторизован.
	likedExpr := "FALSE"
	if p.ViewerID > 0 {
		args = append(args, p.ViewerID)
		likedExpr = fmt.Sprintf("EXISTS (SELECT 1 FROM likes l WHERE l.track_id = t.id AND l.user_id = $%d)", len(args))
	}

	args = append(args, p.Limit, p.Offset)
	limitArg := len(args) - 1
	offsetArg := len(args)

	q := fmt.Sprintf(`
		SELECT
			t.id, t.title, t.artist_name, t.genre, t.duration_sec,
			t.audio_mime, t.cover_path IS NOT NULL,
			t.cover_color, t.cover_emoji,
			t.plays_count, t.likes_count, t.comments_count,
			t.waveform_peaks,
			t.moderation_status,
			t.created_at,
			u.id, u.username,
			t.artist_id, a.slug, COALESCE(a.is_verified, FALSE),
			%s
		FROM tracks t
		LEFT JOIN users u   ON u.id = t.uploader_id
		LEFT JOIN artists a ON a.id = t.artist_id
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, likedExpr, strings.Join(where, " AND "), orderBy, limitArg, offsetArg)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Track
	for rows.Next() {
		var (
			t          Track
			uid        *int64
			un         *string
			peaksRaw   []byte
			artistSlug *string
		)
		if err := rows.Scan(
			&t.ID, &t.Title, &t.ArtistName, &t.Genre, &t.DurationSec,
			&t.AudioMIME, &t.HasCover,
			&t.CoverColor, &t.CoverEmoji,
			&t.PlaysCount, &t.LikesCount, &t.CommentsCount,
			&peaksRaw,
			&t.ModerationStatus,
			&t.CreatedAt,
			&uid, &un,
			&t.ArtistID, &artistSlug, &t.ArtistVerified,
			&t.LikedByMe,
		); err != nil {
			return nil, err
		}
		if uid != nil && un != nil {
			t.Uploader = &Uploader{ID: *uid, Username: *un}
		}
		if artistSlug != nil {
			t.ArtistSlug = *artistSlug
		}
		t.WaveformPeaks = decodePeaks(peaksRaw)
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetByID — детальная информация о треке для страницы плеера.
// viewerID > 0 → заполняем liked_by_me (используется в /api/tracks/{id}).
// Pending-треки видны только своему uploader'у и админам (последнее проверяется
// в хендлере). Тут просто разрешаем pending если viewer == uploader.
func (r *Repository) GetByID(ctx context.Context, id int64, viewerID int64) (*Track, error) {
	const q = `
		SELECT
			t.id, t.title, t.artist_name, t.genre, t.duration_sec,
			t.audio_mime, t.cover_path IS NOT NULL,
			t.cover_color, t.cover_emoji,
			t.plays_count, t.likes_count, t.comments_count,
			t.waveform_peaks,
			t.moderation_status,
			t.created_at,
			u.id, u.username,
			t.artist_id, a.slug, COALESCE(a.is_verified, FALSE),
			CASE WHEN $2 > 0 THEN
			    EXISTS (SELECT 1 FROM likes l WHERE l.track_id = t.id AND l.user_id = $2)
			ELSE FALSE END
		FROM tracks t
		LEFT JOIN users u   ON u.id = t.uploader_id
		LEFT JOIN artists a ON a.id = t.artist_id
		WHERE t.id = $1 AND t.is_published = TRUE
		  AND (t.moderation_status = 'approved' OR t.uploader_id = $2)
	`
	var (
		t          Track
		uid        *int64
		un         *string
		peaksRaw   []byte
		artistSlug *string
	)
	err := r.pool.QueryRow(ctx, q, id, viewerID).Scan(
		&t.ID, &t.Title, &t.ArtistName, &t.Genre, &t.DurationSec,
		&t.AudioMIME, &t.HasCover,
		&t.CoverColor, &t.CoverEmoji,
		&t.PlaysCount, &t.LikesCount, &t.CommentsCount,
		&peaksRaw,
		&t.ModerationStatus,
		&t.CreatedAt,
		&uid, &un,
		&t.ArtistID, &artistSlug, &t.ArtistVerified,
		&t.LikedByMe,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTrackNotFound
		}
		return nil, err
	}
	if uid != nil && un != nil {
		t.Uploader = &Uploader{ID: *uid, Username: *un}
	}
	if artistSlug != nil {
		t.ArtistSlug = *artistSlug
	}
	t.WaveformPeaks = decodePeaks(peaksRaw)
	return &t, nil
}

// MediaInfo — путь и метаданные одного файла трека.
type MediaInfo struct {
	Path  string // относительно media root
	MIME  string
	Bytes int64
}

func (r *Repository) Audio(ctx context.Context, id int64) (*MediaInfo, error) {
	var m MediaInfo
	err := r.pool.QueryRow(ctx, `
		SELECT audio_path, audio_mime, audio_bytes FROM tracks
		WHERE id = $1 AND is_published = TRUE
	`, id).Scan(&m.Path, &m.MIME, &m.Bytes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTrackNotFound
		}
		return nil, err
	}
	return &m, nil
}

func (r *Repository) Cover(ctx context.Context, id int64) (*MediaInfo, error) {
	var (
		path  *string
		mime  *string
		bytes *int64
	)
	err := r.pool.QueryRow(ctx, `
		SELECT cover_path, cover_mime, cover_bytes FROM tracks
		WHERE id = $1 AND is_published = TRUE
	`, id).Scan(&path, &mime, &bytes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTrackNotFound
		}
		return nil, err
	}
	if path == nil {
		return nil, ErrCoverNotSet
	}
	return &MediaInfo{Path: *path, MIME: *mime, Bytes: *bytes}, nil
}

// IncrementPlays — асинхронный инкремент в горутине.
// CHECK: trigger / DB не блокируется надолго (1 update by PK).
func (r *Repository) IncrementPlays(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE tracks SET plays_count = plays_count + 1 WHERE id = $1`, id)
	return err
}

// AddLike — INSERT ... ON CONFLICT DO NOTHING (идемпотентно).
// likes_count в tracks обновляется триггером trg_likes_count.
// Возвращает true если лайк действительно добавлен (был новым).
func (r *Repository) AddLike(ctx context.Context, userID, trackID int64) (added bool, err error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO likes (user_id, track_id) VALUES ($1, $2)
		ON CONFLICT (user_id, track_id) DO NOTHING
	`, userID, trackID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// RemoveLike — идемпотентно. Возвращает true если был удалён.
func (r *Repository) RemoveLike(ctx context.Context, userID, trackID int64) (removed bool, err error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM likes WHERE user_id = $1 AND track_id = $2
	`, userID, trackID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// TrackExists — для валидации перед лайком (избегаем 500 при FK-violation).
func (r *Repository) TrackExists(ctx context.Context, id int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM tracks WHERE id = $1 AND is_published = TRUE)`, id,
	).Scan(&exists)
	return exists, err
}

// SetWaveform — записывает peaks (массив float32 0..1, ~120 значений)
// как JSONB. Вызывается из горутины после Upload, никого не блокирует.
func (r *Repository) SetWaveform(ctx context.Context, trackID int64, peaks []float32) error {
	jsonBytes, err := encodePeaks(peaks)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `UPDATE tracks SET waveform_peaks = $2 WHERE id = $1`,
		trackID, jsonBytes)
	return err
}

// UpdateInput — частичный апдейт трека. Все поля опциональны:
// апдейт делается только для тех, что не nil.
type UpdateInput struct {
	Title      *string
	ArtistName *string
	Genre      *string

	// Если CoverPath != nil — обновляем (заменяем) обложку.
	// При этом OldCoverPath будет заполнен значением, которое было до апдейта,
	// чтобы хендлер мог удалить старый файл с диска.
	CoverPath  *string
	CoverMIME  *string
	CoverBytes *int64

	CoverColor *string
	CoverEmoji *string
}

// Update — обновляет редактируемые поля трека владельцем.
// Проверяется uploader_id == ownerID; если не совпало — ErrTrackNotFound.
// Возвращает старый cover_path (если был и он заменяется/обнуляется), чтобы
// вызывающий мог удалить файл с диска.
func (r *Repository) Update(ctx context.Context, trackID, ownerID int64, in UpdateInput) (oldCover *string, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var (
		dbOwner    *int64
		dbCoverOld *string
	)
	err = tx.QueryRow(ctx,
		`SELECT uploader_id, cover_path FROM tracks WHERE id = $1 AND is_published = TRUE FOR UPDATE`,
		trackID,
	).Scan(&dbOwner, &dbCoverOld)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTrackNotFound
		}
		return nil, err
	}
	if dbOwner == nil || *dbOwner != ownerID {
		return nil, ErrNotOwner
	}

	sets := []string{}
	args := []any{}
	idx := 1
	add := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, idx))
		args = append(args, val)
		idx++
	}

	if in.Title != nil {
		add("title", *in.Title)
	}
	if in.ArtistName != nil {
		add("artist_name", *in.ArtistName)
	}
	if in.Genre != nil {
		if *in.Genre == "" {
			sets = append(sets, "genre = NULL")
		} else {
			add("genre", *in.Genre)
		}
	}
	if in.CoverPath != nil {
		add("cover_path", *in.CoverPath)
		add("cover_mime", *in.CoverMIME)
		add("cover_bytes", *in.CoverBytes)
	}
	if in.CoverColor != nil {
		add("cover_color", *in.CoverColor)
	}
	if in.CoverEmoji != nil {
		add("cover_emoji", *in.CoverEmoji)
	}
	if len(sets) == 0 {
		return nil, nil
	}

	args = append(args, trackID)
	q := "UPDATE tracks SET " + strings.Join(sets, ", ") + fmt.Sprintf(" WHERE id = $%d", idx)
	if _, err := tx.Exec(ctx, q, args...); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Возвращаем старую обложку только если её действительно меняли.
	if in.CoverPath != nil {
		return dbCoverOld, nil
	}
	return nil, nil
}

// DeleteByOwner — удаляет трек владельцем. Возвращает audio_path / cover_path,
// чтобы вызывающий мог удалить файлы с диска.
func (r *Repository) DeleteByOwner(ctx context.Context, trackID, ownerID int64) (audio string, cover *string, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", nil, err
	}
	defer tx.Rollback(ctx)

	var (
		dbOwner *int64
		audioP  string
		coverP  *string
	)
	err = tx.QueryRow(ctx,
		`SELECT uploader_id, audio_path, cover_path FROM tracks WHERE id = $1 FOR UPDATE`,
		trackID,
	).Scan(&dbOwner, &audioP, &coverP)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, ErrTrackNotFound
		}
		return "", nil, err
	}
	if dbOwner == nil || *dbOwner != ownerID {
		return "", nil, ErrNotOwner
	}
	if _, err := tx.Exec(ctx, `DELETE FROM tracks WHERE id = $1`, trackID); err != nil {
		return "", nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", nil, err
	}
	return audioP, coverP, nil
}

var (
	ErrTrackNotFound = errors.New("track not found")
	ErrCoverNotSet   = errors.New("cover not set")
	ErrNotOwner      = errors.New("not the owner")
)
