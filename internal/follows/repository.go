package follows

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// TopUploader — DTO для блока "Рекомендуемые".
type TopUploader struct {
	UserID         int64  `json:"user_id"`
	Username       string `json:"username"`
	TotalPlays     int64  `json:"total_plays"`
	TracksCount    int64  `json:"tracks_count"`
	FollowersCount int64  `json:"followers_count"`
	IsFollowedByMe bool   `json:"is_followed_by_me"`
	HasAvatar      bool   `json:"has_avatar"`
}

var (
	ErrSelfFollow      = errors.New("cannot follow yourself")
	ErrUserNotFound    = errors.New("user not found")
	ErrAlreadyFollowed = errors.New("already followed")
)

// Top — топ-N юзеров по сумме prosluшиваний из materialised view top_uploaders.
// Если viewerID > 0 — заполняем is_followed_by_me.
//
// REFRESH MATERIALIZED VIEW делается отдельно (см. Refresh ниже),
// чтобы блок отдавался за O(log N) без агрегаций при каждом запросе.
func (r *Repository) Top(ctx context.Context, limit int, viewerID, excludeUserID int64) ([]TopUploader, error) {
	if limit <= 0 || limit > 50 {
		limit = 3
	}
	args := []any{limit}
	where := "1 = 1"
	if excludeUserID > 0 {
		args = append(args, excludeUserID)
		where = "tu.user_id <> $2"
	}
	followCol := "FALSE"
	if viewerID > 0 {
		args = append(args, viewerID)
		followCol = "EXISTS (SELECT 1 FROM follows f WHERE f.followee_id = tu.user_id AND f.follower_id = $" + itoa(len(args)) + ")"
	}
	q := `
		SELECT
			tu.user_id, tu.username, tu.total_plays, tu.tracks_count, tu.followers_count,
			` + followCol + `,
			(u.avatar_path IS NOT NULL)
		FROM top_uploaders tu
		LEFT JOIN users u ON u.id = tu.user_id
		WHERE ` + where + `
		ORDER BY tu.total_plays DESC, tu.user_id ASC
		LIMIT $1
	`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TopUploader
	for rows.Next() {
		var u TopUploader
		if err := rows.Scan(
			&u.UserID, &u.Username, &u.TotalPlays, &u.TracksCount, &u.FollowersCount, &u.IsFollowedByMe,
			&u.HasAvatar,
		); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// Refresh — обновляет materialised view top_uploaders в фоне.
// Concurrently — без блокировки SELECT'ов; занимает <100ms на тысячах юзеров.
func (r *Repository) Refresh(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `REFRESH MATERIALIZED VIEW CONCURRENTLY top_uploaders`)
	return err
}

// Follow — INSERT с DO NOTHING (идемпотентно). Возвращает true если был добавлен.
// CHECK constraint follows_no_self не даёт подписаться на себя.
func (r *Repository) Follow(ctx context.Context, followerID, followeeID int64) (bool, error) {
	if followerID == followeeID {
		return false, ErrSelfFollow
	}
	// Проверяем, что followee существует.
	var exists bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND is_active = TRUE)`, followeeID,
	).Scan(&exists); err != nil {
		return false, err
	}
	if !exists {
		return false, ErrUserNotFound
	}
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2)
		ON CONFLICT (follower_id, followee_id) DO NOTHING
	`, followerID, followeeID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *Repository) Unfollow(ctx context.Context, followerID, followeeID int64) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2
	`, followerID, followeeID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// IsFollowed — проверка для UI.
func (r *Repository) IsFollowed(ctx context.Context, followerID, followeeID int64) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2)
	`, followerID, followeeID).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return ok, err
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	buf := make([]byte, 0, 10)
	for i > 0 {
		buf = append([]byte{byte('0' + i%10)}, buf...)
		i /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
