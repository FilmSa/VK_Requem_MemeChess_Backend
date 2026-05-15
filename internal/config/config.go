package config

import (
	"os"
	"strings"
)

type Config struct {
	HTTPPort         string
	JWTSecret        string
	PostgresDSN      string
	FrontendJoinBase string // e.g. http://localhost:5173; used to build /play/{game_id} links
	UploadsDir       string
}

func Load() Config {
	return Config{
		HTTPPort:         firstEnv("PORT", "HTTP_PORT", "8080"),
		JWTSecret:        getEnv("JWT_SECRET", "super-secret-dev-key"),
		PostgresDSN:      getEnv("POSTGRES_DSN", "postgres://memechess:memechess@localhost:5432/meme_chess?sslmode=disable"),
		FrontendJoinBase: strings.TrimSuffix(getEnv("FRONTEND_JOIN_BASE", "http://localhost:5173"), "/"),
		UploadsDir:       strings.TrimSpace(getEnv("UPLOADS_DIR", "uploads")),
	}
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
func firstEnv(keys ...string) string {
	lastIndex := len(keys) - 1
	if lastIndex < 0 {
		return ""
	}

	fallback := keys[lastIndex]
	for _, key := range keys[:lastIndex] {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}

	return fallback
}
