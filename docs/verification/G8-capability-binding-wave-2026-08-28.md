# G8 capability binding wave — Node streams and backpressure

Date: 2026-08-28  
Owner: Question Brain editorial release  
Status: approved, immutable release published

## Scope

This wave promotes eight reviewed semantic-neighbor proposals for the
Node.js + TypeScript path. Each candidate was inspected for an explicit
Node streams/backpressure contract before being accepted through the
authenticated review API. Unrelated Java, Kafka, generic error-handling, and
cross-runtime candidates remain proposed and are not silently promoted.

Accepted proposal IDs:

| Question | Proposal ID | Review decision |
| --- | --- | --- |
| `question.q797` | `1e665ef1-b1a0-4bd3-8754-9f801753eaea` | accepted |
| `question.q790` | `846b90c2-93d2-4dec-8717-dd9d882faf7f` | accepted |
| `question.q788` | `2f062c6a-7e06-45b4-beb0-d08fef104347` | accepted |
| `question.c099` | `003917e8-601a-4e70-a34e-4627341710cf` | accepted |
| `question.q223` | `39b8a5df-6519-4611-a190-1006139a98e1` | accepted |
| `question.q789` | `43fe06c9-6af9-44ec-9150-0076c22f4cf6` | accepted |
| `question.q795` | `f43bfbba-445b-432c-a411-0a2a07b5cdf4` | accepted |
| `question.q793` | `7180084e-26a5-4b91-96a0-d5f415cd121a` | accepted |

The accepted prompts cover production stream patterns, `highWaterMark`, the
four stream types, memory-bounded flow, Readable modes, async iteration, and
object mode. They are supporting evidence for the canonical capability
`capability.nodejs.streams-backpressure`; the release compiler still decides
whether a card is a learner-visible primary question or supporting prompt.

## Release evidence

- Manifest: `G8-capability-binding-manifest-2026-08-28.json`
- Approved report: `G8-capability-binding-release-2026-08-28.json`
- Binding release: `question-capability-release-d4af7d903f948362`
- Question release: `question-release-d00a14931e607336`
- Capability registry: `capability-registry-2026-08-25-v3`
- Manifest entries: 1,591
- Bound cards: 36
- Binding rows: 38
- Theory-only cards: 1,555
- Invalid/stale/missing/extra entries: 0/0/0/0

The release is contract-valid and approved. It is not a claim that the full
production curriculum is complete: the Lab path projection still reports
open lesson, activity, checkpoint, primary-question, and supporting-prompt
backlog. The next wave must review another bounded capability set and then
re-run the cross-repository release gates.
