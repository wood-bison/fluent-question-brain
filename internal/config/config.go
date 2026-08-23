package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	EmbeddingProfile string
	// EmbeddingProviderEndpoint and EmbeddingModel select the real embedding
	// backend (local Ollama, e.g. bge-m3). When the endpoint is empty the API
	// falls back to the deterministic hash provider and hash profile so tests
	// and offline pipelines keep working without network access.
	EmbeddingProviderEndpoint string
	EmbeddingModel            string
	// SearchMinRankScore and SearchMinSemanticScore implement the relevance
	// cutoff (QB-BUG-3): a candidate survives only when the fused rank score
	// reaches SearchMinRankScore (two stages agreeing) or its semantic score
	// alone reaches SearchMinSemanticScore (one strong semantic match).
	SearchMinRankScore     float64
	SearchMinSemanticScore float64
	OTELEndpoint           string
}

func FromEnv() (Config, error) {
	c := Config{
		HTTPAddr:         valueOr("HTTP_ADDR", ":8080"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		EmbeddingProfile: valueOr("EMBEDDING_PROFILE", "semantic-v1"),
		EmbeddingProviderEndpoint: os.Getenv("EMBEDDING_PROVIDER_ENDPOINT"),
		EmbeddingModel:            valueOr("EMBEDDING_MODEL", "bge-m3"),
		OTELEndpoint:              os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	}
	var err error
	if c.SearchMinRankScore, err = floatEnv("SEARCH_MIN_RANK_SCORE", 0.02); err != nil {
		return Config{}, err
	}
	if c.SearchMinSemanticScore, err = floatEnv("SEARCH_MIN_SEMANTIC_SCORE", 0.505); err != nil {
		return Config{}, err
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

func floatEnv(key string, defaultValue float64) (float64, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number: %w", key, err)
	}
	return value, nil
}

func valueOr(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
