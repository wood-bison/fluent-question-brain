package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wood-bison/fluent-question-brain/internal/normalize"
	"github.com/wood-bison/fluent-question-brain/internal/store"
)

const SourceSystem = "fluent-question-vault"

var cardDirectories = map[string]bool{
	"Question Cards":      true,
	"Concept Cards":       true,
	"Best Practice Cards": true,
	"Behavioral Cards":    true,
}

type Options struct {
	Root          string
	Files         []string
	DatabaseURL   string
	WorkspaceKey  string
	WorkspaceName string
	DryRun        bool
	ReportPath    string
}

type Item struct {
	SourceRef   string `json:"source_ref"`
	StableKey   string `json:"stable_key,omitempty"`
	ContentHash string `json:"content_hash,omitempty"`
	Action      string `json:"action"`
	QuestionID  string `json:"question_id,omitempty"`
	Error       string `json:"error,omitempty"`
}

type Report struct {
	SourceSystem string         `json:"source_system"`
	SourceRoot   string         `json:"source_root"`
	WorkspaceKey string         `json:"workspace_key"`
	DryRun       bool           `json:"dry_run"`
	StartedAt    time.Time      `json:"started_at"`
	FinishedAt   time.Time      `json:"finished_at"`
	RunID        string         `json:"run_id,omitempty"`
	Totals       map[string]int `json:"totals"`
	Archived     int64          `json:"archived"`
	Items        []Item         `json:"items"`
}

func Run(ctx context.Context, options Options) (Report, error) {
	started := time.Now().UTC()
	files, root, mode, err := resolveFiles(options)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		SourceSystem: SourceSystem,
		SourceRoot:   root,
		WorkspaceKey: options.WorkspaceKey,
		DryRun:       options.DryRun,
		StartedAt:    started,
		Totals:       map[string]int{"files": len(files)},
		Items:        make([]Item, 0, len(files)),
	}
	if options.WorkspaceKey == "" {
		options.WorkspaceKey = "fluent-interview"
	}
	if options.WorkspaceName == "" {
		options.WorkspaceName = "Fluent Interview"
	}

	var db *store.Postgres
	if !options.DryRun {
		if options.DatabaseURL == "" {
			return Report{}, fmt.Errorf("database-url is required unless --dry-run is set")
		}
		db, err = store.Open(ctx, options.DatabaseURL)
		if err != nil {
			return Report{}, err
		}
		defer db.Close()
		report.RunID, err = db.StartImportRun(ctx, options.WorkspaceKey, options.WorkspaceName, SourceSystem, root, mode)
		if err != nil {
			return Report{}, err
		}
	}

	for _, file := range files {
		item := Item{SourceRef: file}
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			item.Action = "invalid"
			item.Error = readErr.Error()
			report.Items = append(report.Items, item)
			report.Totals[item.Action]++
			if db != nil {
				if err := db.RecordImportItem(ctx, store.ImportItem{RunID: report.RunID, SourceRef: item.SourceRef, Action: item.Action, Error: item.Error}); err != nil {
					return finishWithError(ctx, db, report, err)
				}
			}
			continue
		}
		card, parseErr := normalize.ParseMarkdown(file, content)
		if parseErr != nil {
			item.Action = "invalid"
			item.Error = parseErr.Error()
			report.Items = append(report.Items, item)
			report.Totals[item.Action]++
			if db != nil {
				if err := db.RecordImportItem(ctx, store.ImportItem{RunID: report.RunID, SourceRef: item.SourceRef, Action: item.Action, Error: item.Error}); err != nil {
					return finishWithError(ctx, db, report, err)
				}
			}
			continue
		}
		item.StableKey = card.StableKey
		item.ContentHash = card.Hash
		if options.DryRun {
			item.Action = "would_create"
		} else {
			stored, upsertErr := db.UpsertCard(ctx, card, options.WorkspaceKey, options.WorkspaceName)
			if upsertErr != nil {
				item.Action = "invalid"
				item.Error = upsertErr.Error()
				report.Totals[item.Action]++
				report.Items = append(report.Items, item)
				if err := db.RecordImportItem(ctx, store.ImportItem{RunID: report.RunID, SourceRef: item.SourceRef, StableKey: item.StableKey, ContentHash: item.ContentHash, Action: item.Action, Error: item.Error}); err != nil {
					return finishWithError(ctx, db, report, err)
				}
				continue
			}
			item.Action = stored.Action
			item.QuestionID = stored.QuestionID
		}
		report.Items = append(report.Items, item)
		report.Totals[item.Action]++
		if db != nil {
			if err := db.RecordImportItem(ctx, store.ImportItem{RunID: report.RunID, SourceRef: item.SourceRef, StableKey: item.StableKey, ContentHash: item.ContentHash, Action: item.Action, QuestionID: item.QuestionID}); err != nil {
				return finishWithError(ctx, db, report, err)
			}
		}
	}

	if db != nil && !options.DryRun && mode == "reconcile" {
		report.Archived, err = db.ArchiveMissingSourceQuestions(ctx, options.WorkspaceKey, SourceSystem, root, report.RunID)
		if err != nil {
			return finishWithError(ctx, db, report, err)
		}
	}
	report.FinishedAt = time.Now().UTC()
	if db != nil {
		if err := db.FinishImportRun(ctx, report.RunID, "succeeded", report.Totals); err != nil {
			return Report{}, err
		}
	}
	if options.ReportPath != "" {
		if err := writeReport(options.ReportPath, report); err != nil {
			return Report{}, err
		}
	}
	return report, nil
}

func finishWithError(ctx context.Context, db *store.Postgres, report Report, cause error) (Report, error) {
	_ = db.FinishImportRun(ctx, report.RunID, "failed", report.Totals)
	return report, cause
}

func resolveFiles(options Options) ([]string, string, string, error) {
	if len(options.Files) > 0 {
		files := make([]string, 0, len(options.Files))
		for _, raw := range options.Files {
			file, err := filepath.Abs(raw)
			if err != nil {
				return nil, "", "", err
			}
			files = append(files, file)
		}
		sort.Strings(files)
		return files, filepath.Dir(files[0]), "single_file", nil
	}
	if options.Root == "" {
		return nil, "", "", fmt.Errorf("root or file is required")
	}
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return nil, "", "", err
	}
	files, err := collectFiles(root)
	if err != nil {
		return nil, "", "", err
	}
	return files, root, "reconcile", nil
}

func collectFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) == 0 || !cardDirectories[parts[0]] {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan vault: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func writeReport(path string, report Report) error {
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode import report: %w", err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		return fmt.Errorf("write import report: %w", err)
	}
	return nil
}
