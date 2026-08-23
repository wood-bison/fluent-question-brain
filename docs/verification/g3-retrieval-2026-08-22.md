# G3 retrieval evidence

Date: 2026-08-22

The full G2 import plus the approved release produced 2,399 locale embeddings with the deterministic
`semantic-dev-hash-v1` profile. The indexer drained 1,368 outbox events,
wrote the complete `2,399` vector set, and reported zero failures. The
post-release database has zero pending outbox events.

The API contract is now:

```sh
curl -sS -X POST http://localhost:48127/v1/search \
  -H 'content-type: application/json' \
  -d '{"query":"Kafka ordering","locale":"en","limit":5}'

curl -sS 'http://localhost:48127/v1/questions/question.q001?locale=ru'
```

Search responses include exact, FTS, trigram, and semantic scores plus a
`match_stages` explanation. The current sample returned `question.q023` first
with `fts` and `trigram` stages, followed by other Kafka ordering cards.

## Index benchmark

The benchmark uses a stable existing vector as the query and compares the same
top-10 request:

```text
corpus: 2399 vectors
exact sequential scan: 10.481 ms
IVFFlat (lists=16, probes=16): 1.496 ms
HNSW (m=16, ef_search=64): 0.312 ms
IVFFlat recall@10: 1.00
HNSW recall@10: 1.00
```

HNSW is selected for the development profile in migration `0004`. A real
provider or a new dimension must create a new profile, repeat recall@k and
latency measurements, and receive a separate index; profiles are never
mutated in place.
