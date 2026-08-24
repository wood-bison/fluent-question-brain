# Content phases 4.2/4.5/4.6/5.3 — authoring checkpoint (2026-08-24)

Status: audit checkpoint only. No Question Brain production card, legacy mapping,
fixture, or main branch was changed by this checkpoint.

## Scope and source decision

`HANDOFF-CONTENT.md` is the execution plan for source-led content work. The
legacy `Track`/`Group`/`Topic` fields in `docs/contracts/taxonomy.md` are the
import contract. They are deliberately not interchangeable with the Lab v1
`Program`/`Path`/`Domain`/`Capability` crosswalk. New cards in these phases
must therefore carry only a controlled legacy taxonomy placement; a Lab
mapping is not inferred from it and is not added to the vault card.

The source extracts supplied for this work were found in a previous temporary
scratchpad, outside both repositories:

```
/private/tmp/claude-502/-Users-sergeyzhechko-developer-fluent-interview-fluent-question-brain/
  44ab5f54-1f06-43e9-8a17-7f3a7cf4e1d6/scratchpad/notion/
```

Read-only inventory at audit time:

| source | observed material |
|---|---:|
| `JavaScript_Вопросы — Angular и экосистема.txt` | 232 lines; Angular/RxJS/testing/routing/DI/CD/NgRx/zone.js prompts and source notes |
| `JavaScript_Вопросы — Node js.txt` | 143 lines; Node runtime/streams/workers/design-patterns/Docker/data/security prompts and source notes |
| `JavaScript_Вопросы — Тех скрининг.txt` | 62 lines; screening prompts |
| `NET_С# скрининг.txt` | 35 lines; .NET/C# screening prompts |
| `Web Architecture_Формат секции Web Architecture.txt` | 107 lines; 24-prompt section structure |
| `Web Architecture_Матрица оценки кандидата — Web Architecture.txt` | 103 lines; 0–3 scoring rubric |
| `JavaScript_Задачи — Angular и экосистема.txt` | 25 lines; six CSV task briefs, with three difficulty levels in the source |

These extracts are not part of the vault commit and cannot be treated as a
stable release input without a source artifact or an explicit review record.
The temporary Lab archive also contains rich `Q*` cards, but it is a generated
archive rather than an approved source-vault batch; it is intentionally not
copied wholesale. This prevents an unreviewed mass import and preserves the
source-honesty requirement in the handoff.

## Baseline and non-mutation checks

At this checkpoint:

- Question Vault `main` is clean at `f2d0750` (`1369` markdown files under
  `Question Cards`; `NT-601`…`NT-621` are the existing Notion task cards).
- Question Brain content branch is
  `codex/content-phases-4-2-4-5-4-6-5-3-2026-08-24`, clean at `180f472`.
- The current production release remains `1591` cards. I1's explicit mapping
  release remains the no-inference baseline: `mapped=0`, `unmapped=1591`,
  `proposed=accepted=rejected=0`, with no v1 keys inferred from legacy fields.
- No existing card files were edited, renamed, or re-keyed. The pre-existing
  `NT-601`…`NT-621` task set is excluded from the next batch to avoid duplicate
  content and to keep executable tasks attached to their question cards.

## Phase-specific audit findings

### 4.2 — Notion framework questions and tasks

The source contains enough prompts for the requested Angular/RxJS, Node.js,
technical-screening, .NET/C#, and Web Architecture batches. The source also
contains incomplete answer notes (`in progress`, links, and interviewer hints)
and six CSV task briefs. A safe import must transform each prompt into the
vault format, preserve the Russian question, and retain an explicit
`Question`/`Task` distinction. The six CSV briefs must not be represented as
free-standing tasks: each needs a question card first and a runtime task
reference if executable.

The source's `Web Architecture` matrix is a rubric, not a question topic. It
must be carried as a structured `## Rubric`/scoring section on the relevant
cards, not placed in `Group` or used as a v1 capability.

The first implementation gate is therefore a source-backed batch manifest:
stable `NT-*` IDs, exact source prompt, controlled topic, and an answer/source
status for every card. Until that manifest is reviewed, importing generated
`Q*` archive cards would be unsafe.

### 4.5 — KupiBilet

The extracted KupiBilet material is a two-page question list without answers.
The two Swift/iOS tasks are explicitly out of scope. The remaining questions
can be imported only as question cards with an honest “source has no answer”
status or a reviewed answer, not with generated reference solutions.

### 4.6 — JavaScript competency matrix

The handoff says the PDF is named `JavaScript_Иннотех.pdf`, while the document
itself is the “Матрица компетенций для JavaScript разработчиков Интернет банка
ВТБ Онлайн”, signed by EPAM (effective 15-Apr-2020). Future cards must use
`Company: EPAM / ВТБ`. This provenance correction is mandatory; `Иннотех` would
misstate the source. The 118-question breakdown in the handoff is accepted as
the target, but the PDF/extracted question-and-answer artifact is not present in
the tracked vault at this checkpoint.

### 5.3 — Yandex Femida

The handoff describes 17 `WO1`, 9 `WO2`, and approximately 41 `AA` screenshot
cards. Conditions are available in the temporary extracts, while reference
solutions are generally absent. A safe card may contain only the condition,
what the interviewer should hear, and source-backed traps. It must not grow a
fabricated `## Solution`; source difficulty must remain blank except for the
two explicitly time-boxed tasks. The two RLE specifications remain separate,
and the “sorted squares”/“longest substring equal letters” suitability flags
must be preserved.

## Decision and next gate

This checkpoint is committed so the parent branch has a durable, reviewable
record rather than an implicit or silent partial import. It is not a claim
that any of the four phases is complete. The next content commit should add
only a reviewed 4.2 manifest/card batch, run `qb-import --strict-taxonomy
--dry-run`, and attach release/quality evidence before 4.5 is started. Phase
commits must remain separate and must not be merged to `main` without parent
approval.

