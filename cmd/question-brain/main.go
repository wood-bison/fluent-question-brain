package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wood-bison/fluent-question-brain/internal/config"
	"github.com/wood-bison/fluent-question-brain/internal/embedding"
	"github.com/wood-bison/fluent-question-brain/internal/httpapi"
	"github.com/wood-bison/fluent-question-brain/internal/store"
	"github.com/wood-bison/fluent-question-brain/internal/telemetry"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	cfg, err := config.FromEnv()
	if err != nil {
		logger.Error("configuration rejected", "error", err)
		os.Exit(2)
	}
	if len(os.Args) == 2 && os.Args[1] == "--healthcheck" {
		if err := databaseReachable(cfg.DatabaseURL); err != nil {
			logger.Error("database healthcheck failed", "error", err)
			os.Exit(1)
		}
		return
	}
	shutdownTelemetry, err := telemetry.Init(context.Background(), "question-brain-api", cfg.OTELEndpoint)
	if err != nil {
		logger.Error("telemetry initialization failed", "error", err, "endpoint", cfg.OTELEndpoint)
		os.Exit(2)
	}
	defer func() {
		if err := telemetry.Shutdown(context.Background(), shutdownTelemetry); err != nil {
			logger.Error("telemetry shutdown failed", "error", err)
		}
	}()
	database, err := store.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(2)
	}
	defer database.Close()
	if cfg.EmbeddingProviderEndpoint != "" {
		database.UseEmbedding(
			embedding.NewOllamaProvider(cfg.EmbeddingProviderEndpoint, cfg.EmbeddingModel),
			cfg.EmbeddingProfile,
		)
	} else {
		database.UseEmbedding(embedding.HashProvider{}, embedding.ProfileKey)
	}
	logger.Info("embedding provider selected",
		"profile", cfg.EmbeddingProfile,
		"endpoint_configured", cfg.EmbeddingProviderEndpoint != "",
		"model", cfg.EmbeddingModel)

	apiHandler := httpapi.New(cfg.DatabaseURL, database).Handler()
	rootHandler := http.NewServeMux()
	rootHandler.Handle("/", apiHandler)
	rootHandler.Handle("GET /metrics", telemetry.MetricsHandler())

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           telemetry.HTTP(rootHandler),
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown failed", "error", err)
		}
	}()

	logger.Info("question brain api starting", "addr", cfg.HTTPAddr, "embedding_profile", cfg.EmbeddingProfile, "otel_endpoint", cfg.OTELEndpoint, "metrics_path", "/metrics", "stage", "G5")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func databaseReachable(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return errors.New("invalid database URL")
	}
	port := parsed.Port()
	if port == "" {
		port = "5432"
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(parsed.Hostname(), port), 700*time.Millisecond)
	if err != nil {
		return err
	}
	return conn.Close()
}
