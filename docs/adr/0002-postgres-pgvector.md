# ADR 0002: PostgreSQL plus pgvector

## Status

Accepted for G1; index choice remains benchmark-gated.

## Decision

Use one PostgreSQL cluster with `vector`, `pg_trgm`, and full-text search. Store
embeddings in a profile-specific table with a fixed dimension per profile. Do
not introduce a separate vector or graph database until a measured workload
shows that the single transactional boundary cannot meet the SLO.

## Consequences

- Revisions, relationships, exact search, semantic search, and outbox events
  share transaction and backup semantics.
- HNSW is an optimization, not a correctness requirement. Exact search remains
  the first-stage candidate source and the recall benchmark oracle.
- A new embedding model or dimension is additive: create a profile, backfill,
  compare, then switch reads.
