package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// CapabilityAliasSupersessionReviewContractVersion is the versioned operator
// boundary for canonical capability identity changes.  It is deliberately
// separate from graph and binding releases: accepting a rename changes
// historical resolution, not learner mastery or the current graph.
const CapabilityAliasSupersessionReviewContractVersion = "question-brain.capability-alias-supersession-review.v1"

const CapabilityAliasSupersessionDecisionContractVersion = "question-brain.capability-alias-supersession-decision.v1"

var capabilityAliasActions = map[string]struct{}{"alias": {}, "supersedes": {}}

// CapabilityAliasSupersessionProposal is an answer-free operator projection.
// source_key is the historical/legacy key being renamed; canonical_key is the
// active registry identity that future releases must select.
type CapabilityAliasSupersessionProposal struct {
	ID           string          `json:"id"`
	WorkspaceKey string          `json:"workspace_key"`
	Action       string          `json:"action"`
	SourceKey    string          `json:"source_key"`
	CanonicalKey string          `json:"canonical_key"`
	Reason       string          `json:"reason"`
	Source       string          `json:"source"`
	Provenance   json.RawMessage `json:"provenance"`
	Status       string          `json:"status"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	DecidedAt    *time.Time      `json:"decided_at,omitempty"`
	DecidedBy    string          `json:"decided_by,omitempty"`
}

type CapabilityAliasSupersessionProposalRequest struct {
	WorkspaceKey string          `json:"workspace_key"`
	Action       string          `json:"action"`
	SourceKey    string          `json:"source_key"`
	CanonicalKey string          `json:"canonical_key"`
	Reason       string          `json:"reason"`
	Source       string          `json:"source"`
	Provenance   json.RawMessage `json:"provenance,omitempty"`
}

func normalizeCapabilityAliasProposal(request CapabilityAliasSupersessionProposalRequest) (CapabilityAliasSupersessionProposalRequest, error) {
	request.WorkspaceKey = strings.TrimSpace(request.WorkspaceKey)
	request.Action = strings.ToLower(strings.TrimSpace(request.Action))
	request.SourceKey = strings.TrimSpace(request.SourceKey)
	request.CanonicalKey = strings.TrimSpace(request.CanonicalKey)
	request.Reason = strings.TrimSpace(request.Reason)
	request.Source = strings.TrimSpace(request.Source)
	if request.WorkspaceKey == "" {
		request.WorkspaceKey = "fluent-interview"
	}
	if request.Source == "" {
		request.Source = "question-brain-editorial"
	}
	if request.Provenance == nil {
		request.Provenance = json.RawMessage(`{}`)
	}
	if _, ok := capabilityAliasActions[request.Action]; !ok {
		return CapabilityAliasSupersessionProposalRequest{}, fmt.Errorf("unsupported capability alias action %q", request.Action)
	}
	if request.SourceKey == "" || request.CanonicalKey == "" || request.SourceKey == request.CanonicalKey {
		return CapabilityAliasSupersessionProposalRequest{}, fmt.Errorf("distinct source_key and canonical_key are required")
	}
	if request.Reason == "" {
		return CapabilityAliasSupersessionProposalRequest{}, fmt.Errorf("reason is required")
	}
	if len(request.Provenance) == 0 || !json.Valid(request.Provenance) || string(request.Provenance) == "null" {
		return CapabilityAliasSupersessionProposalRequest{}, fmt.Errorf("provenance must be a JSON object")
	}
	var object map[string]any
	if err := json.Unmarshal(request.Provenance, &object); err != nil || object == nil {
		return CapabilityAliasSupersessionProposalRequest{}, fmt.Errorf("provenance must be a JSON object")
	}
	return request, nil
}

// ListCapabilityAliasSupersessionProposals returns only the requested review
// state. It intentionally has no prompt/answer joins: identity changes are
// resolved by stable keys and registry evidence.
func (p *Postgres) ListCapabilityAliasSupersessionProposals(ctx context.Context, workspaceKey, status string) ([]CapabilityAliasSupersessionProposal, error) {
	workspaceKey = strings.TrimSpace(workspaceKey)
	status = strings.TrimSpace(status)
	if workspaceKey == "" {
		return nil, fmt.Errorf("workspace key is required")
	}
	if status == "" {
		status = "proposed"
	}
	rows, err := p.pool.Query(ctx, `
		select proposal.id::text, workspace.stable_key, proposal.action,
		       proposal.source_key, proposal.canonical_key, proposal.reason,
		       proposal.source, proposal.provenance, proposal.status,
		       proposal.created_at, proposal.updated_at,
		       proposal.decided_at, coalesce(proposal.decided_by, '')
		from content.taxonomy_capability_alias_supersession_proposal proposal
		join content.workspace workspace on workspace.id = proposal.workspace_id
		where workspace.stable_key = $1 and proposal.status = $2
		order by proposal.created_at, proposal.id
	`, workspaceKey, status)
	if err != nil {
		return nil, fmt.Errorf("query capability alias review proposals: %w", err)
	}
	defer rows.Close()
	proposals := make([]CapabilityAliasSupersessionProposal, 0)
	for rows.Next() {
		var proposal CapabilityAliasSupersessionProposal
		if err := rows.Scan(
			&proposal.ID, &proposal.WorkspaceKey, &proposal.Action,
			&proposal.SourceKey, &proposal.CanonicalKey, &proposal.Reason,
			&proposal.Source, &proposal.Provenance, &proposal.Status,
			&proposal.CreatedAt, &proposal.UpdatedAt,
			&proposal.DecidedAt, &proposal.DecidedBy,
		); err != nil {
			return nil, fmt.Errorf("scan capability alias review proposal: %w", err)
		}
		if len(proposal.Provenance) == 0 {
			proposal.Provenance = json.RawMessage(`{}`)
		}
		proposals = append(proposals, proposal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate capability alias review proposals: %w", err)
	}
	return proposals, nil
}

// CreateCapabilityAliasSupersessionProposal stages an explicit identity
// change. It is idempotent for the same workspace/action/source/canonical
// tuple and never mutates a previously decided proposal.
func (p *Postgres) CreateCapabilityAliasSupersessionProposal(ctx context.Context, request CapabilityAliasSupersessionProposalRequest, actor string) (CapabilityAliasSupersessionProposal, error) {
	request, err := normalizeCapabilityAliasProposal(request)
	if err != nil {
		return CapabilityAliasSupersessionProposal{}, err
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "question-brain-editorial"
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CapabilityAliasSupersessionProposal{}, fmt.Errorf("begin capability alias proposal: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var proposalID string
	err = tx.QueryRow(ctx, `
		insert into content.taxonomy_capability_alias_supersession_proposal
		  (workspace_id, action, source_key, canonical_key, reason, source, provenance)
		select workspace.id, $2, $3, $4, $5, $6, $7::jsonb
		from content.workspace
		where workspace.stable_key = $1
		  and exists (select 1 from content.taxonomy_capability where stable_key = $4 and lifecycle = 'active')
		on conflict (workspace_id, action, source_key, canonical_key) do nothing
		returning id::text
	`, request.WorkspaceKey, request.Action, request.SourceKey, request.CanonicalKey, request.Reason, request.Source, request.Provenance).Scan(&proposalID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, `
			select proposal.id::text
			from content.taxonomy_capability_alias_supersession_proposal proposal
			join content.workspace workspace on workspace.id = proposal.workspace_id
			where workspace.stable_key = $1 and proposal.action = $2
			  and proposal.source_key = $3 and proposal.canonical_key = $4
		`, request.WorkspaceKey, request.Action, request.SourceKey, request.CanonicalKey).Scan(&proposalID); err != nil {
			return CapabilityAliasSupersessionProposal{}, fmt.Errorf("resolve capability alias proposal: %w", err)
		}
	} else if err != nil {
		return CapabilityAliasSupersessionProposal{}, fmt.Errorf("insert capability alias proposal: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into content.audit_event (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
		select workspace.id, 'capability_alias_supersession_proposal', $2::uuid,
		       'capability.alias_supersession.proposed', $3, $4::jsonb
		from content.workspace where workspace.stable_key = $1
		on conflict do nothing
	`, request.WorkspaceKey, proposalID, actor, request.Provenance); err != nil {
		return CapabilityAliasSupersessionProposal{}, fmt.Errorf("audit capability alias proposal: %w", err)
	}
	proposal, err := scanCapabilityAliasSupersessionProposal(ctx, tx, proposalID)
	if err != nil {
		return CapabilityAliasSupersessionProposal{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CapabilityAliasSupersessionProposal{}, fmt.Errorf("commit capability alias proposal: %w", err)
	}
	return proposal, nil
}

// DecideCapabilityAliasSupersessionProposal is a compare-and-set writer. An
// accepted decision materialises exactly one canonical registry fact in the
// same transaction; a repeated identical decision returns the existing row.
func (p *Postgres) DecideCapabilityAliasSupersessionProposal(ctx context.Context, proposalID, decision, actor, rationale string) (CapabilityAliasSupersessionProposal, error) {
	proposalID = strings.TrimSpace(proposalID)
	decision = strings.ToLower(strings.TrimSpace(decision))
	actor = strings.TrimSpace(actor)
	rationale = strings.TrimSpace(rationale)
	if proposalID == "" || (decision != "accepted" && decision != "rejected") || actor == "" || rationale == "" {
		return CapabilityAliasSupersessionProposal{}, fmt.Errorf("proposal id, accepted/rejected decision, actor, and rationale are required")
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CapabilityAliasSupersessionProposal{}, fmt.Errorf("begin capability alias decision: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var current CapabilityAliasSupersessionProposal
	if err := scanCapabilityAliasSupersessionProposalForUpdate(ctx, tx, proposalID, &current); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CapabilityAliasSupersessionProposal{}, fmt.Errorf("capability alias proposal %w", pgx.ErrNoRows)
		}
		return CapabilityAliasSupersessionProposal{}, err
	}
	if current.Status != "proposed" {
		if current.Status != decision {
			return CapabilityAliasSupersessionProposal{}, fmt.Errorf("%w: current status is %q, requested %q", ErrReviewConflict, current.Status, decision)
		}
		return current, nil
	}
	if decision == "accepted" {
		if err := materializeCapabilityAliasSupersession(ctx, tx, current, rationale); err != nil {
			return CapabilityAliasSupersessionProposal{}, err
		}
	}
	if _, err := tx.Exec(ctx, `
		update content.taxonomy_capability_alias_supersession_proposal
		set status = $2, reason = $3, decided_by = $4, decided_at = now(), updated_at = now()
		where id = $1::uuid and status = 'proposed'
	`, proposalID, decision, rationale, actor); err != nil {
		return CapabilityAliasSupersessionProposal{}, fmt.Errorf("record capability alias decision: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into content.audit_event (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
		select workspace_id, 'capability_alias_supersession_proposal', id,
		       'capability.alias_supersession.decided', $2,
		       jsonb_build_object('decision', $3::text, 'rationale', $4::text, 'action', action, 'source_key', source_key, 'canonical_key', canonical_key)
		from content.taxonomy_capability_alias_supersession_proposal
		where id = $1::uuid
	`, proposalID, actor, decision, rationale); err != nil {
		return CapabilityAliasSupersessionProposal{}, fmt.Errorf("audit capability alias decision: %w", err)
	}
	current, err = scanCapabilityAliasSupersessionProposal(ctx, tx, proposalID)
	if err != nil {
		return CapabilityAliasSupersessionProposal{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CapabilityAliasSupersessionProposal{}, fmt.Errorf("commit capability alias decision: %w", err)
	}
	return current, nil
}

func materializeCapabilityAliasSupersession(ctx context.Context, tx pgx.Tx, proposal CapabilityAliasSupersessionProposal, rationale string) error {
	if proposal.Action == "alias" {
		_, err := tx.Exec(ctx, `
			insert into content.taxonomy_capability_alias (alias_key, canonical_key, reason)
			values ($1, $2, $3)
			on conflict (alias_key) do update
			set reason = excluded.reason
			where content.taxonomy_capability_alias.canonical_key = excluded.canonical_key
		`, proposal.SourceKey, proposal.CanonicalKey, rationale)
		if err != nil {
			return fmt.Errorf("materialize capability alias: %w", err)
		}
		var canonical string
		if err := tx.QueryRow(ctx, `select canonical_key from content.taxonomy_capability_alias where alias_key = $1`, proposal.SourceKey).Scan(&canonical); err != nil {
			return fmt.Errorf("verify capability alias: %w", err)
		}
		if canonical != proposal.CanonicalKey {
			return fmt.Errorf("capability alias %q already points to %q", proposal.SourceKey, canonical)
		}
		return nil
	}
	if proposal.Action == "supersedes" {
		var lifecycle string
		if err := tx.QueryRow(ctx, `select lifecycle from content.taxonomy_capability where stable_key = $1`, proposal.CanonicalKey).Scan(&lifecycle); err != nil {
			return fmt.Errorf("resolve supersession canonical key: %w", err)
		}
		if lifecycle != "active" {
			return fmt.Errorf("supersession canonical key %q is not active", proposal.CanonicalKey)
		}
		var exists bool
		if err := tx.QueryRow(ctx, `select exists(select 1 from content.taxonomy_capability where stable_key = $1)`, proposal.SourceKey).Scan(&exists); err != nil {
			return fmt.Errorf("resolve superseded capability key: %w", err)
		}
		if !exists {
			return fmt.Errorf("superseded capability key %q does not exist", proposal.SourceKey)
		}
		_, err := tx.Exec(ctx, `
			insert into content.taxonomy_capability_supersedes (superseded_key, canonical_key, reason)
			values ($1, $2, $3)
			on conflict (superseded_key) do update
			set reason = excluded.reason
			where content.taxonomy_capability_supersedes.canonical_key = excluded.canonical_key
		`, proposal.SourceKey, proposal.CanonicalKey, rationale)
		if err != nil {
			return fmt.Errorf("materialize capability supersession: %w", err)
		}
		return nil
	}
	return fmt.Errorf("unsupported capability alias action %q", proposal.Action)
}

func scanCapabilityAliasSupersessionProposal(ctx context.Context, tx pgx.Tx, proposalID string) (CapabilityAliasSupersessionProposal, error) {
	var proposal CapabilityAliasSupersessionProposal
	if err := scanCapabilityAliasSupersessionProposalForUpdate(ctx, tx, proposalID, &proposal); err != nil {
		return CapabilityAliasSupersessionProposal{}, fmt.Errorf("read capability alias proposal: %w", err)
	}
	return proposal, nil
}

func scanCapabilityAliasSupersessionProposalForUpdate(ctx context.Context, tx pgx.Tx, proposalID string, proposal *CapabilityAliasSupersessionProposal) error {
	return tx.QueryRow(ctx, `
		select proposal.id::text, workspace.stable_key, proposal.action,
		       proposal.source_key, proposal.canonical_key, proposal.reason,
		       proposal.source, proposal.provenance, proposal.status,
		       proposal.created_at, proposal.updated_at,
		       proposal.decided_at, coalesce(proposal.decided_by, '')
		from content.taxonomy_capability_alias_supersession_proposal proposal
		join content.workspace workspace on workspace.id = proposal.workspace_id
		where proposal.id = $1::uuid
		for update
	`, proposalID).Scan(
		&proposal.ID, &proposal.WorkspaceKey, &proposal.Action,
		&proposal.SourceKey, &proposal.CanonicalKey, &proposal.Reason,
		&proposal.Source, &proposal.Provenance, &proposal.Status,
		&proposal.CreatedAt, &proposal.UpdatedAt,
		&proposal.DecidedAt, &proposal.DecidedBy,
	)
}
