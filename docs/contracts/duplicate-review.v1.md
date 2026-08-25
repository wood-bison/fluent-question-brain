# Question Brain duplicate review contract v1

Question Brain owns duplicate candidates and their decisions. The quality
report remains a diagnostic summary (for example, exact-prompt groups), while
the operator queue reads the durable `content.duplicate_candidate` table so a
candidate is never hidden merely because its prompts are not byte-for-byte
equal.

## Read

```text
GET /v1/duplicates/review?workspace=fluent-interview&status=proposed
```

`status=proposed` is the Workbench spelling for the table's `decision=open`
state. The response is `question-brain.duplicate-review.v1` and contains
revision-pinned stable keys, exact/semantic scores, current decision, and
actor. Only current production revisions are returned; stale or fixture rows
are excluded from the learner-facing review projection.

## Decide

```text
POST /v1/duplicates/decision
```

The request requires the internal Question Brain token, two distinct stable
keys, scores in `[0, 1]`, a terminal decision (`not_duplicate`,
`keep_separate`, or `merge`) and a rationale. The writer resolves both current
revisions transactionally, upserts the candidate, and records an audit event.
The browser never connects to Postgres.

This split is intentional: quality tells us where to investigate; the durable
review contract tells the operator exactly which candidate to resolve.
