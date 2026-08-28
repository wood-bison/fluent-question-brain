# G14 capability binding wave — Node/Nest runtime depth

Date: 2026-08-28  
Owner: Question Brain editorial release  
Status: approved, immutable release published

## Scope

G14 extends the Node.js/TypeScript learner path with reviewed supporting
evidence for deferred execution, Promise combinators and async iterators,
NestJS OAuth/JWT and object-level authorization, V8 heap/GC and resource
retention, worker selection, and the Node/Bun runtime boundary. A binding is
searchable evidence for a capability; it is not a runnable activity,
lesson, or completion checkpoint.

Every candidate was checked against the released catalog before acceptance:
the proposal stable key, revision, canonical `metadata.path_key`, capability
key, and role had to agree. The release compiler is the final fail-closed
guard.

## Accepted reviewed proposals

The following 15 proposals were accepted as `supporting_evidence` by actor
`question-brain-g14-node-nest-review`:

| Stable key | Proposal ID | Capability |
| --- | --- | --- |
| `question.q777` | `725ce0d9-35e7-478f-87c7-e39d524d2f3f` | `capability.runtime.deferred` |
| `question.c098` | `eb4db756-8360-46e1-b08f-28103719d7ba` | `capability.runtime.deferred` |
| `question.q046` | `96f4e021-2ba1-4e3b-b5d2-ac373d910cc7` | `capability.runtime.deferred` |
| `question.q779` | `e244d38f-33eb-4cc5-8d28-e60459389372` | `capability.runtime.deferred` |
| `question.q168` | `96f83120-7bd3-43ff-85b6-33f3373a95f5` | `capability.runtime.deferred` |
| `question.q783` | `4393d146-a0a4-4e63-b59f-9391ca7e5c7e` | `capability.runtime.deferred` |
| `question.q1000` | `d629b85e-4a19-4d33-958d-12167c5b76e5` | `capability.http-api.authentication-authorization` |
| `question.q416` | `f38d1392-b5ec-4c40-8feb-75542175897d` | `capability.http-api.authentication-authorization` |
| `question.q814` | `a5f04481-a0f3-4e15-9b92-d6f3e6beae10` | `capability.nodejs.memory-retention` |
| `question.q812` | `8cf2cda2-e031-49cb-bd19-522945c48028` | `capability.nodejs.memory-retention` |
| `question.q794` | `0609303f-c514-4986-88b7-38c532827a81` | `capability.nodejs.memory-retention` |
| `question.q798` | `90caefe2-4666-444c-a207-110e554a1b48` | `capability.nodejs.bounded-concurrency` |
| `question.q775` | `21a3d2cf-ff33-4753-82ca-47b3fac11891` | `capability.nodejs.bounded-concurrency` |
| `question.q806` | `31b533c5-e9e7-4e73-b68d-7e6baa19d9a3` | `capability.nodejs.bounded-concurrency` |
| `question.q948` | `06368713-d523-486d-b2da-d77c9445f05e` | `capability.nodejs.event-loop-ordering` |

The release adds 15 new binding rows and six new bound cards relative to the
G13D projection; other accepted rows already had a valid reviewed capability
crosswalk and are retained without duplicate card identities.

## Rejected path-mismatched proposals

Five auth proposals were initially accepted during review, then immediately
revoked through the integrity endpoint by actor
`question-brain-g14-integrity-remediation`. The released catalog maps
`question.q426`, `question.q427`, `question.q716`, `question.c040`, and
`question.c005` to `path.system-design`, while the proposals claimed
`path.nodejs-typescript`. They are therefore excluded rather than silently
reclassified into the Node path.

## Release evidence

- Manifest: `G14-capability-binding-manifest-2026-08-28.json`
- Approved report: `G14-capability-binding-release-2026-08-28.json`
- Binding release: `question-capability-release-a58d8763d4f628a4`
- Question release: `question-release-d00a14931e607336`
- Capability registry: `capability-registry-2026-08-25-v3`
- Manifest entries: 1,591
- Bound cards: 98
- Binding rows: 133
- Theory-only cards: 1,493
- Invalid/stale/missing/extra entries: 0/0/0/0
- `blocked=false`, `approved=true`

The release remains a content slice, not a production-ready curriculum. The
Lab projection still owns lesson/checkpoint/activity completion and keeps the
remaining theory-only queue explicit.
