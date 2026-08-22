# Compose stack

The project name is always `fluent-question-brain`. This makes the stack
obvious in Docker Desktop and gives operators a predictable stop/start path:

```sh
docker compose -p fluent-question-brain -f deploy/compose/compose.yaml up -d --build
docker compose -p fluent-question-brain -f deploy/compose/compose.yaml ps
docker compose -p fluent-question-brain -f deploy/compose/compose.yaml logs -f api jaeger
docker compose -p fluent-question-brain -f deploy/compose/compose.yaml down
```

## Reserved local ports

| Surface | Host port | Container port |
| --- | ---: | ---: |
| Go API | `48127` | `8080` |
| Payload authoring studio | `48128` | `3000` |
| Postgres + pgvector | `55437` | `5432` |
| Jaeger UI | `56686` | `16686` |
| Jaeger OTLP/gRPC | `54317` | `4317` |
| Jaeger OTLP/HTTP | `54318` | `4318` |

The host bindings are loopback-only. The API exports OTLP/gRPC to
`jaeger:4317` inside the Compose network; open `http://localhost:56686` to
inspect local traces.

The only persistent volume is the named Postgres volume. Payload uses the same
database connection but only the isolated `cms` schema; its migrations are
committed under `apps/cms/src/migrations` and run before the CMS server starts.
Backups and a restore drill are mandatory before the legacy runtime is retired.
