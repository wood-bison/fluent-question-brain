# G15 capability binding wave — .NET/C# async and cancellation boundaries

Date: 2026-08-28  
Owner: Question Brain editorial release  
Status: approved, immutable release published

## Scope

G15 is a deliberately bounded content wave for the `.NET/C#` learner path. It
reviews fourteen released cards that directly discuss async/concurrency,
`CancellationToken`, cancellation propagation, or the related C# runtime
boundary. The cards are attached to the existing
`capability.dotnet.cancellation-boundary` capability as
`supporting_evidence`.

This is a crosswalk release, not a claim that fourteen runnable labs or
completion checkpoints exist. The Lab projection must continue to keep lesson,
checkpoint, activity, and runtime readiness gaps explicit.

Before acceptance, every proposal was checked against the current released
catalog: stable key, revision, canonical `metadata.path_key`, capability key,
role, and question release all had to agree. The release compiler and the
exact registry release are fail-closed guards.

## Accepted reviewed proposals

The following proposals were accepted by actor
`question-brain-g15-dotnet-async-review` from
`capability-registry-2026-08-25-v3`:

| Stable key | Proposal ID | Capability | Canonical path |
| --- | --- | --- | --- |
| `question.c113` | `a62f7ad0-993d-43b5-8c1b-cd352dc6c8b7` | `capability.dotnet.cancellation-boundary` | `path.dotnet-csharp` |
| `question.q1129` | `b87f0363-cef8-4fb2-9237-f9d52d43c5fe` | `capability.dotnet.cancellation-boundary` | `path.dotnet-csharp` |
| `question.q1131` | `caedcd68-a579-4001-b582-0969e402d747` | `capability.dotnet.cancellation-boundary` | `path.dotnet-csharp` |
| `question.q1133` | `54283ff3-0f90-4532-b9c8-458520a4ef05` | `capability.dotnet.cancellation-boundary` | `path.dotnet-csharp` |
| `question.q1134` | `5c8cfa3d-ff48-4716-8644-813594dc0d30` | `capability.dotnet.cancellation-boundary` | `path.dotnet-csharp` |
| `question.q281` | `cdabceb9-47f7-48fd-aa2c-9f2b8f5a7b51` | `capability.dotnet.cancellation-boundary` | `path.dotnet-csharp` |
| `question.q283` | `b195a5cb-4b2d-47a5-93f6-053f0be4fe40` | `capability.dotnet.cancellation-boundary` | `path.dotnet-csharp` |
| `question.q295` | `52b8399b-9ea9-4339-a8fd-84c0cc3b4abe` | `capability.dotnet.cancellation-boundary` | `path.dotnet-csharp` |
| `question.q456` | `2707a3ae-7116-479c-8d4e-63cfb3a36aa0` | `capability.dotnet.cancellation-boundary` | `path.dotnet-csharp` |
| `question.q923` | `6f511525-870f-433e-8e8f-56444f0a6518` | `capability.dotnet.cancellation-boundary` | `path.dotnet-csharp` |
| `question.q936` | `231ba177-3772-4486-98b6-259b536967fd` | `capability.dotnet.cancellation-boundary` | `path.dotnet-csharp` |
| `question.q939` | `19484fe2-fe94-4666-91c1-25159a91fbe2` | `capability.dotnet.cancellation-boundary` | `path.dotnet-csharp` |
| `question.q940` | `b06dc25d-43de-424c-ab52-0065e65493c4` | `capability.dotnet.cancellation-boundary` | `path.dotnet-csharp` |
| `question.q941` | `3c0072fb-f153-4f53-aa16-79e1392b4f16` | `capability.dotnet.cancellation-boundary` | `path.dotnet-csharp` |

The release compiler reports all fourteen as `bound`, with no invalid, missing,
or extra manifest entries. No new capability key was invented in this wave.

## Stale proposal remediation

The first review pass found duplicate proposals from the older registry-v2
snapshot. Those stale accepted rows were revoked before the release was
compiled; no stale row is part of the published v3 tuple. Their exact IDs,
actor, and rationale are preserved in the Question Brain audit log rather than
duplicated in the immutable release manifest.

They were revoked by actor `question-brain-g15-registry-integrity` with an
explicit supersession rationale. The immutable release consumes only the v3
IDs listed above; the revocation audit events remain the source of truth for
the superseded IDs (they are intentionally not copied into the release
manifest).

## Release evidence

- Manifest: `G15-capability-binding-manifest-2026-08-28.json`
- Approved report: `G15-capability-binding-release-2026-08-28.json`
- Binding release: `question-capability-release-6aed990a298ef65b`
- Question release: `question-release-d00a14931e607336`
- Capability registry: `capability-registry-2026-08-25-v3`
- Manifest entries: 1,591
- Bound cards: 112
- Binding rows: 147
- Theory-only cards: 1,479
- Invalid/stale/missing/extra entries: 0/0/0/0
- `blocked=false`, `approved=true`

The release is a content slice. It does not make the overall curriculum
production-ready: the Lab projection still owns executable stations and must
close the remaining theory-only, lesson, checkpoint, activity, and runtime
queues in later bounded waves.
