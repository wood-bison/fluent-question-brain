package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wood-bison/fluent-question-brain/internal/capabilitybinding"
	"github.com/wood-bison/fluent-question-brain/internal/search"
	"github.com/wood-bison/fluent-question-brain/internal/taxonomy"
)

type CapabilityBindingReleaseRequest struct {
	WorkspaceKey string
	Manifest     capabilitybinding.Manifest
	Actor        string
	Approve      bool
}

type CapabilityBindingReleaseReport struct {
	ContractVersion             string                      `json:"contract_version"`
	TaxonomyVersion             string                      `json:"taxonomy_version"`
	WorkspaceKey                string                      `json:"workspace_key"`
	QuestionReleaseID           string                      `json:"question_release_id"`
	CapabilityRegistryReleaseID string                      `json:"capability_registry_release_id"`
	BindingReleaseID            string                      `json:"binding_release_id"`
	GeneratedAt                 time.Time                   `json:"generated_at"`
	Approved                    bool                        `json:"approved"`
	Blocked                     bool                        `json:"blocked"`
	Published                   int                         `json:"published"`
	ManifestEntries             int                         `json:"manifest_entries"`
	Bound                       int                         `json:"bound"`
	TheoryOnly                  int                         `json:"theory_only"`
	NeedsNewCapability          int                         `json:"needs_new_capability"`
	Rejected                    int                         `json:"rejected"`
	Bindings                    int                         `json:"bindings"`
	Changed                     int                         `json:"changed"`
	Unchanged                   int                         `json:"unchanged"`
	Invalid                     int                         `json:"invalid"`
	MissingManifest             int                         `json:"missing_manifest"`
	ExtraManifest               int                         `json:"extra_manifest"`
	BlockedReasons              []string                    `json:"blocked_reasons,omitempty"`
	Coverage                    []CapabilityBindingCoverage `json:"coverage,omitempty"`
}

type CapabilityBindingCoverage struct {
	Dimension       string `json:"dimension"`
	Key             string `json:"key"`
	Cards           int    `json:"cards"`
	Bound           int    `json:"bound"`
	TheoryOnly      int    `json:"theory_only"`
	NeedsCapability int    `json:"needs_new_capability"`
	Rejected        int    `json:"rejected"`
}

type CapabilityBindingRollbackReport struct {
	WorkspaceKey      string `json:"workspace_key"`
	PreviousReleaseID string `json:"previous_release_id,omitempty"`
	RestoredReleaseID string `json:"restored_release_id"`
	RestoredBindings  int    `json:"restored_bindings"`
	Approved          bool   `json:"approved"`
	Blocked           bool   `json:"blocked"`
	BlockedReason     string `json:"blocked_reason,omitempty"`
}

// RollbackCapabilityBindings restores a previously released binding snapshot
// as the active learner projection. It changes only active pointers and the
// compatibility projection; immutable release items and review decisions are
// never rewritten.
func (p *Postgres) RollbackCapabilityBindings(ctx context.Context, workspaceKey, targetReleaseID, actor string, approve bool) (CapabilityBindingRollbackReport, error) {
	workspaceKey = strings.TrimSpace(workspaceKey)
	targetReleaseID = strings.TrimSpace(targetReleaseID)
	actor = strings.TrimSpace(actor)
	if workspaceKey == "" || targetReleaseID == "" {
		return CapabilityBindingRollbackReport{}, fmt.Errorf("workspace_key and target_release_id are required")
	}
	if actor == "" {
		actor = "question-brain-capability-binding-rollback"
	}
	var workspaceID, status string
	if err := p.pool.QueryRow(ctx, `select id::text from content.workspace where stable_key = $1`, workspaceKey).Scan(&workspaceID); err != nil {
		return CapabilityBindingRollbackReport{}, fmt.Errorf("read capability rollback workspace: %w", err)
	}
	if err := p.pool.QueryRow(ctx, `select status from content.question_capability_binding_release where binding_release_id = $1 and workspace_id = $2::uuid`, targetReleaseID, workspaceID).Scan(&status); err != nil {
		return CapabilityBindingRollbackReport{}, fmt.Errorf("read target capability release: %w", err)
	}
	report := CapabilityBindingRollbackReport{WorkspaceKey: workspaceKey, RestoredReleaseID: targetReleaseID, Approved: approve}
	if status == "active" {
		return report, nil
	}
	if status != "rolled_back" {
		report.Blocked = true
		report.BlockedReason = fmt.Sprintf("target release has unsupported status %q", status)
		return report, nil
	}
	if !approve {
		return report, nil
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CapabilityBindingRollbackReport{}, fmt.Errorf("begin capability rollback: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var previousID *string
	if err := tx.QueryRow(ctx, `
		select binding_release_id from content.question_capability_binding_release
		where workspace_id = $1::uuid and status = 'active' for update
	`, workspaceID).Scan(&previousID); err != nil && err != pgx.ErrNoRows {
		return CapabilityBindingRollbackReport{}, fmt.Errorf("read active capability release: %w", err)
	}
	if previousID != nil {
		report.PreviousReleaseID = *previousID
		if *previousID == targetReleaseID {
			return report, nil
		}
		if _, err := tx.Exec(ctx, `
			update content.question_capability_binding_release
			set status = 'rolled_back', rolled_back_at = now(), rolled_back_by = $2
			where binding_release_id = $1
		`, *previousID, actor); err != nil {
			return CapabilityBindingRollbackReport{}, fmt.Errorf("close active capability release: %w", err)
		}
		if _, err := tx.Exec(ctx, `delete from content.question_capability where binding_release_id = $1`, *previousID); err != nil {
			return CapabilityBindingRollbackReport{}, fmt.Errorf("clear active capability projection: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
		update content.question_capability_binding_release
		set status = 'active', rolled_back_at = null, rolled_back_by = null
		where binding_release_id = $1 and workspace_id = $2::uuid and status = 'rolled_back'
	`, targetReleaseID, workspaceID); err != nil {
		return CapabilityBindingRollbackReport{}, fmt.Errorf("activate target capability release: %w", err)
	}
	if err := tx.QueryRow(ctx, `
		select count(*) from content.question_capability_binding_release_item where binding_release_id = $1
	`, targetReleaseID).Scan(&report.RestoredBindings); err != nil {
		return CapabilityBindingRollbackReport{}, fmt.Errorf("count restored capability bindings: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into content.question_capability
		(revision_id, path_key, capability_key, mapping_state, mapping_version, source,
		 role, provenance, confidence, question_release_id, capability_registry_release_id,
		 binding_release_id, source_proposal_id)
		select item.revision_id, item.path_key, item.capability_key, 'accepted', $2,
		       'question-brain-capability-binding-rollback', item.role, item.provenance,
		       item.confidence, release.question_release_id,
		       release.capability_registry_release_id, release.binding_release_id,
		       item.source_proposal_id
		from content.question_capability_binding_release_item item
		join content.question_capability_binding_release release
		  on release.binding_release_id = item.binding_release_id
		where item.binding_release_id = $1
		on conflict (revision_id, path_key, capability_key) do update set
		 mapping_state = 'accepted', mapping_version = excluded.mapping_version,
		 source = excluded.source, role = excluded.role, provenance = excluded.provenance,
		 confidence = excluded.confidence, question_release_id = excluded.question_release_id,
		 capability_registry_release_id = excluded.capability_registry_release_id,
		 binding_release_id = excluded.binding_release_id, source_proposal_id = excluded.source_proposal_id
	`, targetReleaseID, taxonomy.Version); err != nil {
		return CapabilityBindingRollbackReport{}, fmt.Errorf("restore capability projection: %w", err)
	}
	metadata, _ := json.Marshal(map[string]any{
		"target_release_id":   targetReleaseID,
		"previous_release_id": report.PreviousReleaseID,
		"restored_bindings":   report.RestoredBindings,
	})
	if _, err := tx.Exec(ctx, `
		insert into content.audit_event (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
		values ($1::uuid, 'question_capability_binding_release', $1::uuid, 'question.capability.bindings.rolled_back', $2, $3::jsonb)
	`, workspaceID, actor, metadata); err != nil {
		return CapabilityBindingRollbackReport{}, fmt.Errorf("write capability rollback audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return CapabilityBindingRollbackReport{}, fmt.Errorf("commit capability rollback: %w", err)
	}
	return report, nil
}

// GenerateCapabilityBindingManifest creates a complete review queue from
// explicit current curriculum rows only. Existing reviewed runtime mappings
// become bound candidates (with their legacy key recorded as evidence); cards
// without a reviewed station become theory_only, or needs_new_capability when
// the editorial payload explicitly references an executable TaskFamily. No
// title, Topic, breadcrumb, or embedding is consulted.
func (p *Postgres) GenerateCapabilityBindingManifest(ctx context.Context, workspaceKey, registryReleaseID, source string) (capabilitybinding.Manifest, error) {
	workspaceKey = strings.TrimSpace(workspaceKey)
	registryReleaseID = strings.TrimSpace(registryReleaseID)
	source = strings.TrimSpace(source)
	if workspaceKey == "" || registryReleaseID == "" || source == "" {
		return capabilitybinding.Manifest{}, fmt.Errorf("workspace_key, registry_release_id, and source are required")
	}
	release, err := p.Release(ctx, search.ReleaseRequest{WorkspaceKey: workspaceKey})
	if err != nil {
		return capabilitybinding.Manifest{}, fmt.Errorf("read question release: %w", err)
	}
	rows, err := p.pool.Query(ctx, `
		select q.stable_key, qr.id::text, qr.content_hash,
		       coalesce(mapping.path_key, ''), coalesce(mapping.domain_key, ''),
		       mapping.capability_key,
		       (jsonb_typeof(qr.normalized_payload->'task') = 'object'
		        and coalesce(qr.normalized_payload->'task'->>'task_family_key', '') <> '')
		from content.question q
		join content.question_revision qr on qr.id = q.current_revision_id
		left join content.question_curriculum_mapping mapping
		  on mapping.revision_id = qr.id and mapping.mapping_state = 'accepted'
		where q.workspace_id = (select id from content.workspace where stable_key = $1)
		  and q.status = 'published' and q.content_kind = 'production'
		order by q.stable_key
	`, workspaceKey)
	if err != nil {
		return capabilitybinding.Manifest{}, fmt.Errorf("query current capability review queue: %w", err)
	}
	defer rows.Close()
	entries := make([]capabilitybinding.Entry, 0)
	for rows.Next() {
		var stableKey, revisionID, contentHash, pathKey, domainKey string
		var legacyCapability *string
		var executable bool
		if err := rows.Scan(&stableKey, &revisionID, &contentHash, &pathKey, &domainKey, &legacyCapability, &executable); err != nil {
			return capabilitybinding.Manifest{}, fmt.Errorf("scan current capability review queue: %w", err)
		}
		entry := capabilitybinding.Entry{
			StableKey: stableKey, RevisionID: revisionID, ContentHash: contentHash,
			Disposition: "theory_only",
			Rationale:   "No reviewed learner station is attached; keep the card released and searchable without manufacturing a capability.",
		}
		if executable {
			entry.Disposition = "needs_new_capability"
			entry.Rationale = "The card references an executable TaskFamily but no reviewed capability binding exists yet."
		}
		if legacyCapability != nil && strings.TrimSpace(*legacyCapability) != "" {
			capabilityKey := strings.TrimSpace(*legacyCapability)
			placement, resolveErr := taxonomy.ResolvePlacement(taxonomy.DefaultProgramKey, pathKey, domainKey, capabilityKey, "accepted")
			if resolveErr != nil {
				return capabilitybinding.Manifest{}, fmt.Errorf("resolve capability %s for %s: %w", capabilityKey, stableKey, resolveErr)
			}
			entry.Disposition = "bound"
			entry.Rationale = "Inherited from the reviewed curriculum capability crosswalk; canonical key is resolved before release."
			confidence := 1.0
			entry.Bindings = []capabilitybinding.Binding{{
				PathKey: pathKey, CapabilityKey: placement.CapabilityKey, Role: "primary",
				Provenance: "question-brain-reviewed-curriculum-crosswalk", Confidence: &confidence,
				Evidence: map[string]any{"legacy_capability_key": capabilityKey, "domain_key": domainKey},
			}}
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return capabilitybinding.Manifest{}, fmt.Errorf("iterate current capability review queue: %w", err)
	}
	return capabilitybinding.Manifest{
		ContractVersion:             capabilitybinding.ContractVersion,
		TaxonomyVersion:             taxonomy.Version,
		WorkspaceKey:                workspaceKey,
		QuestionReleaseID:           release.ReleaseID,
		CapabilityRegistryReleaseID: registryReleaseID,
		Source:                      source,
		Entries:                     entries,
	}, nil
}

type currentCapabilityRevision struct {
	StableKey  string
	RevisionID string
	Hash       string
	PathKey    string
	DomainKey  string
	Locales    []string
	Topics     []string
	CardKind   string
}

type CapabilityNeighborProposalReport struct {
	ProfileKey      string  `json:"profile_key"`
	ProfileRevision string  `json:"profile_revision"`
	MinSimilarity   float64 `json:"min_similarity"`
	MaxCandidates   int     `json:"max_candidates"`
	Targets         int     `json:"targets"`
	Candidates      int     `json:"candidates"`
	Unchanged       int     `json:"unchanged"`
}

// StageCapabilityNeighborProposals creates review-only candidates from
// existing active pgvector embeddings and reviewed capability exemplars. It
// never changes a disposition or a learner release. Exact editorial
// crosswalks remain authoritative; semantic neighbors only explain a possible
// placement for a human review.
func (p *Postgres) StageCapabilityNeighborProposals(ctx context.Context, workspaceKey, registryReleaseID, source string) (CapabilityNeighborProposalReport, error) {
	workspaceKey = strings.TrimSpace(workspaceKey)
	registryReleaseID = strings.TrimSpace(registryReleaseID)
	source = strings.TrimSpace(source)
	if workspaceKey == "" || registryReleaseID == "" || source == "" {
		return CapabilityNeighborProposalReport{}, fmt.Errorf("workspace_key, registry_release_id, and source are required")
	}
	release, err := p.Release(ctx, search.ReleaseRequest{WorkspaceKey: workspaceKey})
	if err != nil {
		return CapabilityNeighborProposalReport{}, fmt.Errorf("read question release: %w", err)
	}
	var profileKey, profileRevision string
	var minSimilarity float64
	var maxCandidates int
	if err := p.pool.QueryRow(ctx, `
		select profile_key, revision, min_similarity::float8, max_candidates
		from content.capability_binding_profile_config
		where profile_key = 'semantic-neighbor-v1'
	`).Scan(&profileKey, &profileRevision, &minSimilarity, &maxCandidates); err != nil {
		return CapabilityNeighborProposalReport{}, fmt.Errorf("read capability neighbor profile: %w", err)
	}
	var workspaceID string
	if err := p.pool.QueryRow(ctx, `select id::text from content.workspace where stable_key = $1`, workspaceKey).Scan(&workspaceID); err != nil {
		return CapabilityNeighborProposalReport{}, fmt.Errorf("read capability proposal workspace: %w", err)
	}
	rows, err := p.pool.Query(ctx, `
		with exemplar_embeddings as (
			select distinct on (coalesce(alias.canonical_key, mapping.capability_key), mapping.path_key, qr.id)
				coalesce(alias.canonical_key, mapping.capability_key) as capability_key,
				mapping.path_key, qr.id as revision_id, embedding.embedding
			from content.question q
			join content.question_revision qr on qr.id = q.current_revision_id
			join content.question_curriculum_mapping mapping
			  on mapping.revision_id = qr.id and mapping.mapping_state = 'accepted'
			left join content.taxonomy_capability_alias alias
			  on alias.alias_key = mapping.capability_key
			join content.taxonomy_capability capability
			  on capability.stable_key = coalesce(alias.canonical_key, mapping.capability_key)
			 and capability.lifecycle = 'active'
			join content.question_locale locale on locale.revision_id = qr.id
			join content.question_embedding embedding
			  on embedding.locale_id = locale.id and embedding.profile_key = 'semantic-v1'
			where q.workspace_id = $1::uuid and q.status = 'published' and q.content_kind = 'production'
			  and mapping.capability_key is not null
			order by coalesce(alias.canonical_key, mapping.capability_key), mapping.path_key, qr.id,
				case when locale.locale = 'en' then 0 when locale.locale = 'ru' then 1 else 2 end
		), target_embeddings as (
			select distinct on (qr.id)
				q.stable_key, qr.id as revision_id, embedding.embedding
			from content.question q
			join content.question_revision qr on qr.id = q.current_revision_id
			join content.question_locale locale on locale.revision_id = qr.id
			join content.question_embedding embedding
			  on embedding.locale_id = locale.id and embedding.profile_key = 'semantic-v1'
			where q.workspace_id = $1::uuid and q.status = 'published' and q.content_kind = 'production'
			order by qr.id, case when locale.locale = 'en' then 0 when locale.locale = 'ru' then 1 else 2 end
		), scored as (
			select target.stable_key, target.revision_id, exemplar.path_key,
			       exemplar.capability_key, exemplar.revision_id as exemplar_revision_id,
			       (1 - (target.embedding <=> exemplar.embedding))::float8 as similarity
			from target_embeddings target
			cross join exemplar_embeddings exemplar
			where target.revision_id <> exemplar.revision_id
		), ranked as (
			select *, row_number() over (
				partition by stable_key order by similarity desc, capability_key, path_key
			) as candidate_rank
			from scored
			where similarity >= $2::float8
		)
		select stable_key, revision_id::text, path_key, capability_key,
		       exemplar_revision_id::text, similarity::float8
		from ranked
		where candidate_rank <= $3
		order by stable_key, candidate_rank
	`, workspaceID, minSimilarity, maxCandidates)
	if err != nil {
		return CapabilityNeighborProposalReport{}, fmt.Errorf("query capability neighbor candidates: %w", err)
	}
	defer rows.Close()
	type candidate struct {
		stableKey, revisionID, pathKey, capabilityKey, exemplarRevisionID string
		similarity                                                        float64
	}
	candidates := make([]candidate, 0)
	targets := map[string]struct{}{}
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.stableKey, &item.revisionID, &item.pathKey, &item.capabilityKey, &item.exemplarRevisionID, &item.similarity); err != nil {
			return CapabilityNeighborProposalReport{}, fmt.Errorf("scan capability neighbor candidate: %w", err)
		}
		candidates = append(candidates, item)
		targets[item.stableKey] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return CapabilityNeighborProposalReport{}, fmt.Errorf("iterate capability neighbor candidates: %w", err)
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CapabilityNeighborProposalReport{}, fmt.Errorf("begin capability proposal staging: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	unchanged := 0
	for _, item := range candidates {
		evidence, _ := json.Marshal(map[string]any{
			"method":               "pgvector-semantic-neighbor",
			"profile_key":          profileKey,
			"profile_revision":     profileRevision,
			"similarity":           item.similarity,
			"exemplar_revision_id": item.exemplarRevisionID,
		})
		command, err := tx.Exec(ctx, `
			insert into content.question_capability_binding_proposal
			(workspace_id, revision_id, path_key, capability_key, role, provenance,
			 confidence, evidence, question_release_id, capability_registry_release_id,
			 status, rationale, source)
			values ($1::uuid, $2::uuid, $3, $4, 'supporting_evidence', $5,
			 $6, $7::jsonb, $8, $9, 'proposed',
			 'Semantic neighbor of a reviewed capability exemplar; human review required.', $10)
			on conflict (workspace_id, revision_id, path_key, capability_key, role, capability_registry_release_id)
			do update set confidence = excluded.confidence, evidence = excluded.evidence,
			 source = excluded.source, rationale = excluded.rationale
			where content.question_capability_binding_proposal.status = 'proposed'
		`, workspaceID, item.revisionID, item.pathKey, item.capabilityKey,
			"semantic-neighbor-v1", item.similarity, evidence, release.ReleaseID, registryReleaseID, source)
		if err != nil {
			return CapabilityNeighborProposalReport{}, fmt.Errorf("stage capability neighbor proposal for %s: %w", item.stableKey, err)
		}
		if command.RowsAffected() == 0 {
			unchanged++
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return CapabilityNeighborProposalReport{}, fmt.Errorf("commit capability proposal staging: %w", err)
	}
	return CapabilityNeighborProposalReport{ProfileKey: profileKey, ProfileRevision: profileRevision,
		MinSimilarity: minSimilarity, MaxCandidates: maxCandidates, Targets: len(targets),
		Candidates: len(candidates), Unchanged: unchanged}, nil
}

// ReleaseCapabilityBindings validates a complete, revision-pinned reviewed
// disposition and optionally makes it the single active station release. It
// never derives a capability from a title, legacy Topic, breadcrumb, or
// embedding. A theory-only disposition is a first-class result, not missing
// data.
func (p *Postgres) ReleaseCapabilityBindings(ctx context.Context, request CapabilityBindingReleaseRequest) (CapabilityBindingReleaseReport, error) {
	workspaceKey := strings.TrimSpace(request.WorkspaceKey)
	if workspaceKey == "" {
		return CapabilityBindingReleaseReport{}, fmt.Errorf("workspace key is required")
	}
	actor := strings.TrimSpace(request.Actor)
	if actor == "" {
		actor = "question-brain-capability-binding-release"
	}
	entries, err := request.Manifest.Normalize()
	if err != nil {
		return CapabilityBindingReleaseReport{}, err
	}
	if strings.TrimSpace(request.Manifest.WorkspaceKey) != workspaceKey {
		return CapabilityBindingReleaseReport{}, fmt.Errorf("manifest workspace_key %q does not match request workspace %q", request.Manifest.WorkspaceKey, workspaceKey)
	}
	questionRelease, err := p.Release(ctx, search.ReleaseRequest{WorkspaceKey: workspaceKey})
	if err != nil {
		return CapabilityBindingReleaseReport{}, fmt.Errorf("read question release: %w", err)
	}
	if request.Manifest.QuestionReleaseID != questionRelease.ReleaseID {
		return CapabilityBindingReleaseReport{}, fmt.Errorf("manifest question_release_id %q does not match current release %q", request.Manifest.QuestionReleaseID, questionRelease.ReleaseID)
	}
	current, workspaceID, err := p.currentCapabilityRevisions(ctx, workspaceKey)
	if err != nil {
		return CapabilityBindingReleaseReport{}, err
	}
	capabilities := make(map[string]struct{})
	for _, entry := range entries {
		for _, binding := range entry.Bindings {
			capabilities[binding.CapabilityKey] = struct{}{}
		}
	}
	known, err := p.activeCapabilities(ctx, capabilities)
	if err != nil {
		return CapabilityBindingReleaseReport{}, err
	}
	report := CapabilityBindingReleaseReport{
		ContractVersion:             capabilitybinding.ContractVersion,
		TaxonomyVersion:             taxonomy.Version,
		WorkspaceKey:                workspaceKey,
		QuestionReleaseID:           questionRelease.ReleaseID,
		CapabilityRegistryReleaseID: strings.TrimSpace(request.Manifest.CapabilityRegistryReleaseID),
		BindingReleaseID:            capabilitybinding.Fingerprint(request.Manifest, entries),
		GeneratedAt:                 time.Now().UTC(),
		Approved:                    request.Approve,
		Published:                   len(current),
		ManifestEntries:             len(entries),
		BlockedReasons:              make([]string, 0),
	}
	byStableKey := make(map[string]capabilitybinding.Entry, len(entries))
	for _, entry := range entries {
		byStableKey[entry.StableKey] = entry
		row, ok := current[entry.StableKey]
		if !ok {
			report.ExtraManifest++
			continue
		}
		if entry.RevisionID != row.RevisionID {
			report.Invalid++
			report.BlockedReasons = append(report.BlockedReasons, fmt.Sprintf("%s pins revision %s, current is %s", entry.StableKey, entry.RevisionID, row.RevisionID))
		}
		if entry.ContentHash != row.Hash {
			report.Invalid++
			report.BlockedReasons = append(report.BlockedReasons, fmt.Sprintf("%s pins hash %s, current is %s", entry.StableKey, entry.ContentHash, row.Hash))
		}
		switch entry.Disposition {
		case "bound":
			report.Bound++
		case "theory_only":
			report.TheoryOnly++
		case "needs_new_capability":
			report.NeedsNewCapability++
		case "rejected":
			report.Rejected++
		}
		report.Bindings += len(entry.Bindings)
		if entry.Disposition == "bound" {
			if row.PathKey == "" {
				report.Invalid++
				report.BlockedReasons = append(report.BlockedReasons, fmt.Sprintf("%s is bound but has no accepted curriculum path", entry.StableKey))
			}
			for _, binding := range entry.Bindings {
				if binding.PathKey != row.PathKey {
					report.Invalid++
					report.BlockedReasons = append(report.BlockedReasons, fmt.Sprintf("%s binding path %q does not match accepted path %q", entry.StableKey, binding.PathKey, row.PathKey))
				}
				if _, ok := known[binding.CapabilityKey]; !ok {
					report.Invalid++
					report.BlockedReasons = append(report.BlockedReasons, fmt.Sprintf("capability %q is absent or not active in registry", binding.CapabilityKey))
				}
			}
		}
	}
	for stableKey := range current {
		if _, ok := byStableKey[stableKey]; !ok {
			report.MissingManifest++
		}
	}
	if report.MissingManifest > 0 {
		report.BlockedReasons = append(report.BlockedReasons, fmt.Sprintf("%d current production revisions are missing from the complete capability disposition", report.MissingManifest))
	}
	if report.ExtraManifest > 0 {
		report.BlockedReasons = append(report.BlockedReasons, fmt.Sprintf("%d manifest rows are not current published production revisions", report.ExtraManifest))
	}
	if report.Invalid > 0 {
		report.BlockedReasons = append(report.BlockedReasons, "capability manifest contains stale pins, inactive capabilities, or path mismatches")
	}
	if len(current) != len(entries) {
		report.BlockedReasons = append(report.BlockedReasons, "capability manifest does not cover every current production revision")
	}
	report.Coverage = capabilityBindingCoverage(entries, current)
	sort.Strings(report.BlockedReasons)
	report.Blocked = report.MissingManifest > 0 || report.ExtraManifest > 0 || report.Invalid > 0 || len(current) != len(entries)
	if report.Blocked || !request.Approve {
		return report, nil
	}

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CapabilityBindingReleaseReport{}, fmt.Errorf("begin capability binding release transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// One active learner projection per workspace. Previous releases remain
	// immutable and can be restored by a later explicit rollback operation.
	if _, err := tx.Exec(ctx, `
		update content.question_capability_binding_release
		set status = 'rolled_back', rolled_back_at = now(), rolled_back_by = $2
		where workspace_id = $1::uuid and status = 'active'
	`, workspaceID, actor); err != nil {
		return CapabilityBindingReleaseReport{}, fmt.Errorf("close active capability binding release: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		delete from content.question_capability
		where binding_release_id in (
			select binding_release_id from content.question_capability_binding_release
			where workspace_id = $1::uuid and status = 'rolled_back'
		)
	`, workspaceID); err != nil {
		return CapabilityBindingReleaseReport{}, fmt.Errorf("clear previous active capability projection: %w", err)
	}
	metadata, _ := json.Marshal(map[string]any{
		"source":                         request.Manifest.Source,
		"question_release_id":            report.QuestionReleaseID,
		"capability_registry_release_id": report.CapabilityRegistryReleaseID,
		"binding_release_id":             report.BindingReleaseID,
		"entries":                        report.ManifestEntries,
		"bindings":                       report.Bindings,
	})
	if _, err := tx.Exec(ctx, `
		insert into content.question_capability_binding_release
		(binding_release_id, workspace_id, question_release_id, capability_registry_release_id,
		 status, binding_count, bound_count, theory_only_count, needs_new_capability_count,
		 rejected_count, source_hash, actor)
		values ($1, $2::uuid, $3, $4, 'active', $5, $6, $7, $8, $9, encode(digest($10::text, 'sha256'), 'hex'), $11)
		on conflict (binding_release_id) do update set
			workspace_id = excluded.workspace_id,
			question_release_id = excluded.question_release_id,
			capability_registry_release_id = excluded.capability_registry_release_id,
			status = 'active', binding_count = excluded.binding_count,
			bound_count = excluded.bound_count, theory_only_count = excluded.theory_only_count,
			needs_new_capability_count = excluded.needs_new_capability_count,
			rejected_count = excluded.rejected_count, source_hash = excluded.source_hash,
			actor = excluded.actor, rolled_back_at = null, rolled_back_by = null
	`, report.BindingReleaseID, workspaceID, report.QuestionReleaseID,
		report.CapabilityRegistryReleaseID, report.Bindings, report.Bound,
		report.TheoryOnly, report.NeedsNewCapability, report.Rejected, metadata, actor); err != nil {
		return CapabilityBindingReleaseReport{}, fmt.Errorf("write capability binding release: %w", err)
	}
	for _, entry := range entries {
		row := current[entry.StableKey]
		if _, err := tx.Exec(ctx, `
			insert into content.question_capability_review
			(workspace_id, revision_id, question_release_id, capability_registry_release_id,
			 disposition, rationale, source, updated_at)
			values ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, now())
			on conflict (workspace_id, revision_id, question_release_id, capability_registry_release_id)
			do update set disposition = excluded.disposition, rationale = excluded.rationale,
			 source = excluded.source, updated_at = now()
		`, workspaceID, row.RevisionID, report.QuestionReleaseID, report.CapabilityRegistryReleaseID,
			entry.Disposition, entry.Rationale, request.Manifest.Source); err != nil {
			return CapabilityBindingReleaseReport{}, fmt.Errorf("write disposition for %s: %w", entry.StableKey, err)
		}
		for _, binding := range entry.Bindings {
			var proposalID string
			var evidence []byte
			if binding.Evidence != nil {
				evidence, _ = json.Marshal(binding.Evidence)
			} else {
				evidence = []byte(`{}`)
			}
			if err := tx.QueryRow(ctx, `
				insert into content.question_capability_binding_proposal
				(workspace_id, revision_id, path_key, capability_key, role, provenance,
				 confidence, evidence, question_release_id, capability_registry_release_id,
				 status, rationale, source, decided_at, decided_by)
				values ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::jsonb, $9, $10,
				 'accepted', $11, $12, now(), $13)
				on conflict (workspace_id, revision_id, path_key, capability_key, role, capability_registry_release_id)
				do update set status = 'accepted', confidence = excluded.confidence,
				 evidence = excluded.evidence, rationale = excluded.rationale,
				 source = excluded.source, decided_at = now(), decided_by = excluded.decided_by
				returning id::text
			`, workspaceID, row.RevisionID, binding.PathKey, binding.CapabilityKey, binding.Role,
				binding.Provenance, binding.Confidence, evidence, report.QuestionReleaseID,
				report.CapabilityRegistryReleaseID, entry.Rationale, request.Manifest.Source, actor).Scan(&proposalID); err != nil {
				return CapabilityBindingReleaseReport{}, fmt.Errorf("write binding proposal for %s: %w", entry.StableKey, err)
			}
			if _, err := tx.Exec(ctx, `
				insert into content.question_capability_binding_release_item
				(binding_release_id, revision_id, path_key, capability_key, role, provenance, confidence, source_proposal_id)
				values ($1, $2::uuid, $3, $4, $5, $6, $7, $8::uuid)
				on conflict do nothing
			`, report.BindingReleaseID, row.RevisionID, binding.PathKey, binding.CapabilityKey,
				binding.Role, binding.Provenance, binding.Confidence, proposalID); err != nil {
				return CapabilityBindingReleaseReport{}, fmt.Errorf("write release binding for %s: %w", entry.StableKey, err)
			}
			if _, err := tx.Exec(ctx, `
				insert into content.question_capability
				(revision_id, path_key, capability_key, mapping_state, mapping_version, source,
				 role, provenance, confidence, question_release_id, capability_registry_release_id,
				 binding_release_id, source_proposal_id)
				values ($1::uuid, $2, $3, 'accepted', $4, $5, $6, $7, $8, $9, $10, $11, $12::uuid)
				on conflict (revision_id, path_key, capability_key) do update set
				 mapping_state = 'accepted', mapping_version = excluded.mapping_version,
				 source = excluded.source, role = excluded.role, provenance = excluded.provenance,
				 confidence = excluded.confidence, question_release_id = excluded.question_release_id,
				 capability_registry_release_id = excluded.capability_registry_release_id,
				 binding_release_id = excluded.binding_release_id, source_proposal_id = excluded.source_proposal_id
			`, row.RevisionID, binding.PathKey, binding.CapabilityKey, taxonomy.Version,
				request.Manifest.Source, binding.Role, binding.Provenance, binding.Confidence,
				report.QuestionReleaseID, report.CapabilityRegistryReleaseID, report.BindingReleaseID, proposalID); err != nil {
				return CapabilityBindingReleaseReport{}, fmt.Errorf("materialize question capability for %s: %w", entry.StableKey, err)
			}
		}
	}
	if _, err := tx.Exec(ctx, `
		insert into content.audit_event (workspace_id, aggregate_type, aggregate_id, event_type, actor, metadata)
		values ($1::uuid, 'question_capability_binding_release', $2::uuid, 'question.capability.bindings.released', $3, $4::jsonb)
	`, workspaceID, workspaceID, actor, metadata); err != nil {
		return CapabilityBindingReleaseReport{}, fmt.Errorf("write capability release audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return CapabilityBindingReleaseReport{}, fmt.Errorf("commit capability binding release: %w", err)
	}
	return report, nil
}

func capabilityBindingCoverage(entries []capabilitybinding.Entry, current map[string]currentCapabilityRevision) []CapabilityBindingCoverage {
	type key struct{ dimension, value string }
	rows := map[key]*CapabilityBindingCoverage{}
	ensure := func(dimension, value string) *CapabilityBindingCoverage {
		if strings.TrimSpace(value) == "" {
			value = "unmapped"
		}
		k := key{dimension: dimension, value: value}
		if rows[k] == nil {
			rows[k] = &CapabilityBindingCoverage{Dimension: dimension, Key: value}
		}
		return rows[k]
	}
	countDisposition := func(row *CapabilityBindingCoverage, disposition string) {
		row.Cards++
		switch disposition {
		case "bound":
			row.Bound++
		case "theory_only":
			row.TheoryOnly++
		case "needs_new_capability":
			row.NeedsCapability++
		case "rejected":
			row.Rejected++
		}
	}
	for _, entry := range entries {
		row, ok := current[entry.StableKey]
		if !ok {
			continue
		}
		values := map[string][]string{
			"path": []string{row.PathKey}, "domain": []string{row.DomainKey},
			"locale": row.Locales, "card_kind": []string{row.CardKind}, "topic": row.Topics,
		}
		for dimension, dimensionValues := range values {
			if len(dimensionValues) == 0 {
				dimensionValues = []string{"unmapped"}
			}
			seen := map[string]struct{}{}
			for _, value := range dimensionValues {
				value = strings.TrimSpace(value)
				if value == "" {
					value = "unmapped"
				}
				if _, exists := seen[value]; exists {
					continue
				}
				seen[value] = struct{}{}
				countDisposition(ensure(dimension, value), entry.Disposition)
			}
		}
		if len(entry.Bindings) == 0 {
			continue
		}
		for _, binding := range entry.Bindings {
			coverage := ensure("capability", binding.CapabilityKey)
			coverage.Cards++
			coverage.Bound++
		}
	}
	result := make([]CapabilityBindingCoverage, 0, len(rows))
	for _, row := range rows {
		result = append(result, *row)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Dimension == result[j].Dimension {
			return result[i].Key < result[j].Key
		}
		return result[i].Dimension < result[j].Dimension
	})
	return result
}

func (p *Postgres) currentCapabilityRevisions(ctx context.Context, workspaceKey string) (map[string]currentCapabilityRevision, string, error) {
	var workspaceID string
	if err := p.pool.QueryRow(ctx, `select id::text from content.workspace where stable_key = $1`, workspaceKey).Scan(&workspaceID); err != nil {
		return nil, "", fmt.Errorf("read capability binding workspace: %w", err)
	}
	rows, err := p.pool.Query(ctx, `
		select q.stable_key, qr.id::text, qr.content_hash,
		       coalesce(mapping.path_key, ''), coalesce(mapping.domain_key, ''),
		       coalesce(locales.locales, '{}'::text[]),
		       coalesce(topics.topics, '{}'::text[]),
		       coalesce(nullif(qr.normalized_payload->>'card_kind', ''),
		                nullif(qr.normalized_payload->>'group', ''), 'unknown')
		from content.question q
		join content.question_revision qr on qr.id = q.current_revision_id
		left join content.question_curriculum_mapping mapping on mapping.revision_id = qr.id
		left join lateral (
			select array_agg(distinct locale order by locale) as locales
			from content.question_locale where revision_id = qr.id
		) locales on true
		left join lateral (
			select array_agg(distinct topic.title order by topic.title) as topics
			from content.question_topic relation
			join content.topic topic on topic.id = relation.topic_id
			where relation.question_id = q.id
		) topics on true
		where q.workspace_id = $1::uuid and q.status = 'published' and q.content_kind = 'production'
		order by q.stable_key
	`, workspaceID)
	if err != nil {
		return nil, "", fmt.Errorf("query current capability revisions: %w", err)
	}
	defer rows.Close()
	result := make(map[string]currentCapabilityRevision)
	for rows.Next() {
		var row currentCapabilityRevision
		if err := rows.Scan(&row.StableKey, &row.RevisionID, &row.Hash, &row.PathKey, &row.DomainKey, &row.Locales, &row.Topics, &row.CardKind); err != nil {
			return nil, "", fmt.Errorf("scan current capability revision: %w", err)
		}
		result[row.StableKey] = row
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate current capability revisions: %w", err)
	}
	return result, workspaceID, nil
}

func (p *Postgres) activeCapabilities(ctx context.Context, keys map[string]struct{}) (map[string]struct{}, error) {
	if len(keys) == 0 {
		return map[string]struct{}{}, nil
	}
	values := make([]string, 0, len(keys))
	for key := range keys {
		values = append(values, key)
	}
	rows, err := p.pool.Query(ctx, `
		select stable_key from content.taxonomy_capability
		where stable_key = any($1::text[]) and lifecycle = 'active'
	`, values)
	if err != nil {
		return nil, fmt.Errorf("query active capability registry: %w", err)
	}
	defer rows.Close()
	result := make(map[string]struct{}, len(values))
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan active capability registry: %w", err)
		}
		result[key] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active capability registry: %w", err)
	}
	return result, nil
}
