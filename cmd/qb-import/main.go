package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/wood-bison/fluent-question-brain/internal/ingest"
)

func main() {
	file := flag.String("file", "", "path to one vault markdown card")
	root := flag.String("root", "", "vault root; imports the four canonical card directories")
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "Postgres connection URL")
	workspaceKey := flag.String("workspace-key", "fluent-interview", "stable workspace key")
	workspaceName := flag.String("workspace-name", "Fluent Interview", "workspace display name")
	dryRun := flag.Bool("dry-run", false, "parse and report without changing Postgres")
	reportPath := flag.String("report", "", "write a machine-readable JSON report to this path")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if *file == "" && *root == "" {
		logger.Error("file or root is required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	report, err := ingest.Run(ctx, ingest.Options{
		Root: *root, Files: nonEmpty(*file), DatabaseURL: *databaseURL,
		WorkspaceKey: *workspaceKey, WorkspaceName: *workspaceName,
		DryRun: *dryRun, ReportPath: *reportPath,
	})
	if err != nil {
		logger.Error("import run failed", "error", err)
		os.Exit(1)
	}
	logger.Info("import run complete", "run_id", report.RunID, "source_root", report.SourceRoot, "dry_run", report.DryRun, "totals", report.Totals, "archived", report.Archived)
}

func nonEmpty(file string) []string {
	if file == "" {
		return nil
	}
	return []string{file}
}
