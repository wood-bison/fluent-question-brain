package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ErrReviewConflict is returned when an operator tries to overwrite a review
// proposal that another operator has already resolved.
var ErrReviewConflict = errors.New("review proposal changed concurrently")

// CapabilityBindingProposal is an answer-free operator projection. The
// immutable release pins and evidence are included; normalized answer payloads
// are intentionally never selected.
type CapabilityBindingProposal struct {
	ID                          string          `json:"id"`
	StableKey                   string          `json:"stable_key"`
	RevisionID                  string          `json:"revision_id"`
	PathKey                     string          `json:"path_key"`
	CapabilityKey               string          `json:"capability_key"`
	Role                        string          `json:"role"`
	Provenance                  string          `json:"provenance"`
	Confidence                  *float64        `json:"confidence,omitempty"`
	Evidence                    json.RawMessage `json:"evidence"`
	QuestionReleaseID           string          `json:"question_release_id"`
	CapabilityRegistryReleaseID string          `json:"capability_registry_release_id"`
	Status                      string          `json:"status"`
	Rationale                   string          `json:"rationale"`
	Source                      string          `json:"source"`
	DecidedBy                   string          `json:"decided_by,omitempty"`
	DecidedAt                   *string         `json:"decided_at,omitempty"`
}

// ListCapabilityBindingProposals returns only proposals in the requested
// lifecycle state. The query joins the revision back to its canonical card key
// but never selects localized prompts or answer bodies.
func (p *Postgres) ListCapabilityBindingProposals(ctx context.Context, workspaceKey, status string) ([]CapabilityBindingProposal, error) {
	workspaceKey = strings.TrimSpace(workspaceKey)
	status = strings.TrimSpace(status)
	if workspaceKey == "" {
		return nil, fmt.Errorf("workspace key is required")
	}
	if status == "" {
		status = "proposed"
	}
	rows, err := p.pool.Query(ctx, `
		select proposal.id::text, question.stable_key, proposal.revision_id::text,
		       proposal.path_key, proposal.capability_key, proposal.role,
		       proposal.provenance, proposal.confidence::float8, proposal.evidence,
		       proposal.question_release_id, proposal.capability_registry_release_id,
		       proposal.status, proposal.rationale, proposal.source,
		       coalesce(proposal.decided_by, ''), proposal.decided_at::text
		from content.question_capability_binding_proposal proposal
		join content.question_revision revision on revision.id = proposal.revision_id
		join content.question question on question.id = revision.question_id
		where proposal.workspace_id = (select id from content.workspace where stable_key = $1)
		  and proposal.status = $2
		order by proposal.created_at, proposal.id
	`, workspaceKey, status)
	if err != nil {
		return nil, fmt.Errorf("query capability binding review proposals: %w", err)
	}
	defer rows.Close()
	proposals := make([]CapabilityBindingProposal, 0)
	for rows.Next() {
		var proposal CapabilityBindingProposal
		if err := rows.Scan(
			&proposal.ID, &proposal.StableKey, &proposal.RevisionID, &proposal.PathKey,
			&proposal.CapabilityKey, &proposal.Role, &proposal.Provenance,
			&proposal.Confidence, &proposal.Evidence, &proposal.QuestionReleaseID,
			&proposal.CapabilityRegistryReleaseID, &proposal.Status, &proposal.Rationale,
			&proposal.Source, &proposal.DecidedBy, &proposal.DecidedAt,
		); err != nil {
			return nil, fmt.Errorf("scan capability binding review proposal: %w", err)
		}
		if len(proposal.Evidence) == 0 {
			proposal.Evidence = json.RawMessage(`{}`)
		}
		proposals = append(proposals, proposal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate capability binding review proposals: %w", err)
	}
	return proposals, nil
}

// DecideCapabilityBindingProposal records a compare-and-set decision. A
// repeated identical decision is idempotent; a different decision after the
// proposal was resolved is an explicit conflict, never last-write-wins.
func (p *Postgres) DecideCapabilityBindingProposal(ctx context.Context, proposalID, decision, actor, rationale string) (CapabilityBindingProposal, error) {
	proposalID = strings.TrimSpace(proposalID)
	decision = strings.TrimSpace(decision)
	actor = strings.TrimSpace(actor)
	rationale = strings.TrimSpace(rationale)
	if proposalID == "" || (decision != "accepted" && decision != "rejected") || actor == "" || rationale == "" {
		return CapabilityBindingProposal{}, fmt.Errorf("proposal id, accepted/rejected decision, actor, and rationale are required")
	}
	var proposal CapabilityBindingProposal
	err := p.pool.QueryRow(ctx, `
		update content.question_capability_binding_proposal proposal
		set status = $2, rationale = $3, decided_by = $4, decided_at = now()
		where proposal.id = $1::uuid and proposal.status = 'proposed'
		returning proposal.id::text,
		  (select question.stable_key from content.question_revision revision join content.question question on question.id = revision.question_id where revision.id = proposal.revision_id),
		  proposal.revision_id::text, proposal.path_key, proposal.capability_key, proposal.role,
		  proposal.provenance, proposal.confidence::float8, proposal.evidence,
		  proposal.question_release_id, proposal.capability_registry_release_id,
		  proposal.status, proposal.rationale, proposal.source, proposal.decided_by, proposal.decided_at::text
	`, proposalID, decision, rationale, actor).Scan(
		&proposal.ID, &proposal.StableKey, &proposal.RevisionID, &proposal.PathKey,
		&proposal.CapabilityKey, &proposal.Role, &proposal.Provenance,
		&proposal.Confidence, &proposal.Evidence, &proposal.QuestionReleaseID,
		&proposal.CapabilityRegistryReleaseID, &proposal.Status, &proposal.Rationale,
		&proposal.Source, &proposal.DecidedBy, &proposal.DecidedAt,
	)
	if err == nil {
		return proposal, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return CapabilityBindingProposal{}, fmt.Errorf("decide capability binding proposal: %w", err)
	}
	var currentStatus string
	if err := p.pool.QueryRow(ctx, `
		select status from content.question_capability_binding_proposal where id = $1::uuid
	`, proposalID).Scan(&currentStatus); err != nil {
		return CapabilityBindingProposal{}, fmt.Errorf("read capability binding proposal after compare-and-set: %w", err)
	}
	if currentStatus != decision {
		return CapabilityBindingProposal{}, fmt.Errorf("%w: current status is %q, requested %q", ErrReviewConflict, currentStatus, decision)
	}
	if err := p.pool.QueryRow(ctx, `
		select proposal.id::text,
		  (select question.stable_key from content.question_revision revision join content.question question on question.id = revision.question_id where revision.id = proposal.revision_id),
		  proposal.revision_id::text, proposal.path_key, proposal.capability_key, proposal.role,
		  proposal.provenance, proposal.confidence::float8, proposal.evidence,
		  proposal.question_release_id, proposal.capability_registry_release_id,
		  proposal.status, proposal.rationale, proposal.source, coalesce(proposal.decided_by, ''), proposal.decided_at::text
		from content.question_capability_binding_proposal proposal
		where proposal.id = $1::uuid
	`, proposalID).Scan(
		&proposal.ID, &proposal.StableKey, &proposal.RevisionID, &proposal.PathKey,
		&proposal.CapabilityKey, &proposal.Role, &proposal.Provenance,
		&proposal.Confidence, &proposal.Evidence, &proposal.QuestionReleaseID,
		&proposal.CapabilityRegistryReleaseID, &proposal.Status, &proposal.Rationale,
		&proposal.Source, &proposal.DecidedBy, &proposal.DecidedAt,
	); err != nil {
		return CapabilityBindingProposal{}, fmt.Errorf("read resolved capability binding proposal: %w", err)
	}
	return proposal, nil
}
