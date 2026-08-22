# Operations runbook

## Start and stop

```sh
docker compose -p fluent-question-brain -f deploy/compose/compose.yaml up -d --build
docker compose -p fluent-question-brain -f deploy/compose/compose.yaml ps
docker compose -p fluent-question-brain -f deploy/compose/compose.yaml logs --tail=200 api cms postgres jaeger
docker compose -p fluent-question-brain -f deploy/compose/compose.yaml down
```

The stack reserves loopback-only host ports so it does not collide with the
older Lab runtime: API `48127`, Payload `48128`, Postgres `55437`, Jaeger UI
`56686`, OTLP/gRPC `54317`, and OTLP/HTTP `54318`. Change them only through the `QB_*_PORT`
variables and keep the values distinct.

## Authoring and publish

Open Payload at `http://localhost:48128/admin`. Drafts and versions are stored
in the `cms` schema. A publish calls the Go API with the internal token; the Go
transaction creates the immutable revision, audit event, and outbox item. If
the API is unavailable, Payload returns the publish error and the draft stays
available for retry.

The index worker can be run as a one-shot Compose-network job while the worker
service is being promoted to a long-running deployment:

```sh
docker run --rm --network fluent-question-brain_default \
  -v "$PWD":/src -w /src golang:1.24-bookworm \
  sh -c 'go run ./cmd/qb-index --database-url postgres://question_brain:question_brain@postgres:5432/question_brain?sslmode=disable'
```

## Tracing

The Go API emits HTTP server spans through OpenTelemetry OTLP/gRPC to the
local Jaeger all-in-one service. Open `http://localhost:56686`, make a request
to `http://localhost:48127/`, and select service `question-brain-api`.
Jaeger in this development Compose stack uses transient in-memory storage;
production retention requires an explicit persistent backend and collector.

## Metrics and correlation

`GET /metrics` exposes dependency-free Prometheus text with request count,
error count, cumulative duration, response bytes, and in-flight requests. It
intentionally has no route or query labels, preventing untrusted input from
creating metric-cardinality explosions. Every API request has a stable
`X-Request-ID` and a structured completion log with `request_id`, `trace_id`,
and `span_id`; query strings, bodies, and secrets are excluded.

Production retention is an explicit deployment policy, not a local Jaeger
default:

| Signal | Local Compose | Production baseline | Rule |
| --- | --- | --- | --- |
| API logs | Docker log driver | 30 days | JSON, redacted, rotate by size and age |
| Metrics | none by default | 90 days | remote-write or durable Prometheus storage |
| Traces | Jaeger memory | 14 days | persistent backend, restricted access |
| Audit/outbox | Postgres | 365 days minimum | retain immutable audit; archive published outbox |

Adjust the baseline only through a release decision that records the owner,
retention reason, and deletion/restore test.

## Readiness checks

`/health/live` only answers whether the process is alive. `/health/ready`
requires the Postgres TCP endpoint to be reachable. SQL migration state is
also checked in the deployment smoke test; a TCP-open database is not proof
that the schema is current.

## Data safety

The named volume is `fluent-question-brain-postgres`. Back it up before any
destructive migration, test a restore into a fresh volume, and record the
result in the release checklist. Do not use `docker compose down -v` against a
production project.

The operator rollback endpoint is internal-token protected:

```sh
curl -X POST http://localhost:48127/v1/questions/<stable-key>/rollback \
  -H 'content-type: application/json' \
  -H "x-question-brain-token: ${QUESTION_BRAIN_INTERNAL_TOKEN}" \
  -d '{"revision_id":"<immutable-revision-id>"}'
```

Rollback changes only `content.question.current_revision_id`; the previous
revision remains immutable and the pointer update, audit event, and outbox
notification commit together.

## Resource policy

The API has bounded HTTP timeouts. Database pool limits, worker concurrency,
queue lag, and Postgres memory settings become explicit configuration before
G3 load testing. A local Compose stack is not a production capacity claim.

## Release drills

Run `make g5-smoke` before a release. It checks the committed migration
boundary, exercises 60 live/search requests, restores a custom-format
`pg_dump` into a disposable database, restarts the API and Jaeger, and rolls a
published question back to an earlier immutable revision. The captured output
belongs in `docs/verification/g5-hardening-2026-08-22.md` and the signed
retirement checklist.
