# Compose stack

The project name is always `fluent-question-brain`. This makes the stack
obvious in Docker Desktop and gives operators a predictable stop/start path:

```sh
docker compose -p fluent-question-brain -f deploy/compose/compose.yaml up -d --build
docker compose -p fluent-question-brain -f deploy/compose/compose.yaml ps
docker compose -p fluent-question-brain -f deploy/compose/compose.yaml logs -f api
docker compose -p fluent-question-brain -f deploy/compose/compose.yaml down
```

The only persistent volume in G1 is the named Postgres volume. Backups and a
restore drill are mandatory before the legacy runtime is retired. Payload is
added as a separate `cms` service in G4, after its package lock and ownership
boundary are reviewed.

