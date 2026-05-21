package playlists

import (
	"errors"
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

// List — GET /api/playlists?user_id=…
// Если user_id отсутствует и есть кука — возвращаем плейлисты текущего юзера.
// Если user_id указан — публичные плейлисты этого юзера (или все если это сам юзер).
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	viewer := auth.UserIDFromContext(r.Context())
	uidStr := r.URL.Query().Get("user_id")

	var ownerID int64
	if uidStr == "" {
		if viewer == 0 {
			httpx.WriteUnauthorized(w)
			return
		}
		ownerID = viewer
	} else {
		v, err := strconv.ParseInt(uidStr, 10, 64)
		if err != nil || v <= 0 {
			httpx.WriteBadRequest(w, "invalid user_id")
			return
		}
		ownerID = v
	}

	pls, err := h.repo.ListByUser(r.Context(), ownerID, viewer)
	if err != nil {
		log.Printf("list playlists: %v", err)
		httpx.WriteInternal(w)
		return
	}
	if pls == nil {
		pls = []Playlist{}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"playlists": pls})
}

type createReq struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	CoverColor  string `json:"cover_color"`
	CoverEmoji  string `json:"cover_emoji"`
}

// Create — POST /api/playlists  (cookie required)
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	uid := auth.UserIDFromContext(r.Context())
	if uid == 0 {
		httpx.WriteUnauthorized(w)
		return
	}
	var req createReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteBadRequest(w, "Некорректный JSON")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || len(req.Title) > 80 {
		httpx.WriteBadRequest(w, "Название плейлиста: 1..80 символов")
		return
	}
	if len(req.Description) > 500 {
		httpx.WriteBadRequest(w, "Описание не больше 500 символов")
		return
	}
	if req.CoverColor != "" && !validHex(req.CoverColor) {
		httpx.WriteBadRequest(w, "cover_color должен быть #RRGGBB")
		return
	}
	p, err := h.repo.Create(r.Context(), CreateInput{
		UserID:      uid,
		Title:       req.Title,
		Description: strings.TrimSpace(req.Description),
		CoverColor:  req.CoverColor,
		CoverEmoji:  req.CoverEmoji,
	})
	if err != nil {
		log.Printf("create playlist: %v", err)
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"playlist": p})
}

// Get — GET /api/playlists/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		httpx.WriteBadRequest(w, "invalid id")
		return
	}
	viewer := auth.UserIDFromContext(r.Context())
	p, err := h.repo.GetByID(r.Context(), id, viewer)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			httpx.WriteError(w, http.StatusNotFound, "Playlist not found", "not_found")
		case errors.Is(err, ErrForbidden):
			httpx.WriteError(w, http.StatusForbidden, "Forbidden", "forbidden")
		default:
			log.Printf("get playlist: %v", err)
			httpx.WriteInternal(w)
		}
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"playlist": p})
}

type updateReq struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	CoverColor  *string `json:"cover_color,omitempty"`
	CoverEmoji  *string `json:"cover_emoji,omitempty"`
	IsPublic    *bool   `json:"is_public,omitempty"`
}

// Update — PATCH /api/playlists/{id}  (owner)
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
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
	var req updateReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteBadRequest(w, "Некорректный JSON")
		return
	}
	in := UpdateInput{}
	if req.Title != nil {
		t := strings.TrimSpace(*req.Title)
		if t == "" || len(t) > 80 {
			httpx.WriteBadRequest(w, "Название: 1..80 символов")
			return
		}
		in.Title = &t
	}
	if req.Description != nil {
		d := strings.TrimSpace(*req.Description)
		if len(d) > 500 {
			httpx.WriteBadRequest(w, "Описание не больше 500 символов")
			return
		}
		in.Description = &d
	}
	if req.CoverColor != nil {
		if !validHex(*req.CoverColor) {
			httpx.WriteBadRequest(w, "cover_color должен быть #RRGGBB")
			return
		}
		in.CoverColor = req.CoverColor
	}
	if req.CoverEmoji != nil {
		in.CoverEmoji = req.CoverEmoji
	}
	if req.IsPublic != nil {
		in.IsPublic = req.IsPublic
	}
	if err := h.repo.Update(r.Context(), id, uid, in); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			httpx.WriteError(w, http.StatusNotFound, "Playlist not found", "not_found")
		default:
			log.Printf("update playlist: %v", err)
			httpx.WriteInternal(w)
		}
		return
	}
	p, err := h.repo.GetByID(r.Context(), id, uid)
	if err != nil {
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"playlist": p})
}

// Delete — DELETE /api/playlists/{id}  (owner)
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
	if err := h.repo.Delete(r.Context(), id, uid); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "Playlist not found", "not_found")
			return
		}
		log.Printf("delete playlist: %v", err)
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type addTrackReq struct {
	TrackID int64 `json:"track_id"`
}

// AddTrack — POST /api/playlists/{id}/tracks  (owner)
func (h *Handler) AddTrack(w http.ResponseWriter, r *http.Request) {
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
	var req addTrackReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteBadRequest(w, "Некорректный JSON")
		return
	}
	if req.TrackID <= 0 {
		httpx.WriteBadRequest(w, "track_id обязателен")
		return
	}
	err := h.repo.AddTrack(r.Context(), id, uid, req.TrackID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			httpx.WriteError(w, http.StatusNotFound, "Playlist not found", "not_found")
		case errors.Is(err, ErrForbidden):
			httpx.WriteError(w, http.StatusForbidden, "Forbidden", "forbidden")
		case errors.Is(err, ErrTrackNotFound):
			httpx.WriteError(w, http.StatusNotFound, "Track not found", "not_found")
		case errors.Is(err, ErrAlreadyInPlaylist):
			httpx.WriteError(w, http.StatusConflict, "Трек уже в плейлисте", "already_in_playlist")
		default:
			log.Printf("add track to playlist: %v", err)
			httpx.WriteInternal(w)
		}
		return
	}
	p, err := h.repo.GetByID(r.Context(), id, uid)
	if err != nil {
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"playlist": p})
}

// RemoveTrack — DELETE /api/playlists/{id}/tracks/{track_id}  (owner)
func (h *Handler) RemoveTrack(w http.ResponseWriter, r *http.Request) {
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
	tidStr := chi.URLParam(r, "track_id")
	tid, err := strconv.ParseInt(tidStr, 10, 64)
	if err != nil || tid <= 0 {
		httpx.WriteBadRequest(w, "invalid track_id")
		return
	}
	if err := h.repo.RemoveTrack(r.Context(), id, uid, tid); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			httpx.WriteError(w, http.StatusNotFound, "Playlist not found", "not_found")
		case errors.Is(err, ErrForbidden):
			httpx.WriteError(w, http.StatusForbidden, "Forbidden", "forbidden")
		default:
			log.Printf("remove track from playlist: %v", err)
			httpx.WriteInternal(w)
		}
		return
	}
	p, err := h.repo.GetByID(r.Context(), id, uid)
	if err != nil {
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"playlist": p})
}

func parseID(r *http.Request) (int64, bool) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// validHex — true для строки вида "#aabbcc". Длинных HSL и rgba не поддерживаем.
func validHex(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for i := 1; i < 7; i++ {
		c := s[i]
		hex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !hex {
			return false
		}
	}
	return true
}
