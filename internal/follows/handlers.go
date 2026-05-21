package follows

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/bogdan/kirpichmusic/internal/auth"
	"github.com/bogdan/kirpichmusic/internal/httpx"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

// Top — GET /api/users/top?limit=3
// Виден всем; viewer-флаг is_followed_by_me заполнится только для авторизованных.
func (h *Handler) Top(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 3
	}
	viewer := auth.UserIDFromContext(r.Context())
	users, err := h.repo.Top(r.Context(), limit, viewer, viewer)
	if err != nil {
		log.Printf("top uploaders: %v", err)
		httpx.WriteInternal(w)
		return
	}
	if users == nil {
		users = []TopUploader{}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"users": users})
}

// Follow — POST /api/users/{id}/follow (cookie required)
func (h *Handler) Follow(w http.ResponseWriter, r *http.Request) {
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
	if _, err := h.repo.Follow(r.Context(), uid, id); err != nil {
		switch {
		case errors.Is(err, ErrSelfFollow):
			httpx.WriteBadRequest(w, "Нельзя подписаться на самого себя")
			return
		case errors.Is(err, ErrUserNotFound):
			httpx.WriteError(w, http.StatusNotFound, "User not found", "not_found")
			return
		default:
			log.Printf("follow: %v", err)
			httpx.WriteInternal(w)
			return
		}
	}
	// Refresh MV в фоне — чтобы followers_count в "Рекомендуемых" обновился.
	go h.scheduleRefresh()
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "is_followed": true})
}

// Unfollow — DELETE /api/users/{id}/follow (cookie required)
func (h *Handler) Unfollow(w http.ResponseWriter, r *http.Request) {
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
	if _, err := h.repo.Unfollow(r.Context(), uid, id); err != nil {
		log.Printf("unfollow: %v", err)
		httpx.WriteInternal(w)
		return
	}
	go h.scheduleRefresh()
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "is_followed": false})
}

// scheduleRefresh — отдельный контекст с таймаутом, чтобы тяжёлый
// REFRESH не задерживал ответ. CONCURRENTLY → не блокирует SELECT'ы.
func (h *Handler) scheduleRefresh() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := h.repo.Refresh(ctx); err != nil {
		log.Printf("refresh top_uploaders: %v", err)
	}
}

func parseID(r *http.Request) (int64, bool) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
