# Operations runbook

## Start and stop

```sh
docker compose -p fluent-question-brain -f deploy/compose/compose.yaml up -d --build
docker compose -p fluent-question-brain -f deploy/compose/compose.yaml ps
docker compose -p fluent-question-brain -f deploy/compose/compose.yaml logs --tail=200 api postgres
docker compose -p fluent-question-brain -f deploy/compose/compose.yaml down
```

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

