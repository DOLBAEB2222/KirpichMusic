package auth

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bogdan/kirpichmusic/internal/httpx"
)

type ctxKey int

const (
	userIDKey  ctxKey = 1
	isAdminKey ctxKey = 2
)

// UserIDFromContext — достаёт user_id из контекста запроса
// (выставлено RequireAuth middleware'ом).
func UserIDFromContext(ctx context.Context) int64 {
	v, _ := ctx.Value(userIDKey).(int64)
	return v
}

// IsAdminFromContext — true если в контексте есть отметка "админ" (выставляется
// RequireAdmin или OptionalAuth при наличии cookie).
func IsAdminFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(isAdminKey).(bool)
	return v
}

func contextWithUserID(ctx context.Context, uid int64) context.Context {
	return context.WithValue(ctx, userIDKey, uid)
}

func contextWithAdmin(ctx context.Context, admin bool) context.Context {
	return context.WithValue(ctx, isAdminKey, admin)
}

// RequireAuth — middleware: проверяет cookie, кладёт user_id в context.
// Если сессия отсутствует/просрочена/отозвана — 401.
func RequireAuth(sess *SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, err := sess.Cookie(r)
			if err != nil || tok == "" {
				httpx.WriteUnauthorized(w)
				return
			}
			s, err := sess.Lookup(r.Context(), tok)
			if err != nil {
				httpx.WriteUnauthorized(w)
				return
			}
			ctx := contextWithUserID(r.Context(), s.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin — middleware поверх RequireAuth: дополнительно проверяет
// is_admin = TRUE в БД. Возвращает 403 "Forbidden" если юзер не админ.
// Использует pool, чтобы избежать stale-флага из контекста сессии.
func RequireAdmin(sess *SessionStore, pool *pgxpool.Pool) func(http.Handler) http.Handler {
	requireAuth := RequireAuth(sess)
	return func(next http.Handler) http.Handler {
		return requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uid := UserIDFromContext(r.Context())
			if uid == 0 {
				httpx.WriteUnauthorized(w)
				return
			}
			var isAdmin bool
			err := pool.QueryRow(r.Context(),
				`SELECT is_admin FROM users WHERE id=$1 AND is_active=TRUE`,
				uid,
			).Scan(&isAdmin)
			if err != nil || !isAdmin {
				httpx.WriteError(w, http.StatusForbidden, "Доступ запрещён", "forbidden")
				return
			}
			next.ServeHTTP(w, r.WithContext(contextWithAdmin(r.Context(), true)))
		}))
	}
}

// OptionalAuth — кладёт user_id и is_admin в context если cookie валидна;
// иначе пропускает запрос дальше как анонимный (без 401).
// Нужен для публичных эндпоинтов, которые хотят знать "лайкнул ли текущий
// юзер этот трек" / "подписан ли" / "может ли редактировать".
//
// pool используется чтобы один раз за запрос подгрузить is_admin (та же
// query что в RequireAdmin), тогда нижестоящие хендлеры могут вызвать
// IsAdminFromContext без дополнительного хождения в БД.
func OptionalAuth(sess *SessionStore, pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, err := sess.Cookie(r)
			if err == nil && tok != "" {
				if s, lerr := sess.Lookup(r.Context(), tok); lerr == nil {
					ctx := contextWithUserID(r.Context(), s.UserID)
					if pool != nil {
						var isAdmin bool
						_ = pool.QueryRow(ctx,
							`SELECT is_admin FROM users WHERE id=$1 AND is_active=TRUE`,
							s.UserID,
						).Scan(&isAdmin)
						if isAdmin {
							ctx = contextWithAdmin(ctx, true)
						}
					}
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
