package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/wood-bison/fluent-question-brain/internal/store"
)

func main() {
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "Postgres connection URL")
	workspaceKey := flag.String("workspace-key", "fluent-interview", "stable workspace key")
	left := flag.String("left", "", "left question stable key")
	right := flag.String("right", "", "right question stable key")
	decision := flag.String("decision", "not_duplicate", "open, keep_separate, merge, or not_duplicate")
	exact := flag.Float64("exact-score", 0, "normalized exact/trigram similarity")
	semantic := flag.Float64("semantic-score", 0, "semantic similarity")
	actor := flag.String("actor", "g1-audit", "audit actor")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if *databaseURL == "" || *left == "" || *right == "" {
		logger.Error("database-url, left, and right are required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := store.Open(ctx, *databaseURL)
	if err != nil {
		logger.Error("open database failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.RecordDuplicateDecision(ctx, store.DuplicateDecision{
		WorkspaceKey:   *workspaceKey,
		LeftStableKey:  *left,
		RightStableKey: *right,
		ExactScore:     *exact,
		SemanticScore:  *semantic,
		Decision:       *decision,
		Actor:          *actor,
	}); err != nil {
		logger.Error("record duplicate decision failed", "error", err)
		os.Exit(1)
	}
	logger.Info("duplicate decision recorded", "left", *left, "right", *right, "decision", *decision)
}
