# Taxonomy — where a card lives and how its topic is decided

Generated from the live database on 2026-08-24.

## Curriculum taxonomy v1 (Lab cross-system model)

Question Brain has two intentionally separate placement vocabularies:

1. The legacy content graph below (`Track`, `Group`, `Topic`) describes where a
   source card was authored and what kind of card it is.
2. The explicit curriculum crosswalk (`Program`, `Path`, `Domain`,
   `Capability`) describes how a reviewed card may be projected into Fluent
   Engineering Lab.

`Group` is never a `Domain`, and `Topic` is never a `Capability`. A Lab binding
   is valid only when these fields are authored explicitly; the importer does
   not infer a path or capability from a track, group, topic, title, or task
   `breadcrumb`.

The machine-readable key space is versioned as
`question-brain.taxonomy.v1`:

| Level | Canonical key | Values |
|---|---|---|
| Program | `program.backend-engineer` | Backend Engineer |
| Path | `path.<slug>` | `nodejs-typescript`, `java-spring`, `dotnet-csharp`, `go`, `frontend`, `system-design`, `algorithms`, `behavioral` |
| Shared domain | `domain.<slug>` | `runtime`, `http-api`, `data-postgresql`, `distributed-systems`, `os-networking`, `testing`, `delivery-observability` |
| Capability | `capability.<domain-slug>.<slug>` | reviewed Lab station; not generated from a Topic |

The optional canonical payload fields are `program_key`, `path_key`,
`domain_key`, `capability_key`, `mapping_state`, and `mapping_version`.
`mapping_state` is `proposed`, `accepted`, or `rejected`: when any v1 key is
present but `mapping_state` is omitted, it defaults to `proposed`; a card with
no v1 keys at all carries no mapping fields and reads as `unmapped`. A
capability requires an explicit path and domain. The
revision-scoped `content.question_capability` table is many-to-many. Its
current learner projection is populated only by the reviewed G7 binding
release; a mass import never manufactures learner stations. Every current card
also receives an explicit G7 disposition (`bound`, `theory_only`,
`needs_new_capability`, or `rejected`) so an unbound card is visible rather
than silently missing.

`stage_key` remains a read-only compatibility projection for older Lab clients.
When a new mapping has `domain_key`, the catalog may expose the same value as
`stage_key`; new clients must prefer `domain_key` and `path_key`.

### Explicit mapping release

The revision-scoped `content.question_curriculum_mapping` table is the
release seam for this cross-system contract. It stores at most one
Program/Path/Domain decision for each current revision, with an optional
reviewed Capability. Its `mapping_state` is one of `unmapped`, `proposed`,
`accepted`, or `rejected`; `unmapped` rows have no v1 keys and are an explicit
audit result, not a guessed placement. The many-to-many
`content.question_capability` relation remains the station-level relation and
is not replaced by this table.

`cmd/qb-map-release` accepts a complete JSON manifest with stable keys,
revision/content-hash pins, and explicit canonical fields. A mapped row must
pin both the current `revision_id` and `content_hash`; a capability must exist
in the reviewed capability inventory. The command is dry-run by default and
requires `--approve` to write rows. `--unmapped-current` is the safe baseline
for a release with no reviewed inventory: it records every current production
revision as `unmapped` without reading or inferring from Track, Group, Topic,
title, or task hints. Missing or extra stable keys block approval, and the
report's `mapping_release_id` is deterministic for the pinned manifest.

Example of a mapped manifest row (the file must cover the complete release):

```json
{
  "stable_key": "question.oz-101",
  "revision_id": "00000000-0000-0000-0000-000000000001",
  "content_hash": "<sha256-of-current-normalized-payload>",
  "program_key": "program.backend-engineer",
  "path_key": "path.go",
  "domain_key": "domain.runtime",
  "mapping_state": "proposed"
}
```

The manifest is the only place where an editorial mapping decision is made;
the importer and API never reconstruct it from legacy fields. Use
`--unmapped-current --approve --report <file>` to establish an auditable
no-inference baseline before the reviewed manifest exists.

### Capability registry v2: canonical stations and historical aliases

The reviewed capability registry is the only source of learner-facing station
identity. A capability key is stable and immutable; its human title may be
localized, but a rename never changes the key. The registry uses the
`content.taxonomy_capability` lifecycle (`active`, `deprecated`, `retired`) and
an explicit `display_slug`. Only `active` capabilities may be selected by a
new curriculum release. A deprecated or retired key remains queryable for
history, evidence, and migration diagnostics, but cannot silently create a
new station.

Capabilities and shared domains are many-to-many. The canonical relation is
`content.taxonomy_capability_domain(capability_key, domain_key, role)`, where
`role` is `primary` or `secondary`; the old `taxonomy_capability.domain_key`
column is retained only as a compatibility display projection for older
clients. A capability may therefore be reused by several paths while keeping
one station identity and one evidence history.

Renames are explicit and reversible. The registry records old names in
`content.taxonomy_capability_alias` and the replacement relation in
`content.taxonomy_capability_supersedes`. Imports and releases resolve an
alias to its canonical active key, never by fuzzy matching or by title. The
historical task-shaped keys (`capability.runtime.node-event-loop-001`, and
the other v1 keys listed in the migration) are deprecated aliases; their
existing mapping, task, and learner evidence remains readable until a later
release migrates it deliberately.

The additive migration is
`db/migrations/0015_capability_registry_v2.sql`. Its reviewed disposition is
`docs/manifests/capability-registry-2026-08-24.json`; the no-write proof is
`docs/verification/g2-capability-migration-dry-run-2026-08-24.json`. The
migration is idempotent and must be applied and smoke-tested before a release
that writes capability bindings. It creates registry metadata only; it does
not bulk-assign the current published cards or rewrite release hashes. Card-to-capability
placement remains a separate reviewed operation in the later G7 gate.

### Question ↔ Capability binding release (G7)

The station-level decision is owned by the separate
`question-brain.capability-binding.v1` contract and
`db/migrations/0018_question_capability_bindings.sql`. A complete manifest
pins every current question revision and records one explicit disposition. A
`bound` entry may contain several capability bindings and relationship roles;
`theory_only` is released/searchable without a Run button; `needs_new_capability`
marks executable content waiting for a reviewed station; `rejected` is an
audited exclusion. The only writer is `cmd/qb-capability-release`, which is
dry-run by default and creates an immutable active binding release. The live
baseline is recorded in
`docs/verification/G7-QUESTION-CAPABILITY-2026-08-25.md`.

### Topic registry proposal (review queue)

For the next curriculum slice, the repository also carries an exact-topic
editorial registry at
`docs/manifests/curriculum-topic-registry-2026-08-24.json`. It is a review
queue, not a runtime taxonomy and not an importer fallback. Each row names one
canonical topic title, one explicit Path/Domain decision, a rationale, and
`review_state: proposed`. The proposal generator joins only the exact primary
topic title returned by the released catalog. It never uses prefixes, Track,
Group, card title, breadcrumb, embeddings, or task metadata; an absent topic
row remains `unmapped`.

The registry includes a separate `path.python` stack. It was added because the
released corpus contains Python cards; leaving those cards under an unrelated
stack would make path counts look complete while hiding content.

The generated
`releases/curriculum-mapping-2026-08-24-editorial-proposal.json` is complete and
revision-pinned. The explicit exact-topic review produced
`releases/curriculum-mapping-2026-08-24-editorial-approved.json` with
`1,591/1,591` accepted Program/Path/Domain rows and zero unknown topics. This
release makes every card discoverable under a named path and shared domain; it
does **not** create Lab stations, runtime buttons, mastery, or prerequisite
edges. Those require a separate reviewed `capability_key` release. The prior
19-station runtime crosswalk remains the rollback baseline.

For an existing Compose PostgreSQL volume, initdb mounts are not replayed.
Run `scripts/apply-curriculum-mapping-migration.sh` once, verify the migration
with `scripts/migration-smoke.sh`, and only then rebuild/restart the API. The
SQL is idempotent (`if not exists`) and does not rewrite question revisions.

### Controlled legacy Topic aliases

`content.topic` and `question_topic` remain the source of truth for legacy
content placement. The controlled registry has exactly three reviewed aliases:

| Raw payload label | Canonical Topic | Reason |
|---|---|---|
| `Distributed Systems / Resilience` | `Distributed Systems & Resilience` | separator drift |
| `Go / Channels & Select` | `Go / Channels & select` | capitalization drift |
| `Go / Sync Patterns` | `Go / Sync & Patterns` | conjunction drift |

Strict imports (`qb-import --strict-taxonomy`) reject an unknown legacy Topic;
the default importer remains warning-only for compatibility with historical
vaults. Aliases never rewrite an existing revision payload or its hash.

## Legacy content placement model

A legacy card is placed in the content graph by three header fields. Nothing
else places it in that graph; a curriculum binding is a separate, explicit
cross-system projection described above.

| Field | Meaning | Values |
|---|---|---|
| `Track` | coarse axis | Backend, Frontend, Fullstack, Algorithms, Angular, PL/SQL |
| `Group` | kind of card | see below |
| `Topic` | subject | one entry from this file |

Historically `Topic` was free text: the importer slugified it to `topic.<slug>`
and created the topic when it did not exist. That compatibility behavior is
still warning-only by default, so old vaults can be audited without being
blocked. New batches must run `qb-import --strict-taxonomy`: an unknown Topic
is rejected before a database write, while the three explicit aliases above
are accepted with a canonical-topic warning. Strict Topic validation does not
validate or infer the separate v1 Path/Domain/Capability mapping.

## An executable task has no topic of its own

A runnable task lives in `fluent-task-runtime/tasks/<taskId>/`, never as a copy
in the vault. It reaches a topic through the question it assesses:

```
node-event-loop-001            task.json in Task Runtime
  questionKeys: [question.c009]
    └─ question.c009           card in Question Brain
         Topic: Node / Event Loop & Scheduling
         Track: Backend
```

So the rule is: **a task is never an orphan.** It must reference at least one
question. If no question covers it yet, write the question first — that is what
gives the task its place in the tree.

`breadcrumb` and `concepts` in `task.json` are display and search hints. They are
NOT a second taxonomy and must not be used to decide where a task belongs; when
they disagree with the referenced question's topic, the question wins.

## Rules for a new card

1. **Reuse a topic from the list below** whenever one fits. Match the spelling
   exactly, including the spaces around the slash.
2. **A new topic needs a reason** — no existing topic fits, and you expect at
   least three cards in it. One-card topics fragment the tree without helping
   anyone find anything.
3. **Naming is `Area / Subtopic`**, or a bare `Area` for a broad subject. No
   third level.
4. **Do not add a top-level Area** without agreement; the filters and the learner
   graph are built around the existing ones.
5. **`Topic` is never empty.** A card without it imports unplaced and the
   importer only warns.

## Group — the kind of card

Use exactly one of these four for anything new:

| Group | When |
|---|---|
| `Common Questions` | explain-a-concept card |
| `Practical Tasks` | condition + starter + expected solution |
| `System Design` | a design case with requirements |
| `Behavioral` | experience and conduct |

`System Design Screening` stays on the eight existing multiple-choice cards.
The other values below are historical drift, not a menu to pick from.

## Known drift to clean up, not to copy

- Five names mean "this is a task": `Practical Tasks` (14), `Live Coding` (3),
  `Live Coding & SQL Tasks` (2), `Algorithms Practice` (1), `Code Review` (1).
- Three PL/SQL variants: `PL/SQL`, `PL/SQL / Oracle`, `PL/SQL / Performance`.
- 54 production cards carry an empty `Group`.
- `systems` is a topic with zero cards and a lowercase name — an import slip.
- `Angular` holds 30 topics for 58 cards and `RxJS` 10 for 12: over-fragmented.


### Node — 19 topics, 251 cards

- `Node / Async & Promises` → `topic.node-async-promises` (13)
- `Node / Bun` → `topic.node-bun` (13)
- `Node / Event Loop & Scheduling` → `topic.node-event-loop-scheduling` (19)
- `Node / JS Fundamentals` → `topic.node-js-fundamentals` (27)
- `Node / Modules & Packaging` → `topic.node-modules-packaging` (11)
- `Node / NestJS / Auth & Security` → `topic.node-nestjs-auth-security` (11)
- `Node / NestJS / DI & Architecture` → `topic.node-nestjs-di-architecture` (10)
- `Node / NestJS / Data & TypeORM` → `topic.node-nestjs-data-typeorm` (11)
- `Node / NestJS / Performance & Realtime` → `topic.node-nestjs-performance-realtime` (24)
- `Node / NestJS / Request Pipeline` → `topic.node-nestjs-request-pipeline` (7)
- `Node / NestJS / Testing & Production` → `topic.node-nestjs-testing-production` (11)
- `Node / NestJS / Validation & Errors` → `topic.node-nestjs-validation-errors` (11)
- `Node / Runtime Ops & Debugging` → `topic.node-runtime-ops-debugging` (14)
- `Node / Streams & Backpressure` → `topic.node-streams-backpressure` (11)
- `Node / TS Applied` → `topic.node-ts-applied` (12)
- `Node / TS Tooling` → `topic.node-ts-tooling` (11)
- `Node / TS Type System` → `topic.node-ts-type-system` (12)
- `Node / V8 & GC` → `topic.node-v8-gc` (11)
- `Node / Workers` → `topic.node-workers` (12)

### Java — 4 topics, 181 cards

- `Java / Concurrency & Async` → `topic.java-concurrency-async` (27)
- `Java / Core Language` → `topic.java-core-language` (105)
- `Java / JVM & GC` → `topic.java-jvm-gc` (13)
- `Java / Spring & Spring Boot` → `topic.java-spring-spring-boot` (36)

### Go — 9 topics, 80 cards

- `Go / Channels & select` → `topic.go-channels-select` (13)
- `Go / Errors & defer & panic` → `topic.go-errors-defer-panic` (11)
- `Go / Goroutines & Scheduler` → `topic.go-goroutines-scheduler` (14)
- `Go / Graceful Shutdown` → `topic.go-graceful-shutdown` (1)  ⚠️ single
- `Go / Interfaces & Generics` → `topic.go-interfaces-generics` (12)
- `Go / Memory & GC` → `topic.go-memory-gc` (13)
- `Go / Package API Design` → `topic.go-package-api-design` (1)  ⚠️ single
- `Go / Sync & Patterns` → `topic.go-sync-patterns` (12)
- `Go / Tooling & Testing` → `topic.go-tooling-testing` (3)

### Behavioral — 7 topics, 78 cards

- `Behavioral` → `topic.behavioral` (5)
- `Behavioral / Competency` → `topic.behavioral-competency` (3)
- `Behavioral / Conflict & Teamwork` → `topic.behavioral-conflict-teamwork` (2)
- `Behavioral / Leadership Principles` → `topic.behavioral-leadership-principles` (12)
- `Behavioral / Project Deep Dive` → `topic.behavioral-project-deep-dive` (30)
- `Behavioral / Recruiter Screen` → `topic.behavioral-recruiter-screen` (21)
- `Behavioral / Self & Motivation` → `topic.behavioral-self-motivation` (5)

### .NET — 8 topics, 75 cards

- `.NET / ASP.NET Core` → `topic.net-asp-net-core` (7)
- `.NET / Async & Concurrency` → `topic.net-async-concurrency` (12)
- `.NET / C# Language & Type System` → `topic.net-c-language-type-system` (24)
- `.NET / CLR & GC` → `topic.net-clr-gc` (13)
- `.NET / DI & Lifetimes` → `topic.net-di-lifetimes` (4)
- `.NET / EF Core` → `topic.net-ef-core` (12)
- `.NET / LINQ & Collections` → `topic.net-linq-collections` (1)  ⚠️ single
- `.NET / Performance & Diagnostics` → `topic.net-performance-diagnostics` (2)

### Databases & Data Modeling — 1 topics, 68 cards

- `Databases & Data Modeling` → `topic.databases-data-modeling` (68)

### Architecture & Design Patterns — 1 topics, 61 cards

- `Architecture & Design Patterns` → `topic.architecture-design-patterns` (61)

### Angular — 30 topics, 58 cards

- `Angular / Architecture` → `topic.angular-architecture` (1)  ⚠️ single
- `Angular / Build & Tooling` → `topic.angular-build-tooling` (1)  ⚠️ single
- `Angular / Change Detection` → `topic.angular-change-detection` (6)
- `Angular / Change Detection & Lifecycle` → `topic.angular-change-detection-lifecycle` (1)  ⚠️ single
- `Angular / Code Review` → `topic.angular-code-review` (2)
- `Angular / Components` → `topic.angular-components` (1)  ⚠️ single
- `Angular / Components & DI` → `topic.angular-components-di` (1)  ⚠️ single
- `Angular / Components & Templates` → `topic.angular-components-templates` (2)
- `Angular / Content Projection` → `topic.angular-content-projection` (2)
- `Angular / DI & RxJS` → `topic.angular-di-rxjs` (1)  ⚠️ single
- `Angular / DI scopes` → `topic.angular-di-scopes` (1)  ⚠️ single
- `Angular / Dependency Injection` → `topic.angular-dependency-injection` (4)
- `Angular / Directives` → `topic.angular-directives` (3)
- `Angular / Forms` → `topic.angular-forms` (5)
- `Angular / Forms & DI` → `topic.angular-forms-di` (1)  ⚠️ single
- `Angular / HttpClient` → `topic.angular-httpclient` (1)  ⚠️ single
- `Angular / Ivy` → `topic.angular-ivy` (1)  ⚠️ single
- `Angular / Lifecycle` → `topic.angular-lifecycle` (1)  ⚠️ single
- `Angular / Migration` → `topic.angular-migration` (1)  ⚠️ single
- `Angular / NgRx` → `topic.angular-ngrx` (5)
- `Angular / Pipes` → `topic.angular-pipes` (1)  ⚠️ single
- `Angular / Pipes & CD` → `topic.angular-pipes-cd` (1)  ⚠️ single
- `Angular / Routing` → `topic.angular-routing` (2)
- `Angular / RxJS` → `topic.angular-rxjs` (5)
- `Angular / Services & DI` → `topic.angular-services-di` (1)  ⚠️ single
- `Angular / Signals` → `topic.angular-signals` (3)
- `Angular / Signals & Pipes` → `topic.angular-signals-pipes` (1)  ⚠️ single
- `Angular / Templates` → `topic.angular-templates` (1)  ⚠️ single
- `Angular / Testing` → `topic.angular-testing` (1)  ⚠️ single
- `Angular / zone.js` → `topic.angular-zone-js` (1)  ⚠️ single

### System Design — 2 topics, 57 cards

- `System Design` → `topic.system-design` (19)
- `System Design / Product Cases` → `topic.system-design-product-cases` (38)

### ORM & Persistence Performance — 1 topics, 49 cards

- `ORM & Persistence Performance` → `topic.orm-persistence-performance` (49)

### Delivery, Observability & Cloud — 1 topics, 43 cards

- `Delivery, Observability & Cloud` → `topic.delivery-observability-cloud` (43)

### API Design — 1 topics, 38 cards

- `API Design` → `topic.api-design` (38)

### Auth & Backend Security — 1 topics, 38 cards

- `Auth & Backend Security` → `topic.auth-backend-security` (38)

### Algorithms — 10 topics, 37 cards

- `Algorithms` → `topic.algorithms` (28)
- `Algorithms / Binary Search` → `topic.algorithms-binary-search` (1)  ⚠️ single
- `Algorithms / Design` → `topic.algorithms-design` (1)  ⚠️ single
- `Algorithms / Graph` → `topic.algorithms-graph` (1)  ⚠️ single
- `Algorithms / Greedy` → `topic.algorithms-greedy` (1)  ⚠️ single
- `Algorithms / Hash Table` → `topic.algorithms-hash-table` (1)  ⚠️ single
- `Algorithms / Sliding Window` → `topic.algorithms-sliding-window` (1)  ⚠️ single
- `Algorithms / Stack` → `topic.algorithms-stack` (1)  ⚠️ single
- `Algorithms / Two Pointers` → `topic.algorithms-two-pointers` (1)  ⚠️ single
- `Algorithms / Union-Find` → `topic.algorithms-union-find` (1)  ⚠️ single

### Messaging & Event Streaming — 1 topics, 35 cards

- `Messaging & Event Streaming` → `topic.messaging-event-streaming` (35)

### React Rendering — 1 topics, 35 cards

- `React Rendering / Hooks` → `topic.react-rendering-hooks` (35)

### Answer Technique — 1 topics, 24 cards

- `Answer Technique` → `topic.answer-technique` (24)

### Distributed Systems & Resilience — 1 topics, 22 cards

- `Distributed Systems & Resilience` → `topic.distributed-systems-resilience` (22)

### Oracle — 1 topics, 19 cards

- `Oracle / SQL Craft & Analytics` → `topic.oracle-sql-craft-analytics` (19)

### OS, Networking & Concurrency Fundamentals — 1 topics, 19 cards

- `OS, Networking & Concurrency Fundamentals` → `topic.os-networking-concurrency-fundamentals` (19)

### Testing — 1 topics, 15 cards

- `Testing` → `topic.testing` (15)

### AI-assisted Development — 1 topics, 14 cards

- `AI-assisted Development` → `topic.ai-assisted-development` (14)

### Frontend Performance — 1 topics, 14 cards

- `Frontend Performance` → `topic.frontend-performance` (14)

### Frontend System Design — 1 topics, 14 cards

- `Frontend System Design` → `topic.frontend-system-design` (14)

### Caching — 1 topics, 13 cards

- `Caching` → `topic.caching` (13)

### Python — 4 topics, 17 cards

- `Python / Concurrency & GIL` → `topic.python-concurrency-gil` (2)
- `Python / Iterators & Generators` → `topic.python-iterators-generators` (2)
- `Python / Metaprogramming` → `topic.python-metaprogramming` (4)
- `Python / OOP & Data Model` → `topic.python-oop-data-model` (9)

### RxJS — 10 topics, 12 cards

- `RxJS / Caching` → `topic.rxjs-caching` (1)  ⚠️ single
- `RxJS / Combination` → `topic.rxjs-combination` (1)  ⚠️ single
- `RxJS / Completion` → `topic.rxjs-completion` (1)  ⚠️ single
- `RxJS / Custom Operators` → `topic.rxjs-custom-operators` (1)  ⚠️ single
- `RxJS / Error Handling` → `topic.rxjs-error-handling` (1)  ⚠️ single
- `RxJS / Higher-order Mapping` → `topic.rxjs-higher-order-mapping` (2)
- `RxJS / Hot vs Cold` → `topic.rxjs-hot-vs-cold` (1)  ⚠️ single
- `RxJS / Multicasting` → `topic.rxjs-multicasting` (1)  ⚠️ single
- `RxJS / Observable Creation` → `topic.rxjs-observable-creation` (2)
- `RxJS / Subjects` → `topic.rxjs-subjects` (1)  ⚠️ single

### Frontend Security — 1 topics, 11 cards

- `Frontend Security / Accessibility` → `topic.frontend-security-accessibility` (11)

### Frontend — 1 topics, 4 cards

- `Frontend / Styling` → `topic.frontend-styling` (4)

### AI — 1 topics, 2 cards

- `AI / RAG / Search` → `topic.ai-rag-search` (2)

### Git — 1 topics, 2 cards

- `Git / Tooling` → `topic.git-tooling` (2)

### JavaScript — 1 topics, 2 cards

- `JavaScript / DOM Events` → `topic.javascript-dom-events` (2)

### Networking — 1 topics, 2 cards

- `Networking / Browser` → `topic.networking-browser` (2)

### SQL — 1 topics, 2 cards

- `SQL / Joins & Aggregation` → `topic.sql-joins-aggregation` (2)

### State Management — 1 topics, 2 cards

- `State Management` → `topic.state-management` (2)

### JS Async Internals — 1 topics, 1 cards

- `JS Async Internals` → `topic.js-async-internals` (1)  ⚠️ single

### Node Runtime & Streams — 1 topics, 1 cards

- `Node Runtime & Streams` → `topic.node-runtime-streams` (1)  ⚠️ single

### Web Architecture — 1 topics, 1 cards

- `Web Architecture / Browser Storage` → `topic.web-architecture-browser-storage` (1)  ⚠️ single

### systems — 1 topics, 0 cards

- `systems` → `topic.systems` (0)  ⚠️ empty

---

**Legacy registry snapshot: 133 topic rows, 1392 production placements.**

`Go / Runtime **Legacy registry snapshot: 132 topic rows, 1392 production placements.** Language` added 2026-08-24 with review: the Ozon Go batch
(63 tasks) needed one language/runtime home for theory, practice and
screening cards; no existing Go topic covered them without fragmenting.

The authoritative count is the live `content.topic` registry joined to
`question_topic`; it includes the zero-card historical row `systems`. The
answer-free `/v1/quality.topics` projection is now built from that registry,
not from raw payload labels. Before taxonomy v1, the endpoint exposed 134 raw
labels: 131 active canonical topics plus the three aliases listed above. Thus
`132` and `134` were measuring different things, not two competing canonical
registries.
