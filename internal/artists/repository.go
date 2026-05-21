// Package artists — отдельная сущность артиста (анти-дубль, верификация,
// собственный профиль). См. db/migrations/006_artists.sql.
package artists

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound — артист не найден.
var ErrNotFound = errors.New("artist not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Artist — DTO профиля артиста.
type Artist struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	Slug             string    `json:"slug,omitempty"`
	Description      string    `json:"description,omitempty"`
	HasAvatar        bool      `json:"has_avatar"`
	HasBanner        bool      `json:"has_banner"`
	IsVerified       bool      `json:"is_verified"`
	ClaimedByUserID  *int64    `json:"claimed_by_user_id,omitempty"`
	ClaimedByName    string    `json:"claimed_by_username,omitempty"`
	TracksCount      int64     `json:"tracks_count"`
	CreatedAt        time.Time `json:"created_at"`
}

// Suggestion — лёгкий объект для autocomplete при загрузке трека.
type Suggestion struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Slug       string  `json:"slug,omitempty"`
	HasAvatar  bool    `json:"has_avatar"`
	IsVerified bool    `json:"is_verified"`
	// Similarity 0..1 (0 = не похоже, 1 = идентично). Точные совпадения — 1.0.
	Similarity float64 `json:"similarity,omitempty"`
}

// ─── slug ───────────────────────────────────────────────────────────────

// translitMap — простая транслитерация кириллицы для slug'ов.
var translitMap = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "yo",
	'ж': "zh", 'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m",
	'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u",
	'ф': "f", 'х': "h", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "sch",
	'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// MakeSlug строит безопасный slug из имени.
// Возвращает пустую строку если не удалось (тогда фронт открывает по id).
func MakeSlug(name string) string {
	low := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range low {
		if t, ok := translitMap[r]; ok {
			b.WriteString(t)
		} else if r < 0x80 {
			b.WriteRune(r)
		}
	}
	s := slugRe.ReplaceAllString(b.String(), "-")
	s = strings.Trim(s, "-")
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

// ─── core CRUD ─────────────────────────────────────────────────────────

// FindOrCreateByName — находит существующего артиста по CITEXT-имени или создаёт нового.
// Транзакционно безопасно (UNIQUE по name защищает от гонки).
func (r *Repository) FindOrCreateByName(ctx context.Context, name string) (int64, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, false, errors.New("empty artist name")
	}

	// 1) Пытаемся найти.
	var id int64
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM artists WHERE name = $1 LIMIT 1`, name,
	).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, err
	}

	// 2) Создаём с slug. Если slug коллидирует — добавляем -2, -3, ...
	baseSlug := MakeSlug(name)
	for attempt := 0; attempt < 50; attempt++ {
		slug := baseSlug
		if attempt > 0 {
			if baseSlug == "" {
				slug = ""
			} else {
				slug = baseSlug + "-" + intToStr(attempt+1)
			}
		}
		var slugVal any = slug
		if slug == "" {
			slugVal = nil
		}
		err := r.pool.QueryRow(ctx,
			`INSERT INTO artists (name, slug) VALUES ($1, $2)
			 ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
			 RETURNING id`,
			name, slugVal,
		).Scan(&id)
		if err == nil {
			return id, true, nil
		}
		// pq slug-unique конфликт — пробуем со счётчиком.
		if isUniqueViolation(err, "artists_slug_key") {
			continue
		}
		return 0, false, err
	}
	return 0, false, errors.New("could not generate unique slug")
}

// GetByID
func (r *Repository) GetByID(ctx context.Context, id int64) (*Artist, error) {
	return r.lookup(ctx, "a.id = $1", id)
}

// GetBySlug
func (r *Repository) GetBySlug(ctx context.Context, slug string) (*Artist, error) {
	return r.lookup(ctx, "a.slug = $1", slug)
}

func (r *Repository) lookup(ctx context.Context, where string, arg any) (*Artist, error) {
	const base = `
		SELECT a.id, a.name, COALESCE(a.slug,''), COALESCE(a.description,''),
		       (a.avatar_path IS NOT NULL),
		       (a.banner_path IS NOT NULL),
		       a.is_verified,
		       a.claimed_by_user_id,
		       COALESCE(u.username, ''),
		       (SELECT COUNT(*) FROM tracks t
		         WHERE t.artist_id = a.id
		           AND t.is_published = TRUE
		           AND t.moderation_status = 'approved')::BIGINT,
		       a.created_at
		  FROM artists a
		  LEFT JOIN users u ON u.id = a.claimed_by_user_id
		 WHERE `
	var ar Artist
	err := r.pool.QueryRow(ctx, base+where, arg).Scan(
		&ar.ID, &ar.Name, &ar.Slug, &ar.Description,
		&ar.HasAvatar, &ar.HasBanner,
		&ar.IsVerified,
		&ar.ClaimedByUserID,
		&ar.ClaimedByName,
		&ar.TracksCount,
		&ar.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &ar, nil
}

// Suggest — fuzzy поиск для автокомплита при загрузке трека.
//   - Точное совпадение (CITEXT) идёт первым с similarity=1.
//   - Дальше — pg_trgm similarity, отфильтровано по порогу 0.3.
//   - Limit ≤ 10.
func (r *Repository) Suggest(ctx context.Context, q string, limit int) ([]Suggestion, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return []Suggestion{}, nil
	}
	if limit <= 0 || limit > 10 {
		limit = 5
	}
	// pg_trgm similarity('Король и Шут', 'король') ≈ 0.45
	// Возвращаем точные + похожие; точное всегда сверху.
	const sql = `
		WITH ranked AS (
			SELECT id, name, COALESCE(slug,'') AS slug,
			       (avatar_path IS NOT NULL) AS has_avatar,
			       is_verified,
			       CASE WHEN name = $1 THEN 1.0
			            ELSE similarity(name::text, $1)
			       END AS sim
			  FROM artists
			 WHERE name = $1 OR similarity(name::text, $1) > 0.3
		)
		SELECT id, name, slug, has_avatar, is_verified, sim
		  FROM ranked
		 ORDER BY sim DESC, name ASC
		 LIMIT $2
	`
	rows, err := r.pool.Query(ctx, sql, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Suggestion{}
	for rows.Next() {
		var s Suggestion
		if err := rows.Scan(&s.ID, &s.Name, &s.Slug, &s.HasAvatar, &s.IsVerified, &s.Similarity); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// List — для админки и индекса (?q= поиск, ?claimed=true).
type ListParams struct {
	Query        string
	OnlyClaimed  bool
	OnlyVerified bool
	Limit        int
	Offset       int
}

func (r *Repository) List(ctx context.Context, p ListParams) ([]Artist, error) {
	if p.Limit <= 0 || p.Limit > 200 {
		p.Limit = 50
	}
	args := []any{}
	where := []string{"1=1"}
	if p.Query != "" {
		args = append(args, "%"+p.Query+"%")
		where = append(where, "a.name ILIKE $"+intToStr(len(args)))
	}
	if p.OnlyClaimed {
		where = append(where, "a.claimed_by_user_id IS NOT NULL")
	}
	if p.OnlyVerified {
		where = append(where, "a.is_verified = TRUE")
	}
	args = append(args, p.Limit, p.Offset)
	limI := intToStr(len(args) - 1)
	offI := intToStr(len(args))

	q := `
		SELECT a.id, a.name, COALESCE(a.slug,''), COALESCE(a.description,''),
		       (a.avatar_path IS NOT NULL),
		       (a.banner_path IS NOT NULL),
		       a.is_verified,
		       a.claimed_by_user_id,
		       COALESCE(u.username,''),
		       (SELECT COUNT(*) FROM tracks t
		         WHERE t.artist_id = a.id
		           AND t.is_published = TRUE
		           AND t.moderation_status = 'approved')::BIGINT,
		       a.created_at
		  FROM artists a
		  LEFT JOIN users u ON u.id = a.claimed_by_user_id
		 WHERE ` + strings.Join(where, " AND ") + `
		 ORDER BY a.name ASC
		 LIMIT $` + limI + ` OFFSET $` + offI

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Artist
	for rows.Next() {
		var ar Artist
		if err := rows.Scan(
			&ar.ID, &ar.Name, &ar.Slug, &ar.Description,
			&ar.HasAvatar, &ar.HasBanner,
			&ar.IsVerified,
			&ar.ClaimedByUserID,
			&ar.ClaimedByName,
			&ar.TracksCount,
			&ar.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, ar)
	}
	if out == nil {
		out = []Artist{}
	}
	return out, rows.Err()
}

// IsClaimedBy возвращает (claimed_by, ok). Используется в track-upload
// для определения, нужна ли модерация.
func (r *Repository) ClaimedBy(ctx context.Context, artistID int64) (*int64, error) {
	var cid *int64
	err := r.pool.QueryRow(ctx,
		`SELECT claimed_by_user_id FROM artists WHERE id = $1`, artistID,
	).Scan(&cid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return cid, nil
}

// ─── helpers ───────────────────────────────────────────────────────────

func intToStr(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	bp := len(buf)
	for i > 0 {
		bp--
		buf[bp] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		bp--
		buf[bp] = '-'
	}
	return string(buf[bp:])
}

func isUniqueViolation(err error, constraintName string) bool {
	// pgx-специфичный — но смотрим без импорта pgconn чтобы пакет был лёгкий.
	s := err.Error()
	return strings.Contains(s, "unique constraint") &&
		(constraintName == "" || strings.Contains(s, constraintName))
}
