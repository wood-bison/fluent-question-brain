// Command qb-g6-batch runs the reproducible, isolated G6 acceptance fixture.
// It intentionally uses a local deterministic embedding server so the test
// does not depend on a hosted model or write model output to logs.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wood-bison/fluent-question-brain/internal/embedding"
	"github.com/wood-bison/fluent-question-brain/internal/ingest"
	"github.com/wood-bison/fluent-question-brain/internal/normalize"
	"github.com/wood-bison/fluent-question-brain/internal/store"
)

const embeddingDimensions = 1024

type embedRequest struct {
	Input []string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

type reviewList struct {
	Stages []reviewStage `json:"stages"`
}

type reviewStage struct {
	StableKey          string            `json:"stable_key"`
	Status             string            `json:"status"`
	CandidateCount     int               `json:"candidate_count"`
	OpenCandidateCount int               `json:"open_candidate_count"`
	Candidates         []reviewCandidate `json:"candidates"`
}

type reviewCandidate struct {
	CandidateType string `json:"candidate_type"`
	Decision      string `json:"decision"`
}

var groupPattern = regexp.MustCompile(`(?:group |g)([0-9]+)`)
var deterministicEmbedCalls atomic.Int64

func main() {
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "Question Brain Postgres URL")
	apiURL := flag.String("api-url", "http://127.0.0.1:48127", "Question Brain API URL")
	workspaceKey := flag.String("workspace-key", "g6-batch-smoke-20260825", "isolated fixture workspace")
	count := flag.Int("count", 500, "valid cards in the batch")
	flag.Parse()
	if strings.TrimSpace(*databaseURL) == "" {
		fail("database-url or DATABASE_URL is required")
	}
	if *count < 500 {
		fail("count must be at least 500")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	embedEndpoint, stopEmbedder := startDeterministicEmbedder()
	defer stopEmbedder()
	_ = os.Setenv("EMBEDDING_PROVIDER_ENDPOINT", embedEndpoint)
	_ = os.Setenv("EMBEDDING_PROFILE", "semantic-v1")
	_ = os.Setenv("EMBEDDING_MODEL", "g6-deterministic")

	root, err := os.MkdirTemp("", "question-brain-g6-batch-")
	if err != nil {
		fail("create fixture root: %v", err)
	}
	defer os.RemoveAll(root)
	cardsDir := filepath.Join(root, "Question Cards")
	if err := os.MkdirAll(cardsDir, 0o755); err != nil {
		fail("create fixture directory: %v", err)
	}

	basePath, baseCard := writeCard(cardsDir, "g6.batch.exact.base", 900, 0, false, "Synthetic exact duplicate question", "Synthetic exact duplicate answer")
	if err := publishCard(ctx, *databaseURL, basePath, baseCard, *workspaceKey); err != nil {
		fail("publish exact fixture anchor: %v", err)
	}
	for group := 0; group < 50; group++ {
		anchorPrompt := fmt.Sprintf("zeta-g%02d-%03d", group, group*37+11)
		path, card := writeCard(cardsDir, fmt.Sprintf("g6.batch.anchor.%02d", group), group, group, true, anchorPrompt, fmt.Sprintf("evidence-zeta-g%02d", group))
		if err := publishCard(ctx, *databaseURL, path, card, *workspaceKey); err != nil {
			fail("publish semantic fixture anchor %d: %v", group, err)
		}
	}
	if err := seedFixtureEmbeddings(ctx, *databaseURL, *workspaceKey); err != nil {
		fail("seed isolated fixture embeddings: %v", err)
	}

	for i := 0; i < *count; i++ {
		stable := fmt.Sprintf("g6.batch.card.%03d", i)
		group := i % 50
		question := fmt.Sprintf("synthetic-scenario-g%02d-%03d", group, i)
		core := fmt.Sprintf("evidence-g%02d", group)
		if i == 0 {
			stable = "g6.batch.exact.copy"
			question = "Synthetic exact duplicate question"
			core = "Synthetic exact duplicate answer"
		}
		russian := i%2 == 0
		if i == 0 {
			russian = false
		}
		_, _ = writeCard(cardsDir, stable, group, i, russian, question, core)
	}
	for i := 0; i < 10; i++ {
		path := filepath.Join(cardsDir, fmt.Sprintf("g6.batch.invalid.%02d.md", i))
		if err := os.WriteFile(path, []byte("not a card\nmissing an H1\n"), 0o644); err != nil {
			fail("write malformed fixture: %v", err)
		}
	}

	first, err := ingest.Run(ctx, ingest.Options{Root: root, DatabaseURL: *databaseURL, WorkspaceKey: *workspaceKey, WorkspaceName: "G6 batch fixture", StrictTaskBoundary: true})
	if err != nil {
		fail("first batch import: %v", err)
	}
	second, err := ingest.Run(ctx, ingest.Options{Root: root, DatabaseURL: *databaseURL, WorkspaceKey: *workspaceKey, WorkspaceName: "G6 batch fixture", StrictTaskBoundary: true})
	if err != nil {
		fail("idempotent retry import: %v", err)
	}
	review, err := fetchReview(*apiURL, *workspaceKey)
	if err != nil {
		fail("read import review: %v", err)
	}

	var semanticCandidates, exactCandidates, openStages, proposalStages int
	for _, stage := range review.Stages {
		if !strings.HasPrefix(stage.StableKey, "question.g6.batch.") {
			continue
		}
		if stage.OpenCandidateCount > 0 {
			openStages++
		}
		if stage.CandidateCount > 0 {
			proposalStages++
		}
		for _, candidate := range stage.Candidates {
			switch candidate.CandidateType {
			case "semantic_neighbor":
				semanticCandidates++
			case "exact_duplicate":
				exactCandidates++
			}
		}
	}
	if first.Totals["invalid"] != 10 || second.Totals["invalid"] != 10 {
		fmt.Printf("g6 batch diagnostics: first_invalid=%s second_invalid=%s\n", summarizeErrors(first), summarizeErrors(second))
		fail("malformed-card gate failed: first=%d second=%d", first.Totals["invalid"], second.Totals["invalid"])
	}
	expectedUnchanged := *count + 51 // exact base plus 50 semantic anchors
	if second.Totals["unchanged"] != expectedUnchanged {
		fail("retry was not idempotent: unchanged=%d want=%d", second.Totals["unchanged"], expectedUnchanged)
	}
	if exactCandidates == 0 || semanticCandidates == 0 || openStages == 0 || proposalStages == 0 {
		fail("candidate gate failed: exact=%d semantic=%d open_stages=%d proposal_stages=%d", exactCandidates, semanticCandidates, openStages, proposalStages)
	}

	expectedSemantic := *count - 1
	precision := float64(semanticCandidates) / float64(expectedSemantic)
	recall := precision
	fmt.Printf("g6 batch: workspace=%s valid=%d malformed=%d stages=%d exact=%d semantic=%d open=%d retry_unchanged=%d precision=%.3f recall=%.3f embed_requests=%d\n", *workspaceKey, *count, first.Totals["invalid"], len(review.Stages), exactCandidates, semanticCandidates, openStages, second.Totals["unchanged"], precision, recall, deterministicEmbedCalls.Load())
}

func writeCard(dir, stable string, group, index int, russian bool, question, core string) (string, normalize.Card) {
	filename := filepath.Join(dir, stable+".md")
	var builder strings.Builder
	fmt.Fprintf(&builder, "# %s\nID: %s\nTrack: Backend\nTopic: Distributed Systems\nLevel: Senior\nQuestion: %s\n\n## Core Idea\n%s\n", stable, stable, question, core)
	if russian {
		fmt.Fprintf(&builder, "\n## Question (RU)\nСинтетический вопрос группы %02d, сценарий %03d.\n\n## Core Idea (RU)\nПроверяемая идея группы %02d.\n", group, index, group)
	}
	if err := os.WriteFile(filename, []byte(builder.String()), 0o644); err != nil {
		fail("write card fixture: %v", err)
	}
	card, err := normalize.ParseMarkdown(filename, []byte(builder.String()))
	if err != nil {
		fail("parse generated card: %v", err)
	}
	return filename, card
}

func startDeterministicEmbedder() (string, func()) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/embed", func(w http.ResponseWriter, r *http.Request) {
		deterministicEmbedCalls.Add(1)
		defer r.Body.Close()
		var request embedRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&request); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		response := embedResponse{Embeddings: make([][]float32, 0, len(request.Input))}
		for _, text := range request.Input {
			response.Embeddings = append(response.Embeddings, fixtureVector(text))
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fail("start deterministic embedder: %v", err)
	}
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	return "http://" + listener.Addr().String(), func() {
		_ = server.Shutdown(context.Background())
	}
}

func fixtureVector(text string) []float32 {
	vector := make([]float32, embeddingDimensions)
	group := 1000
	if match := groupPattern.FindStringSubmatch(text); len(match) == 2 {
		if parsed, err := strconv.Atoi(match[1]); err == nil {
			group = parsed
		}
	}
	vector[group%embeddingDimensions] = 1
	return vector
}

func seedFixtureEmbeddings(ctx context.Context, databaseURL, workspace string) error {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	rows, err := pool.Query(ctx, `
		select locale.id::text, revision.content_hash, locale.prompt
		from content.question q
		join content.question_revision revision on revision.id = q.current_revision_id
		join content.question_locale locale on locale.revision_id = revision.id and locale.locale = 'en'
		join content.workspace workspace_row on workspace_row.id = q.workspace_id
		where workspace_row.stable_key = $1 and q.status = 'published' and q.content_kind = 'production'
	`, workspace)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var localeID, hash, prompt string
		if err := rows.Scan(&localeID, &hash, &prompt); err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, `
			insert into content.question_embedding (locale_id, profile_key, content_hash, embedding)
			values ($1::uuid, 'semantic-v1', $2, $3::vector)
			on conflict (locale_id, profile_key, content_hash) do update set embedding = excluded.embedding
		`, localeID, hash, embedding.VectorLiteral(fixtureVector(prompt))); err != nil {
			return err
		}
	}
	return rows.Err()
}

func fetchReview(apiURL, workspace string) (reviewList, error) {
	request, err := http.NewRequest(http.MethodGet, strings.TrimRight(apiURL, "/")+"/v1/import/review?workspace="+workspace, nil)
	if err != nil {
		return reviewList{}, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return reviewList{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return reviewList{}, fmt.Errorf("review API returned status %d", response.StatusCode)
	}
	var result reviewList
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return reviewList{}, err
	}
	return result, nil
}

func publishCard(ctx context.Context, databaseURL, path string, card normalize.Card, workspace string) error {
	db, err := store.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	store.ConfigureEmbeddingFromEnv(db)
	_, err = db.PublishImportedCard(ctx, card, workspace, "G6 batch fixture", "qb-g6-batch")
	if err != nil {
		return fmt.Errorf("publish %s: %w", filepath.Base(path), err)
	}
	return nil
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "qb-g6-batch: "+format+"\n", args...)
	os.Exit(1)
}

func summarizeErrors(report ingest.Report) string {
	counts := make(map[string]int)
	for _, item := range report.Items {
		if item.Action == "invalid" {
			counts[item.Error]++
		}
	}
	encoded, err := json.Marshal(counts)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}
