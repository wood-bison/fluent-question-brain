// Command qb-map-release is the explicit Question Brain ↔ Fluent Lab
// curriculum mapping release boundary. It defaults to a dry-run; --approve
// is required to write revision-scoped mapping rows. --unmapped-current is a
// safe baseline that records every current production revision without
// guessing any Path, Domain, or Capability from legacy labels.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/wood-bison/fluent-question-brain/internal/mapping"
	"github.com/wood-bison/fluent-question-brain/internal/store"
)

func main() {
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "Postgres connection URL")
	workspaceKey := flag.String("workspace-key", "fluent-interview", "stable workspace key")
	manifestPath := flag.String("manifest", "", "complete JSON curriculum mapping manifest")
	unmappedCurrent := flag.Bool("unmapped-current", false, "record an explicit unmapped row for every current production revision")
	actor := flag.String("actor", "question-brain-curriculum-mapping-release", "audit actor")
	approve := flag.Bool("approve", false, "materialize the validated mapping; without this flag the command is dry-run only")
	reportPath := flag.String("report", "", "optional JSON report path")
	flag.Parse()

	if *databaseURL == "" {
		fail("-database-url or DATABASE_URL is required")
	}
	if (*manifestPath == "" && !*unmappedCurrent) || (*manifestPath != "" && *unmappedCurrent) {
		fail("choose exactly one of -manifest or -unmapped-current")
	}

	var manifest *mapping.Manifest
	if *manifestPath != "" {
		data, err := os.ReadFile(*manifestPath)
		if err != nil {
			fail("read mapping manifest: %v", err)
		}
		decoded, err := mapping.Decode(data)
		if err != nil {
			fail("decode mapping manifest: %v", err)
		}
		manifest = &decoded
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	db, err := store.Open(ctx, *databaseURL)
	if err != nil {
		fail("open database: %v", err)
	}
	defer db.Close()
	report, err := db.ReleaseCurriculumMapping(ctx, store.CurriculumMappingReleaseRequest{
		WorkspaceKey:    *workspaceKey,
		Manifest:        manifest,
		UnmappedCurrent: *unmappedCurrent,
		Actor:           *actor,
		Approve:         *approve,
	})
	if err != nil {
		fail("curriculum mapping release failed: %v", err)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fail("encode curriculum mapping report: %v", err)
	}
	if *reportPath != "" {
		if err := os.WriteFile(*reportPath, append(encoded, '\n'), 0o644); err != nil {
			fail("write curriculum mapping report: %v", err)
		}
	}
	fmt.Println(string(encoded))
	if report.Blocked {
		os.Exit(1)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
