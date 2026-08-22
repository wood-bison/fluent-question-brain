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

## Resource policy

The API has bounded HTTP timeouts. Database pool limits, worker concurrency,
queue lag, and Postgres memory settings become explicit configuration before
G3 load testing. A local Compose stack is not a production capacity claim.
