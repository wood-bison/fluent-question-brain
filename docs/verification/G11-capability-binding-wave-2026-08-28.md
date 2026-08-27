# G11 capability binding wave — deferred execution and event-loop ordering

Date: 2026-08-28  
Owner: Question Brain editorial release  
Status: approved, immutable release published

## Scope

Eight direct Node.js async scheduling prompts were accepted for
`capability.runtime.deferred` on `path.nodejs-typescript`. The slice covers
microtask checkpoints, six event-loop phases, `nextTick`/Promise/
`setImmediate` ordering, starvation, Promise resolution, process-level async
failures, and timer drift. Browser-only and unrelated language prompts stayed
out of the release.

Accepted proposal IDs:

| Question | Proposal ID |
| --- | --- |
| `question.q771` | `172494c4-c417-4be6-8b59-ee9fdf5fad27` |
| `question.q767` | `b155120e-e157-4596-ac09-8913d7fdea76` |
| `question.q222` | `6f64f5cc-e739-4ea5-b72e-67048aaff0c5` |
| `question.q785` | `293663a6-3ab8-4c3f-898b-2b31a3ad4512` |
| `question.q773` | `e2f554ce-ee00-4e08-93d0-323571b9a223` |
| `question.c142` | `8a698027-8c10-4567-af0c-2416a25484b1` |
| `question.q781` | `8bc7f9d3-8e1a-4a37-bd22-7341912d51ef` |
| `question.q772` | `37e3c1e5-4cae-4164-8fc7-d7ca136ed789` |

All rows are supporting evidence. Acceptance does not create a runnable task,
change a learner completion result, or bypass the immutable Lab placement
compiler.

## Release evidence

- Manifest: `G11-capability-binding-manifest-2026-08-28.json`
- Approved report: `G11-capability-binding-release-2026-08-28.json`
- Binding release: `question-capability-release-c07677c65057b105`
- Question release: `question-release-d00a14931e607336`
- Capability registry: `capability-registry-2026-08-25-v3`
- Manifest entries: 1,591
- Bound cards: 59
- Binding rows: 67
- Theory-only cards: 1,532
- Invalid/stale/missing/extra entries: 0/0/0/0

The release is contract-valid and approved. Curriculum lessons, checkpoints,
activities, primary selection, and remaining capability review are still
open and must be closed in later bounded waves.
