package ingest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const unknownTopicCard = `# T001 — Unknown topic
ID: T001
Track: Backend
Topic: Node / Not In Registry
Question: Explain it.

## Core Idea

Answer.
`

func TestStrictTaxonomyRejectsUnknownTopicBeforeWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "T001.md")
	if err := os.WriteFile(path, []byte(unknownTopicCard), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	report, err := Run(context.Background(), Options{Files: []string{path}, DryRun: true, StrictTaxonomy: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Totals["invalid"] != 1 || report.Items[0].Action != "invalid" {
		t.Fatalf("strict report = %#v", report)
	}
	if report.Items[0].Error == "" || len(report.Items[0].Warnings) == 0 {
		t.Fatalf("strict taxonomy evidence missing = %#v", report.Items[0])
	}
}

func TestDefaultTaxonomyModeWarnsAndKeepsUnknownTopic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "T001.md")
	if err := os.WriteFile(path, []byte(unknownTopicCard), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	report, err := Run(context.Background(), Options{Files: []string{path}, DryRun: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Totals["would_create"] != 1 || report.Items[0].Action != "would_create" {
		t.Fatalf("warning-only report = %#v", report)
	}
	if len(report.Items[0].Warnings) == 0 || report.Items[0].Error != "" {
		t.Fatalf("warning-only taxonomy evidence = %#v", report.Items[0])
	}
}

func TestStrictTaskBoundaryRejectsEmbeddedSolution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "T002.md")
	card := `# T002 — Runtime task
ID: T002
Track: Backend
Topic: Node / Event Loop & Scheduling
Task-Contract-Version: question-brain.task-brief.v1
Task-Kind: runtime_task_reference
Task-Family-Key: task-family.rate-limiter
Question: Implement it.

## Task

Implement the brief.

## Solution

This belongs in Task Runtime.
`
	if err := os.WriteFile(path, []byte(card), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	report, err := Run(context.Background(), Options{Files: []string{path}, DryRun: true, StrictTaskBoundary: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Totals["invalid"] != 1 || report.Items[0].Action != "invalid" || report.Items[0].Error == "" {
		t.Fatalf("strict task boundary evidence missing = %#v", report)
	}
}
