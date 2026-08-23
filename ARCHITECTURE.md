# Architecture — Question Brain

## One sentence

Question Brain is a multi-project content graph with immutable localized
revisions, deterministic exact search, measured semantic retrieval, and an
auditable publish/index pipeline.

## Ownership map

```text
Obsidian vault / imports
          │  one-way import, never runtime reads
          ▼
Payload CMS (editorial drafts, versions, localization, review UI)
          │  explicit promote command + outbox event
          ▼
Go Question Brain API ─────── PostgreSQL content schema
          │                         ├─ exact / FTS / trigram search
          │                         ├─ graph edges and placement
          │                         ├─ pgvector embeddings
          │                         └─ outbox + audit trail
          ▼
Fluent Engineering Lab and other clients
```

There is exactly one canonical writer for published content: the Go command
boundary. Payload does not write the `content` schema directly. The CMS may
own its own `cms` schema and its own version history, but publication is a
transactional hand-off that creates a canonical revision and an outbox event.

Fluent Engineering Lab is not being replaced or archived. It remains the
interview-preparation product; after the Question Brain gates are green, its
question reads and learner flows will use the versioned Question Brain API.

## Why Go + PostgreSQL + pgvector

- Go gives a small, predictable runtime with explicit cancellation, bounded
  concurrency, and a natural fit for a performance-sensitive API and workers.
- PostgreSQL keeps relational invariants, graph edges, revisions, localization,
  audit records, full-text search, and vectors in one transactional boundary.
- `pg_trgm` catches reworded and cross-language near-duplicates before an
  embedding is needed.
- `pgvector` is the semantic retrieval layer. HNSW is enabled only after a
  recall/latency benchmark against exact candidates; the schema keeps the
  embedding profile and content hash so a model change is a backfill, not a
  destructive rewrite.
- A separate vector or graph database is not justified at the current scale or
  consistency requirements. A measured bottleneck is a prerequisite for
  introducing one.

## Data model rules

1. `question` is the stable identity; `question_revision` is immutable.
2. Localized text belongs to a revision and is addressed by an explicit BCP-47
   locale (`ru`, `en`, `en-US`, …).
3. Graph relationships are typed rows, not UI-only links. A relation can be
   `prerequisite`, `related`, `contrast`, `follow_up`, or `example_of`.
4. The content hash covers the canonical, normalized revision payload. It is
   the idempotency key for import, duplicate detection, embedding, and export.
5. Published state is explicit. A draft, rejected duplicate, or stale
   embedding is never accidentally served as current content.
6. Every cross-system change has an audit record and an outbox event.

## Search path

The API search pipeline is intentionally staged:

1. Normalize the query and workspace/locale filters.
2. Exact lookup by stable key, slug, and content hash.
3. PostgreSQL full-text and trigram candidates.
4. Semantic candidates from the matching embedding profile.
5. Reciprocal-rank fusion with deterministic tie-breakers.
6. Return evidence: revision id, locale, match stages, scores, and graph
   placement. A client must be able to explain why a card was returned.

Approximate vector search is never allowed to silently reduce recall under a
tenant/topic filter. We use iterative scans or a measured partition/index
strategy; exact search remains an explicit first-stage candidate source.

## Performance envelope

- Start as a modular monolith, not a fleet of microservices.
- Use a bounded database pool and per-request deadlines.
- Keep writes transactional and push embedding generation to an idempotent
  outbox worker.
- Do not create an HNSW index before a benchmark demonstrates its value. At
  tens of thousands of questions, a well-indexed exact/FTS path can still be
  faster and easier to operate.
- Keep vectors in a profile-specific table so `halfvec`, quantization, a new
  model, or a new dimension can be evaluated without a flag day migration.
- Record p50/p95/p99 latency, recall@k against exact search, queue lag, import
  throughput, duplicate precision, and index build time.

## Observability and safety

The Go API and workers emit structured logs, traces, and metrics with a
correlation id, workspace id, revision id, and outbox event id. Prompt/content
payloads are not logged by default; hashes and redacted metadata are enough to
replay a problem safely. OpenTelemetry is the common instrumentation contract.

For local verification, the API exports OTLP/gRPC to Jaeger all-in-one at
`jaeger:4317` inside the Compose network. Jaeger is an observability sink, not
another content store: it does not own questions, revisions, graph edges, or
embeddings. The local image uses transient storage; a production deployment
must choose retention, access control, and a persistent trace backend.

The production stack has one Compose project name, explicit health checks,
bounded resource settings, and a documented stop/start path. Backups and
restore drills are required for every release.

## What is deliberately not in G1

- No second vector store, graph database, or event bus.
- No direct SQL writes from the learning UI.
- No full collection import until one-card round-trip and duplicate/placement
  decisions are proven.
- No CMS-generated schema migrations against the canonical `content` schema.

## Design references

- [Go relational database guide](https://go.dev/doc/database/) — context
  cancellation, transactions, and managed connection pools.
- [pgvector reference](https://github.com/pgvector/pgvector) — filtered ANN
  search, iterative scans, multi-tenant index trade-offs, and the official
  `0.8.6-pg18` image.
- [Payload Postgres adapter](https://payloadcms.com/docs/database/postgres) —
  the CMS adapter and migration boundary.
- [Payload versions](https://payloadcms.com/docs/versions/overview) and
  [localization](https://payloadcms.com/docs/configuration/localization) —
  editorial history and bilingual fields.
- [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/) — shared
  traces, metrics, and logs contract.
