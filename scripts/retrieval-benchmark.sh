#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")

# The query vector is an existing embedding so the benchmark is deterministic
# and repeatable without requiring a provider key. Exact search is forced to a
# sequential scan; IVFFlat and HNSW are then measured with their own knobs.
"${compose[@]}" exec -T postgres psql -U question_brain -d question_brain -v ON_ERROR_STOP=1 <<'SQL'
drop index if exists content.question_embedding_ivfflat_dev;
create index question_embedding_ivfflat_dev
  on content.question_embedding using ivfflat (embedding vector_cosine_ops)
  with (lists = 16);

\echo '=== corpus ==='
select count(*) as embeddings,
       count(distinct locale_id) as locales,
       count(distinct profile_key) as profiles
from content.question_embedding
where profile_key = 'semantic-dev-hash-v1';

\echo '=== exact ==='
set enable_indexscan = off;
set enable_bitmapscan = off;
drop table if exists benchmark_exact;
create temporary table benchmark_exact as
select locale_id
from content.question_embedding
order by embedding <=> (select embedding from content.question_embedding where profile_key = 'semantic-dev-hash-v1' order by locale_id limit 1)
limit 10;
explain (analyze, buffers, format text)
select locale_id, 1 - (embedding <=> (select embedding from content.question_embedding where profile_key = 'semantic-dev-hash-v1' order by locale_id limit 1)) as score
from content.question_embedding
order by embedding <=> (select embedding from content.question_embedding where profile_key = 'semantic-dev-hash-v1' order by locale_id limit 1)
limit 10;

\echo '=== ivfflat ==='
reset enable_indexscan;
reset enable_bitmapscan;
set enable_seqscan = off;
set ivfflat.probes = 16;
drop index if exists content.question_embedding_hnsw_dev;
drop table if exists benchmark_ivfflat;
create temporary table benchmark_ivfflat as
select locale_id
from content.question_embedding
order by embedding <=> (select embedding from content.question_embedding where profile_key = 'semantic-dev-hash-v1' order by locale_id limit 1)
limit 10;
explain (analyze, buffers, format text)
select locale_id, 1 - (embedding <=> (select embedding from content.question_embedding where profile_key = 'semantic-dev-hash-v1' order by locale_id limit 1)) as score
from content.question_embedding
order by embedding <=> (select embedding from content.question_embedding where profile_key = 'semantic-dev-hash-v1' order by locale_id limit 1)
limit 10;
select count(*) as recall_hits, round(count(*) / 10.0, 2) as recall_at_10
from benchmark_ivfflat ivf join benchmark_exact exact_result using (locale_id);

\echo '=== hnsw ==='
drop index if exists content.question_embedding_hnsw_dev;
create index question_embedding_hnsw_dev
  on content.question_embedding using hnsw (embedding vector_cosine_ops)
  with (m = 16, ef_construction = 64);
set hnsw.ef_search = 64;
drop table if exists benchmark_hnsw;
create temporary table benchmark_hnsw as
select locale_id
from content.question_embedding
order by embedding <=> (select embedding from content.question_embedding where profile_key = 'semantic-dev-hash-v1' order by locale_id limit 1)
limit 10;
explain (analyze, buffers, format text)
select locale_id, 1 - (embedding <=> (select embedding from content.question_embedding where profile_key = 'semantic-dev-hash-v1' order by locale_id limit 1)) as score
from content.question_embedding
order by embedding <=> (select embedding from content.question_embedding where profile_key = 'semantic-dev-hash-v1' order by locale_id limit 1)
limit 10;
select count(*) as recall_hits, round(count(*) / 10.0, 2) as recall_at_10
from benchmark_hnsw hnsw join benchmark_exact exact_result using (locale_id);
SQL
