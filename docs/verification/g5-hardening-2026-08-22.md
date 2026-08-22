# G5 production-hardening evidence — 2026-08-22

Status: **passed locally** against the `fluent-question-brain` Compose stack.
The commands below are repeatable; the final output is captured from the
release run.

## Gates covered

| Drill | Command | Result |
| --- | --- | --- |
| Migration boundary | `bash scripts/migration-smoke.sh` | passed |
| Load and latency | `bash scripts/load-smoke.sh` | passed, 60 requests, p95 40.25 ms |
| Backup + restore | `bash scripts/backup-restore-smoke.sh` | passed, 20,090,314-byte dump, 1,372 questions restored |
| Failure injection | `bash scripts/failure-injection-smoke.sh` | passed, API and Jaeger recovered |
| Immutable rollback | `bash scripts/rollback-smoke.sh` | passed, pointer/audit/outbox path verified |
| Redaction | `go test ./internal/telemetry -run TestHTTPWithMetricsCorrelatesSafely` | passed |

## Observability contract

- `GET /metrics` reports request, error, duration, response-byte, and
  in-flight counters without route/query labels.
- JSON completion logs include `request_id`, `trace_id`, `span_id`, status, and
  duration only. Query strings and bodies are covered by the redaction test and
  are not emitted.
- Jaeger receives the same server span through OTLP/gRPC and remains a sink,
  not a content store.

## Backup and restore

The drill uses `pg_dump -Fc`, restores into a disposable
`question_brain_restore_smoke` database, verifies that the canonical question
table is non-empty, and drops only that disposable database. It does not touch
the named production-like volume.

## Rollback semantics

`POST /v1/questions/{stableKey}/rollback` is internal-token protected. It
updates only the current-revision pointer; immutable revision rows are never
rewritten. The transaction records `question.revision.rolled_back` and an
indexer outbox event atomically. A second rollback can restore the prior
revision using its recorded `revision_id`.

## Retention decision

Local Jaeger storage is transient. The production baseline is 30 days for
redacted logs, 90 days for metrics, 14 days for traces, and at least 365 days
for audit/outbox history; a deployment may change these only through the
release checklist with an owner and restore test.
