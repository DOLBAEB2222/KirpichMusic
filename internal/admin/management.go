// management.go — админские эндпоинты для управления пользователями и треками.
package admin

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/bogdan/kirpichmusic/internal/auth"
	"github.com/bogdan/kirpichmusic/internal/httpx"
)

// ─── USERS ──────────────────────────────────────────────────────────────

type adminUser struct {
	ID         int64     `json:"id"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`
	IsAdmin    bool      `json:"is_admin"`
	IsActive   bool      `json:"is_active"`
	HasAvatar  bool      `json:"has_avatar"`
	TracksCount int64    `json:"tracks_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// ListUsers — GET /api/admin/users?q=&limit=&offset=
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	args := []any{}
	where := []string{"1=1"}
	if q != "" {
		args = append(args, "%"+q+"%")
		where = append(where, "(u.username ILIKE $"+strconv.Itoa(len(args))+
			" OR u.email::TEXT ILIKE $"+strconv.Itoa(len(args))+")")
	}
	args = append(args, limit, offset)
	limI := strconv.Itoa(len(args) - 1)
	offI := strconv.Itoa(len(args))

	sql := `
		SELECT u.id, u.username, u.email::TEXT, u.is_admin, u.is_active,
		       (u.avatar_path IS NOT NULL),
		       (SELECT COUNT(*) FROM tracks t WHERE t.uploader_id = u.id)::BIGINT,
		       u.created_at
		  FROM users u
		 WHERE ` + strings.Join(where, " AND ") + `
		 ORDER BY u.id ASC
		 LIMIT $` + limI + ` OFFSET $` + offI

	rows, err := h.pool.Query(r.Context(), sql, args...)
	if err != nil {
		log.Printf("admin list users: %v", err)
		httpx.WriteInternal(w)
		return
	}
	defer rows.Close()
	out := []adminUser{}
	for rows.Next() {
		var u adminUser
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.IsAdmin, &u.IsActive,
			&u.HasAvatar, &u.TracksCount, &u.CreatedAt); err != nil {
			httpx.WriteInternal(w)
			return
		}
		out = append(out, u)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"users": out})
}

type patchUserInput struct {
	IsAdmin  *bool `json:"is_admin,omitempty"`
	IsActive *bool `json:"is_active,omitempty"`
}

// PatchUser — PATCH /api/admin/users/{id}
//   body: {"is_admin": true} | {"is_active": false}
//
// Самозащита: запрещаем админу СНИМАТЬ права с самого себя через этот endpoint
// (иначе можно нечаянно лишиться доступа). Удалить /заблокировать может только
// другой админ.
func (h *Handler) PatchUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	var in patchUserInput
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		httpx.WriteBadRequest(w, "invalid body")
		return
	}
	if in.IsAdmin == nil && in.IsActive == nil {
		httpx.WriteBadRequest(w, "nothing to update")
		return
	}
	me := auth.UserIDFromContext(r.Context())
	if id == me && (in.IsAdmin != nil && !*in.IsAdmin) {
		httpx.WriteBadRequest(w, "Нельзя снять админ-права с самого себя")
		return
	}
	if id == me && (in.IsActive != nil && !*in.IsActive) {
		httpx.WriteBadRequest(w, "Нельзя деактивировать самого себя")
		return
	}
	sets := []string{}
	args := []any{}
	if in.IsAdmin != nil {
		args = append(args, *in.IsAdmin)
		sets = append(sets, "is_admin = $"+strconv.Itoa(len(args)))
	}
	if in.IsActive != nil {
		args = append(args, *in.IsActive)
		sets = append(sets, "is_active = $"+strconv.Itoa(len(args)))
	}
	args = append(args, id)
	q := "UPDATE users SET " + strings.Join(sets, ", ") + " WHERE id = $" + strconv.Itoa(len(args))
	if _, err := h.pool.Exec(r.Context(), q, args...); err != nil {
		log.Printf("admin patch user: %v", err)
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ─── TRACKS — модерация и удаление ──────────────────────────────────────

type adminTrack struct {
	ID               int64     `json:"id"`
	Title            string    `json:"title"`
	ArtistName       string    `json:"artist_name"`
	ArtistID         *int64    `json:"artist_id,omitempty"`
	UploaderID       *int64    `json:"uploader_id,omitempty"`
	UploaderName     string    `json:"uploader_username,omitempty"`
	ModerationStatus string    `json:"moderation_status"`
	CreatedAt        time.Time `json:"created_at"`
}

// ListPendingTracks — GET /api/admin/tracks/pending
func (h *Handler) ListPendingTracks(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), `
		SELECT t.id, t.title, t.artist_name, t.artist_id,
		       t.uploader_id, COALESCE(u.username, ''),
		       t.moderation_status, t.created_at
		  FROM tracks t
		  LEFT JOIN users u ON u.id = t.uploader_id
		 WHERE t.moderation_status = 'pending'
		 ORDER BY t.created_at ASC
		 LIMIT 200
	`)
	if err != nil {
		log.Printf("admin pending tracks: %v", err)
		httpx.WriteInternal(w)
		return
	}
	defer rows.Close()
	out := []adminTrack{}
	for rows.Next() {
		var t adminTrack
		if err := rows.Scan(&t.ID, &t.Title, &t.ArtistName, &t.ArtistID,
			&t.UploaderID, &t.UploaderName, &t.ModerationStatus, &t.CreatedAt); err != nil {
			httpx.WriteInternal(w)
			return
		}
		out = append(out, t)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"tracks": out})
}

// ApproveTrack — POST /api/admin/tracks/{id}/approve
func (h *Handler) ApproveTrack(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	uid := auth.UserIDFromContext(r.Context())
	res, err := h.pool.Exec(r.Context(), `
		UPDATE tracks
		   SET moderation_status = 'approved',
		       moderated_by = $1,
		       moderated_at = now(),
		       is_published = TRUE
		 WHERE id = $2 AND moderation_status = 'pending'
	`, uid, id)
	if err != nil {
		log.Printf("admin approve track: %v", err)
		httpx.WriteInternal(w)
		return
	}
	if res.RowsAffected() == 0 {
		httpx.WriteError(w, http.StatusNotFound, "Трек не найден или уже обработан", "not_found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// RejectTrack — POST /api/admin/tracks/{id}/reject
//   Удаляем запись + файл аудио + обложку. Это «отказ модерации»,
//   а не «дисциплинарное удаление» (последнее — DeleteTrack ниже).
func (h *Handler) RejectTrack(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	h.deletePendingOrAny(w, r, id, true /*onlyPending*/)
}

// DeleteTrack — DELETE /api/admin/tracks/{id}
//   Полное удаление любого трека админом. Файлы тоже удаляются.
func (h *Handler) DeleteTrack(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	h.deletePendingOrAny(w, r, id, false)
}

func (h *Handler) deletePendingOrAny(w http.ResponseWriter, r *http.Request, id int64, onlyPending bool) {
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.WriteInternal(w)
		return
	}
	defer tx.Rollback(r.Context())

	var (
		audioPath string
		coverPath *string
		modStatus string
	)
	q := `SELECT audio_path, cover_path, moderation_status FROM tracks WHERE id = $1 FOR UPDATE`
	if err := tx.QueryRow(r.Context(), q, id).Scan(&audioPath, &coverPath, &modStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "Трек не найден", "not_found")
			return
		}
		httpx.WriteInternal(w)
		return
	}
	if onlyPending && modStatus != "pending" {
		httpx.WriteError(w, http.StatusBadRequest, "Можно отклонять только pending-треки", "bad_state")
		return
	}
	if _, err := tx.Exec(r.Context(), `DELETE FROM tracks WHERE id = $1`, id); err != nil {
		log.Printf("admin delete track: %v", err)
		httpx.WriteInternal(w)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteInternal(w)
		return
	}
	// Удаляем файлы вне транзакции (если БД ок, а файлы недоступны — это не катастрофа).
	h.removeFile(audioPath)
	if coverPath != nil {
		h.removeFile(*coverPath)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) removeFile(rel string) {
	if rel == "" || h.mediaRoot == "" {
		return
	}
	cleaned := filepath.Clean("/" + rel)
	abs := filepath.Join(h.mediaRoot, cleaned)
	abs = filepath.Clean(abs)
	if !strings.HasPrefix(abs, h.mediaRoot+string(filepath.Separator)) {
		return
	}
	_ = os.Remove(abs)
}

// withTimeout — короткий хелпер для контекстов под админ-запросы.
func withTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 5*time.Second)
}

var _ = withTimeout // не используется напрямую, но останется для будущих расширений.
