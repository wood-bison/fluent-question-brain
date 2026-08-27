# G13C capability binding wave — Node runtime depth

Date: 2026-08-28  
Owner: Question Brain editorial release  
Status: approved, immutable release published

## Scope

G13C is a bounded Node.js/runtime follow-up to G13B. The wave adds only
practice-ready supporting evidence whose canonical question revision is already
in the released question set and whose proposal path matches the authoritative
path map. It deepens timer scheduling, production event-loop diagnosis, stream
cancellation, and worker-pool/transfer-list mechanics. A bound card is
searchable evidence for a capability; it is not silently promoted to a new lab,
checkpoint, or completion state.

The compiler ran against the immutable question release and capability registry
`capability-registry-2026-08-25-v3`. It failed closed for path-mismatched
proposals before approval; only the path-valid rows below entered the release.

## Newly promoted reviewed proposals

These five rows were not bound in G13B and are newly present in the G13C
manifest. They retain the stable question key, immutable revision, and proposal
provenance. Existing valid bindings remain unchanged and are deduplicated by the
release compiler.

| Stable key | Proposal ID | Capability | Review note |
| --- | --- | --- | --- |
| `question.q1025` | `74512017-4657-4c76-a3d8-3315cd0e19af` | `capability.nodejs.event-loop-ordering` | NestJS `@Cron`/`@Interval`/`@Timeout` scheduling and the multi-instance timer trap |
| `question.q359` | `1f880041-6074-4801-9315-dbfce512e3c1` | `capability.nodejs.cpu-bound-work` | Diagnose a Node.js p99 regression at 5K RPS and distinguish event-loop blocking |
| `question.q784` | `c329fd84-8a78-42a2-8912-4a0446081ec1` | `capability.nodejs.streams-backpressure` | Propagate `AbortController` cancellation through a stream pipeline |
| `question.q800` | `c25b9b86-5a56-4c3d-b674-29ef52b59f8d` | `capability.nodejs.cpu-bound-work` | `worker_threads` cloning versus transfer-list semantics |
| `question.q803` | `380d3de0-6a89-4f8e-a6e6-6e916381d09b` | `capability.nodejs.cpu-bound-work` | Reusable worker-pool design and Piscina sizing trade-offs |

## Integrity guard exercised

Three current-v3 proposals were accepted for review and then revoked before
release because their claimed Node path disagreed with the canonical question
revision:

| Proposal ID | Stable key | Canonical path | Claimed path | Result |
| --- | --- | --- | --- | --- |
| `14d41957-cbe9-4997-b0df-10dc514b9a34` | `question.c026` | `path.system-design` | `path.nodejs-typescript` | revoked; excluded |
| `de113371-288d-46ac-ab31-39ef2e4c5ef1` | `question.q1100` | `path.frontend` | `path.nodejs-typescript` | revoked; excluded |
| `5c6c8560-bf9b-49ee-a0e8-102de374c9b9` | `question.q1101` | `path.frontend` | `path.nodejs-typescript` | revoked; excluded |

The compiler therefore remained fail-closed: no invalid, stale, missing, or
extra manifest row was approved. A handful of older registry-v2 review rows
remain in the audit history, but they are outside the v3 source/registry tuple
and cannot affect G13C.

## Release evidence

- Manifest: `G13C-capability-binding-manifest-2026-08-28.json`
- Approved report: `G13C-capability-binding-release-2026-08-28.json`
- Binding release: `question-capability-release-185232db4689818b`
- Question release: `question-release-d00a14931e607336`
- Capability registry: `capability-registry-2026-08-25-v3`
- Manifest entries: 1,591
- Bound cards: 78 (G13B: 73; +5)
- Binding rows: 102 (G13B: 87)
- Theory-only cards: 1,513
- Invalid/stale/missing/extra entries: 0/0/0/0
- `blocked=false`, `approved=true`

The release is consumable only through the pinned Lab projection. Lesson,
checkpoint, activity, primary-slot, and remaining theory-only queues stay
explicit in the curriculum backlog; this wave does not fabricate missing
learning stations.
