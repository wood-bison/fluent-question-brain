package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wood-bison/fluent-question-brain/internal/embedding"
	"github.com/wood-bison/fluent-question-brain/internal/store"
)

// skippedEventTypes lists outbox events that carry no embedding work. They are
// marked published so they never block the queue. Any unknown event type that
// also has no resolvable revision fails loudly instead of silently succeeding.
var skippedEventTypes = map[string]bool{
	"question.graph.released": true,
}

func main() {
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "Postgres connection URL")
	profile := flag.String("profile", embedding.ProfileKey, "embedding profile")
	batchSize := flag.Int("batch-size", 50, "outbox batch size")
	once := flag.Bool("once", false, "process one batch and exit")
	interval := flag.Duration("interval", 10*time.Second, "poll interval while the outbox is empty (service mode)")
	healthcheck := flag.Bool("healthcheck", false, "verify database connectivity and exit")
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
	if *healthcheck {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		db, err := store.Open(ctx, *databaseURL)
		if err != nil {
			logger.Error("database healthcheck failed", "error", err)
			os.Exit(1)
		}
		db.Close()
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := store.Open(ctx, *databaseURL)
	if err != nil {
		logger.Error("open database failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	provider := embedding.HashProvider{}
	var processed, skipped, failed, vectors int
	for {
		n, cycleProcessed, cycleSkipped, cycleFailed, cycleVectors, claimErr := processBatch(ctx, db, provider, *profile, *batchSize, logger)
		processed += cycleProcessed
		skipped += cycleSkipped
		failed += cycleFailed
		vectors += cycleVectors
		if claimErr != nil {
			logger.Error("claim outbox failed", "error", claimErr)
			os.Exit(1)
		}
		if n == 0 || *once {
			break
		}
	}
	logger.Info("embedding outbox drained",
		"profile", *profile,
		"events_processed", processed,
		"events_skipped", skipped,
		"events_failed", failed,
		"vectors_written", vectors)
	if *once {
		return
	}
	// Service mode: keep polling until SIGINT/SIGTERM.
	for {
		select {
		case <-ctx.Done():
			logger.Info("indexer shutting down",
				"events_processed", processed,
				"events_skipped", skipped,
				"events_failed", failed,
				"vectors_written", vectors)
			return
		case <-time.After(*interval):
		}
		n, cycleProcessed, cycleSkipped, cycleFailed, cycleVectors, claimErr := processBatch(ctx, db, provider, *profile, *batchSize, logger)
		processed += cycleProcessed
		skipped += cycleSkipped
		failed += cycleFailed
		vectors += cycleVectors
		if claimErr != nil {
			logger.Error("claim outbox failed", "error", claimErr)
			os.Exit(1)
		}
		if n > 0 {
			logger.Info("embedding outbox drained",
				"profile", *profile,
				"events_processed", processed,
				"events_skipped", skipped,
				"events_failed", failed,
				"vectors_written", vectors)
		}
	}
}

// processBatch claims and processes up to batchSize events. It returns the
// number of claimed items plus per-outcome counters. An empty claim means the
// queue is drained; a non-nil error means claiming itself failed.
func processBatch(ctx context.Context, db *store.Postgres, provider embedding.HashProvider, profile string, batchSize int, logger *slog.Logger) (int, int, int, int, int, error) {
	items, err := db.ClaimOutbox(ctx, batchSize)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	var processed, skipped, failed, vectors int
	for _, item := range items {
		revisionID, resolvable := resolveRevisionID(item)
		if !resolvable {
			if skippedEventTypes[item.EventType] {
				// Known event type with no embedding work: complete it so it
				// never blocks the queue, but record that it was skipped.
				if pubErr := db.MarkOutboxPublished(ctx, item.ID); pubErr != nil {
					failed++
					continue
				}
				skipped++
				logger.Info("skipped non-embedding outbox event", "event_id", item.ID, "event_type", item.EventType)
				continue
			}
			failErr := fmt.Sprintf("cannot resolve revision for event type %q (aggregate_type %q)", item.EventType, item.AggregateType)
			_ = db.MarkOutboxFailed(ctx, item.ID, failErr, item.Attempts)
			failed++
			continue
		}
		locales, localeErr := db.RevisionLocales(ctx, revisionID)
		if localeErr != nil {
			_ = db.MarkOutboxFailed(ctx, item.ID, localeErr.Error(), item.Attempts)
			failed++
			continue
		}
		if len(locales) == 0 {
			// Fail closed: an empty locale list used to be marked published
			// with zero vectors written, which silently starved the index.
			_ = db.MarkOutboxFailed(ctx, item.ID, fmt.Sprintf("no locales found for revision %s", revisionID), item.Attempts)
			failed++
			continue
		}
		itemFailed := false
		for _, locale := range locales {
			vector := embedding.VectorLiteral(provider.Embed(locale.Text))
			if err := db.UpsertEmbedding(ctx, locale.ID, profile, locale.ContentHash, vector); err != nil {
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
	return len(items), processed, skipped, failed, vectors, nil
}

// resolveRevisionID maps an outbox event to the question revision it indexes.
// Revision-scoped events carry the revision directly as their aggregate id;
// translation events aggregate on the question id and carry the pinned
// revision in the payload instead.
func resolveRevisionID(item store.OutboxItem) (string, bool) {
	if item.AggregateType == "question_revision" && item.AggregateID != "" {
		return item.AggregateID, true
	}
	var payload struct {
		RevisionID string `json:"revision_id"`
	}
	if len(item.Payload) > 0 && json.Unmarshal(item.Payload, &payload) == nil && payload.RevisionID != "" {
		return payload.RevisionID, true
	}
	return "", false
}
