# Capability binding review HTTP contract v1

Question Brain remains the write authority for capability placement proposals.
The contract is answer-free and release-aware.

## Read

`GET /v1/capability-bindings/review?workspace=fluent-interview&status=proposed`

returns `question-brain.capability-binding-review.v1` with `proposals`. Each
proposal carries the card stable key, revision, path/capability keys, role,
provenance, confidence, evidence, question and registry release IDs, status and
current rationale. It does not return normalized payloads or localized answer
content.

## Decide

`POST /v1/capability-bindings/review/{proposalID}/decision`

requires `X-Question-Brain-Token`, accepts `accepted` or `rejected`, and
requires a rationale. `X-Question-Brain-Actor` is recorded in `decided_by` and
the audit event. The compare-and-set is idempotent for the same decision and
returns conflict for a competing decision. Acceptance alone does not alter the
learner projection: a validated binding release must still be published.
