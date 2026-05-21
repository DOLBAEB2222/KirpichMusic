// Package playlists — пользовательские подборки треков.
//
// Owner-only mutation: только владелец плейлиста может его редактировать,
// добавлять/удалять треки, переименовывать, удалять. Просмотр — публичный
// для is_public=true; для приватных доступ только владельцу (если будем
// добавлять переключатель — пока всё public, поле в БД зарезервировано).
package playlists

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

// Playlist — DTO для отдачи клиенту.
type Playlist struct {
	ID           int64     `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	CoverColor   string    `json:"cover_color"`
	CoverEmoji   string    `json:"cover_emoji"`
	IsPublic     bool      `json:"is_public"`
	TracksCount  int       `json:"tracks_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	OwnerID      int64     `json:"owner_id"`
	OwnerName    string    `json:"owner_name"`
	IsOwn        bool      `json:"is_own"`
	// Tracks — заполняется только в GetByID; в списке оставляем nil.
	Tracks []PlaylistTrack `json:"tracks,omitempty"`
}

type PlaylistTrack struct {
	ID         int64   `json:"id"`
	Title      string  `json:"title"`
	ArtistName string  `json:"artist_name"`
	Position   int     `json:"position"`
	HasCover   bool    `json:"has_cover"`
	CoverColor *string `json:"cover_color,omitempty"`
	CoverEmoji *string `json:"cover_emoji,omitempty"`
	DurationS  int     `json:"duration_sec"`
}

type CreateInput struct {
	UserID      int64
	Title       string
	Description string
	CoverColor  string
	CoverEmoji  string
}

// Create — INSERT и возврат полного объекта.
func (r *Repository) Create(ctx context.Context, in CreateInput) (*Playlist, error) {
	var (
		id  int64
		now time.Time
	)
	var descrArg any
	if in.Description == "" {
		descrArg = nil
	} else {
		descrArg = in.Description
	}
	color := in.CoverColor
	if color == "" {
		color = "#1a1a2e"
	}
	emoji := in.CoverEmoji
	if emoji == "" {
		emoji = "🎵"
	}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO playlists (user_id, title, description, cover_color, cover_emoji)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, in.UserID, in.Title, descrArg, color, emoji).Scan(&id, &now)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id, in.UserID)
}

// ListByUser — все плейлисты юзера. Если viewerID != ownerID,
// возвращаем только публичные.
func (r *Repository) ListByUser(ctx context.Context, ownerID, viewerID int64) ([]Playlist, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.title, COALESCE(p.description, ''),
		       p.cover_color, p.cover_emoji, p.is_public, p.tracks_count,
		       p.created_at, p.updated_at, p.user_id, u.username
		FROM playlists p
		JOIN users u ON u.id = p.user_id
		WHERE p.user_id = $1
		  AND ($2::BIGINT = $1 OR p.is_public = TRUE)
		ORDER BY p.created_at DESC
	`, ownerID, viewerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Playlist{}
	for rows.Next() {
		var p Playlist
		if err := rows.Scan(&p.ID, &p.Title, &p.Description,
			&p.CoverColor, &p.CoverEmoji, &p.IsPublic, &p.TracksCount,
			&p.CreatedAt, &p.UpdatedAt, &p.OwnerID, &p.OwnerName); err != nil {
			return nil, err
		}
		p.IsOwn = viewerID != 0 && viewerID == p.OwnerID
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetByID — плейлист с треками. Доступ: всегда для владельца, или если is_public.
func (r *Repository) GetByID(ctx context.Context, id, viewerID int64) (*Playlist, error) {
	var p Playlist
	err := r.pool.QueryRow(ctx, `
		SELECT p.id, p.title, COALESCE(p.description, ''),
		       p.cover_color, p.cover_emoji, p.is_public, p.tracks_count,
		       p.created_at, p.updated_at, p.user_id, u.username
		FROM playlists p
		JOIN users u ON u.id = p.user_id
		WHERE p.id = $1
	`, id).Scan(&p.ID, &p.Title, &p.Description,
		&p.CoverColor, &p.CoverEmoji, &p.IsPublic, &p.TracksCount,
		&p.CreatedAt, &p.UpdatedAt, &p.OwnerID, &p.OwnerName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.IsOwn = viewerID != 0 && viewerID == p.OwnerID
	if !p.IsPublic && !p.IsOwn {
		return nil, ErrForbidden
	}

	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.title, t.artist_name, pt.position,
		       t.cover_path IS NOT NULL,
		       t.cover_color, t.cover_emoji, t.duration_sec
		FROM playlist_tracks pt
		JOIN tracks t ON t.id = pt.track_id
		WHERE pt.playlist_id = $1 AND t.is_published = TRUE
		ORDER BY pt.position ASC, pt.added_at ASC
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t PlaylistTrack
		if err := rows.Scan(&t.ID, &t.Title, &t.ArtistName, &t.Position,
			&t.HasCover, &t.CoverColor, &t.CoverEmoji, &t.DurationS); err != nil {
			return nil, err
		}
		p.Tracks = append(p.Tracks, t)
	}
	if p.Tracks == nil {
		p.Tracks = []PlaylistTrack{}
	}
	return &p, rows.Err()
}

type UpdateInput struct {
	Title       *string
	Description *string
	CoverColor  *string
	CoverEmoji  *string
	IsPublic    *bool
}

// Update — частичный апдейт. Только владелец.
func (r *Repository) Update(ctx context.Context, id, ownerID int64, in UpdateInput) error {
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
	if in.Description != nil {
		if *in.Description == "" {
			sets = append(sets, "description = NULL")
		} else {
			add("description", *in.Description)
		}
	}
	if in.CoverColor != nil {
		add("cover_color", *in.CoverColor)
	}
	if in.CoverEmoji != nil {
		add("cover_emoji", *in.CoverEmoji)
	}
	if in.IsPublic != nil {
		add("is_public", *in.IsPublic)
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id, ownerID)
	q := "UPDATE playlists SET " + strings.Join(sets, ", ") +
		fmt.Sprintf(" WHERE id = $%d AND user_id = $%d", idx, idx+1)
	tag, err := r.pool.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete — только владелец.
func (r *Repository) Delete(ctx context.Context, id, ownerID int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM playlists WHERE id=$1 AND user_id=$2`, id, ownerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AddTrack — добавляет трек в конец плейлиста (position = max+1).
// Идемпотентно по PRIMARY KEY (playlist_id, track_id).
func (r *Repository) AddTrack(ctx context.Context, playlistID, ownerID, trackID int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var dbOwner int64
	err = tx.QueryRow(ctx,
		`SELECT user_id FROM playlists WHERE id=$1 FOR UPDATE`, playlistID,
	).Scan(&dbOwner)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if dbOwner != ownerID {
		return ErrForbidden
	}

	// Проверим что трек существует.
	var trackOK bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM tracks WHERE id=$1 AND is_published=TRUE)`, trackID,
	).Scan(&trackOK); err != nil {
		return err
	}
	if !trackOK {
		return ErrTrackNotFound
	}

	var nextPos int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(position), 0) + 1 FROM playlist_tracks WHERE playlist_id=$1`,
		playlistID,
	).Scan(&nextPos); err != nil {
		return err
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO playlist_tracks (playlist_id, track_id, position)
		VALUES ($1, $2, $3)
		ON CONFLICT (playlist_id, track_id) DO NOTHING
	`, playlistID, trackID, nextPos)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAlreadyInPlaylist
	}
	return tx.Commit(ctx)
}

// RemoveTrack — удаляет трек из плейлиста. Только владелец.
func (r *Repository) RemoveTrack(ctx context.Context, playlistID, ownerID, trackID int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var dbOwner int64
	err = tx.QueryRow(ctx,
		`SELECT user_id FROM playlists WHERE id=$1 FOR UPDATE`, playlistID,
	).Scan(&dbOwner)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if dbOwner != ownerID {
		return ErrForbidden
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM playlist_tracks WHERE playlist_id=$1 AND track_id=$2`,
		playlistID, trackID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

var (
	ErrNotFound          = errors.New("playlist not found")
	ErrForbidden         = errors.New("not the owner")
	ErrTrackNotFound     = errors.New("track not found")
	ErrAlreadyInPlaylist = errors.New("already in playlist")
)
