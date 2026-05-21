package httpx

import "net/http"

// SecurityHeaders — лёгкий security-middleware. Проставляет:
//   X-Content-Type-Options: nosniff   — нельзя интерпретировать text/plain как JS
//   X-Frame-Options:        DENY      — нельзя iframe'ить (защита от clickjacking)
//   Referrer-Policy:        same-origin
//   Permissions-Policy:     отключает камеры/микрофоны и т.д.
//   Strict-Transport-Security в prod (если HTTPS).
//
// CSP не выставляем здесь — фронтовый index.html использует inline-скрипты
// и onclick-обработчики, поэтому потребует 'unsafe-inline' для script-src,
// что обесценивает CSP. Лучше оставить дефолт и навешивать CSP отдельно
// после рефакторинга фронта (вынос JS в отдельный файл, делегирование событий).
func SecurityHeaders(secureProd bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "same-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			if secureProd {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}
