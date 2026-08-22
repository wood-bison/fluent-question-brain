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
exists. A `topic_key` query parameter narrows the same release index to one
topic.

The catalog defaults to the learner-safe production release. Development
records are classified explicitly with `content_kind=fixture` and are excluded
from the default response. The response reports `include_fixtures=false`,
`excluded_fixtures`, and `excluded_non_production` so an operator can see that
the boundary was applied. A diagnostic request may opt in with
`include_fixtures=true`; the Lab never sends that flag and must not use the
resulting release for learner projections.

`GET /v1/release` is the complete machine-readable `QuestionRelease` manifest
(`question-brain.release.v1`). It pins every stable key to its current
revision/content hash, available locales, source reference, quality state, and
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

## Runtime read switch

The production-shaped Lab adapter is enabled after the published parity gate:

```text
QUESTION_BRAIN_READS=1
QUESTION_BRAIN_BASE_URL=http://127.0.0.1:48127
QUESTION_BRAIN_WORKSPACE=fluent-interview
QUESTION_BRAIN_TIMEOUT_MS=1200
```

The dependency-free reference adapter is committed at
`integrations/fluent-engineering-lab/question-brain-client.ts`; the Lab can
vendor it or copy the same contract into its own workspace without coupling
its learner UI to this repository's Go implementation details.

When the flag is `1`, only the read projection changes. The existing learner
contract remains the response shape; missing graph metadata is treated as a
closed, reviewable projection error rather than guessed in the browser. The
catalog is the bounded exception for profile-scoped `Explore freely`: it
allows the Lab to render every published card while marking placement and
mastery as provisional. Write and authoring operations stay in Payload → Go
API and are never proxied from the learner UI.

## Locale contract

The Lab sends the active UI locale explicitly (`en` or `ru`) on every card
read. If a locale is absent, the Go API returns the configured fallback and
still reports the actual `locale` in the response. UI copy and question content
are therefore switched independently without duplicating source records.

## Rollout and rollback

1. Run the parity report for the same stable-key slice against the current Lab
   archive and Question Brain.
2. Enable `QUESTION_BRAIN_READS=1` only in a disposable Lab process and compare
   counts, locale coverage, `content_hash`, and graph placement.
3. Promote the flag for one learner profile at a time; keep the old archive
   read path available as a read-only fallback for the rollout window.
4. Roll back by setting the flag to `0`; no data migration or schema rollback
   is required.

This is deliberately a contract and adapter seam, not a second copy of the
question registry. The full cutover report is
`fluent-engineering-lab/docs/verification/g4-lab-parity-2026-08-22.json`:
`1146/1146` archive questions match published Question Brain content across
the complete stable-key/locale slice, with zero missing, prompt, content, or
transport mismatches. The Lab archive is retained only as a read-only recovery
projection; source-vault publication is owned by the approved `qb-release`
command or Payload → Go promote boundary.
