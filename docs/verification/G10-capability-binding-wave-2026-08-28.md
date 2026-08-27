# G10 capability binding wave — Node memory and bounded concurrency

Date: 2026-08-28  
Owner: Question Brain editorial release  
Status: approved after fail-closed path remediation

## Scope

This wave reviewed two related Node.js capability slices. Six direct memory
retention prompts and five direct bounded-concurrency prompts were accepted
for the Node.js + TypeScript path. A seventh concurrency candidate
(`question.q1047`) was intentionally revoked when the release compiler found
that its current canonical path was `path.go`; the Node wording alone was not
allowed to override the source-of-truth path.

### Accepted memory-retention proposals

| Question | Proposal ID |
| --- | --- |
| `question.q227` | `b18a5050-8b63-4624-926e-ac55bd260bd1` |
| `question.q100` | `5c2a8e84-2495-420f-8b21-7baac4699745` |
| `question.q808` | `68522182-e7ae-43ad-b8a8-e07e1e281dee` |
| `question.q818` | `3e40d94c-c9cb-4229-afc2-0a7dc6ab10a2` |
| `question.q817` | `78f1daff-8f8d-4ccd-b5d2-4c52f4f8166b` |
| `question.q815` | `a792e8b2-8ad9-40fc-a6ad-a18cc2e42da6` |

The prompts cover high-memory diagnosis, NestJS leaks, worker-pool buildup,
V8 external memory and leak-safe references, common leak patterns, and V8
heap tuning.

### Accepted bounded-concurrency proposals

| Question | Proposal ID |
| --- | --- |
| `question.q046` | `fb749508-953c-4d1d-8fa2-9c56507e9dd1` |
| `question.q045` | `1cf6513e-53d5-4d09-90c1-e2c0a23503d2` |
| `question.q312` | `84b43551-4895-4b1a-8854-1905269e435d` |
| `question.q779` | `37f931b2-7c5d-4c59-816a-7f4cccdc1d81` |
| `question.q361` | `92d3c4ad-86f1-457e-8f32-98f424bff20e` |

The accepted prompts cover Promise fan-out, request concurrency, database
pool bottlenecks, Promise combinator choices, and production pool exhaustion.

### Fail-closed remediation

| Question | Proposal ID | Outcome |
| --- | --- | --- |
| `question.q1047` | `6760bee1-3d14-46b5-a084-fcbc299c4153` | revoked via integrity API |

The attempted G10 dry-run stopped with a typed path mismatch before approval.
The proposal was then revoked with actor
`question-brain-integrity-remediation-w20`, and the final dry-run/approval
completed successfully. The mismatch and revocation remain in the Brain
audit event stream.

## Release evidence

- Manifest: `G10-capability-binding-manifest-2026-08-28.json`
- Approved report: `G10-capability-binding-release-2026-08-28.json`
- Binding release: `question-capability-release-52e0e40e9fb286c1`
- Question release: `question-release-d00a14931e607336`
- Capability registry: `capability-registry-2026-08-25-v3`
- Manifest entries: 1,591
- Bound cards: 56
- Binding rows: 59
- Theory-only cards: 1,535
- Invalid/stale/missing/extra entries: 0/0/0/0

The release is approved and contract-valid. It does not close the remaining
lesson, checkpoint, activity, primary-question, or supporting-prompt targets;
Lab must consume this release and keep those gaps explicit.
