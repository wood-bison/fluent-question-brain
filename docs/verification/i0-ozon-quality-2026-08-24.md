# I0 Ozon content quality evidence

Date: 2026-08-24

I0 was closed against the authoritative source vault before any subsequent
content phase. The source-vault repair is local commit `f2d0750` (`fix: repair
malformed Ozon prompts and titles`); it was not pushed.

## Scope and preservation

The repair touched 19 malformed Ozon cards: OZ-106, OZ-109, OZ-110, OZ-111,
OZ-118, OZ-123, OZ-124, OZ-125, OZ-128, OZ-132, OZ-134, OZ-136, OZ-139,
OZ-140, OZ-142, OZ-145, OZ-156, OZ-159, and OZ-160. The changes are limited to
truncated learner prompts, extracted code-column titles, and their matching
semantic H1/task lead-ins. No answer was invented; cards whose source rubric
has no answer still say that the answer is absent.

The approved release report `/tmp/qb/i0-approved-after.json` recorded 1,572
unchanged cards and 19 updated cards out of 1,591. Thus the 63-card Ozon Go
bank was not regenerated or overwritten wholesale.

## Gates

The shared Go gate now rejects empty/metadata-copy prompts, extracted PDF
controls/layout markers, sentence fragments (for example a prompt ending in
`для`, `у нас`, or a dangling comma), and code fragments promoted to titles.
`GET /v1/quality` exposes the additive `checks.semantic_shape_issues` counter;
the existing `checks.degenerate_prompts` counter remains the release gate and
counts each card once.

Observed results after the approved release and indexer catch-up:

```text
release dry-run: files=1591 validated=1591 would_publish=1591 invalid=0
import dry-run:  files=1591 would_create=1591 invalid=0
approved release: files=1591 validated=1591 updated=19 unchanged=1572
/v1/quality: total=1591 published=1591 missing_en=0 missing_ru=0
             graph_unplaced=0 outbox_pending=0 locales_without_embedding=0
             degenerate_prompts=0 semantic_shape_issues=0
```

The import dry-run reported 49 legacy warning-only metadata notices and one
unrecognized non-card file; neither affected the 1,591 card release. No strict
taxonomy import or mass source-content import was performed.

## Code checks

The quality package tests cover the new fragment and code-title cases, plus
the existing PDF artifact/layout cases. The full Go test suite passed:

```text
go test ./internal/... ./cmd/...
```

The Question Brain implementation is committed separately from the source
vault and will be included in the I0 reconciliation commit.
