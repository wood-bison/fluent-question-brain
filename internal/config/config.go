package config

import (
	"fmt"
	"net/url"
	"os"
)

type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	EmbeddingProfile string
	OTELEndpoint     string
}

func FromEnv() (Config, error) {
	c := Config{
		HTTPAddr:         valueOr("HTTP_ADDR", ":8080"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		EmbeddingProfile: valueOr("EMBEDDING_PROFILE", "semantic-v1"),
		OTELEndpoint:     os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	}
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	parsed, err := url.Parse(c.DatabaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return Config{}, fmt.Errorf("DATABASE_URL must be a valid URL")
	}
	return c, nil
}

func valueOr(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
