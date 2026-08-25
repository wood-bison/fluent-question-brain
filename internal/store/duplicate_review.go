package store

import (
	"context"
	"fmt"
	"strings"
)

// DuplicateReviewContractVersion is the answer-free queue consumed by the
// operator Workbench. It deliberately reads the durable candidate table rather
// than deriving a queue only from exact prompt matches in the quality report.
const DuplicateReviewContractVersion = "question-brain.duplicate-review.v1"

// DuplicateReviewCandidate is a stable, revision-pinned duplicate proposal.
// Localized prompts are resolved by the Lab projection and are not returned by
// this write-authority service.
type DuplicateReviewCandidate struct {
	ID              string  `json:"id"`
	WorkspaceKey    string  `json:"workspace_key"`
	LeftStableKey   string  `json:"left_stable_key"`
	LeftRevisionID  string  `json:"left_revision_id"`
	RightStableKey  string  `json:"right_stable_key"`
	RightRevisionID string  `json:"right_revision_id"`
	ExactScore      float64 `json:"exact_score,omitempty"`
	SemanticScore   float64 `json:"semantic_score,omitempty"`
	Decision        string  `json:"decision"`
	DecidedBy       string  `json:"decided_by,omitempty"`
}

// ListDuplicateReviewCandidates returns only current production revisions.
// The Workbench passes status=proposed for consistency with the other queues;
// the duplicate table uses the historical lifecycle value open.
func (p *Postgres) ListDuplicateReviewCandidates(ctx context.Context, workspaceKey, status string) ([]DuplicateReviewCandidate, error) {
	workspaceKey = strings.TrimSpace(workspaceKey)
	if workspaceKey == "" {
		return nil, fmt.Errorf("workspace key is required")
	}
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "" || status == "proposed" {
		status = "open"
	}
	if status != "open" && status != "keep_separate" && status != "merge" && status != "not_duplicate" {
		return nil, fmt.Errorf("unsupported duplicate review status %q", status)
	}
	rows, err := p.pool.Query(ctx, `
		select candidate.id::text,
		       workspace.stable_key,
		       left_question.stable_key, candidate.left_revision_id::text,
		       right_question.stable_key, candidate.right_revision_id::text,
		       coalesce(candidate.exact_score, 0)::float8,
		       coalesce(candidate.semantic_score, 0)::float8,
		       candidate.decision, coalesce(candidate.decided_by, '')
		from content.duplicate_candidate candidate
		join content.workspace workspace on workspace.id = candidate.workspace_id
		join content.question_revision left_revision on left_revision.id = candidate.left_revision_id
		join content.question left_question on left_question.id = left_revision.question_id
		join content.question_revision right_revision on right_revision.id = candidate.right_revision_id
		join content.question right_question on right_question.id = right_revision.question_id
		where workspace.stable_key = $1
		  and candidate.decision = $2
		  and left_question.content_kind = 'production'
		  and right_question.content_kind = 'production'
		  and left_question.current_revision_id = candidate.left_revision_id
		  and right_question.current_revision_id = candidate.right_revision_id
		order by candidate.created_at, candidate.id
	`, workspaceKey, status)
	if err != nil {
		return nil, fmt.Errorf("query duplicate review candidates: %w", err)
	}
	defer rows.Close()
	candidates := make([]DuplicateReviewCandidate, 0)
	for rows.Next() {
		var candidate DuplicateReviewCandidate
		if err := rows.Scan(
			&candidate.ID, &candidate.WorkspaceKey,
			&candidate.LeftStableKey, &candidate.LeftRevisionID,
			&candidate.RightStableKey, &candidate.RightRevisionID,
			&candidate.ExactScore, &candidate.SemanticScore,
			&candidate.Decision, &candidate.DecidedBy,
		); err != nil {
			return nil, fmt.Errorf("scan duplicate review candidate: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate duplicate review candidates: %w", err)
	}
	return candidates, nil
}
