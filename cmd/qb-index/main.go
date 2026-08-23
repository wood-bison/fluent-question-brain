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
	profile := flag.String("profile", "semantic-v1", "embedding profile key to write vectors under")
	endpoint := flag.String("embedding-endpoint", os.Getenv("EMBEDDING_PROVIDER_ENDPOINT"), "embedding provider endpoint (e.g. local Ollama); empty selects the deterministic hash provider")
	model := flag.String("embedding-model", valueOrEnv("EMBEDDING_MODEL", "bge-m3"), "embedding model name at the provider")
	batchSize := flag.Int("batch-size", 50, "outbox batch size")
	embedBatchSize := flag.Int("embed-batch-size", 32, "texts per provider round trip while backfilling")
	once := flag.Bool("once", false, "process one outbox batch and exit")
	backfill := flag.Bool("backfill", false, "embed every current-revision locale missing a vector under the target profile, then continue draining")
	interval := flag.Duration("interval", 10*time.Second, "poll interval between batches in service mode")
	healthcheck := flag.Bool("healthcheck", false, "verify database connectivity and exit")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if *databaseURL == "" {
		logger.Error("database-url is required")
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

	var provider embedding.Provider
	if *endpoint == "" {
		provider = embedding.HashProvider{}
		logger.Info("using deterministic hash embedding provider", "profile", *profile)
	} else {
		provider = embedding.NewOllamaProvider(*endpoint, *model)
		logger.Info("using ollama embedding provider", "profile", *profile, "endpoint", *endpoint, "model", *model)
	}
	if err := ensureProfile(ctx, db, *profile); err != nil {
		logger.Error("embedding profile check failed", "profile", *profile, "error", err)
		os.Exit(2)
	}

	if *backfill {
		if err := runBackfill(ctx, db, provider, *profile, *embedBatchSize, logger); err != nil {
			logger.Error("backfill failed", "error", err)
			os.Exit(1)
		}
	}

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

// ensureProfile fails fast when the requested profile is unknown to the
// database instead of letting every vector insert die on the foreign key.
func ensureProfile(ctx context.Context, db *store.Postgres, profileKey string) error {
	active, err := db.EmbeddingProfileActive(ctx, profileKey)
	if err != nil {
		return fmt.Errorf("read embedding profile %q: %w", profileKey, err)
	}
	if !active {
		return fmt.Errorf("embedding profile %q is not active in content.embedding_profile", profileKey)
	}
	return nil
}

// runBackfill walks every current-revision locale without a vector under the
// target profile and embeds it. Existing vectors under other profiles are
// never touched; profile_key keeps generations side by side for rollback.
func runBackfill(ctx context.Context, db *store.Postgres, provider embedding.Provider, profileKey string, batchSize int, logger *slog.Logger) error {
	locales, err := db.LocalesMissingEmbedding(ctx, profileKey)
	if err != nil {
		return fmt.Errorf("list locales missing embeddings: %w", err)
	}
	if len(locales) == 0 {
		logger.Info("backfill complete: nothing missing", "profile", profileKey)
		return nil
	}
	logger.Info("backfill starting", "profile", profileKey, "locales", len(locales))
	if batchSize <= 0 {
		batchSize = 32
	}
	var written, failed int
	for start := 0; start < len(locales); start += batchSize {
		if ctx.Err() != nil {
			return fmt.Errorf("backfill interrupted after %d written, %d failed", written, failed)
		}
		end := start + batchSize
		if end > len(locales) {
			end = len(locales)
		}
		batch := locales[start:end]
		texts := make([]string, len(batch))
		for i, locale := range batch {
			texts[i] = locale.Text
		}
		vectors, embedErr := provider.EmbedBatch(ctx, texts)
		if embedErr != nil {
			failed += len(batch)
			logger.Error("backfill batch embed failed", "size", len(batch), "error", embedErr)
			continue
		}
		for i, locale := range batch {
			if err := db.UpsertEmbedding(ctx, locale.ID, profileKey, locale.ContentHash, embedding.VectorLiteral(vectors[i])); err != nil {
				failed++
				logger.Error("backfill upsert failed", "locale_id", locale.ID, "error", err)
				continue
			}
			written++
		}
	}
	logger.Info("backfill complete", "profile", profileKey, "vectors_written", written, "vectors_failed", failed)
	if failed > 0 {
		return fmt.Errorf("backfill finished with %d failed vectors", failed)
	}
	return nil
}

// processBatch claims and processes up to batchSize events. It returns the
// number of claimed items plus per-outcome counters. An empty claim means the
// queue is drained; a non-nil error means claiming itself failed.
func processBatch(ctx context.Context, db *store.Postgres, provider embedding.Provider, profile string, batchSize int, logger *slog.Logger) (int, int, int, int, int, error) {
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
			failMessage := fmt.Sprintf("cannot resolve revision for event type %q (aggregate_type %q)", item.EventType, item.AggregateType)
			_ = db.MarkOutboxFailed(ctx, item.ID, failMessage, item.Attempts)
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
			vector, embedErr := provider.Embed(ctx, locale.Text)
			if embedErr != nil {
				itemFailed = true
				failed++
				_ = db.MarkOutboxFailed(ctx, item.ID, embedErr.Error(), item.Attempts)
				break
			}
			if err := db.UpsertEmbedding(ctx, locale.ID, profile, locale.ContentHash, embedding.VectorLiteral(vector)); err != nil {
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

func valueOrEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
