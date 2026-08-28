# Question search contract

`POST /v1/search` returns staged, explainable retrieval over the pinned
released content. This document records the scoring stages and the relevance
cutoff introduced for QB-BUG-3; the cutoff values are configuration, not
constants buried in code.

## Stages

Candidates are gathered from four independent stages:

| stage    | gate                                   | evidence                     |
|----------|----------------------------------------|------------------------------|
| exact    | stable_key or slug equals the query    | identity match               |
| fts      | `ts_rank_cd` over the search document  | lexical match                |
| trigram  | `similarity(prompt, query) >= 0.15`    | fuzzy lexical match          |
| semantic | cosine distance >= `0.50` under the active embedding profile | meaning match |

Fusion is reciprocal rank fusion with `k = 60`; every stage that surfaced a
candidate contributes `1 / (60 + rank)`. The merged value is reported as
`rank_score`, and `match_stages` lists which stages produced each result.

Semantic retrieval is best-effort for the interactive API. The query embedding
call has a sub-second local budget so a cold or busy Ollama/bge-m3 process cannot
block the learner's search. When that budget is exceeded (or the provider is
temporarily unavailable), the request continues through exact, FTS and trigram
stages with semantic scoring disabled; an exact or lexical result is still
authoritative. A caller cancellation is propagated instead of being converted
to a partial response. This keeps the Question Brain usable while the local
advisory model is serving another request and does not change the released
content or its ranking thresholds when embeddings are available.

## Relevance cutoff (QB-BUG-3)

A candidate is returned only when at least one of these holds:

1. it matched `exact` (identity matches must never be hidden), or
2. its fused `rank_score >= SEARCH_MIN_RANK_SCORE` (default `0.02` — under
   RRF with `k = 60` this requires two independent stages to agree; a single
   weak stage contributes at most `1/61 ≈ 0.0164`), or
3. its `semantic_score >= SEARCH_MIN_SEMANTIC_SCORE` (default `0.505`) — one
   strong multilingual embedding match is meaningful evidence on its own,
   which keeps cross-language queries working without a second agreeing
   stage.

Both values come from environment configuration (`SEARCH_MIN_RANK_SCORE`,
`SEARCH_MIN_SEMANTIC_SCORE`) so they can be tuned without a rebuild. Lowering
them admits noisier results; raising them risks hiding real ones. The
semantic floor is deliberately narrow and measured, not round: on the
2026-08-23 corpus with bge-m3, unrelated-query positives score <= 0.5008
(`fibonacci generator`) while genuine cross-language paraphrases score
>= 0.5089 (`как устроен сборщик мусора`, `дедлок`, `garbage collector` in the
opposite locale). Changing the embedding profile invalidates this calibration
and requires re-measuring before the defaults are reused. The guard
rail when tuning: `event loop`, `idempotency`, and `rate limiter` (en) must
keep returning their cards, while `fibonacci generator` (en) stays empty and
the cross-language probes from the 2026-08-23 audit stay non-empty.

An empty result set is a valid answer: a query that matches nothing above the
cutoff returns `results: []` instead of an unrelated card.

## Dimension filters

The request accepts optional `level` and `company` fields. Both are exact,
case-insensitive equality filters against indexed columns on
`content.question`; an absent filter never restricts results, and there is no
fallback from a filtered dimension to unfiltered content.
