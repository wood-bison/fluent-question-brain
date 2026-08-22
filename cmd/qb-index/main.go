package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/wood-bison/fluent-question-brain/internal/embedding"
	"github.com/wood-bison/fluent-question-brain/internal/store"
)

func main() {
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "Postgres connection URL")
	profile := flag.String("profile", embedding.ProfileKey, "embedding profile")
	batchSize := flag.Int("batch-size", 50, "outbox batch size")
	once := flag.Bool("once", false, "process one batch and exit")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if *databaseURL == "" {
		logger.Error("database-url is required")
		os.Exit(2)
	}
	if *profile != embedding.ProfileKey {
		logger.Error("unsupported local profile", "profile", *profile, "supported", embedding.ProfileKey)
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	db, err := store.Open(ctx, *databaseURL)
	if err != nil {
		logger.Error("open database failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	provider := embedding.HashProvider{}
	var processed, failed, vectors int
	for {
		items, claimErr := db.ClaimOutbox(ctx, *batchSize)
		if claimErr != nil {
			logger.Error("claim outbox failed", "error", claimErr)
			os.Exit(1)
		}
		if len(items) == 0 {
			break
		}
		for _, item := range items {
			locales, localeErr := db.RevisionLocales(ctx, item.AggregateID)
			if localeErr != nil {
				failed++
				_ = db.MarkOutboxFailed(ctx, item.ID, localeErr.Error(), item.Attempts)
				continue
			}
			itemFailed := false
			for _, locale := range locales {
				vector := embedding.VectorLiteral(provider.Embed(locale.Text))
				if err := db.UpsertEmbedding(ctx, locale.ID, *profile, locale.ContentHash, vector); err != nil {
					itemFailed = true
					failed++
					_ = db.MarkOutboxFailed(ctx, item.ID, err.Error(), item.Attempts)
					break
				}
				vectors++
			}
			if itemFailed {
				continue
			}
			if err := db.MarkOutboxPublished(ctx, item.ID); err != nil {
				failed++
				continue
			}
			processed++
		}
		if *once {
			break
		}
	}
	logger.Info("embedding outbox drained", "profile", *profile, "events_processed", processed, "vectors_written", vectors, "events_failed", failed)
}
