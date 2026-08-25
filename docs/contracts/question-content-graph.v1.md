# Question Brain content graph contract v1

This contract covers question-to-question semantic relations. It is separate
from the legacy `question_topic` placement graph, which only places a card in
an editorial topic.

## Ownership and lifecycle

Question Brain is the only writer. An editor or an enrichment job creates a
`question_edge_proposal`; a reviewer explicitly decides `accepted`,
`rejected`, or `superseded`. Only accepted proposals whose endpoints are the
current revisions of the same workspace can enter an immutable
`question_graph_release`. Proposed and rejected rows remain auditable but are
never returned as learner graph edges.

The release ID is deterministic from the sorted accepted proposal IDs and
revision IDs. The release stores the Question Brain question release ID and a
source hash, so a content revision or accepted relation change always requires
a new graph release. A rolled-back release is immutable and cannot be reused.

Supported relation kinds are:

`prerequisite`, `related`, `contrast`, `follow_up`, `variant`, `duplicate`,
and `supersedes`.

Every proposal records workspace, both stable keys and pinned revision IDs,
kind, optional confidence, rationale, source, creation time, and deciding
actor/time. Database triggers reject cross-workspace endpoints, missing
revisions, and self-edges. A transactional recursive check rejects an accepted
prerequisite that would create a cycle.

## HTTP API

Read routes are safe projections and do not expose learner answers or runtime
solutions:

```text
GET  /v1/graph/proposals?workspace=fluent-interview&status=proposed
GET  /v1/graph/releases/current?workspace=fluent-interview
GET  /v1/graph/releases/{graphReleaseID}
GET  /v1/graph/neighborhood/{questionStableKey}?workspace=fluent-interview
GET  /v1/graph/prerequisites/{questionStableKey}?workspace=fluent-interview
GET  /v1/graph/contrasts/{questionStableKey}?workspace=fluent-interview
GET  /v1/graph/variants/{questionStableKey}?workspace=fluent-interview
```

Mutation routes require `X-Question-Brain-Token` and an optional
`X-Question-Brain-Actor`:

```text
POST /v1/graph/proposals
POST /v1/graph/proposals/{proposalID}/decision
POST /v1/graph/releases       # {"approve": false} is a dry-run
POST /v1/graph/releases/{graphReleaseID}/rollback
```

The `question-brain.graph-edge.v1` contract version is present in every
projection. A blocked release returns HTTP 409 and lists stale endpoints or
cycle reasons. An active release can be approved repeatedly without creating
duplicate rows; a rolled-back deterministic release cannot be silently
reused.

`/v1/graph/releases/current` is a read-only operator projection of the active
immutable release. It is paired with `POST /v1/graph/releases` using
`{"approve": false}` to show a deterministic after-release ID before any
approval. Neither route mutates learner state.

## CLI boundary

`qb-graph-edges` is the operational equivalent of the API:

```sh
# propose, decide, dry-run, approve, export, and rollback
/qb-graph-edges -from question.q315 -to question.c009 \
  -kind prerequisite -confidence 0.91
/qb-graph-edges -proposal-id <uuid> -decision accepted
/qb-graph-edges -release
/qb-graph-edges -release -approve
/qb-graph-edges -export <graph-release-id>
/qb-graph-edges -rollback <graph-release-id>
```

`question_edge_release` has immutable rows and is workspace-guarded. Lab may
consume the API release projection, but it never reads Question Brain tables
directly.
