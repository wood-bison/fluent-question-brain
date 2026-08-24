// Command qb-calibrate measures the configured duplicate thresholds against a
// reviewed identifier-only calibration set. It never prints prompt text.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type calibrationSet struct {
	ContractVersion     string            `json:"contract_version"`
	CalibrationRevision string            `json:"calibration_revision"`
	ProfileKey          string            `json:"profile_key"`
	Cases               []calibrationCase `json:"cases"`
}

type calibrationCase struct {
	ID    string `json:"id"`
	Left  string `json:"left"`
	Right string `json:"right"`
	Label string `json:"label"`
}

type calibrationReport struct {
	ContractVersion     string  `json:"contract_version"`
	CalibrationRevision string  `json:"calibration_revision"`
	ProfileKey          string  `json:"profile_key"`
	Cases               int     `json:"cases"`
	Evaluated           int     `json:"evaluated"`
	Skipped             int     `json:"skipped"`
	TruePositive        int     `json:"true_positive"`
	FalsePositive       int     `json:"false_positive"`
	FalseNegative       int     `json:"false_negative"`
	TrueNegative        int     `json:"true_negative"`
	Precision           float64 `json:"precision"`
	Recall              float64 `json:"recall"`
	LexicalThreshold    float64 `json:"lexical_threshold"`
	SemanticThreshold   float64 `json:"semantic_threshold"`
}

func main() {
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "Question Brain Postgres URL")
	calibrationPath := flag.String("calibration", "docs/verification/G6-calibration-set-2026-08-25.json", "calibration JSON path")
	workspace := flag.String("workspace-key", "fluent-interview", "workspace stable key")
	flag.Parse()
	if strings.TrimSpace(*databaseURL) == "" {
		fail("database-url or DATABASE_URL is required")
	}
	data, err := os.ReadFile(*calibrationPath)
	if err != nil {
		fail("read calibration: %v", err)
	}
	var set calibrationSet
	if err := json.Unmarshal(data, &set); err != nil {
		fail("decode calibration: %v", err)
	}
	if set.ContractVersion != "question-brain.import-review-calibration.v1" || len(set.Cases) == 0 {
		fail("invalid calibration contract")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, *databaseURL)
	if err != nil {
		fail("open database: %v", err)
	}
	defer pool.Close()
	report := calibrationReport{ContractVersion: set.ContractVersion, CalibrationRevision: set.CalibrationRevision, ProfileKey: set.ProfileKey, Cases: len(set.Cases)}
	if err := pool.QueryRow(ctx, `
		select lexical_threshold::double precision, semantic_threshold::double precision
		from content.duplicate_profile_config
		where profile_key = $1
	`, set.ProfileKey).Scan(&report.LexicalThreshold, &report.SemanticThreshold); err != nil {
		fail("read duplicate profile: %v", err)
	}
	for _, item := range set.Cases {
		var exact bool
		var lexical, semantic float64
		err := pool.QueryRow(ctx, `
			with left_card as (
				select revision.id, revision.normalized_payload, locale.prompt,
				       embedding.embedding
				from content.question question
				join content.workspace workspace on workspace.id = question.workspace_id
				join content.question_revision revision on revision.id = question.current_revision_id
				join content.question_locale locale on locale.revision_id = revision.id and locale.locale = 'en'
				left join content.question_embedding embedding on embedding.locale_id = locale.id and embedding.profile_key = $3
				where workspace.stable_key = $1 and question.stable_key = $2 and question.status = 'published' and question.content_kind = 'production'
			), right_card as (
				select revision.id, revision.normalized_payload, locale.prompt,
				       embedding.embedding
				from content.question question
				join content.workspace workspace on workspace.id = question.workspace_id
				join content.question_revision revision on revision.id = question.current_revision_id
				join content.question_locale locale on locale.revision_id = revision.id and locale.locale = 'en'
				left join content.question_embedding embedding on embedding.locale_id = locale.id and embedding.profile_key = $3
				where workspace.stable_key = $1 and question.stable_key = $4 and question.status = 'published' and question.content_kind = 'production'
			)
			select (left_card.normalized_payload - 'stable_key' - 'slug' - 'title') = (right_card.normalized_payload - 'stable_key' - 'slug' - 'title'),
			       similarity(left_card.prompt, right_card.prompt)::double precision,
			       case when left_card.embedding is null or right_card.embedding is null then null else (1 - (left_card.embedding <=> right_card.embedding))::double precision end
			from left_card cross join right_card
		`, *workspace, item.Left, set.ProfileKey, item.Right).Scan(&exact, &lexical, &semantic)
		if err != nil {
			if err == pgx.ErrNoRows {
				report.Skipped++
				continue
			}
			fail("evaluate calibration case %s: %v", item.ID, err)
		}
		report.Evaluated++
		positive := item.Label == "exact_or_duplicate" || item.Label == "semantic_duplicate"
		predicted := exact || lexical >= report.LexicalThreshold || semantic >= report.SemanticThreshold
		switch {
		case positive && predicted:
			report.TruePositive++
		case positive && !predicted:
			report.FalseNegative++
		case !positive && predicted:
			report.FalsePositive++
		default:
			report.TrueNegative++
		}
	}
	if report.TruePositive+report.FalsePositive > 0 {
		report.Precision = float64(report.TruePositive) / float64(report.TruePositive+report.FalsePositive)
	}
	if report.TruePositive+report.FalseNegative > 0 {
		report.Recall = float64(report.TruePositive) / float64(report.TruePositive+report.FalseNegative)
	}
	encoded, _ := json.Marshal(report)
	fmt.Println(string(encoded))
	if report.Skipped > 0 {
		fail("calibration has %d skipped pairs", report.Skipped)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "qb-calibrate: "+format+"\n", args...)
	os.Exit(1)
}
