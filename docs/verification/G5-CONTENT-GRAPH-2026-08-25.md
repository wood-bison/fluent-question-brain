# G5 — reviewed Question Brain content graph evidence

Date: 2026-08-25
Repository: `fluent-question-brain`
Contract: `question-brain.graph-edge.v1`

## Boundary

Question Brain now owns the reviewed semantic content graph. The legacy
`content.question_edge`/`question_topic` placement surfaces remain readable
for historical editorial placement; new question-to-question relations are
written only through the revision-aware graph API/CLI. Lab consumes released
projections and does not read these tables directly.

## Implemented controls

- `question_edge_proposal` stores workspace, pinned endpoint revisions, one of
  seven relation kinds, status, confidence, rationale, source, timestamps, and
  decision actor.
- `question_graph_release` and `question_edge_release` are separate. Released
  rows are immutable and contain only accepted proposals.
- Database triggers reject missing/cross-workspace endpoints and self-edges on
  both proposals and releases. Foreign keys prevent dangling revisions.
- The accepted prerequisite decision runs a recursive cycle check while the
  proposal is locked. A release repeats cycle and stale-revision validation.
- Graph release IDs are deterministic from sorted accepted proposal/revision
  identities and carry the current Question Brain release ID and source hash.
- Active release approval is idempotent. A rolled-back deterministic release
  is immutable and cannot be silently reused.
- `question-brain.graph-edge.v1` is exposed by the proposal, release, and
  neighborhood APIs. Mutations require the internal token and actor header.
- `qb-graph-edges` provides proposal, decision, dry-run, approve, export, and
  rollback operations.

## Live fixture release

The current production Question Brain release is
`question-release-d00a14931e607336`. The reviewed fixture release is:

```text
question-graph-release-7c9d2bf4a73a5d49
status: active
accepted: 7
released: 7
stale endpoints: 0
prerequisite cycles: 0
```

The seven released edges include `prerequisite`, `related`, `contrast`,
`follow_up`, `variant`, and `supersedes` (with two reviewed follow-up edges).
The fixture also contains two rejected proposals:
one explicit duplicate candidate and one reverse prerequisite that attempted to
create a cycle. Rejected rows remain visible in
`GET /v1/graph/proposals?status=rejected` and neither appears in the release.

The prerequisite chain is `question.q315 → question.c009`; the cycle attempt
`question.c009 → question.q315` returned HTTP 409 and was then explicitly
rejected with an actor and rationale. The graph release contains no duplicate
or non-accepted edge.

## Verification

```text
docker compose ... build api                         PASS
docker run golang:1.24-bookworm go test ./...         PASS
make check                                            PASS
make smoke                                            PASS
make graph-smoke                                      PASS
GET /health/ready                                     PASS
GET /v1/graph/proposals                               PASS
GET /v1/graph/releases/{id}                           PASS
GET /v1/graph/neighborhood/question.q315              PASS
GET /v1/graph/prerequisites|contrasts|variants       PASS
POST cycle decision                                   HTTP 409 (expected)
concurrent accept decisions                           both accepted, one row
repeat approve while active                           released=0 (idempotent)
rollback then reuse same deterministic release        rejected (immutable)
same q315 revision for EN and RU                      PASS (locale is not identity)
```

`graph-smoke.sh` verifies both database workspace guards, accepted-only
materialisation, the API projection, and the containerized CLI export. The
migration is repeatable through `scripts/apply-question-graph-migration.sh`.
