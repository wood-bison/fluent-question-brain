// Command qb-capability-release validates, generates, and publishes the
// reviewed Question -> Capability release. It is dry-run by default.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/wood-bison/fluent-question-brain/internal/capabilitybinding"
	"github.com/wood-bison/fluent-question-brain/internal/store"
)

func main() {
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "Postgres connection URL")
	workspaceKey := flag.String("workspace-key", "fluent-interview", "stable workspace key")
	manifestPath := flag.String("manifest", "", "complete JSON capability binding manifest")
	generatePath := flag.String("generate", "", "write a complete review manifest generated from explicit current mappings")
	rollbackID := flag.String("rollback-release", "", "restore a previous binding release by ID")
	registryRelease := flag.String("registry-release", "capability-registry-2026-08-24-v2", "pinned capability registry release")
	actor := flag.String("actor", "question-brain-capability-binding-release", "audit actor")
	source := flag.String("source", "question-brain-g7-reviewed-disposition-2026-08-25", "source for generated review manifest")
	approve := flag.Bool("approve", false, "materialize the validated release; without this flag the command is dry-run")
	reportPath := flag.String("report", "", "optional JSON report path")
	flag.Parse()

	if *databaseURL == "" {
		fail("-database-url or DATABASE_URL is required")
	}
	if *rollbackID != "" {
		if *manifestPath != "" || *generatePath != "" {
			fail("-rollback-release cannot be combined with -manifest or -generate")
		}
	} else if *manifestPath == "" && *generatePath == "" {
		fail("one of -manifest, -generate, or -rollback-release is required")
	}
	if *manifestPath != "" && *generatePath != "" {
		fail("-manifest and -generate cannot be combined")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	db, err := store.Open(ctx, *databaseURL)
	if err != nil {
		fail("open database: %v", err)
	}
	defer db.Close()
	if *rollbackID != "" {
		rollback, err := db.RollbackCapabilityBindings(ctx, *workspaceKey, *rollbackID, *actor, *approve)
		if err != nil {
			fail("capability binding rollback failed: %v", err)
		}
		encoded, err := json.MarshalIndent(rollback, "", "  ")
		if err != nil {
			fail("encode capability rollback report: %v", err)
		}
		fmt.Println(string(encoded))
		if rollback.Blocked {
			os.Exit(1)
		}
		return
	}

	var manifest capabilitybinding.Manifest
	if *generatePath != "" {
		manifest, err = db.GenerateCapabilityBindingManifest(ctx, *workspaceKey, *registryRelease, *source)
		if err != nil {
			fail("generate capability review manifest: %v", err)
		}
		encoded, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			fail("encode generated manifest: %v", err)
		}
		if err := os.WriteFile(*generatePath, append(encoded, '\n'), 0o644); err != nil {
			fail("write generated manifest: %v", err)
		}
		fmt.Printf("generated capability review manifest: %s (%d entries)\n", *generatePath, len(manifest.Entries))
		return
	}
	data, err := os.ReadFile(*manifestPath)
	if err != nil {
		fail("read capability manifest: %v", err)
	}
	manifest, err = capabilitybinding.Decode(data)
	if err != nil {
		fail("decode capability manifest: %v", err)
	}
	report, err := db.ReleaseCapabilityBindings(ctx, store.CapabilityBindingReleaseRequest{
		WorkspaceKey: *workspaceKey, Manifest: manifest, Actor: *actor, Approve: *approve,
	})
	if err != nil {
		fail("capability binding release failed: %v", err)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fail("encode capability release report: %v", err)
	}
	if *reportPath != "" {
		if err := os.WriteFile(*reportPath, append(encoded, '\n'), 0o644); err != nil {
			fail("write capability release report: %v", err)
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
