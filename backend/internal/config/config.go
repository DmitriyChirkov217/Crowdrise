package config

import (
	"os"
	"strings"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	JWTSecret   string
	CORSOrigin  string
}

func Load() Config {
	return Config{
		HTTPAddr:    httpAddr(),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://crowdrise:crowdrise@localhost:5432/crowdrise?sslmode=disable"),
		JWTSecret:   getEnv("JWT_SECRET", "dev-secret-change-me"),
		CORSOrigin:  getEnv("CORS_ORIGIN", "http://localhost:5173"),
	}
}

func httpAddr() string {
	if value := os.Getenv("HTTP_ADDR"); value != "" {
		return normalizeAddr(value)
	}
	if value := os.Getenv("PORT"); value != "" {
		return normalizeAddr(value)
	}
	return ":8080"
}

func normalizeAddr(value string) string {
	if strings.Contains(value, ":") {
		return value
	}
	return ":" + value
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
