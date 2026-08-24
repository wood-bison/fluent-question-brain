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
2026-08-25 against release `question-release-d00a14931e607336` (1591
production cards):

| Counter | Value | Meaning |
| --- | ---: | --- |
| `task_blocks` | 48 | 47 migrated historical briefs plus one reviewed runtime brief |
| `embedded_solutions` | 0 | current learner projection has no Runtime-owned solutions |
| `task_family_references` | 1 | `question.q315` → `task-family.rate-limiter` |
| `task_boundary_violations` | 0 | no v1 card embeds a Runtime-owned solution |

The 47 previous blocks were migrated to new current revisions with
`historical_content`; their old revisions remain immutable and auditable. The
rate-limiter conceptual card now demonstrates the complete join: one theory
question → one TaskBrief → one released TaskFamily with language revisions.

## Verification

- `go test ./...` passed in the Go 1.24 container.
- Compose API/indexer images rebuilt and containers restarted successfully.
- `/health/ready` returned `200` with `database=reachable`.
- `/v1/quality` returned the release and the four task-boundary counters above.
- `/v1/questions/question.q315?locale=en` returned the safe TaskBrief with
  `task-family.rate-limiter`; no solution or hidden-test material crossed the
  API boundary.
- Strict unit tests prove an embedded solution is rejected, a runtime brief
  without a family is rejected, and a legacy unversioned block remains
  readable.
- Historical question revisions and approved verification JSON files were not
  edited.

Contracts:

- `docs/contracts/task-brief.v1.md`
- `docs/contracts/question-revision.md`
- `scripts/migrate_task_blocks_to_taskbrief_v1.sql`
- `scripts/link_rate_limiter_taskbrief.sql`
