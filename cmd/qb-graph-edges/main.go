// Command qb-graph-edges is the reviewed Question Brain graph lifecycle
// boundary. It can propose, decide, release, export, and roll back graph
// edges. Release is a dry-run unless --approve is supplied.
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
	workspace := flag.String("workspace-key", "fluent-interview", "stable workspace key")
	actor := flag.String("actor", "question-brain-graph-operator", "audit actor")
	from := flag.String("from", "", "source question stable key for a proposal")
	to := flag.String("to", "", "target question stable key for a proposal")
	kind := flag.String("kind", "", "edge kind: prerequisite, related, contrast, follow_up, variant, duplicate, supersedes")
	confidence := flag.Float64("confidence", -1, "optional confidence in [0,1]; -1 leaves it unset")
	rationale := flag.String("rationale", "", "editorial rationale or decision note")
	source := flag.String("source", "question-brain-editorial", "proposal provenance")
	proposalID := flag.String("proposal-id", "", "proposal UUID for --decision")
	decision := flag.String("decision", "", "proposal decision: accepted, rejected, superseded")
	release := flag.Bool("release", false, "dry-run the accepted graph release")
	approve := flag.Bool("approve", false, "materialize the graph release (requires --release)")
	rollback := flag.String("rollback", "", "graph release ID to roll back")
	export := flag.String("export", "", "graph release ID to export as JSON")
	reportPath := flag.String("report", "", "optional JSON report path")
	flag.Parse()

	if *databaseURL == "" {
		fail("-database-url or DATABASE_URL is required")
	}
	if *approve && !*release {
		fail("--approve requires --release")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	db, err := store.Open(ctx, *databaseURL)
	if err != nil {
		fail("open database: %v", err)
	}
	defer db.Close()

	var output any
	switch {
	case *from != "" || *to != "" || *kind != "":
		if *from == "" || *to == "" || *kind == "" {
			fail("--from, --to, and --kind are required together")
		}
		var confidenceValue *float64
		if *confidence >= 0 {
			confidenceValue = confidence
		}
		output, err = db.CreateEdgeProposal(ctx, store.EdgeProposalRequest{
			WorkspaceKey: *workspace, FromStableKey: *from, ToStableKey: *to,
			Kind: *kind, Confidence: confidenceValue, Rationale: *rationale, Source: *source,
		}, *actor)
	case *decision != "":
		if *proposalID == "" {
			fail("--proposal-id is required with --decision")
		}
		output, err = db.DecideEdgeProposal(ctx, *proposalID, *decision, *actor, *rationale)
	case *release:
		output, err = db.ReleaseQuestionGraph(ctx, store.GraphReleaseRequest{
			WorkspaceKey: *workspace, Actor: *actor, Approve: *approve,
		})
	case *rollback != "":
		output, err = db.RollbackQuestionGraph(ctx, *rollback, *actor)
	case *export != "":
		output, err = db.GetGraphRelease(ctx, *export)
	default:
		fail("choose a proposal, decision, release, rollback, or export action")
	}
	if err != nil {
		fail("graph operation failed: %v", err)
	}
	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fail("encode graph output: %v", err)
	}
	if *reportPath != "" {
		if err := os.WriteFile(*reportPath, append(encoded, '\n'), 0o644); err != nil {
			fail("write graph report: %v", err)
		}
	}
	fmt.Println(string(encoded))
	if report, ok := output.(store.GraphReleaseReport); ok && report.Blocked {
		os.Exit(1)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
