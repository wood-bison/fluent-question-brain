# Fluent Question Brain

Performance-oriented, reusable question graph and retrieval service.

This is a new repository for the question system. It is intentionally
separate from `fluent-engineering-lab` (the interview-learning application)
and from `fluent-question-vault` (the Obsidian card mirror). The new service
owns the canonical content graph, revisions, search, duplicate detection,
placement decisions, and indexing jobs. Fluent Engineering Lab remains the
product that guides a learner through interview preparation and will consume
this service when the integration gate is opened.

## Architecture decision

The first production boundary is a modular Go service backed by one
PostgreSQL database with `pgvector`, `pg_trgm`, and full-text search. There is
no second vector database and no graph database in the steady-state path.

Payload CMS is part of the product, but it is an authoring surface rather than
another source of truth. Payload drafts live in the `cms` schema and are
promoted through an explicit command into the Go-owned `content` schema. This
keeps versions, localization, review, and search indexing useful without
allowing two writers to silently diverge.

## Current milestone

G0–G5 are closed locally. The full bilingual vault is reconciled and released
as one approved immutable snapshot (`1368/1368` cards, zero duplicate stable
keys/content hashes), exact/FTS/trigram/semantic retrieval is explainable,
Payload drafts publish through a token-protected Go API boundary, and
metrics/logs, backup restore, failure recovery, and immutable revision rollback
are covered by repeatable smoke scripts. Fluent Engineering Lab is still the
learner product; its published-only Question Brain read path is enabled with an
immutable archive fallback. The full Lab parity report is committed in that
repository, so the source-vault cutover is complete without archiving Lab.

See:

- [ARCHITECTURE.md](ARCHITECTURE.md) — system boundaries and performance rules
- [ROADMAP.md](ROADMAP.md) — gated delivery plan; later gates cannot start while
  an earlier gate has an open blocker
- [docs/contracts/question-revision.md](docs/contracts/question-revision.md) —
  canonical content contract
- [docs/ARCHIVE-BOUNDARY.md](docs/ARCHIVE-BOUNDARY.md) — how the old question
  registry remains recoverable while the new system is built
- [docs/verification/g5-vault-release-2026-08-22.json](docs/verification/g5-vault-release-2026-08-22.json)
  — approved source-vault release report

## Run the contract stack

Requirements: Docker Desktop with Compose.

```sh
cp .env.example .env
docker compose -f deploy/compose/compose.yaml up --build
```

The database is exposed on `localhost:55437`, the Go API on
`localhost:48127`, the Payload authoring studio on `localhost:48128`, and the
local Jaeger UI on `localhost:56686`.

```sh
curl -i http://localhost:48127/health/live
curl -i http://localhost:48127/health/ready
curl -i http://localhost:48127/metrics
# Search with explainable provenance
curl -sS -X POST http://localhost:48127/v1/search \
  -H 'content-type: application/json' \
  -d '{"query":"event loop","locale":"en","limit":5}'
# Payload authoring studio
open http://localhost:48128/admin
# Trace UI (local Jaeger all-in-one)
open http://localhost:56686
```

All host bindings are loopback-only and reserved for this stack. Override the
`QB_*_PORT` variables in `.env` only as a complete, collision-free set; the
service-to-service ports inside Compose stay on their standard values.

## Local checks

```sh
make check
# Full local hardening drill: migration, load, backup/restore, outage, rollback
make g5-smoke
```

`make check` validates the SQL/Compose contract and runs Go tests when a Go
toolchain is available. CI will pin a current supported Go toolchain and run
the same checks on every change.
