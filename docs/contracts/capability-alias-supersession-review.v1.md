# Capability alias and supersession review v1

Question Brain is the only writer of canonical capability identity. A rename
is a reviewed registry fact, not a fuzzy search result and not a learner graph
edge.

## Proposal

`source_key` is the old or historical key. `canonical_key` is an existing
active registry capability. `action` is one of:

- `alias` — resolve the old key to the canonical key while retaining the old
  key as an alias;
- `supersedes` — record that an existing capability was replaced by the
  canonical capability and keep the old identity resolvable for historical
  evidence.

Every proposal is scoped to a workspace and carries a rationale, source, and
JSON provenance. The proposal is answer-free. It can be retried by its stable
tuple `(workspace, action, source_key, canonical_key)` without creating a
second decision record.

## HTTP boundary

Read and mutation routes are separate. Mutation routes require
`X-Question-Brain-Token`; `X-Question-Brain-Actor` is recorded in the audit
event.

```text
GET  /v1/capability-aliases/review?workspace=fluent-interview&status=proposed
POST /v1/capability-aliases/review
POST /v1/capability-aliases/review/{proposalID}/decision
```

Create payload:

```json
{
  "workspace_key": "fluent-interview",
  "action": "alias",
  "source_key": "capability.runtime.node-event-loop-001",
  "canonical_key": "capability.nodejs.event-loop-ordering",
  "reason": "remove task sequence from the observable identity",
  "source": "taxonomy-migration-2026-08-25",
  "provenance": {"migration": "g9-canonical-identities"}
}
```

Decision payload:

```json
{"decision":"accepted","rationale":"registry review confirmed the rename"}
```

The list projection uses contract
`question-brain.capability-alias-supersession-review.v1`; decisions use
`question-brain.capability-alias-supersession-decision.v1`.

## Transaction and safety rules

- `accepted` materialises exactly one row in
  `taxonomy_capability_alias` or `taxonomy_capability_supersedes` in the same
  transaction as the proposal decision.
- The canonical key must exist and be `lifecycle=active`.
- A `supersedes` source must exist; aliases may preserve a source label that
  predates the registry.
- Existing mappings to a different canonical key are a conflict, never
  last-write-wins.
- The database supersession trigger rejects cycles; the API returns HTTP 409.
- Repeating the same decision is idempotent. A different decision after a
  proposal is resolved returns `ErrReviewConflict` and cannot overwrite audit
  history.
- Rejecting a proposal changes only its review state. It never writes a
  registry relation and never changes learner progress or a released graph.

The learner projection consumes canonical keys only. Historical evidence is
resolved through the alias/supersession tables, while new releases fail closed
if they select a deprecated key.
