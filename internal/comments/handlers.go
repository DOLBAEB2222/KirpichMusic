package comments

import (
	"log"
	"net/http"
	"strconv"
	"strings"

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

// List — GET /api/tracks/{id}/comments?limit=30&offset=0
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	cs, err := h.repo.List(r.Context(), id, limit, offset)
	if err != nil {
		log.Printf("list comments: %v", err)
		httpx.WriteInternal(w)
		return
	}
	if cs == nil {
		cs = []Comment{}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"comments": cs})
}

// Add — POST /api/tracks/{id}/comments  body: {"body":"..."}
func (h *Handler) Add(w http.ResponseWriter, r *http.Request) {
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
	var in struct {
		Body string `json:"body"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteBadRequest(w, err.Error())
		return
	}
	body := strings.TrimSpace(in.Body)
	if l := len(body); l < 1 || l > 1000 {
		httpx.WriteBadRequest(w, "body должен быть 1..1000 символов")
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
	c, err := h.repo.Add(r.Context(), uid, id, body)
	if err != nil {
		log.Printf("add comment: %v", err)
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"comment": c})
}

func parseID(r *http.Request) (int64, bool) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
