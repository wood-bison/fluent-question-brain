# G4 Payload → Go promote evidence (2026-08-22)

## Runtime

The Compose stack is `fluent-question-brain` with unique loopback ports:

| Service | Host port | Check |
| --- | ---: | --- |
| Go API | `48127` | `/health/live`, `/health/ready` |
| Payload authoring | `48128` | `/admin` (200) |
| Postgres | `55437` | `cms` and `content` schemas |
| Jaeger | `56686` | service catalog/UI |

Payload is pinned to `3.88.0`, Next to `16.3.2`, Node image `22-bookworm-slim`.
Its Postgres adapter uses `schemaName: cms`, `push: false`, and a committed
Payload migration. The container applies `payload migrate` before starting
the standalone Next server.

## End-to-end assertion

An authenticated Payload REST publish was executed with both locales for
`g4.cms5`. The hook sent one canonical JSON payload to the Go API:

```text
POST Payload /api/questions?locale=all
→ POST Go /v1/promote (internal token)
→ content.question status = published
→ content.question_revision source_system = payload-cms
→ content.audit_event event_type = question.promoted
→ content.outbox_event → qb-index
```

Observed API reads after the publish:

```text
GET /v1/questions/g4.cms5?locale=en → prompt "What does CMS publish?"
GET /v1/questions/g4.cms5?locale=ru → prompt "Что публикует CMS?"
status = published for both responses
```

The outbox worker then processed 3 promote events and wrote 5 deterministic
dev-profile vectors with zero failures.

## Boundary checks

- Direct unauthenticated CMS collection reads are denied (`403`).
- Direct learner writes to the Go API are not exposed; `/v1/promote` requires
  `X-Question-Brain-Token` and returns `401` without it.
- Payload and Go API have separate runtimes and separate schema ownership.
- A failed promote returns an error to the editor, so a publish cannot appear
  successful while the canonical revision is missing.

The Lab read switch remains opt-in and is documented in
`docs/contracts/fluent-engineering-lab.md`; its final parity evidence belongs
to the Lab repository, not to this CMS boundary.
