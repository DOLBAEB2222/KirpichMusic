package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config — все настройки приложения. Загружается из env (12-factor).
type Config struct {
	Env            string        // dev | prod
	HTTPAddr       string        // ":8080"
	DatabaseURL    string        // postgres://user:pass@host:5432/db
	SessionTTL     time.Duration // время жизни сессии (по умолчанию 30 дней)
	SessionCookie  string        // имя куки
	CookieDomain   string        // domain куки (опц.)
	CookieSecure   bool          // в dev=false (нет HTTPS), в prod=true
	MediaRoot      string        // куда складывать аудио/обложки (для ШАГА 3)
	MaxUploadBytes int64         // лимит размера загрузки (для ШАГА 3)
	WebRoot        string        // папка со статикой фронтенда (index.html и т.д.)
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:           getEnv("ENV", "dev"),
		HTTPAddr:      getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		SessionCookie: getEnv("SESSION_COOKIE", "km_sid"),
		CookieDomain:  os.Getenv("COOKIE_DOMAIN"),
		MediaRoot:     getEnv("MEDIA_ROOT", "./media"),
		WebRoot:       getEnv("WEB_ROOT", "./web"),
	}
	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	ttl, err := time.ParseDuration(getEnv("SESSION_TTL", "720h"))
	if err != nil {
		return nil, fmt.Errorf("SESSION_TTL: %w", err)
	}
	cfg.SessionTTL = ttl

	mu, err := strconv.Atoi(getEnv("MAX_UPLOAD_MB", "100"))
	if err != nil {
		return nil, fmt.Errorf("MAX_UPLOAD_MB: %w", err)
	}
	cfg.MaxUploadBytes = int64(mu) * 1024 * 1024

	cfg.CookieSecure = cfg.Env != "dev"
	return cfg, nil
}

func getEnv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
