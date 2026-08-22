# Gated delivery plan

Each gate is a hard dependency. We do not begin the next gate while the
previous gate has an open blocker.

## G0 — repository and operating contract (done in this bootstrap)

- [x] Create a separate repository with a stable name and ownership boundary.
- [x] Record Go + PostgreSQL/pgvector + Payload architecture decision.
- [x] Add one Compose project name, env contract, health endpoints, and a
      recoverable legacy boundary.
- [x] Add a migration with extensions, revisions, locales, graph edges,
      embeddings, decisions, and outbox records.

## G1 — canonical schema and one-card round-trip (current gate)

- [x] Define the canonical JSON contract and normalization rules.
- [x] Import exactly one representative question from
      `fluent-question-vault`.
- [x] Persist a revision and its `content_hash` transactionally.
- [x] Export the same card and prove a byte-stable normalized round-trip.
- [ ] Record one duplicate candidate and one placement decision with audit
      evidence.
- [x] Add a migration test against the Compose Postgres image.

## G2 — ingestion and reconciliation

- [ ] Build a streaming vault importer with dry-run, idempotency, and a
      machine-readable report.
- [ ] Handle new, changed, deleted, duplicate, and ambiguous cards without
      destructive writes.
- [ ] Make the importer bilingual and locale-aware.
- [ ] Import the full snapshot only after G1 is green.

## G3 — retrieval and indexing

- [ ] Implement exact/FTS/trigram search first.
- [ ] Add embedding profiles and an outbox worker with retry/backoff.
- [ ] Benchmark exact, IVFFlat, and HNSW candidates; publish recall@k and
      latency before selecting an index.
- [ ] Add RRF ranking, topic/tenant filters, and an explainable result trace.

## G4 — authoring and application integration

- [ ] Build Payload as the editorial UI against the isolated `cms` schema.
- [ ] Implement promote/review commands into the Go API; never dual-write
      published content.
- [ ] Switch Fluent Engineering Lab reads to the versioned API behind a
      feature flag.
- [ ] Verify Russian/English UI and content fallbacks end to end.

## G5 — production hardening and retirement

- [x] Add OpenTelemetry HTTP traces and a local Jaeger Compose sink.
- [ ] Add metrics/log correlation, redaction checks, and production retention
      rules.
- [ ] Run load, restore, migration, and failure-injection drills.
- [ ] Verify backups and a tested rollback path.
- [ ] Retire only the legacy question-registry/runtime path after parity
      evidence and a signed release checklist. The Lab product remains active.

## Definition of done for the product

The system is ready for other projects when a new workspace can import,
review, publish, search, localize, connect, and export questions without
editing application code; every result has explainable provenance; and a
failed worker or migration can be retried or rolled back without data loss.
