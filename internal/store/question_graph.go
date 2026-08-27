package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wood-bison/fluent-question-brain/internal/search"
)

const QuestionGraphContractVersion = "question-brain.graph-edge.v1"

// GraphEdgeKindPolicy is the single semantic registry for question-to-question
// relations.  The registry is deliberately small and explicit: only
// prerequisite edges affect unlock order, while the other relations are
// explanatory navigation and never grant learner progress by themselves.
type GraphEdgeKindPolicy struct {
	Kind            string
	Description     string
	LearnerEffect   string
	RequiresAcyclic bool
}

var questionGraphEdgePolicies = map[string]GraphEdgeKindPolicy{
	"prerequisite": {
		Kind: "prerequisite", Description: "The target concept must be understood first.",
		LearnerEffect: "gates-recommendation", RequiresAcyclic: true,
	},
	"related": {
		Kind: "related", Description: "The target is useful adjacent context.",
		LearnerEffect: "context-only",
	},
	"contrast": {
		Kind: "contrast", Description: "The target is a meaningful alternative or boundary case.",
		LearnerEffect: "comparison-only",
	},
	"follow_up": {
		Kind: "follow_up", Description: "The target deepens the same interview concept.",
		LearnerEffect: "suggests-next",
	},
	"variant": {
		Kind: "variant", Description: "The target is another valid formulation or implementation variant.",
		LearnerEffect: "alternative-only",
	},
	"duplicate": {
		Kind: "duplicate", Description: "The target is materially equivalent and must not be double-counted.",
		LearnerEffect: "deduplicates",
	},
	"supersedes": {
		Kind: "supersedes", Description: "The target replaces an older card or formulation.",
		LearnerEffect: "historical-replacement",
	},
}

// QuestionGraphEdgeKindRegistry returns a deterministic copy for contract and
// audit tooling.  Callers cannot mutate the internal policy map.
func QuestionGraphEdgeKindRegistry() []GraphEdgeKindPolicy {
	result := make([]GraphEdgeKindPolicy, 0, len(questionGraphEdgePolicies))
	for _, policy := range questionGraphEdgePolicies {
		result = append(result, policy)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Kind < result[j].Kind })
	return result
}

type EdgeProposalRequest struct {
	WorkspaceKey  string   `json:"workspace_key"`
	FromStableKey string   `json:"from_stable_key"`
	ToStableKey   string   `json:"to_stable_key"`
	Kind          string   `json:"kind"`
	Confidence    *float64 `json:"confidence,omitempty"`
	Rationale     string   `json:"rationale"`
	Source        string   `json:"source"`
}

type EdgeProposal struct {
	ID             string     `json:"id"`
	WorkspaceKey   string     `json:"workspace_key"`
	FromStableKey  string     `json:"from_stable_key"`
	FromRevisionID string     `json:"from_revision_id"`
	ToStableKey    string     `json:"to_stable_key"`
	ToRevisionID   string     `json:"to_revision_id"`
	Kind           string     `json:"kind"`
	Status         string     `json:"status"`
	Confidence     *float64   `json:"confidence,omitempty"`
	Rationale      string     `json:"rationale,omitempty"`
	Source         string     `json:"source"`
	CreatedAt      time.Time  `json:"created_at"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
	DecidedBy      string     `json:"decided_by,omitempty"`
}

type EdgeDecisionRequest struct {
	Decision  string `json:"decision"`
	Rationale string `json:"rationale,omitempty"`
}

type GraphEdge struct {
	ProposalID     string   `json:"proposal_id,omitempty"`
	FromStableKey  string   `json:"from_stable_key"`
	FromRevisionID string   `json:"from_revision_id"`
	ToStableKey    string   `json:"to_stable_key"`
	ToRevisionID   string   `json:"to_revision_id"`
	Kind           string   `json:"kind"`
	Confidence     *float64 `json:"confidence,omitempty"`
	Rationale      string   `json:"rationale,omitempty"`
}

type GraphRelease struct {
	ContractVersion   string      `json:"contract_version"`
	GraphReleaseID    string      `json:"graph_release_id"`
	WorkspaceKey      string      `json:"workspace_key"`
	QuestionReleaseID string      `json:"question_release_id"`
	Status            string      `json:"status"`
	EdgeCount         int         `json:"edge_count"`
	SourceHash        string      `json:"source_hash"`
	Actor             string      `json:"actor"`
	CreatedAt         time.Time   `json:"created_at"`
	Edges             []GraphEdge `json:"edges,omitempty"`
}

type GraphReleaseRequest struct {
	WorkspaceKey string
	Actor        string
	Approve      bool
}

type GraphReleaseReport struct {
	ContractVersion   string    `json:"contract_version"`
	WorkspaceKey      string    `json:"workspace_key"`
	QuestionReleaseID string    `json:"question_release_id"`
	GraphReleaseID    string    `json:"graph_release_id"`
	GeneratedAt       time.Time `json:"generated_at"`
	Approved          bool      `json:"approved"`
	Blocked           bool      `json:"blocked"`
	Accepted          int       `json:"accepted"`
	Released          int       `json:"released"`
	Stale             int       `json:"stale"`
	Cycles            int       `json:"cycles"`
	InvalidTargets    int       `json:"invalid_targets"`
	ArchivedTargets   int       `json:"archived_targets"`
	EvidenceGaps      int       `json:"evidence_gaps"`
	TestProvenance    int       `json:"test_provenance"`
	BlockedReasons    []string  `json:"blocked_reasons,omitempty"`
}

type GraphNeighborhood struct {
	ContractVersion string       `json:"contract_version"`
	StableKey       string       `json:"stable_key"`
	GraphRelease    GraphRelease `json:"graph_release"`
	Edges           []GraphEdge  `json:"edges"`
}

func normalizeGraphRequest(request EdgeProposalRequest) (EdgeProposalRequest, error) {
	request.WorkspaceKey = strings.TrimSpace(request.WorkspaceKey)
	request.FromStableKey = strings.TrimSpace(request.FromStableKey)
	request.ToStableKey = strings.TrimSpace(request.ToStableKey)
	request.Kind = strings.TrimSpace(request.Kind)
	request.Rationale = strings.TrimSpace(request.Rationale)
	request.Source = strings.TrimSpace(request.Source)
	if request.WorkspaceKey == "" {
		request.WorkspaceKey = "fluent-interview"
	}
	if request.Source == "" {
		request.Source = "question-brain-editorial"
	}
	if request.FromStableKey == "" || request.ToStableKey == "" || request.FromStableKey == request.ToStableKey {
		return EdgeProposalRequest{}, fmt.Errorf("distinct from_stable_key and to_stable_key are required")
	}
	if _, ok := questionGraphEdgePolicies[request.Kind]; !ok {
		return EdgeProposalRequest{}, fmt.Errorf("unsupported graph edge kind %q", request.Kind)
	}
	if request.Confidence != nil && (*request.Confidence < 0 || *request.Confidence > 1) {
		return EdgeProposalRequest{}, fmt.Errorf("confidence must be between 0 and 1")
	}
	if err := validateGraphEvidence(request.Kind, request.Confidence, request.Rationale, request.Source); err != nil {
		return EdgeProposalRequest{}, err
	}
	return request, nil
}

func validateGraphEvidence(kind string, confidence *float64, rationale, source string) error {
	if _, ok := questionGraphEdgePolicies[kind]; !ok {
		return fmt.Errorf("unsupported graph edge kind %q", kind)
	}
	if confidence != nil && *confidence == 1 {
		if strings.TrimSpace(rationale) == "" || strings.TrimSpace(source) == "" {
			return fmt.Errorf("confidence 1.0 requires non-empty rationale and provenance source")
		}
	}
	return nil
}

func isTestProvenance(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(value, "fixture") ||
		strings.Contains(value, "smoke") ||
		strings.Contains(value, "synthetic") ||
		strings.Contains(value, "test")
}

// CreateEdgeProposal stores an explicit, workspace-safe proposal. A repeated
// request for the same revision pair is idempotent and never mutates an
// already-decided proposal.
func (p *Postgres) CreateEdgeProposal(ctx context.Context, request EdgeProposalRequest, actor string) (EdgeProposal, error) {
	request, err := normalizeGraphRequest(request)
	if err != nil {
		return EdgeProposal{}, err
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "question-brain-editorial"
	}
	if request.WorkspaceKey == "fluent-interview" && (isTestProvenance(request.Source) || isTestProvenance(actor)) {
		return EdgeProposal{}, fmt.Errorf("test provenance must use an isolated graph workspace")
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return EdgeProposal{}, fmt.Errorf("begin graph proposal: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var workspaceID, fromRevisionID, toRevisionID string
	if err := tx.QueryRow(ctx, `
		select w.id::text, from_qr.id::text, to_qr.id::text
		from content.workspace w
		join content.question from_q on from_q.workspace_id = w.id and from_q.stable_key = $2 and from_q.status = 'published'
		join content.question_revision from_qr on from_qr.id = from_q.current_revision_id
		join content.question to_q on to_q.workspace_id = w.id and to_q.stable_key = $3 and to_q.status = 'published'
		join content.question_revision to_qr on to_qr.id = to_q.current_revision_id
		where w.stable_key = $1
	`, request.WorkspaceKey, request.FromStableKey, request.ToStableKey).Scan(&workspaceID, &fromRevisionID, &toRevisionID); err != nil {
		return EdgeProposal{}, fmt.Errorf("resolve graph proposal endpoints: %w", err)
	}
	var proposalID string
	err = tx.QueryRow(ctx, `
		insert into content.question_edge_proposal
		  (workspace_id, from_revision_id, to_revision_id, kind, confidence, rationale, source)
		values ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7)
		on conflict (workspace_id, from_revision_id, to_revision_id, kind) do nothing
		returning id::text
	`, workspaceID, fromRevisionID, toRevisionID, request.Kind, request.Confidence, request.Rationale, request.Source).Scan(&proposalID)
	if err == pgx.ErrNoRows {
		if err := tx.QueryRow(ctx, `
			select id::text from content.question_edge_proposal
			where workspace_id = $1::uuid and from_revision_id = $2::uuid and to_revision_id = $3::uuid and kind = $4
		`, workspaceID, fromRevisionID, toRevisionID, request.Kind).Scan(&proposalID); err != nil {
			return EdgeProposal{}, fmt.Errorf("find existing graph proposal: %w", err)
		}
	} else if err != nil {
		return EdgeProposal{}, fmt.Errorf("insert graph proposal: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into content.audit_event (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
		values ($1::uuid, 'question_graph_edge', $2::uuid, 'question.graph.edge.proposed', $3::text, jsonb_build_object('kind', $4::text, 'from', $5::text, 'to', $6::text))
		on conflict do nothing
	`, workspaceID, proposalID, actor, request.Kind, request.FromStableKey, request.ToStableKey); err != nil {
		return EdgeProposal{}, fmt.Errorf("audit graph proposal: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return EdgeProposal{}, fmt.Errorf("commit graph proposal: %w", err)
	}
	return p.readEdgeProposal(ctx, proposalID)
}

func (p *Postgres) readEdgeProposal(ctx context.Context, proposalID string) (EdgeProposal, error) {
	var result EdgeProposal
	err := p.pool.QueryRow(ctx, `
		select proposal.id::text, workspace.stable_key,
		  from_q.stable_key, proposal.from_revision_id::text,
		  to_q.stable_key, proposal.to_revision_id::text,
		  proposal.kind, proposal.status, proposal.confidence,
		  proposal.rationale, proposal.source, proposal.created_at,
		  proposal.decided_at, coalesce(proposal.decided_by, '')
		from content.question_edge_proposal proposal
		join content.workspace workspace on workspace.id = proposal.workspace_id
		join content.question_revision from_revision on from_revision.id = proposal.from_revision_id
		join content.question from_q on from_q.id = from_revision.question_id
		join content.question_revision to_revision on to_revision.id = proposal.to_revision_id
		join content.question to_q on to_q.id = to_revision.question_id
		where proposal.id = $1::uuid
	`, proposalID).Scan(
		&result.ID, &result.WorkspaceKey, &result.FromStableKey, &result.FromRevisionID,
		&result.ToStableKey, &result.ToRevisionID, &result.Kind, &result.Status,
		&result.Confidence, &result.Rationale, &result.Source, &result.CreatedAt,
		&result.DecidedAt, &result.DecidedBy,
	)
	if err != nil {
		return EdgeProposal{}, err
	}
	return result, nil
}

func (p *Postgres) ListEdgeProposals(ctx context.Context, workspaceKey, status string) ([]EdgeProposal, error) {
	workspaceKey = strings.TrimSpace(workspaceKey)
	if workspaceKey == "" {
		workspaceKey = "fluent-interview"
	}
	status = strings.TrimSpace(status)
	rows, err := p.pool.Query(ctx, `
		select proposal.id::text, workspace.stable_key,
		  from_q.stable_key, proposal.from_revision_id::text,
		  to_q.stable_key, proposal.to_revision_id::text,
		  proposal.kind, proposal.status, proposal.confidence,
		  proposal.rationale, proposal.source, proposal.created_at,
		  proposal.decided_at, coalesce(proposal.decided_by, '')
		from content.question_edge_proposal proposal
		join content.workspace workspace on workspace.id = proposal.workspace_id and workspace.stable_key = $1
		join content.question_revision from_revision on from_revision.id = proposal.from_revision_id
		join content.question from_q on from_q.id = from_revision.question_id
		join content.question_revision to_revision on to_revision.id = proposal.to_revision_id
		join content.question to_q on to_q.id = to_revision.question_id
		where ($2 = '' or proposal.status = $2)
		order by proposal.created_at, proposal.id
	`, workspaceKey, status)
	if err != nil {
		return nil, fmt.Errorf("query graph proposals: %w", err)
	}
	defer rows.Close()
	result := make([]EdgeProposal, 0)
	for rows.Next() {
		var proposal EdgeProposal
		if err := rows.Scan(&proposal.ID, &proposal.WorkspaceKey, &proposal.FromStableKey, &proposal.FromRevisionID,
			&proposal.ToStableKey, &proposal.ToRevisionID, &proposal.Kind, &proposal.Status,
			&proposal.Confidence, &proposal.Rationale, &proposal.Source, &proposal.CreatedAt,
			&proposal.DecidedAt, &proposal.DecidedBy); err != nil {
			return nil, fmt.Errorf("scan graph proposal: %w", err)
		}
		result = append(result, proposal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate graph proposals: %w", err)
	}
	return result, nil
}

// DecideEdgeProposal is the only mutable proposal lifecycle operation. An
// accepted prerequisite is checked against all accepted prerequisites inside
// the same transaction, so a concurrent cycle cannot be released.
func (p *Postgres) DecideEdgeProposal(ctx context.Context, proposalID, decision, actor, rationale string) (EdgeProposal, error) {
	decision = strings.TrimSpace(decision)
	if decision != "accepted" && decision != "rejected" && decision != "superseded" {
		return EdgeProposal{}, fmt.Errorf("unsupported graph proposal decision %q", decision)
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "question-brain-reviewer"
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return EdgeProposal{}, fmt.Errorf("begin graph decision: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var workspaceID, fromRevisionID, toRevisionID, kind, status string
	var confidence *float64
	var existingRationale, source string
	if err := tx.QueryRow(ctx, `
		select workspace_id::text, from_revision_id::text, to_revision_id::text, kind, status,
			confidence, rationale, source
		from content.question_edge_proposal where id = $1::uuid for update
	`, proposalID).Scan(&workspaceID, &fromRevisionID, &toRevisionID, &kind, &status, &confidence, &existingRationale, &source); err != nil {
		return EdgeProposal{}, err
	}
	if status == decision {
		if err := tx.Commit(ctx); err != nil {
			return EdgeProposal{}, err
		}
		return p.readEdgeProposal(ctx, proposalID)
	}
	if decision == "accepted" {
		effectiveRationale := strings.TrimSpace(rationale)
		if effectiveRationale == "" {
			effectiveRationale = strings.TrimSpace(existingRationale)
		}
		if effectiveRationale == "" {
			return EdgeProposal{}, fmt.Errorf("accepted graph edge requires reviewer rationale")
		}
		if err := validateGraphEvidence(kind, confidence, effectiveRationale, source); err != nil {
			return EdgeProposal{}, err
		}
	}
	if decision == "accepted" && kind == "prerequisite" {
		var cycle bool
		if err := tx.QueryRow(ctx, `
			with recursive reach(node) as (
				select to_revision_id from content.question_edge_proposal
				where workspace_id = $1::uuid and kind = 'prerequisite' and status = 'accepted' and from_revision_id = $3::uuid
				union
				select proposal.to_revision_id
				from content.question_edge_proposal proposal
				join reach on reach.node = proposal.from_revision_id
				where proposal.workspace_id = $1::uuid and proposal.kind = 'prerequisite' and proposal.status = 'accepted'
			)
			select exists(select 1 from reach where node = $2::uuid)
		`, workspaceID, fromRevisionID, toRevisionID).Scan(&cycle); err != nil {
			return EdgeProposal{}, fmt.Errorf("check prerequisite cycle: %w", err)
		}
		if cycle {
			return EdgeProposal{}, fmt.Errorf("accepting proposal would create a prerequisite cycle")
		}
	}
	if _, err := tx.Exec(ctx, `
		update content.question_edge_proposal
		set status = $2, decided_at = now(), decided_by = $3,
		    rationale = case when $4 <> '' then $4 else rationale end
		where id = $1::uuid
	`, proposalID, decision, actor, strings.TrimSpace(rationale)); err != nil {
		return EdgeProposal{}, fmt.Errorf("update graph proposal decision: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into content.audit_event (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
		values ($1::uuid, 'question_graph_edge', $2::uuid, 'question.graph.edge.' || $3::text, $4::text,
		  jsonb_build_object('kind', $5::text, 'from_revision_id', $6::text, 'to_revision_id', $7::text))
	`, workspaceID, proposalID, decision, actor, kind, fromRevisionID, toRevisionID); err != nil {
		return EdgeProposal{}, fmt.Errorf("audit graph decision: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return EdgeProposal{}, fmt.Errorf("commit graph decision: %w", err)
	}
	return p.readEdgeProposal(ctx, proposalID)
}

// ReleaseQuestionGraph publishes accepted proposals as an immutable release.
// Stale revisions and prerequisite cycles fail closed; proposed/rejected
// edges never enter a learner release.
func (p *Postgres) ReleaseQuestionGraph(ctx context.Context, request GraphReleaseRequest) (GraphReleaseReport, error) {
	workspaceKey := strings.TrimSpace(request.WorkspaceKey)
	if workspaceKey == "" {
		workspaceKey = "fluent-interview"
	}
	actor := strings.TrimSpace(request.Actor)
	if actor == "" {
		actor = "question-brain-graph-release"
	}
	questionRelease, err := p.Release(ctx, search.ReleaseRequest{WorkspaceKey: workspaceKey})
	if err != nil {
		return GraphReleaseReport{}, err
	}
	var workspaceID string
	if err := p.pool.QueryRow(ctx, `select id::text from content.workspace where stable_key = $1`, workspaceKey).Scan(&workspaceID); err != nil {
		return GraphReleaseReport{}, err
	}
	rows, err := p.pool.Query(ctx, `
		select proposal.id::text, proposal.from_revision_id::text, proposal.to_revision_id::text,
		  proposal.kind, proposal.confidence, proposal.rationale,
		  proposal.source, coalesce(proposal.decided_by, ''),
		  from_q.status, to_q.status,
		  (from_q.current_revision_id = proposal.from_revision_id and to_q.current_revision_id = proposal.to_revision_id) as current_revision
		from content.question_edge_proposal proposal
		join content.question_revision from_revision on from_revision.id = proposal.from_revision_id
		join content.question from_q on from_q.id = from_revision.question_id
		join content.question_revision to_revision on to_revision.id = proposal.to_revision_id
		join content.question to_q on to_q.id = to_revision.question_id
		where proposal.workspace_id = $1::uuid and proposal.status = 'accepted'
		order by proposal.id
	`, workspaceID)
	if err != nil {
		return GraphReleaseReport{}, fmt.Errorf("query accepted graph edges: %w", err)
	}
	defer rows.Close()
	type acceptedEdge struct {
		ProposalID, FromRevisionID, ToRevisionID, Kind, Rationale string
		Source, DecidedBy, FromStatus, ToStatus                   string
		Confidence                                                *float64
		Current                                                   bool
	}
	edges := make([]acceptedEdge, 0)
	report := GraphReleaseReport{ContractVersion: QuestionGraphContractVersion, WorkspaceKey: workspaceKey, QuestionReleaseID: questionRelease.ReleaseID, GeneratedAt: time.Now().UTC(), Approved: request.Approve, BlockedReasons: make([]string, 0)}
	for rows.Next() {
		var edge acceptedEdge
		if err := rows.Scan(&edge.ProposalID, &edge.FromRevisionID, &edge.ToRevisionID, &edge.Kind, &edge.Confidence, &edge.Rationale, &edge.Source, &edge.DecidedBy, &edge.FromStatus, &edge.ToStatus, &edge.Current); err != nil {
			return GraphReleaseReport{}, err
		}
		report.Accepted++
		if !edge.Current {
			report.Stale++
		}
		if edge.FromStatus == "archived" || edge.ToStatus == "archived" {
			report.ArchivedTargets++
		}
		if edge.FromStatus != "published" || edge.ToStatus != "published" {
			report.InvalidTargets++
		}
		if strings.TrimSpace(edge.Rationale) == "" || strings.TrimSpace(edge.DecidedBy) == "" {
			report.EvidenceGaps++
		}
		if err := validateGraphEvidence(edge.Kind, edge.Confidence, edge.Rationale, edge.Source); err != nil {
			report.EvidenceGaps++
		}
		if isTestProvenance(edge.Source) || isTestProvenance(edge.DecidedBy) {
			report.TestProvenance++
		}
		edges = append(edges, edge)
	}
	if err := rows.Err(); err != nil {
		return GraphReleaseReport{}, err
	}
	// Stable IDs and all immutable endpoints are part of the release seed.
	seedParts := make([]string, 0, len(edges))
	for _, edge := range edges {
		seedParts = append(seedParts, edge.FromRevisionID+"|"+edge.ToRevisionID+"|"+edge.Kind+"|"+edge.ProposalID)
	}
	sort.Strings(seedParts)
	seed := strings.Join(seedParts, "\n")
	sourceHash := sha256.Sum256([]byte(seed))
	report.GraphReleaseID = "question-graph-release-" + hex.EncodeToString(sourceHash[:8])
	if report.Stale > 0 {
		report.Blocked = true
		report.BlockedReasons = append(report.BlockedReasons, "accepted edge references a non-current question revision")
	}
	if report.InvalidTargets > 0 {
		report.Blocked = true
		report.BlockedReasons = append(report.BlockedReasons, "accepted edge references a non-published question")
	}
	if report.ArchivedTargets > 0 {
		report.Blocked = true
		report.BlockedReasons = append(report.BlockedReasons, "accepted edge references an archived question")
	}
	if report.EvidenceGaps > 0 {
		report.Blocked = true
		report.BlockedReasons = append(report.BlockedReasons, "accepted edge is missing reviewer evidence")
	}
	if report.TestProvenance > 0 {
		report.Blocked = true
		report.BlockedReasons = append(report.BlockedReasons, "accepted edge contains test or fixture provenance")
	}
	if report.Cycles, err = p.countPrerequisiteCycles(ctx, workspaceID); err != nil {
		return GraphReleaseReport{}, err
	}
	if report.Cycles > 0 {
		report.Blocked = true
		report.BlockedReasons = append(report.BlockedReasons, "accepted prerequisite graph contains a cycle")
	}
	if report.Blocked || !request.Approve {
		return report, nil
	}
	// A release ID is deterministic for the accepted revision set. An active
	// row therefore makes an approve call idempotent, while a rolled-back row
	// is immutable and must never be silently reused.
	var existingStatus string
	existingErr := p.pool.QueryRow(ctx, `
		select status from content.question_graph_release where graph_release_id = $1
	`, report.GraphReleaseID).Scan(&existingStatus)
	if existingErr == nil {
		if existingStatus == "active" {
			report.Released = 0
			return report, nil
		}
		return GraphReleaseReport{}, fmt.Errorf("graph release %s was rolled back and cannot be reused", report.GraphReleaseID)
	}
	if existingErr != pgx.ErrNoRows {
		return GraphReleaseReport{}, fmt.Errorf("check existing graph release: %w", existingErr)
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return GraphReleaseReport{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `update content.question_graph_release set status = 'rolled_back', rolled_back_at = now(), rolled_back_by = $2 where workspace_id = $1::uuid and status = 'active'`, workspaceID, actor); err != nil {
		return GraphReleaseReport{}, fmt.Errorf("retire previous graph release: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into content.question_graph_release
		  (graph_release_id, workspace_id, question_release_id, status, edge_count, source_hash, actor)
		values ($1, $2::uuid, $3, 'active', $4, $5, $6)
		on conflict (graph_release_id) do nothing
	`, report.GraphReleaseID, workspaceID, questionRelease.ReleaseID, len(edges), hex.EncodeToString(sourceHash[:]), actor); err != nil {
		return GraphReleaseReport{}, fmt.Errorf("insert graph release: %w", err)
	}
	for _, edge := range edges {
		if _, err := tx.Exec(ctx, `
			insert into content.question_edge_release
			  (graph_release_id, proposal_id, from_revision_id, to_revision_id, kind, confidence, rationale)
			values ($1, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7)
			on conflict do nothing
		`, report.GraphReleaseID, edge.ProposalID, edge.FromRevisionID, edge.ToRevisionID, edge.Kind, edge.Confidence, edge.Rationale); err != nil {
			return GraphReleaseReport{}, fmt.Errorf("materialize graph edge: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
		insert into content.audit_event (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
		values ($1::uuid, 'question_graph', $1::uuid, 'question.graph.released', $2::text,
		  jsonb_build_object('graph_release_id', $3::text, 'question_release_id', $4::text, 'edge_count', $5::int))
	`, workspaceID, actor, report.GraphReleaseID, questionRelease.ReleaseID, len(edges)); err != nil {
		return GraphReleaseReport{}, fmt.Errorf("audit graph release: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into content.outbox_event (aggregate_type, aggregate_id, event_type, idempotency_key, payload)
		values ('question_graph', $1::uuid, 'question.graph.released', $2::text,
		  jsonb_build_object('graph_release_id', $3::text, 'question_release_id', $4::text, 'workspace', $5::text))
		on conflict (idempotency_key) do nothing
	`, workspaceID, "question-graph-release:"+report.GraphReleaseID, report.GraphReleaseID, questionRelease.ReleaseID, workspaceKey); err != nil {
		return GraphReleaseReport{}, fmt.Errorf("emit graph release event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return GraphReleaseReport{}, fmt.Errorf("commit graph release: %w", err)
	}
	report.Released = len(edges)
	return report, nil
}

func (p *Postgres) countPrerequisiteCycles(ctx context.Context, workspaceID string) (int, error) {
	var cycles int
	err := p.pool.QueryRow(ctx, `
		with recursive reach(from_node, node) as (
			select from_revision_id, to_revision_id
			from content.question_edge_proposal
			where workspace_id = $1::uuid and status = 'accepted' and kind = 'prerequisite'
			union
			select reach.from_node, proposal.to_revision_id
			from reach
			join content.question_edge_proposal proposal
			  on proposal.from_revision_id = reach.node
			 and proposal.workspace_id = $1::uuid
			 and proposal.status = 'accepted'
			 and proposal.kind = 'prerequisite'
		)
		select count(*)::int from reach where from_node = node
	`, workspaceID).Scan(&cycles)
	return cycles, err
}

func (p *Postgres) RollbackQuestionGraph(ctx context.Context, graphReleaseID, actor string) (GraphRelease, error) {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "question-brain-graph-rollback"
	}
	var workspaceID string
	if err := p.pool.QueryRow(ctx, `
		update content.question_graph_release
		set status = 'rolled_back', rolled_back_at = now(), rolled_back_by = $2
		where graph_release_id = $1 and status = 'active'
		returning workspace_id::text
	`, graphReleaseID, actor).Scan(&workspaceID); err != nil {
		return GraphRelease{}, err
	}
	return p.GetGraphRelease(ctx, graphReleaseID)
}

func (p *Postgres) GetGraphRelease(ctx context.Context, graphReleaseID string) (GraphRelease, error) {
	var release GraphRelease
	release.ContractVersion = QuestionGraphContractVersion
	var createdAt time.Time
	if err := p.pool.QueryRow(ctx, `
		select release.graph_release_id, workspace.stable_key, release.question_release_id,
		 release.status, release.edge_count, release.source_hash, release.actor, release.created_at
		from content.question_graph_release release
		join content.workspace workspace on workspace.id = release.workspace_id
		where release.graph_release_id = $1
	`, graphReleaseID).Scan(&release.GraphReleaseID, &release.WorkspaceKey, &release.QuestionReleaseID,
		&release.Status, &release.EdgeCount, &release.SourceHash, &release.Actor, &createdAt); err != nil {
		return GraphRelease{}, err
	}
	release.CreatedAt = createdAt
	rows, err := p.pool.Query(ctx, `
		select coalesce(edge.proposal_id::text, ''), from_q.stable_key, edge.from_revision_id::text,
		 to_q.stable_key, edge.to_revision_id::text, edge.kind, edge.confidence, edge.rationale
		from content.question_edge_release edge
		join content.question_revision from_revision on from_revision.id = edge.from_revision_id
		join content.question from_q on from_q.id = from_revision.question_id
		join content.question_revision to_revision on to_revision.id = edge.to_revision_id
		join content.question to_q on to_q.id = to_revision.question_id
		where edge.graph_release_id = $1
		order by edge.from_revision_id, edge.to_revision_id, edge.kind
	`, graphReleaseID)
	if err != nil {
		return GraphRelease{}, err
	}
	defer rows.Close()
	release.Edges = make([]GraphEdge, 0, release.EdgeCount)
	for rows.Next() {
		var edge GraphEdge
		if err := rows.Scan(&edge.ProposalID, &edge.FromStableKey, &edge.FromRevisionID, &edge.ToStableKey, &edge.ToRevisionID, &edge.Kind, &edge.Confidence, &edge.Rationale); err != nil {
			return GraphRelease{}, err
		}
		release.Edges = append(release.Edges, edge)
	}
	if err := rows.Err(); err != nil {
		return GraphRelease{}, err
	}
	return release, nil
}

// CurrentGraphRelease returns the immutable active graph snapshot for a
// workspace. Review clients use this together with the non-mutating release
// dry-run to show an operator exactly which release is being replaced.
func (p *Postgres) CurrentGraphRelease(ctx context.Context, workspaceKey string) (GraphRelease, error) {
	workspaceKey = strings.TrimSpace(workspaceKey)
	if workspaceKey == "" {
		workspaceKey = "fluent-interview"
	}
	var releaseID string
	if err := p.pool.QueryRow(ctx, `
		select release.graph_release_id
		from content.question_graph_release release
		join content.workspace workspace on workspace.id = release.workspace_id
		where workspace.stable_key = $1 and release.status = 'active'
		order by release.created_at desc
		limit 1
	`, workspaceKey).Scan(&releaseID); err != nil {
		return GraphRelease{}, fmt.Errorf("read current graph release: %w", err)
	}
	return p.GetGraphRelease(ctx, releaseID)
}

func (p *Postgres) GraphNeighborhood(ctx context.Context, stableKey, workspaceKey string) (GraphNeighborhood, error) {
	stableKey = strings.TrimSpace(stableKey)
	workspaceKey = strings.TrimSpace(workspaceKey)
	if workspaceKey == "" {
		workspaceKey = "fluent-interview"
	}
	var releaseID string
	if err := p.pool.QueryRow(ctx, `
		select release.graph_release_id from content.question_graph_release release
		join content.workspace workspace on workspace.id = release.workspace_id
		where workspace.stable_key = $1 and release.status = 'active'
		order by release.created_at desc limit 1
	`, workspaceKey).Scan(&releaseID); err != nil {
		return GraphNeighborhood{}, err
	}
	release, err := p.GetGraphRelease(ctx, releaseID)
	if err != nil {
		return GraphNeighborhood{}, err
	}
	edges := make([]GraphEdge, 0)
	for _, edge := range release.Edges {
		if edge.FromStableKey == stableKey || edge.ToStableKey == stableKey {
			edges = append(edges, edge)
		}
	}
	release.Edges = nil
	return GraphNeighborhood{ContractVersion: QuestionGraphContractVersion, StableKey: stableKey, GraphRelease: release, Edges: edges}, nil
}
