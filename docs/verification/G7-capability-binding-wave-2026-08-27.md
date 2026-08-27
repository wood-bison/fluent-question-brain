# G7 capability-binding wave — 2026-08-27

This evidence records the first bounded content wave after the production
closure audit. It is intentionally small and reviewable: it closes ten
high-confidence Node.js event-loop placements and remediates one stale accepted
proposal that made the release compiler fail closed.

## Integrity remediation

Proposal `23bab19f-1949-45ed-8dac-ac027f04afb2` (`question.q315`) was accepted
by an old G9 smoke test with `path.nodejs-typescript`, while the current card
revision belongs to `path.system-design`. The proposal was revoked through the
internal API, not by direct database mutation:

- endpoint: `POST /v1/capability-bindings/review/{proposalID}/revoke`
- actor: `question-brain-integrity-remediation-w18`
- audit event: `question.capability.binding.proposal.revoked`
- immutable historical releases were left unchanged

The release compiler then generated successfully, proving that stale accepted
proposals no longer poison the next projection.

## Reviewed placements

The following current v3 proposals were accepted after manual prompt/revision
review and are now part of the generated manifest:

| Card | Capability | Path | Role |
| --- | --- | --- | --- |
| `question.q099` | `capability.nodejs.event-loop-ordering` | `path.nodejs-typescript` | `supporting_evidence` |
| `question.q767` | `capability.nodejs.event-loop-ordering` | `path.nodejs-typescript` | `supporting_evidence` |
| `question.q785` | `capability.nodejs.event-loop-ordering` | `path.nodejs-typescript` | `supporting_evidence` |
| `question.q771` | `capability.nodejs.event-loop-ordering` | `path.nodejs-typescript` | `supporting_evidence` |
| `question.q276` | `capability.nodejs.event-loop-ordering` | `path.nodejs-typescript` | `supporting_evidence` |
| `question.c097` | `capability.nodejs.event-loop-ordering` | `path.nodejs-typescript` | `supporting_evidence` |
| `question.q777` | `capability.nodejs.event-loop-ordering` | `path.nodejs-typescript` | `supporting_evidence` |
| `question.q045` | `capability.nodejs.event-loop-ordering` | `path.nodejs-typescript` | `supporting_evidence` |
| `question.q773` | `capability.nodejs.event-loop-ordering` | `path.nodejs-typescript` | `supporting_evidence` |
| `question.q044` | `capability.nodejs.event-loop-ordering` | `path.nodejs-typescript` | `supporting_evidence` |

`question.q777` also keeps its existing reviewed primary binding to
`capability.runtime.deferred`; the new event-loop row is additive and
deduplicated by the manifest merger.

## Release evidence

- manifest: `G7-capability-binding-manifest-2026-08-27.json`
- report: `G7-capability-binding-release-2026-08-27.json`
- question release: `question-release-d00a14931e607336`
- registry release: `capability-registry-2026-08-25-v3`
- binding release: `question-capability-release-e7d6f9ad743d4f43`
- entries: `1591`
- bound: `28`
- theory-only: `1563`
- binding rows: `30`
- invalid / missing / extra: `0 / 0 / 0`
- dry-run and approved release: `blocked=false`

The release is a development/content wave, not a claim that the production
curriculum is complete. The remaining path, activity, checkpoint, runtime
family, and coverage queues stay visible in the root release gates.
