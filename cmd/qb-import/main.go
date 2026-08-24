package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/wood-bison/fluent-question-brain/internal/ingest"
)

func main() {
	file := flag.String("file", "", "path to one vault markdown card")
	root := flag.String("root", "", "vault root; imports the four canonical card directories")
	manifest := flag.String("manifest", "", "path to a text file with one card path per line (# comments allowed); batch mode")
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "Postgres connection URL")
	workspaceKey := flag.String("workspace-key", "fluent-interview", "stable workspace key")
	workspaceName := flag.String("workspace-name", "Fluent Interview", "workspace display name")
	dryRun := flag.Bool("dry-run", false, "parse and report without changing Postgres")
	reportPath := flag.String("report", "", "write a machine-readable JSON report to this path")
	strictTaxonomy := flag.Bool("strict-taxonomy", false, "reject cards whose legacy Topic is not in the controlled taxonomy registry")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	files := nonEmpty(*file)
	files = append(files, flag.Args()...)
	if *manifest != "" {
		entries, err := readManifest(*manifest)
		if err != nil {
			logger.Error("read manifest", "error", err)
			os.Exit(2)
		}
		files = append(files, entries...)
	}
	if len(files) == 0 && *root == "" {
		logger.Error("file, manifest, positional paths, or root is required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	report, err := ingest.Run(ctx, ingest.Options{
		Root: *root, Files: files, DatabaseURL: *databaseURL,
		WorkspaceKey: *workspaceKey, WorkspaceName: *workspaceName,
		DryRun: *dryRun, ReportPath: *reportPath, StrictTaxonomy: *strictTaxonomy,
	})
	if err != nil {
		logger.Error("import run failed", "error", err)
		os.Exit(1)
	}
	logger.Info("import run complete", "run_id", report.RunID, "source_root", report.SourceRoot, "dry_run", report.DryRun, "totals", report.Totals, "archived", report.Archived)
}

// readManifest loads one card path per line; blank lines and #-comments are
// ignored so a manifest can live in version control next to the batch.
func readManifest(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, line)
	}
	return entries, nil
}

func nonEmpty(file string) []string {
	if file == "" {
		return nil
	}
	return []string{file}
}
