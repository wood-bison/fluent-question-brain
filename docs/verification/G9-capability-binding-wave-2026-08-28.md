# G9 capability binding wave — Node CPU-bound work

Date: 2026-08-28  
Owner: Question Brain editorial release  
Status: approved, immutable release published

## Scope

This wave promotes ten reviewed semantic-neighbor proposals for the Node.js +
TypeScript path and `capability.nodejs.cpu-bound-work`. The review selected
only prompts with a direct CPU/parallelism contract: worker selection,
CPU-core scaling, libuv thread-pool pressure, synchronous blocking,
SharedArrayBuffer/Atomics, and cluster execution. Java, .NET, generic
distributed-systems, and unrelated event-loop candidates remain proposed.

Accepted proposal IDs:

| Question | Proposal ID | Review decision |
| --- | --- | --- |
| `question.q806` | `96937470-ed6a-41b4-8b05-b1668191cb70` | accepted |
| `question.q224` | `ec2792c5-5622-43ed-a9a8-be5a8ee2754f` | accepted |
| `question.q798` | `60ad7164-6432-41d1-9374-c7463dfbc872` | accepted |
| `question.q225` | `f862f21a-f261-4561-9ba5-8eb83fcb930b` | accepted |
| `question.q776` | `4a078a72-39fa-4504-89b8-16479d1759fc` | accepted |
| `question.q769` | `1a25bbca-5765-4427-9f91-04b163c3f76c` | accepted |
| `question.q228` | `b986dc44-bdd0-4b61-b37f-4824d53d9882` | accepted |
| `question.q802` | `e989be1c-bfe4-422c-8731-3a32e950a37c` | accepted |
| `question.q804` | `160bc599-73da-4a20-b2aa-2d7ace3d810b` | accepted |
| `question.c100` | `8f25509a-a143-4c7b-8159-ee2205803acd` | accepted |

All rows are supporting evidence for the canonical capability. The release
compiler, rather than this editorial wave, decides the learner-visible role;
no task runtime revision or completion result is inferred by acceptance.

## Release evidence

- Manifest: `G9-capability-binding-manifest-2026-08-28.json`
- Approved report: `G9-capability-binding-release-2026-08-28.json`
- Binding release: `question-capability-release-c382a5cf2626758b`
- Question release: `question-release-d00a14931e607336`
- Capability registry: `capability-registry-2026-08-25-v3`
- Manifest entries: 1,591
- Bound cards: 46
- Binding rows: 48
- Theory-only cards: 1,545
- Invalid/stale/missing/extra entries: 0/0/0/0

The release is contract-valid and approved, but the complete learner
curriculum remains open. Lesson/checkpoint/activity authoring and the
remaining capability queue must be closed before a production promotion.
