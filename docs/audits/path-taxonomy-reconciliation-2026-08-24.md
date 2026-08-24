# Path taxonomy reconciliation audit — 2026-08-24

Status: read-only audit plus additive Question Brain implementation. I1 adds
an explicit mapping release seam; it does not infer or rewrite legacy cards.

This audit was performed against the complete `HANDOFF-CONTENT.md`,
`docs/contracts/taxonomy.md`, `docs/contracts/fluent-engineering-lab.md`, the
Lab glossary/curriculum contract, the live Question Brain API, and the live
PostgreSQL content registry. No source-bank content was imported and no
existing question revision was edited.

## Evidence captured before the change

The live read surfaces reported:

| Surface | Observed value | Meaning |
|---|---:|---|
| `/v1/quality.total` | 1392 | published production cards exposed to the default release |
| `/v1/quality.topics` length | 134 | distinct raw `Topic` labels in current payloads (not canonical registry rows) |
| `/v1/quality.checks.published` | 1392 | production release count |
| `/v1/quality.checks.graph_released` | 1392 | one primary legacy topic binding per production card |
| `content.question` rows | 1397 | 1392 production + 5 fixture records |
| `content.topic` rows | 132 | canonical legacy topic registry rows |
| production `question_topic` rows | 1392 | all 1392 production cards have one primary placement |
| topics used by production placements | 131 | one registry row (`systems`) is intentionally empty |

`/v1/quality` also reported zero missing English/Russian locales, zero pending
outbox events, and zero current-revision locales without an active embedding.
Those checks are release evidence, not taxonomy evidence.

## Contradictions and their resolution

### 132 vs 134 topic groups

The numbers counted different layers:

- `docs/contracts/taxonomy.md` was generated from `content.topic`, which has
  132 canonical rows, including the historical zero-card `systems` row.
- The quality endpoint grouped the raw `normalized_payload.topic` strings and
  therefore exposed 134 labels: 131 active canonical topics plus three drift
  labels.
- The three drift labels are `Distributed Systems / Resilience` (one card),
  `Go / Channels & Select` (two cards), and `Go / Sync Patterns` (one card).
  Their canonical counterparts are `Distributed Systems & Resilience`,
  `Go / Channels & select`, and `Go / Sync & Patterns`.

The source of truth is now explicit: `content.topic` + `question_topic` is the
legacy content-graph registry; `/v1/quality.topics` reads that registry and
keeps the zero-card row visible. The three aliases are controlled in
`internal/taxonomy` and `content.taxonomy_alias`. Alias resolution never
rewrites an immutable payload or content hash.

### 1368 vs 1392 cards/placements

`HANDOFF-BRIEF.md` describes the pre-population baseline of 1368 production
cards. `HANDOFF-CONTENT.md` is the subsequent work order and starts from 1392
(the prior 1368 plus 24 cards). The live database confirms 1392 production
cards and 1392 production primary placements; five additional fixture records
are excluded from the default release. These are dated snapshots, not
competing current counts.

### Track vs Path

The handoff format lists legacy `Track` values (`Backend`, `Frontend`,
`Fullstack`, `Algorithms`, `Angular`, `PL/SQL`), while the Lab model defines
eight learner Paths under one Program. They are not a one-to-one mapping:
`Angular` and `PL/SQL` are historical tracks, and `System Design` and
`Behavioral` are learner paths without an equivalent legacy Track value.
Question Brain therefore keeps `Track` as content metadata and does not infer a
Path from it.

### Group vs Domain

`Group` is a card-kind discriminator (`Common Questions`, `Practical Tasks`,
`System Design`, `Behavioral` for new cards). `System Design Screening` remains
historical data for eight existing choice cards. A Lab Domain is a shared
curriculum area (`Runtime`, `HTTP/API`, `Data/PostgreSQL`, `Distributed
Systems`, `OS/Networking`, `Testing`, `Delivery/Observability`). No Group alias
is defined, because treating card kind as subject would corrupt learner
coverage.

### Topic vs Capability

The handoff correctly says that `Topic` is legacy free text and that task
`breadcrumb`/`concepts` are display/search hints, not a second taxonomy. The
new contract makes the boundary executable: a `Capability` is a reviewed Lab
station and must be represented by an explicit `capability_key`; it is never
derived from a Topic, Group, Track, title, or task hint.

## Canonical model and keys

`question-brain.taxonomy.v1` defines:

- Program: `program.backend-engineer` — **Backend Engineer**.
- Paths: `path.nodejs-typescript`, `path.java-spring`,
  `path.dotnet-csharp`, `path.go`, `path.frontend`, `path.system-design`,
  `path.algorithms`, `path.behavioral`.
- Shared Domains: `domain.runtime`, `domain.http-api`,
  `domain.data-postgresql`, `domain.distributed-systems`,
  `domain.os-networking`, `domain.testing`,
  `domain.delivery-observability`.
- Capabilities: reviewed keys of the form
  `capability.<domain-slug>.<slug>`, bound to a path and domain.

The catalog's additive metadata contract is
`program_key/path_key/domain_key/capability_key/mapping_state/mapping_version`.
`stage_key` remains a deprecated compatibility projection of `domain_key` for
older Lab readers. Legacy cards omit all new fields and remain searchable and
released through the existing content graph.

## Additive implementation in Question Brain

| File | Change |
|---|---|
| `internal/taxonomy/registry.go` | canonical Program/Path/Domain registry, capability syntax, strict aliases, legacy Topic snapshot |
| `internal/normalize/card.go`, `payload.go` | optional explicit curriculum fields; canonicalization and validation; old payloads omit them |
| `internal/ingest/importer.go`, `cmd/qb-import/main.go` | warning-only compatibility mode plus `--strict-taxonomy` for new controlled imports |
| `db/migrations/0011_path_domain_capability_taxonomy.sql` | additive registry, alias, and revision-scoped mapping tables; no content backfill |
| `db/migrations/0012_curriculum_mapping_release.sql` | one-row-per-revision explicit Program/Path/Domain release decisions; `unmapped` is an audit state and keys remain nullable until editorial review |
| `apps/cms/...` | optional editorial fields passed through the existing Payload → Go promote seam |
| `internal/search/types.go`, `internal/store/postgres.go` | catalog metadata fields and canonical topic quality buckets; `stage_key` compatibility projection |
| `docs/contracts/taxonomy.md`, `question-revision.md`, `fluent-engineering-lab.md` | source-of-truth, payload, and Lab boundary contracts |

The migration seeds only the one Program, eight Paths, and seven shared
Domains. It intentionally does not manufacture capabilities from 132 Topics
and does not backfill any of the 1392 cards.

## Verification

Executed in a Go 1.24 container because the host does not have Go installed:

```text
go test ./internal/... ./cmd/...
ok   internal/config
ok   internal/httpapi
ok   internal/normalize
ok   internal/store
ok   internal/taxonomy
ok   internal/telemetry
```

Focused tests cover alias convergence, unknown-topic rejection in strict mode,
explicit Path/Domain/Capability validation, legacy hash omission, CMS promote
payload shape, and catalog metadata separation of Group/Topic from the Lab
crosswalk. `scripts/check-contract.sh` also checks the new SQL migration and
compose mount.

## Remaining questions / follow-up

1. Lab should update its catalog adapter to prefer `path_key`/`domain_key` and
   treat `stage_key` as transitional; this audit intentionally changes no Lab
   repository files.
2. The canonical capability inventory and its titles/prerequisite graph still
   need Lab editorial approval. Until then, `content.question_capability`
   remains empty and no legacy card is considered curriculum-bound.
3. Existing Compose volumes need the new SQL migration applied once by the
   operator; a fresh database receives it through the added initdb mount.
4. Mapping legacy cards to Paths/Domains/Capabilities is a separate reviewed
   migration, not an inference or a bulk source import. `qb-map-release
   --unmapped-current --approve` records a no-inference baseline for every
   current production revision; the 2026-08-24 I1 audit recorded 1,591
   `unmapped` rows, zero proposed/accepted/rejected rows, and zero capability
   rows. A reviewed manifest must be complete and pin each mapped row to its
   current revision/content hash before any Path/Domain/Capability values are
   materialized. The machine-readable release audit is
   `docs/verification/i1-curriculum-mapping-2026-08-24.json`.

I1 verification also re-ran the content boundaries after the mapping change:
the source-vault release dry-run validated 1,591/1,591 files, the importer
dry-run planned 1,591 creates with no invalid cards (the existing 49
warning-only compatibility findings and one unrecognized file remain), and
the live API quality audit stayed at `semantic_shape_issues=0`,
`degenerate_prompts=0`, and no warnings.
