// Package users — публичные профили, аватары/шапки и списки фолловеров.
package users

import (
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
	pool    *pgxpool.Pool
	storage *storage.Storage
}

// NewHandler — публичные профили.
// storage может быть nil, если не нужно отдавать/принимать аватары/шапки.
func NewHandler(pool *pgxpool.Pool, st *storage.Storage) *Handler {
	return &Handler{pool: pool, storage: st}
}

// PublicProfile — DTO для публичной страницы юзера.
// НЕ содержит email/прав-чувствительных полей.
type PublicProfile struct {
	ID             int64  `json:"id"`
	Username       string `json:"username"`
	Bio            string `json:"bio"`
	IsAdmin        bool   `json:"is_admin"`
	HasAvatar      bool   `json:"has_avatar"`
	HasBanner      bool   `json:"has_banner"`
	TracksCount    int64  `json:"tracks_count"`
	FollowersCount int64  `json:"followers_count"`
	FollowingCount int64  `json:"following_count"`
	LikesCount     int64  `json:"likes_count"`
	CreatedAt      string `json:"created_at"`
	IsFollowedByMe bool   `json:"is_followed_by_me"`
	IsMe           bool   `json:"is_me"`
}

// GetByUsername — GET /api/users/by-username/{username}
func (h *Handler) GetByUsername(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(chi.URLParam(r, "username"))
	if username == "" {
		httpx.WriteBadRequest(w, "username required")
		return
	}
	h.lookup(w, r, "u.username = $1", username)
}

// GetByID — GET /api/users/{id}
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		httpx.WriteBadRequest(w, "id required")
		return
	}
	h.lookup(w, r, "u.id::TEXT = $1", idStr)
}

func (h *Handler) lookup(w http.ResponseWriter, r *http.Request, where string, arg any) {
	viewer := auth.UserIDFromContext(r.Context())

	var (
		p         PublicProfile
		bio       *string
		createdAt time.Time
	)
	q := `
		SELECT
			u.id, u.username, u.bio, u.is_admin,
			(u.avatar_path IS NOT NULL),
			(u.banner_path IS NOT NULL),
			u.created_at,
			(SELECT COUNT(*) FROM tracks  t WHERE t.uploader_id = u.id AND t.is_published = TRUE)::BIGINT,
			(SELECT COUNT(*) FROM follows f WHERE f.followee_id = u.id)::BIGINT,
			(SELECT COUNT(*) FROM follows f WHERE f.follower_id = u.id)::BIGINT,
			(SELECT COUNT(*) FROM likes   l WHERE l.user_id     = u.id)::BIGINT,
			CASE WHEN $2::BIGINT > 0 THEN
				EXISTS (SELECT 1 FROM follows f
				        WHERE f.follower_id = $2 AND f.followee_id = u.id)
			ELSE FALSE END
		FROM users u
		WHERE ` + where + ` AND u.is_active = TRUE
		LIMIT 1
	`
	err := h.pool.QueryRow(r.Context(), q, arg, viewer).Scan(
		&p.ID, &p.Username, &bio, &p.IsAdmin,
		&p.HasAvatar, &p.HasBanner,
		&createdAt,
		&p.TracksCount, &p.FollowersCount, &p.FollowingCount, &p.LikesCount,
		&p.IsFollowedByMe,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "Пользователь не найден", "not_found")
			return
		}
		log.Printf("user lookup: %v", err)
		httpx.WriteInternal(w)
		return
	}
	if bio != nil {
		p.Bio = *bio
	}
	p.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	p.IsMe = viewer != 0 && viewer == p.ID

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": p})
}

// ─── AVATAR / BANNER : public read ──────────────────────────────────────

// ServeAvatar — GET /api/users/{id}/avatar (публично)
func (h *Handler) ServeAvatar(w http.ResponseWriter, r *http.Request) {
	h.serveImage(w, r, "avatar")
}

// ServeBanner — GET /api/users/{id}/banner (публично)
func (h *Handler) ServeBanner(w http.ResponseWriter, r *http.Request) {
	h.serveImage(w, r, "banner")
}

func (h *Handler) serveImage(w http.ResponseWriter, r *http.Request, kind string) {
	if h.storage == nil {
		httpx.WriteError(w, http.StatusNotFound, "not configured", "not_found")
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
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
	q := "SELECT " + col + ", " + mimeCol + " FROM users WHERE id = $1 AND is_active = TRUE"
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

// ─── AVATAR / BANNER : authenticated upload ─────────────────────────────

// UploadAvatar — POST /api/auth/me/avatar (multipart "avatar")
func (h *Handler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	h.uploadImage(w, r, "avatar")
}

// UploadBanner — POST /api/auth/me/banner (multipart "banner")
func (h *Handler) UploadBanner(w http.ResponseWriter, r *http.Request) {
	h.uploadImage(w, r, "banner")
}

// DeleteAvatar — DELETE /api/auth/me/avatar
func (h *Handler) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	h.deleteImage(w, r, "avatar")
}

// DeleteBanner — DELETE /api/auth/me/banner
func (h *Handler) DeleteBanner(w http.ResponseWriter, r *http.Request) {
	h.deleteImage(w, r, "banner")
}

func (h *Handler) uploadImage(w http.ResponseWriter, r *http.Request, kind string) {
	if h.storage == nil {
		httpx.WriteInternal(w)
		return
	}
	uid := auth.UserIDFromContext(r.Context())
	if uid == 0 {
		httpx.WriteUnauthorized(w)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16<<20) // 16 MiB
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		httpx.WriteBadRequest(w, "Файл слишком большой или некорректен")
		return
	}
	field := kind
	file, header, err := r.FormFile(field)
	if err != nil {
		httpx.WriteBadRequest(w, "Файл не получен ("+field+")")
		return
	}
	defer file.Close()

	var saved *storage.Stored
	if kind == "avatar" {
		saved, err = h.storage.SaveAvatar(file, header)
	} else {
		saved, err = h.storage.SaveBanner(file, header)
	}
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrUnsupportedMIME):
			httpx.WriteBadRequest(w, "Поддерживаются JPG, PNG, WEBP")
		case errors.Is(err, storage.ErrTooLarge):
			httpx.WriteBadRequest(w, "Файл слишком большой")
		case errors.Is(err, storage.ErrEmpty):
			httpx.WriteBadRequest(w, "Файл пустой")
		default:
			log.Printf("save image: %v", err)
			httpx.WriteInternal(w)
		}
		return
	}

	// Атомарно: запоминаем старый путь чтобы удалить с диска.
	colPath := "avatar_path"
	colMime := "avatar_mime"
	colBytes := "avatar_bytes"
	if kind == "banner" {
		colPath = "banner_path"
		colMime = "banner_mime"
		colBytes = "banner_bytes"
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
		"SELECT "+colPath+" FROM users WHERE id = $1 FOR UPDATE", uid,
	).Scan(&oldPath); err != nil {
		_ = h.storage.Remove(saved.RelPath)
		httpx.WriteInternal(w)
		return
	}
	if _, err := tx.Exec(r.Context(),
		"UPDATE users SET "+colPath+" = $1, "+colMime+" = $2, "+colBytes+" = $3 WHERE id = $4",
		saved.RelPath, saved.MIME, saved.Bytes, uid,
	); err != nil {
		_ = h.storage.Remove(saved.RelPath)
		log.Printf("update %s: %v", kind, err)
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
		"url":  "/api/users/" + strconv.FormatInt(uid, 10) + "/" + kind,
	})
}

func (h *Handler) deleteImage(w http.ResponseWriter, r *http.Request, kind string) {
	if h.storage == nil {
		httpx.WriteInternal(w)
		return
	}
	uid := auth.UserIDFromContext(r.Context())
	if uid == 0 {
		httpx.WriteUnauthorized(w)
		return
	}
	colPath := "avatar_path"
	colMime := "avatar_mime"
	colBytes := "avatar_bytes"
	if kind == "banner" {
		colPath = "banner_path"
		colMime = "banner_mime"
		colBytes = "banner_bytes"
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.WriteInternal(w)
		return
	}
	defer tx.Rollback(r.Context())

	var oldPath *string
	if err := tx.QueryRow(r.Context(),
		"SELECT "+colPath+" FROM users WHERE id = $1 FOR UPDATE", uid,
	).Scan(&oldPath); err != nil {
		log.Printf("select %s: %v", kind, err)
		httpx.WriteInternal(w)
		return
	}
	if _, err := tx.Exec(r.Context(),
		"UPDATE users SET "+colPath+" = NULL, "+colMime+" = NULL, "+colBytes+" = NULL WHERE id = $1",
		uid,
	); err != nil {
		log.Printf("delete %s: %v", kind, err)
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

// ─── FOLLOWERS / FOLLOWING ──────────────────────────────────────────────

type ListUserItem struct {
	ID             int64  `json:"id"`
	Username       string `json:"username"`
	Bio            string `json:"bio"`
	HasAvatar      bool   `json:"has_avatar"`
	IsFollowedByMe bool   `json:"is_followed_by_me"`
	IsMe           bool   `json:"is_me"`
}

// GetFollowers — GET /api/users/{id}/followers — кто подписан на этого пользователя.
func (h *Handler) GetFollowers(w http.ResponseWriter, r *http.Request) {
	h.listUsers(w, r, "followers")
}

// GetFollowing — GET /api/users/{id}/following — на кого подписан этот пользователь.
func (h *Handler) GetFollowing(w http.ResponseWriter, r *http.Request) {
	h.listUsers(w, r, "following")
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request, kind string) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	viewer := auth.UserIDFromContext(r.Context())

	var q string
	switch kind {
	case "followers":
		// Юзеры, которые подписаны НА $1 (то есть follower_id = другие, followee_id = $1).
		q = `
			SELECT u.id, u.username, COALESCE(u.bio,''),
			       (u.avatar_path IS NOT NULL),
			       CASE WHEN $2::BIGINT > 0 THEN
				       EXISTS(SELECT 1 FROM follows ff
				              WHERE ff.follower_id = $2 AND ff.followee_id = u.id)
			       ELSE FALSE END
			  FROM follows f
			  JOIN users u ON u.id = f.follower_id
			 WHERE f.followee_id = $1 AND u.is_active = TRUE
			 ORDER BY f.created_at DESC
			 LIMIT 200
		`
	case "following":
		// Юзеры, на которых $1 подписан.
		q = `
			SELECT u.id, u.username, COALESCE(u.bio,''),
			       (u.avatar_path IS NOT NULL),
			       CASE WHEN $2::BIGINT > 0 THEN
				       EXISTS(SELECT 1 FROM follows ff
				              WHERE ff.follower_id = $2 AND ff.followee_id = u.id)
			       ELSE FALSE END
			  FROM follows f
			  JOIN users u ON u.id = f.followee_id
			 WHERE f.follower_id = $1 AND u.is_active = TRUE
			 ORDER BY f.created_at DESC
			 LIMIT 200
		`
	default:
		httpx.WriteBadRequest(w, "bad kind")
		return
	}
	rows, err := h.pool.Query(r.Context(), q, id, viewer)
	if err != nil {
		log.Printf("list %s: %v", kind, err)
		httpx.WriteInternal(w)
		return
	}
	defer rows.Close()
	out := []ListUserItem{}
	for rows.Next() {
		var u ListUserItem
		if err := rows.Scan(&u.ID, &u.Username, &u.Bio, &u.HasAvatar, &u.IsFollowedByMe); err != nil {
			log.Printf("scan %s: %v", kind, err)
			httpx.WriteInternal(w)
			return
		}
		u.IsMe = viewer != 0 && viewer == u.ID
		out = append(out, u)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"users": out, "kind": kind, "user_id": id})
}
