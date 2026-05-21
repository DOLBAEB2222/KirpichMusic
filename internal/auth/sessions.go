package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Длина случайного session-token (256 бит — больше, чем нужно даже NIST'у).
const sessionTokenBytes = 32

// SessionStore — серверные сессии:
//   • в куку отдаётся "сырой" base64url-токен (HttpOnly, Secure в проде, SameSite=Strict)
//   • в БД хранится только SHA-256(token)
// Если БД утечёт, все украденные хеши бесполезны для входа.
type SessionStore struct {
	pool         *pgxpool.Pool
	cookieName   string
	cookieDomain string
	cookieSecure bool
	ttl          time.Duration
}

type Session struct {
	ID        string
	UserID    int64
	ExpiresAt time.Time
}

func NewSessionStore(pool *pgxpool.Pool, cookieName, cookieDomain string, secure bool, ttl time.Duration) *SessionStore {
	return &SessionStore{
		pool:         pool,
		cookieName:   cookieName,
		cookieDomain: cookieDomain,
		cookieSecure: secure,
		ttl:          ttl,
	}
}

// Create — генерирует токен, пишет SHA-256 в БД, возвращает raw-токен и время истечения.
func (s *SessionStore) Create(ctx context.Context, userID int64, userAgent, ip string) (string, time.Time, error) {
	raw := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	expires := time.Now().Add(s.ttl)

	if len(userAgent) > 1024 {
		userAgent = userAgent[:1024]
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (user_id, token_hash, user_agent, ip_inet, expires_at)
		VALUES ($1, $2, $3, NULLIF($4, '')::inet, $5)
	`, userID, hash[:], userAgent, ip, expires)
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

// Lookup — найти активную сессию по raw-токену.
func (s *SessionStore) Lookup(ctx context.Context, token string) (*Session, error) {
	hash := sha256.Sum256([]byte(token))
	var sess Session
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, user_id, expires_at
		FROM sessions
		WHERE token_hash = $1
		  AND revoked_at IS NULL
		  AND expires_at > now()
	`, hash[:]).Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

// Revoke — отозвать сессию по raw-токену.
func (s *SessionStore) Revoke(ctx context.Context, token string) error {
	hash := sha256.Sum256([]byte(token))
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, hash[:])
	return err
}

// SetCookie — выставляет безопасную куку.
func (s *SessionStore) SetCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName,
		Value:    token,
		Path:     "/",
		Domain:   s.cookieDomain,
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,                    // JS не имеет доступа → защита от XSS-кражи
		Secure:   s.cookieSecure,          // только HTTPS в проде
		SameSite: http.SameSiteStrictMode, // защита от CSRF
	})
}

// ClearCookie — стереть куку.
func (s *SessionStore) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName,
		Value:    "",
		Path:     "/",
		Domain:   s.cookieDomain,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

// Cookie — извлечь токен из куки запроса.
func (s *SessionStore) Cookie(r *http.Request) (string, error) {
	c, err := r.Cookie(s.cookieName)
	if err != nil {
		return "", err
	}
	return c.Value, nil
}
