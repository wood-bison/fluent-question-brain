# G1 one-card round-trip evidence

Date: 2026-08-22

Source card: `fluent-question-vault/Question Cards/Q001 — Kafka vs RabbitMQ vs ActiveMQ.md`

Command used inside the Compose network:

```sh
go run ./cmd/qb-import \
  -database-url 'postgres://question_brain:question_brain@postgres:5432/question_brain?sslmode=disable' \
  -file '/vault/Question Cards/Q001 — Kafka vs RabbitMQ vs ActiveMQ.md'
```

Observed result:

```text
imported stable_key=legacy.q001 revision_id=688be8ab-6d22-45ed-8306-b5abd3e94052 content_hash=a07b771cb03e5d20a51a0cf266c193750e7dd5cdec32ffa2b9d8dec024312455 round_trip=ok
```

Running the same import twice produced one revision, one locale, one proposed
placement, one audit event, and one outbox event. The second run returned the
same revision id and hash. The canonical payload is hashed after JSON
normalization, so Postgres JSONB key ordering cannot create a false change.

The Compose smoke script also verified the pgvector extension, canonical
tables, the search-document trigger, and API health against a fresh/healthy
stack.

Still open in G1: an explicit duplicate candidate, a placement decision, and
an automated migration test. Search remains gated until those evidence records
exist.
