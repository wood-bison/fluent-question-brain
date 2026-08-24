# Taxonomy — where a card lives and how its topic is decided

Generated from the live database on 2026-08-24.

## The placement model

A card is placed by three header fields. Nothing else places it.

| Field | Meaning | Values |
|---|---|---|
| `Track` | coarse axis | Backend, Frontend, Fullstack, Algorithms, Angular, PL/SQL |
| `Group` | kind of card | see below |
| `Topic` | subject | one entry from this file |

`Topic` is free text: the importer slugifies it to `topic.<slug>` and creates the
topic when it does not exist. There is no dictionary check. A typo silently
becomes a new topic and a synonym silently splits a subject in two — which is
why this file exists and why it is worth keeping current.

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

### Python — 4 topics, 12 cards

- `Python / Concurrency & GIL` → `topic.python-concurrency-gil` (1)  ⚠️ single
- `Python / Iterators & Generators` → `topic.python-iterators-generators` (1)  ⚠️ single
- `Python / Metaprogramming` → `topic.python-metaprogramming` (3)
- `Python / OOP & Data Model` → `topic.python-oop-data-model` (7)

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

**132 topics, 1392 placements.**
