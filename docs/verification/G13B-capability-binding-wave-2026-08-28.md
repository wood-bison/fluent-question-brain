# G13B capability binding wave — Node runtime foundations

Date: 2026-08-28  
Owner: Question Brain editorial release  
Status: approved, immutable release published

## Scope

G13B is a bounded follow-up to G12. It accepts only direct Node.js/runtime
evidence from the reviewed queue: deferred execution, timer semantics, stream
cleanup/backpressure, and worker-thread lifecycle. These rows enrich the
question graph and supporting evidence; they do not manufacture runnable labs
or mark a learner task complete.

The compiler was run against the current canonical path mapping and capability
registry. One tempting bounded-concurrency proposal (`question.q1071`) was
accepted briefly for review, then revoked before release because its proposal
claimed the stale `path.nodejs-typescript` mapping while the canonical card is
on `path.frontend`. It is therefore theory-only in G13B and is not part of the
published binding release.

## Accepted reviewed proposals released in G13B

All rows are `supporting_evidence` and retain their immutable question/revision
identity. A bound row is searchable evidence, not a new activity or checkpoint.

| Stable key | Proposal ID | Capability | Review note |
| --- | --- | --- | --- |
| `question.q099` | `667e2133-cb6d-4f42-a182-d41ad4bd8ee8` | `capability.runtime.deferred` | Node event-loop overview and deferred-execution foundations |
| `question.q780` | `4ba570a4-b19d-44b7-9586-b980b151c464` | `capability.runtime.deferred` | async/await failure propagation, missing `await`, and floating promises |
| `question.q772` | `13c0773a-aa01-4081-ba6f-61d26f7b3ddb` | `capability.nodejs.event-loop-ordering` | zero-delay timers, timer drift, and Node scheduling semantics |
| `question.q794` | `fc0a266b-7d66-4ce0-a8e5-cce7be78b03d` | `capability.nodejs.streams-backpressure` | stream error cleanup, `destroy`, half-open sockets, `finished`, and `pipeline` |
| `question.q799` | `1d799d3b-bdf1-4062-862e-db89eeecae59` | `capability.nodejs.cpu-bound-work` | `worker_threads` creation, messaging, termination, and error handling |

`question.q099` and `question.q772` also have the existing reviewed
`capability.nodejs.event-loop-ordering`/`capability.runtime.deferred` supporting
links where the registry proves them; the release compiler deduplicates the
card while preserving all valid binding rows.

## Integrity guard exercised

Proposal `1704c80c-ebeb-4ca8-a871-a3cf26ba6411` (`question.q1071`) was revoked
through the auditable review endpoint by actor
`question-brain-g13b-integrity-remediation`. The reason was a path mismatch:
the proposal used `path.nodejs-typescript`, while the canonical question
revision is mapped to `path.frontend`. The compiler therefore failed closed for
that row; G13B contains no invalid, stale, missing, or extra manifest entry.

## Release evidence

- Manifest: `G13B-capability-binding-manifest-2026-08-28.json`
- Approved report: `G13B-capability-binding-release-2026-08-28.json`
- Binding release: `question-capability-release-1bdb768174ee1cbd`
- Question release: `question-release-d00a14931e607336`
- Capability registry: `capability-registry-2026-08-25-v3`
- Manifest entries: 1,591
- Bound cards: 73
- Binding rows: 87
- Theory-only cards: 1,518
- Invalid/stale/missing/extra entries: 0/0/0/0

Generation and approval both returned `blocked=false`. The release is safe to
consume only through the pinned Lab projection. The remaining lesson,
checkpoint, activity, primary-slot, and theory-only queues stay explicit in the
curriculum backlog and are not silently filled by this wave.
