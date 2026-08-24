# Phase 4.6 source batch checkpoint — EPAM / ВТБ JavaScript matrix

Date: 2026-08-24  
Status: manifest-only, review required  
Production import: **not run**

## Scope

This checkpoint deliberately covers one stable source only:

`/Users/sergeyzhechko/Downloads/Банк вопросов/Bigtech/JavaScript_Иннотех.pdf`

The source is a 19-page PDF. Its filename says `Иннотех`, but the document
identifies the competency matrix for **Интернет банк ВТБ Онлайн** and carries an
EPAM legal notice with effective date 15-Apr-2020. The manifest therefore uses
`Company: EPAM / ВТБ`; it does not propagate the misleading filename as
provenance.

Source SHA-256:

```text
be89888900c4667fb9d4238a150cb5145a4a08268b8aa2f5b78218c862643e9d
```

The exact ten selected Russian prompts, source anchors, controlled legacy
`Track`/`Group`/`Topic`, answer provenance, and source gaps are recorded in
[`phase4-6-innotech-js-first10-2026-08-24.json`](../manifests/phase4-6-innotech-js-first10-2026-08-24.json).
No v1 `program_key`, `path_key`, `domain_key`, `capability_key`, or mapping
decision is present: Lab crosswalk remains explicitly out of scope for this
batch.

## Selection and source gaps

The ten rows are the first numbered question prompts encountered in the PDF's
4.2/4.3 sequence:

| IDs | PDF anchors | Count |
|---|---|---:|
| NT-701…NT-703 | p.7, `4.2.1 Принципы проектирования`, questions 1–3 | 3 |
| NT-704…NT-707 | p.8, `4.2.2 Паттерны проектирования`, questions 1–4 | 4 |
| NT-708…NT-709 | p.8, `4.2.3 ООП`, questions 1–2 | 2 |
| NT-710 | p.8, `4.3.1 JS core ES6+`, question 1 | 1 |

All ten prompts have source bullet points below them, so the manifest records
`source_answer_present_not_imported`. No answer text, generated solution,
project claim, or English learning layer has been imported. Four prompts are
short labels (`Классификация`, `Примеры`, `Когда использовать`, `Что это?`)
whose meaning depends on the PDF section heading; they are flagged as
`exact_source_wording_but_short` and must be expanded editorially before a
card can be published. The PDF does not specify a per-question difficulty, so
all `level` values remain null.

## Verification evidence

Read-only source checks:

```text
pdfinfo: pages=19, title=Сценарий проведения собеседования, author=Anton Kim
source anchor check (10 prompts on pages 7–8): PASS
NT-701…NT-710 collision check against Question Vault: PASS
```

Manifest format check:

```sh
jq -e '.schema == "question-brain.source-batch-manifest.v1"
  and (.entries|length == 10)
  and ([.entries[].id]|unique|length == 10)' \
  docs/manifests/phase4-6-innotech-js-first10-2026-08-24.json
```

Result: PASS; 10 unique IDs, 2 controlled Topics, one Group (`Common
Questions`), and no production/mapping fields.

Strict taxonomy dry-run was run against the current vault only; the manifest
was not mounted as card content and no database URL or `--approve` flag was
used:

```sh
docker run --rm \
  -v "$QB":/src \
  -v "$VAULT":/vault \
  -v /tmp/qb-content-batch-2026-08-24:/reports \
  -w /src golang:1.24-bookworm \
  sh -c 'go run ./cmd/qb-import --root /vault --dry-run \
    --strict-taxonomy --report /reports/phase4-6-strict-dry-run.json'
```

Observed result:

```text
files=1591
would_create=1591
invalid=0
warnings=49
unrecognized_files=1
```

The one unrecognized file is the pre-existing vault backlog file:
`Raw Question Backlog/2026-05-26 — Research Missing Question Backlog.md`.
The 49 warnings are existing warning-only taxonomy/content notices; no new
manifest row was imported. The raw dry-run report is intentionally kept out of
the repository because it contains absolute local paths; its SHA-256 is
`4e4fea75ba5004a12ffba171f683abfd29ef934a46c7b28b9abbca6c9403fd9d`.

## Remaining work

1. Editorially expand the four short source prompts while preserving the exact
   Russian prompt and section anchor.
2. Add the English short answer, Russian understanding layer, traps,
   follow-ups, and practice fields required by the interview-card format,
   using only the PDF bullets as answer provenance.
3. Re-run strict taxonomy against a temporary source-vault batch containing
   these cards; only after review may a separate release task consider a
   production import.
4. Keep the remaining Phase 4.6 rows and phases 4.2/4.5/5.3 out of this
   checkpoint.

