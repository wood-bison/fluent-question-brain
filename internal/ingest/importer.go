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
	"github.com/wood-bison/fluent-question-brain/internal/quality"
	"github.com/wood-bison/fluent-question-brain/internal/store"
	"github.com/wood-bison/fluent-question-brain/internal/taxonomy"
)

const SourceSystem = "fluent-question-vault"

var cardDirectories = map[string]bool{
	"Question Cards":      true,
	"Concept Cards":       true,
	"Best Practice Cards": true,
	"Behavioral Cards":    true,
}

type Options struct {
	Root               string
	Files              []string
	DatabaseURL        string
	WorkspaceKey       string
	WorkspaceName      string
	DryRun             bool
	ReportPath         string
	StrictTaxonomy     bool
	StrictTaskBoundary bool
}

type Item struct {
	SourceRef   string   `json:"source_ref"`
	StableKey   string   `json:"stable_key,omitempty"`
	ContentHash string   `json:"content_hash,omitempty"`
	Action      string   `json:"action"`
	QuestionID  string   `json:"question_id,omitempty"`
	Error       string   `json:"error,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

type Report struct {
	SourceSystem      string         `json:"source_system"`
	SourceRoot        string         `json:"source_root"`
	WorkspaceKey      string         `json:"workspace_key"`
	DryRun            bool           `json:"dry_run"`
	StartedAt         time.Time      `json:"started_at"`
	FinishedAt        time.Time      `json:"finished_at"`
	RunID             string         `json:"run_id,omitempty"`
	Totals            map[string]int `json:"totals"`
	Archived          int64          `json:"archived"`
	Items             []Item         `json:"items"`
	UnrecognizedFiles []string       `json:"unrecognized_files,omitempty"`
}

func Run(ctx context.Context, options Options) (Report, error) {
	started := time.Now().UTC()
	files, root, mode, unrecognized, err := resolveFiles(options)
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
	if len(unrecognized) > 0 {
		// Markdown outside the four canonical directories used to disappear
		// silently under --root; report it instead of quietly skipping.
		report.UnrecognizedFiles = unrecognized
		report.Totals["unrecognized_files"] = len(unrecognized)
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
		if issues := quality.CardIssues(card); len(issues) > 0 {
			item.Action = "invalid"
			item.Error = formatQualityIssues(issues)
			report.Totals[item.Action]++
			report.Items = append(report.Items, item)
			if db != nil {
				if err := db.RecordImportItem(ctx, store.ImportItem{RunID: report.RunID, SourceRef: item.SourceRef, StableKey: item.StableKey, ContentHash: item.ContentHash, Action: item.Action, Error: item.Error}); err != nil {
					return finishWithError(ctx, db, report, err)
				}
			}
			continue
		}
		if options.StrictTaskBoundary {
			if issues := quality.TaskBoundaryIssues(card); len(issues) > 0 {
				item.Action = "invalid"
				item.Error = formatQualityIssues(issues)
				report.Totals[item.Action]++
				report.Items = append(report.Items, item)
				if db != nil {
					if err := db.RecordImportItem(ctx, store.ImportItem{RunID: report.RunID, SourceRef: item.SourceRef, StableKey: item.StableKey, ContentHash: item.ContentHash, Action: item.Action, Error: item.Error}); err != nil {
						return finishWithError(ctx, db, report, err)
					}
				}
				continue
			}
		}
		// Empty taxonomy used to pass silently, leaving a card with no place
		// in the topic tree (QB-BUG-6). Surface it as an explicit warning.
		if strings.TrimSpace(card.Track) == "" {
			item.Warnings = append(item.Warnings, "missing Track metadata: card will have no track bucket")
		}
		if strings.TrimSpace(card.Topic) == "" {
			item.Warnings = append(item.Warnings, "missing Topic metadata: card will stay unplaced in the topic tree")
		} else if canonicalTopic, ok := taxonomy.CanonicalTopicTitle(card.Topic); !ok {
			warning := fmt.Sprintf("Topic %q is outside the controlled legacy taxonomy registry", card.Topic)
			item.Warnings = append(item.Warnings, warning)
			if options.StrictTaxonomy {
				item.Action = "invalid"
				item.Error = warning
				report.Totals[item.Action]++
				report.Items = append(report.Items, item)
				if db != nil {
					if err := db.RecordImportItem(ctx, store.ImportItem{RunID: report.RunID, SourceRef: item.SourceRef, StableKey: item.StableKey, ContentHash: item.ContentHash, Action: item.Action, Error: item.Error}); err != nil {
						return finishWithError(ctx, db, report, err)
					}
				}
				continue
			}
		} else if canonicalTopic != card.Topic {
			item.Warnings = append(item.Warnings, fmt.Sprintf("Topic alias resolves to canonical legacy topic %q", canonicalTopic))
		}
		if strings.TrimSpace(card.Level) == "" {
			item.Warnings = append(item.Warnings, "missing Level metadata")
		}
		if len(item.Warnings) > 0 {
			report.Totals["warnings"]++
		}
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

func formatQualityIssues(issues []quality.PromptIssue) string {
	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		parts = append(parts, issue.Code+": "+issue.Message)
	}
	return "content quality gate: " + strings.Join(parts, "; ")
}

func finishWithError(ctx context.Context, db *store.Postgres, report Report, cause error) (Report, error) {
	_ = db.FinishImportRun(ctx, report.RunID, "failed", report.Totals)
	return report, cause
}

func resolveFiles(options Options) ([]string, string, string, []string, error) {
	if len(options.Files) > 0 {
		files := make([]string, 0, len(options.Files))
		for _, raw := range options.Files {
			file, err := filepath.Abs(raw)
			if err != nil {
				return nil, "", "", nil, err
			}
			files = append(files, file)
		}
		sort.Strings(files)
		return files, filepath.Dir(files[0]), "single_file", nil, nil
	}
	if options.Root == "" {
		return nil, "", "", nil, fmt.Errorf("root or file is required")
	}
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return nil, "", "", nil, err
	}
	files, unrecognized, err := collectFiles(root)
	if err != nil {
		return nil, "", "", nil, err
	}
	return files, root, "reconcile", unrecognized, nil
}

func collectFiles(root string) ([]string, []string, error) {
	var files, unrecognized []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) == 0 || !cardDirectories[parts[0]] {
			base := filepath.Base(path)
			isContentCard := strings.EqualFold(filepath.Ext(path), ".md") &&
				!strings.HasPrefix(base, "_") &&
				!strings.EqualFold(base, "readme.md")
			if isContentCard {
				unrecognized = append(unrecognized, rel)
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			unrecognized = append(unrecognized, rel)
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("scan vault: %w", err)
	}
	sort.Strings(files)
	sort.Strings(unrecognized)
	return files, unrecognized, nil
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
