# G2 identity repair and release evidence — 2026-08-22

One behavioral card contained a copied H1 prefix (`B011`) while its explicit
metadata identified the card as `B012`. The importer previously trusted the
heading and collapsed two distinct cards into one stable key. The normalizer
now treats `ID:` metadata as the identity source of truth and has a regression
test for this exact shape.

After the repair, the canonical source vault was re-imported idempotently:

```text
files=1368 created=1 unchanged=1367 archived=0 invalid=0
```

The approved release dry-run then validated all `1368/1368` cards with zero
duplicate stable keys and zero duplicate content hashes. The release command
published the immutable snapshot transactionally; the outbox indexer drained
`1372` events and wrote `2393` vectors with zero failures. Post-release
Postgres checks reported `pending_outbox=0` and `published=1373` (the extra
five are the existing integration-smoke cards).
