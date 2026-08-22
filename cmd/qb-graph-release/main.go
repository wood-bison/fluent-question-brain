// Command qb-graph-release is the explicit graph publication boundary. It
// defaults to a dry-run; --approve is required to materialize accepted source
// placements into content.question_topic.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/wood-bison/fluent-question-brain/internal/store"
)

func main() {
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "Postgres connection URL")
	workspaceKey := flag.String("workspace-key", "fluent-interview", "stable workspace key")
	actor := flag.String("actor", "question-brain-graph-release", "audit actor")
	approve := flag.Bool("approve", false, "materialize the validated graph; without this flag the command is dry-run only")
	reportPath := flag.String("report", "", "optional JSON report path")
	flag.Parse()

	if *databaseURL == "" {
		fail("-database-url or DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := store.Open(ctx, *databaseURL)
	if err != nil {
		fail("open database: %v", err)
	}
	defer db.Close()
	report, err := db.ReleaseGraph(ctx, store.GraphPlacementRequest{
		WorkspaceKey: *workspaceKey,
		Actor:        *actor,
		Approve:      *approve,
	})
	if err != nil {
		fail("graph release failed: %v", err)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fail("encode graph release report: %v", err)
	}
	if *reportPath != "" {
		if err := os.WriteFile(*reportPath, append(encoded, '\n'), 0o644); err != nil {
			fail("write graph release report: %v", err)
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
