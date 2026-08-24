# G4 verification — TaskBrief boundary and Runtime ownership

Date: 2026-08-25
Status: **complete for the additive v1 boundary**

## Decision

Question Brain owns the learner-facing `TaskBrief`: condition, input/schema or
signature, walkthrough, difficulty, rubric, and an optional stable
`task_family_key`. Task Runtime owns executable source, starter workspace,
reference solution, hidden tests, harness, OCI image, limits, and sandbox
policy. Fluent Lab owns the attempt/run/evidence projection.

The new `question-brain.task-brief.v1` contract is opt-in so immutable legacy
revisions are not rewritten. Its strict validator accepts only the four task
kinds (`discussion_prompt`, `design_exercise`, `runtime_task_reference`,
`historical_content`), requires a family key for a runtime reference, and
rejects any embedded `solution`. Use `--strict-task-boundary` on both
`qb-import` and `qb-release` for a new batch or source release.

## Live production inventory

The rebuilt Question Brain API was checked at `http://127.0.0.1:48127` on
2026-08-25 against release `question-release-d550846f4743c4d3` (1591
production cards):

| Counter | Value | Meaning |
| --- | ---: | --- |
| `task_blocks` | 47 | historical/practical blocks currently visible in published cards |
| `embedded_solutions` | 43 | legacy blocks retaining source material for immutable provenance |
| `task_family_references` | 0 | no published card has opted into TaskBrief v1 yet |
| `task_boundary_violations` | 0 | no v1 card embeds a Runtime-owned solution |

The non-zero legacy counters are intentionally visible debt, not silently
rewritten data. The next content batch must convert a card to TaskBrief v1 and
join it to a released TaskFamily before it is published.

## Verification

- `go test ./...` passed in the Go 1.24 container.
- Compose API/indexer images rebuilt and containers restarted successfully.
- `/health/ready` returned `200` with `database=reachable`.
- `/v1/quality` returned the release and the four task-boundary counters above.
- Strict unit tests prove an embedded solution is rejected, a runtime brief
  without a family is rejected, and a legacy unversioned block remains
  readable.
- Historical question revisions and approved verification JSON files were not
  edited.

Contracts:

- `docs/contracts/task-brief.v1.md`
- `docs/contracts/question-revision.md`
