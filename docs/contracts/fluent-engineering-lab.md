# Fluent Engineering Lab integration contract (G4)

Fluent Engineering Lab remains the learner-facing product. Question Brain is
the independent source for canonical question revisions, bilingual content,
graph placement, and retrieval. The Lab must not connect to the `content` or
`cms` schemas directly.

## Read boundary

The Lab talks to the versioned HTTP API only:

```text
POST /v1/search
GET  /v1/questions/{stable_key}?locale=en|ru
GET  /v1/catalog?workspace=fluent-interview&locale=en&offset=0&limit=2000
GET  /v1/release?workspace=fluent-interview
```

`GET /v1/catalog` is the release-aware index used by the Lab's `Explore
freely` mode. It returns only the current published revision for each card,
typed placement metadata, topic relations, locale availability, and a
deterministic `release_id`. It intentionally does not return answer bodies;
the Lab follows a selected `stable_key` with `GET /v1/questions/{stable_key}`.
The response contract is `question-brain.catalog.v1`, and `total`/`offset`/
`limit` make the index safe to page or cache without guessing whether a card
exists. Optional `topic_key`, `track`, `level`, and `company` query parameters
narrow the same release index without changing its release identity. These
dimensions come from canonical card metadata; they are filters, not a second
taxonomy.

The catalog defaults to the learner-safe production release. Development
records are classified explicitly with `content_kind=fixture` and are excluded
from the default response. The response reports `include_fixtures=false`,
`excluded_fixtures`, and `excluded_non_production` so an operator can see that
the boundary was applied. A diagnostic request may opt in with
`include_fixtures=true`; the Lab never sends that flag and must not use the
resulting release for learner projections.

Each catalog item's optional `metadata` may carry the explicit curriculum
crosswalk from `question-brain.taxonomy.v1`:

```json
{
  "program_key": "program.backend-engineer",
  "path_key": "path.nodejs-typescript",
  "domain_key": "domain.runtime",
  "capability_key": "capability.runtime.event-loop",
  "mapping_state": "accepted",
  "mapping_version": "question-brain.taxonomy.v1"
}
```

The Lab must only project rows with an explicit `path_key`/`domain_key` (and,
for station-level work, `capability_key`) into its curriculum graph. Missing
keys mean `unmapped`; `proposed` is review debt, not learner readiness. The
legacy `stage_key` field is retained as a compatibility projection of
`domain_key` for older Lab clients and is not a second taxonomy. `Track`,
`Group`, and legacy `Topic` are content metadata only and must never be used as
an inferred path, domain, or capability. The revision-scoped
`content.question_capability` relation is many-to-many and release-aware.

`GET /v1/release` is the complete machine-readable `QuestionRelease` manifest
(`question-brain.release.v1`). It pins every stable key to its current
revision/content hash, available locales, source system, quality state, and
graph state (`released`, `accepted-pending`, `proposed`, or `unplaced`). Its
`checks` block makes missing locales and graph blockers measurable without
opening answer bodies. `source_snapshot_id` is the deterministic content
fingerprint used by the release; a graph publication remains a separate,
explicit release boundary.

Every search response carries `provenance.explainable=true`, the active
pipeline (`exact`, `fts`, `trigram`, and the semantic profile), per-result
match stages, and a stable `revision_id`/`content_hash`. The Lab can safely
cache a response by `(workspace, locale, query, topic_key, revision_id)` and
invalidate it when the graph release changes.

## Runtime read boundary

The production Lab adapter is always pointed at the released Question Brain
boundary:

```text
QUESTION_BRAIN_BASE_URL=http://127.0.0.1:48127
QUESTION_BRAIN_WORKSPACE=fluent-interview
QUESTION_BRAIN_TIMEOUT_MS=1200
```

The dependency-free reference adapter is committed at
`integrations/fluent-engineering-lab/question-brain-client.ts`; the Lab can
vendor it or copy the same contract into its own workspace without coupling
its learner UI to this repository's Go implementation details.

The learner contract remains the response shape; missing graph metadata is
treated as a closed, reviewable projection error rather than guessed in the
browser. The catalog is the only index for profile-scoped `Explore freely`:
it renders every published card while marking placement and mastery as
provisional. Write and authoring operations stay in Payload → Go API and are
never proxied from the learner UI.

## Locale contract

The Lab sends the active UI locale explicitly (`en` or `ru`) on every card
read. If that locale is absent, the Go API returns a typed not-found result;
it never substitutes another language. UI copy and question content are
therefore switched independently without hiding translation debt.

## Rollout and rollback

1. Run the parity report for the same stable-key slice against the released
   Question Brain catalog.
2. Compare counts, locale coverage, `content_hash`, and graph placement in a
   disposable Lab process.
3. Promote the same released boundary to the learner environment only after
   the report is green; no alternate content source is enabled.

This is deliberately a contract and adapter seam, not a second copy of the
question registry. The full cutover report is
`fluent-engineering-lab/docs/verification/g4-lab-parity-2026-08-22.json`:
`1146/1146` source questions match published Question Brain content across the
complete stable-key/locale slice, with zero missing, prompt, content, or
transport mismatches. Source-vault publication is owned by the approved
`qb-release` command or Payload → Go promote boundary.
