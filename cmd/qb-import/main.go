package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/wood-bison/fluent-question-brain/internal/normalize"
	"github.com/wood-bison/fluent-question-brain/internal/store"
)

func main() {
	file := flag.String("file", "", "path to one vault markdown card")
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "Postgres connection URL")
	workspaceKey := flag.String("workspace-key", "fluent-interview", "stable workspace key")
	workspaceName := flag.String("workspace-name", "Fluent Interview", "workspace display name")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if *file == "" || *databaseURL == "" {
		logger.Error("file and database-url are required")
		os.Exit(2)
	}

	content, err := os.ReadFile(*file)
	if err != nil {
		logger.Error("read card failed", "error", err, "file", *file)
		os.Exit(1)
	}
	card, err := normalize.ParseMarkdown(*file, content)
	if err != nil {
		logger.Error("normalize card failed", "error", err, "file", *file)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := store.Open(ctx, *databaseURL)
	if err != nil {
		logger.Error("open database failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	stored, err := db.UpsertCard(ctx, card, *workspaceKey, *workspaceName)
	if err != nil {
		logger.Error("import card failed", "error", err, "stable_key", card.StableKey)
		os.Exit(1)
	}

	canonical, err := normalize.CanonicalJSON(stored.Payload)
	if err != nil {
		logger.Error("canonicalize exported payload failed", "error", err)
		os.Exit(1)
	}
	exportedHash := normalize.HashCanonicalJSON(canonical)
	if exportedHash != card.Hash || stored.Hash != card.Hash {
		logger.Error("round-trip hash mismatch", "source_hash", card.Hash, "stored_hash", stored.Hash, "exported_hash", exportedHash)
		os.Exit(1)
	}

	fmt.Printf("imported stable_key=%s revision_id=%s content_hash=%s round_trip=ok\n", card.StableKey, stored.RevisionID, card.Hash)
}
