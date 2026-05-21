package comments

import (
	"context"
	"errors"
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

// Comment — DTO для отдачи клиенту.
type Comment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	Author    Author    `json:"author"`
}

type Author struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

// Add — INSERT, comments_count в tracks обновляется триггером trg_comments_count.
// Возвращает только-что добавленный коммент с join по users.
func (r *Repository) Add(ctx context.Context, userID, trackID int64, body string) (*Comment, error) {
	const q = `
		WITH inserted AS (
			INSERT INTO comments (track_id, user_id, body)
			VALUES ($1, $2, $3)
			RETURNING id, user_id, body, created_at
		)
		SELECT i.id, i.body, i.created_at, u.id, u.username
		FROM inserted i
		JOIN users u ON u.id = i.user_id
	`
	var c Comment
	if err := r.pool.QueryRow(ctx, q, trackID, userID, body).Scan(
		&c.ID, &c.Body, &c.CreatedAt, &c.Author.ID, &c.Author.Username,
	); err != nil {
		return nil, err
	}
	return &c, nil
}

// List — постранично, новые сверху (использует idx_comments_track_created).
func (r *Repository) List(ctx context.Context, trackID int64, limit, offset int) ([]Comment, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.body, c.created_at, u.id, u.username
		FROM comments c
		JOIN users u ON u.id = c.user_id
		WHERE c.track_id = $1
		ORDER BY c.created_at DESC
		LIMIT $2 OFFSET $3
	`, trackID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.Body, &c.CreatedAt, &c.Author.ID, &c.Author.Username); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// TrackExists — для валидации перед добавлением (избегаем 500 при FK).
func (r *Repository) TrackExists(ctx context.Context, trackID int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM tracks WHERE id = $1 AND is_published = TRUE)`, trackID,
	).Scan(&exists)
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return exists, err
}
