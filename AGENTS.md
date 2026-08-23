# AGENTS.md — Fluent Question Brain

Question Brain is the independent, reusable source of truth for interview and
other learning questions. It stores immutable localized revisions, typed graph
relationships, search indexes, embeddings, placement decisions, and release
audit history. Fluent Engineering Lab consumes released projections from this
service; it does not own a second copy of the question graph.

## Goals

- Support tens of thousands of questions without a second vector or graph
  database until measured latency/recall evidence justifies one.
- Make every import, duplicate decision, placement, translation, embedding and
  graph release deterministic, reviewable and reversible.
- Provide explainable exact, full-text, trigram and semantic retrieval with
  locale/workspace filters and provenance in every result.
- Give editors a productive Payload CMS surface while keeping one canonical
  transactional writer for published content.
- Serve multiple Fluent products through a stable versioned API, not through
  direct SQL access or filesystem reads.

## Ownership and boundaries

```text
Obsidian/import adapters ──one-way import──> Payload drafts (cms schema)
                                               │ explicit promote
                                               ▼
                                  Go Question Brain API (content schema)
                                  ├─ revisions/locales/graph/audit/outbox
                                  ├─ PostgreSQL FTS + pg_trgm + pgvector
                                  └─ release/search/embedding workers
                                               │ released API only
                                               ▼
                                  Fluent Lab and other clients
```

- Go owns the `content` schema and the publish command.
- Payload owns only editorial drafts, versions and review UI in `cms`.
- The Obsidian vault is an import source, never a runtime read path.
- Clients cannot write SQL, bypass release state, or serve drafts.
- A duplicate candidate may be resolved as `not_duplicate`; that decision stays
  in the audit trail and is not re-opened by a hidden fallback.

## Structure

```text
cmd/                         Go API and worker entrypoints
internal/                    domain, HTTP, persistence, import, search, jobs, telemetry
apps/cms/                    Payload authoring application and schema
db/migrations/               content/cms schema migrations and indexes
deploy/compose/              one local Compose project, health checks and Jaeger
integrations/fluent-*/       typed client contracts for downstream products
docs/contracts/              revision, quality, graph and translation contracts
docs/verification/           reproducible release and hardening evidence
scripts/                     import, release, smoke and recovery checks
ARCHITECTURE.md              system and performance decisions
ROADMAP.md                   gated delivery plan
```

## Data and search invariants

1. `question` is the stable identity; `question_revision` is immutable.
2. Locales are explicit BCP-47 values (`en`, `ru`, etc.) with provenance. A
   missing locale is a release error, not a fallback to another language.
3. Graph edges are typed rows (`prerequisite`, `related`, `contrast`,
   `follow_up`, `example_of`), not UI-only links.
4. Normalized content hashes are idempotency keys for imports, duplicates,
   embeddings and exports.
5. Published state is explicit. Drafts, rejected duplicates and stale
   embeddings are never returned as current content.
6. Every cross-system write produces an audit record and an outbox event.
7. Search is staged: exact → FTS/trigram candidates → measured semantic
   candidates → reciprocal-rank fusion. Results include match stages, scores,
   revision/locale and graph placement.

`pgvector` is an index inside PostgreSQL, not a separate source. HNSW or another
approximate index may be enabled only after a recorded recall/latency benchmark;
exact candidates remain the recall guardrail.

## Operational contract

```bash
cp .env.example .env
docker compose -f deploy/compose/compose.yaml up --build
make check
make g5-smoke
curl http://127.0.0.1:48127/health/ready
open http://127.0.0.1:48128/admin
open http://127.0.0.1:56686
```

The stack uses the reserved loopback ports documented in `README.md`; override
the complete Question Brain port set only when checking for collisions. Jaeger
is an observability sink, never a content store. Backups, restore drills and
release rollback are required for a production change.

## Working agreement

- Treat `ROADMAP.md` as a gated plan: do not start a later gate with an open
  blocker in an earlier gate.
- Update the contract, migration, tests and verification evidence together.
- Keep imports idempotent and fail closed on malformed cards or contradictory
  graph/release data.
- Never add a local question fallback, a second vector store, a graph database,
  a direct CMS write into `content`, or a fake “ready” response.
- Keep prompts and answer bodies out of logs by default; use hashes and
  redacted IDs for traces and metrics.
- Run `git diff --check` and the relevant Docker/Go checks before commit. The
  canonical branch is `main`; changes are pushed only after verification.
