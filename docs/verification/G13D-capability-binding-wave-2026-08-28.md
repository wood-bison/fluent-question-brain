# G13D capability binding wave — Node orchestration depth

Date: 2026-08-28  
Owner: Question Brain editorial release  
Status: approved, immutable release published

## Scope

G13D is the next bounded Node.js/runtime wave after G13C. It covers the
orchestration boundary that learners meet in production Node/Nest services:
offloading slow work to BullMQ and choosing NestJS microservice transport and
message-pattern semantics. The release keeps the distinction between a
supporting question and a runnable activity: a bound card is searchable
evidence for a capability, not an implicit lab or completion checkpoint.

Generation and approval used the immutable question release
`question-release-d00a14931e607336` and registry
`capability-registry-2026-08-25-v3`. The canonical path guard ran before
approval and rejected any proposal whose claimed path differed from the
released question revision.

## Newly promoted reviewed proposals

These two cards were not bound in G13C and are newly present in the G13D
manifest. Both retain their stable key, revision, path, and proposal
provenance.

| Stable key | Proposal ID | Capability | Review note |
| --- | --- | --- | --- |
| `question.q1024` | `999c11ad-9414-4560-b16f-dba837c8fc53` | `capability.nodejs.event-loop-ordering` | BullMQ queue registration, retry/backoff, worker processing, and dead-letter handling keep slow work off the NestJS request loop |
| `question.q1029` | `922ec906-d3b6-4623-b205-bf7653b69feb` | `capability.nodejs.event-loop-ordering` | NestJS TCP/Redis/RabbitMQ/Kafka transports, `@MessagePattern`/`@EventPattern`, acknowledgements, retries, and hybrid apps |

The reviewed `q768` poll-phase and `q770` kernel-async-I/O proposals were
already represented by valid released cards; the compiler deduplicated the
card identity while preserving the valid binding provenance. They therefore do
not inflate the newly-bound-card count.

## Integrity guard exercised

Proposal `acb2bf81-6c4f-42cc-93f8-3db8db69a4d3` (`question.q1072`) was accepted
for review and then revoked through the integrity endpoint by actor
`question-brain-g13d-integrity-remediation`. It claimed `path.nodejs-typescript`,
but the canonical released revision is mapped to `path.frontend`. The compiler
failed closed for that row, so it is excluded from G13D rather than silently
reclassified.

## Release evidence

- Manifest: `G13D-capability-binding-manifest-2026-08-28.json`
- Approved report: `G13D-capability-binding-release-2026-08-28.json`
- Binding release: `question-capability-release-da343f56ed8db7bc`
- Question release: `question-release-d00a14931e607336`
- Capability registry: `capability-registry-2026-08-25-v3`
- Manifest entries: 1,591
- Bound cards: 80 (G13C: 78; +2)
- Binding rows: 106 (G13C: 102)
- Theory-only cards: 1,511
- Invalid/stale/missing/extra entries: 0/0/0/0
- `blocked=false`, `approved=true`

The release is consumable only through the pinned Lab projection. Remaining
lesson, checkpoint, activity, primary-slot, and theory-only work stays in the
curriculum backlog; G13D does not manufacture missing learning stations.
