# G16 capability binding wave — .NET/C# await mechanics

Date: 2026-08-28  
Owner: Question Brain editorial release  
Status: approved, immutable release published

## Scope

G16 closes the two remaining direct `.NET/C#` async candidates identified in
the current catalog: the compiler-generated `async`/`await` state machine and
the synchronization-context boundary represented by `ConfigureAwait(false)`.
Both cards are attached to the existing
`capability.dotnet.cancellation-boundary` capability as
`supporting_evidence` on the canonical `path.dotnet-csharp` path.

This is a reviewed crosswalk increment only. It does not create runnable tasks,
lessons, checkpoints, or a production-ready .NET curriculum. Those remain
explicit Lab backlog items.

The review verified the exact stable key, current revision, content hash,
canonical `metadata.path_key`, capability, role, question release, and registry
release before acceptance. The fail-closed release compiler then validated the
complete 1,591-card manifest.

## Accepted reviewed proposals

Accepted by actor `question-brain-g16-dotnet-async-review` from
`capability-registry-2026-08-25-v3`:

| Stable key | Proposal ID | Capability | Canonical path |
| --- | --- | --- | --- |
| `question.q280` | `8131a7d2-a880-4b0a-8a01-4a993ee91a03` | `capability.dotnet.cancellation-boundary` | `path.dotnet-csharp` |
| `question.q282` | `be6f89f3-9bbc-4457-a419-aee1bc5e81d8` | `capability.dotnet.cancellation-boundary` | `path.dotnet-csharp` |

No path-mismatched proposal was accepted in this wave. Existing v3 bindings
from G15 and earlier waves were retained without duplicate identities.

## Release evidence

- Manifest: `G16-capability-binding-manifest-2026-08-28.json`
- Approved report: `G16-capability-binding-release-2026-08-28.json`
- Binding release: `question-capability-release-4c9a0a309536f892`
- Question release: `question-release-d00a14931e607336`
- Capability registry: `capability-registry-2026-08-25-v3`
- Manifest entries: 1,591
- Bound cards: 114
- Binding rows: 149
- Theory-only cards: 1,477
- Invalid/stale/missing/extra entries: 0/0/0/0
- `blocked=false`, `approved=true`

The next projection must consume this exact immutable tuple. The overall
curriculum remains a development slice until the Lab lesson/checkpoint/activity
targets and the remaining Brain coverage queue are closed and re-verified.
