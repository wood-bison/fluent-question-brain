# Question graph release contract

`cmd/qb-graph-release` is the only command that turns the deterministic
source-topic placement proposals into the published Question Brain graph. A
normal vault import may create `content.placement_decision` rows with
`decision = proposed`, but it never mutates learner-visible graph edges.

The command is deliberately two-phase:

```sh
# Inspect the current immutable question release (no write)
go run ./cmd/qb-graph-release \
  --database-url "$QUESTION_BRAIN_DATABASE_URL" \
  --workspace fluent-interview \
  --report graph-release-dry-run.json

# Publish only after the report is reviewed
go run ./cmd/qb-graph-release \
  --database-url "$QUESTION_BRAIN_DATABASE_URL" \
  --workspace fluent-interview \
  --actor release-operator \
  --approve \
  --report graph-release.json
```

The report is `question-brain.graph-placement.v1`. It records the question
release ID, deterministic graph release ID, inspected question count,
proposed/accepted placement counts, materialized edges, and blocking reasons.
The approved operation is transactional and idempotent:

- every published production revision must have exactly one source-topic
  proposal in the same workspace;
- the edge is materialized as `question_topic(relation = primary)`;
- proposed decisions become `accepted` with actor, method, and graph release
  provenance;
- one audit event and one outbox event are written per graph release ID; and
- re-running the same release creates no duplicate edges or events.

This baseline intentionally proves graph coverage, not semantic prerequisite
quality. Curated `prerequisite`, `related`, and `contrast` edges remain an
authored extension and must be reviewed as a later graph revision rather than
silently inferred from embeddings.

The current production verification is saved at
[`docs/verification/g6-graph-placement-release-2026-08-22.json`](../verification/g6-graph-placement-release-2026-08-22.json).
