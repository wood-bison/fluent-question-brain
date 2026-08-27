# G12 capability binding wave — Node async depth and integrity remediation

Date: 2026-08-28  
Owner: Question Brain editorial release  
Status: approved, immutable release published

## Scope

G12 adds a reviewed, answer-free slice of direct Node.js/runtime evidence. The
wave deepens `capability.nodejs.event-loop-ordering` with microtask/macrotask
ordering, async/await scheduling, the poll phase, libuv socket boundaries,
process-level async failures, and `nextTick` starvation. It also adds direct
`capability.runtime.deferred` prompts for async debugging, promisify/callbackify,
async iterators, NestJS lifecycle hooks, and browser/runtime scheduling
comparisons. Every accepted row was checked against the current canonical
`path.nodejs-typescript` mapping before review.

The wave deliberately did not accept cache, system-design, Bun, database, or
other semantically adjacent candidates merely because vector similarity was
high. A stale accepted cache slice discovered during preflight was revoked
through the auditable integrity endpoint before release generation.

## Accepted reviewed proposals

All rows are `supporting_evidence`; acceptance does not create a runnable task
or change learner completion.

| Stable key | Proposal ID | Capability | Review note |
| --- | --- | --- | --- |
| `question.q222` | `d9221d97-1ef1-44e9-a0d4-115dfc2d1cb9` | `capability.nodejs.event-loop-ordering` | `nextTick` vs Promise microtasks vs `setImmediate` |
| `question.q168` | `fe4d7878-0675-451d-8968-365cec81b385` | `capability.nodejs.event-loop-ordering` | browser Promise/microtask/macrotask comparison |
| `question.q228` | `4aa4dbb4-2350-4076-9ed3-9b18a3d25616` | `capability.nodejs.event-loop-ordering` | synchronous file/JSON work and event-loop blocking |
| `question.q778` | `37fa1053-e403-47bc-bf7a-f4bfbdd21f66` | `capability.nodejs.event-loop-ordering` | async/await suspension and scheduling |
| `question.q768` | `69a10662-2df5-44e7-802a-f76784f48643` | `capability.nodejs.event-loop-ordering` | poll phase and timer/I/O interaction |
| `question.q770` | `a039f736-a49d-419e-b292-931bfce5e3d4` | `capability.nodejs.event-loop-ordering` | kernel async sockets and libuv boundary |
| `question.q781` | `1bf5b057-7026-48d8-b898-734a26febe1c` | `capability.nodejs.event-loop-ordering` | unhandled rejection/uncaught exception policy |
| `question.q773` | `090534ea-cca1-4aa8-8ed3-bc537fb8c1b0` | `capability.nodejs.event-loop-ordering` | unbounded `nextTick` starvation |
| `question.av-137` | `9bfc0565-f8e9-445e-98e7-23a670a54488` | `capability.runtime.deferred` | predict console ordering as a practice prompt |
| `question.q787` | `24dcf48d-36f8-45bf-badd-e78743dcd050` | `capability.runtime.deferred` | async debugging, stack and timing evidence |
| `question.q778` | `b90f4d99-cfed-496c-a96f-ad98c757f435` | `capability.runtime.deferred` | async/await internals (second reviewed capability) |
| `question.q786` | `d13a9df8-b9ce-42dd-928f-d5adf250415f` | `capability.runtime.deferred` | `util.promisify`/`callbackify` boundary |
| `question.q783` | `70db239a-d3a2-48b9-802c-9bb5124f95d5` | `capability.runtime.deferred` | async iterators and generators |
| `question.q966` | `12c6d5fa-714d-4c34-b3a8-bcd2fd2fe899` | `capability.runtime.deferred` | NestJS lifecycle and shutdown hooks |
| `question.q452` | `a273145c-b0a5-4c12-a4f8-06ad77fced0c` | `capability.runtime.deferred` | browser rendering/event-loop interleave |

## Revocations before release

The release compiler correctly failed closed on five previously accepted
cache-invalidation proposals whose proposal path was `path.nodejs-typescript`
while the current canonical mapping was `path.system-design`. They were
revoked with actor `question-brain-integrity-remediation-g12` and remain in
audit history; no invalid row is present in G12:

`2faabaab-ac31-4d3f-8095-df3e4dc2ede2`,
`a4dc8639-5ea7-455d-a1e5-22cb57c9b8ef`,
`5c1d0932-1a4b-4824-b0ba-8a7eabf5cde1`,
`e41785eb-bbe4-48dd-8771-5a37ed566780`,
`4c84e606-c48d-416b-b5b6-1f990118dfa8`.

## Release evidence

- Manifest: `G12-capability-binding-manifest-2026-08-28.json`
- Approved report: `G12-capability-binding-release-2026-08-28.json`
- Binding release: `question-capability-release-1f6516006bbfd61d`
- Question release: `question-release-d00a14931e607336`
- Capability registry: `capability-registry-2026-08-25-v3`
- Manifest entries: 1,591
- Bound cards: 70
- Binding rows: 82
- Theory-only cards: 1,521
- Invalid/stale/missing/extra entries: 0/0/0/0

Dry-run and approval both returned `blocked=false`, `invalid=0`,
`missing_manifest=0`, and `extra_manifest=0`. The release is safe to consume
only through the pinned Lab projection; lessons, checkpoints, primary slots,
activities, and the remaining theory-only coverage are still open.
