# G2 evidence — canonical capability registry

Date: 2026-08-24  
Status: **complete**  
Repository: `fluent-question-brain`  
Gate: G2 — capability registry, canonical naming, aliases, and domain bindings

## Decision

Question Brain now has a reviewed, canonical capability registry. Capability
identity is stable and separate from the old task-shaped station names. Shared
domains are represented by an explicit many-to-many relation; historical keys
are preserved as deprecated aliases and supersedes links. No question release
or question revision was rewritten by this gate.

The reviewed disposition is committed in
`docs/manifests/capability-registry-2026-08-24.json`. The dry-run report is
`docs/verification/g2-capability-migration-dry-run-2026-08-24.json`.

## Migration and live state

Migration: `db/migrations/0015_capability_registry_v2.sql`  
Applied with `scripts/apply-curriculum-mapping-migration.sh` and verified by
`scripts/migration-smoke.sh`.

Live Question Brain Postgres counts after the migration:

| Metric | Count |
|---|---:|
| Capability/domain bindings | 26 |
| Alias rows | 11 |
| Supersedes rows | 11 |
| Active canonical capabilities | 15 |
| Deprecated historical capabilities | 11 |
| Published cards | 1,596 |
| Embeddings | 10,653 |

The 15 active capabilities are the reviewed station inventory. The 11
deprecated rows are retained for history and migration diagnostics. The
1,596 cards are intentionally **not** bulk-assigned to capabilities here;
card placement is a separate reviewed operation in G7.

## Safety properties proven

- Migration is additive and idempotent; it was applied twice to an isolated
  restored Postgres volume without errors.
- Existing capability keys, mapping references, task references, card
  revisions, and release hashes remain readable.
- The dry-run has `writeMode: none`, covers all 15 current keys, reports
  `unresolved: []`, and records 18 Task Runtime task references.
- Alias resolution is explicit in the Go taxonomy registry; no fuzzy title,
  prefix, embedding, breadcrumb, or task inference is used.
- New release manifests reject deprecated alias keys; historical ingest still
  resolves them to the canonical key for backwards-compatible evidence reads.
- Alias and supersedes rows have foreign-key protection, and the database
  trigger `taxonomy_capability_supersedes_cycle` rejects cyclic provenance.
- The compatibility `domain_key` column remains available for older clients;
  new consumers must use `taxonomy_capability_domain`.

## Checks

```text
scripts/migration-smoke.sh                         PASS
make contract                                      PASS
docker Go 1.24: go test ./...                      PASS
python3 scripts/g2-capability-dry-run.py ...       PASS (unresolved=[])
isolated migration applied twice                   PASS (idempotent)
deprecated release key validator                  PASS
alias/supersedes integrity smoke                  PASS
Question Brain /health/ready                       PASS
```

## Follow-up boundary

G2 does not create learner stations, unlock paths, or decide which card belongs
to which capability. Those decisions require the TaskFamily contract (G3),
the graph/release join (G5/G8), and the reviewed card bindings (G7). No later
gate may skip those reviews by treating the registry as an automatic mapper.
