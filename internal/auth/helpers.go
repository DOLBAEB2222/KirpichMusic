package auth

import (
	"errors"
	"net"
	"net/http"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// validEmail — стандартный mail.ParseAddress + требование точки в домене.
func validEmail(s string) bool {
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return false
	}
	parts := strings.Split(addr.Address, "@")
	if len(parts) != 2 {
		return false
	}
	if !strings.Contains(parts[1], ".") {
		return false
	}
	return true
}

// validUsername — те же правила, что в DB-CHECK (длина 2..32, разрешённые символы).
func validUsername(s string) bool {
	n := len(s)
	if n < 2 || n > 32 {
		return false
	}
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' || r == '.' || r == '-' || r == ' '
		if !ok {
			return false
		}
	}
	return true
}

// isUniqueViolation — пойман ли SQLSTATE 23505.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// isCheckViolation — пойман ли SQLSTATE 23514.
func isCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}

// clientIP — IP без порта. Доверять X-Forwarded-For имеет смысл только за reverse-proxy
// со сконфигурированным TrustedProxies (в chi это middleware.RealIP).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}
	return host
}
