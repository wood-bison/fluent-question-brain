#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -p fluent-question-brain -f "${repo_root}/deploy/compose/compose.yaml")
api_port="${QB_HTTP_PORT:-48127}"

"${compose[@]}" config >/dev/null
"${compose[@]}" exec -T api /question-brain --healthcheck
"${compose[@]}" exec -T postgres psql -U question_brain -d question_brain -v ON_ERROR_STOP=1 \
  -c "select 1 from information_schema.schemata where schema_name = 'content';" \
  -c "select 1 from information_schema.schemata where schema_name = 'cms';" \
  -c "select 1 from information_schema.tables where table_schema = 'content' and table_name = 'question_curriculum_mapping';" \
  -c "select 1 from information_schema.tables where table_schema = 'content' and table_name = 'taxonomy_capability_domain';" \
  -c "select 1 from information_schema.tables where table_schema = 'content' and table_name = 'taxonomy_capability_alias';" \
  -c "select 1 from information_schema.tables where table_schema = 'content' and table_name = 'taxonomy_capability_supersedes';" \
  -c "select 1 from pg_trigger where tgname = 'taxonomy_capability_supersedes_cycle';" \
  -c 'do $fn$ begin if exists (select 1 from content.taxonomy_capability_alias a left join content.taxonomy_capability c on c.stable_key = a.canonical_key where c.stable_key is null) then raise exception using message = $msg$dangling capability alias$msg$; end if; end $fn$;' \
  -c 'do $fn$ begin if exists (with recursive walk(start_key, current_key, path) as (select superseded_key, canonical_key, array[superseded_key, canonical_key] from content.taxonomy_capability_supersedes union all select w.start_key, s.canonical_key, w.path || s.canonical_key from content.taxonomy_capability_supersedes s join walk w on s.superseded_key = w.current_key where not s.canonical_key = any(w.path)) select 1 from walk w join content.taxonomy_capability_supersedes s on s.superseded_key = w.current_key where s.canonical_key = any(w.path)) then raise exception using message = $msg$capability supersedes cycle$msg$; end if; end $fn$;' \
  -c "select 1 from content.taxonomy_path where stable_key = 'path.python';" \
  -c "select 1 from cms.payload_migrations where name = '20260822_101040_initial';" >/dev/null
curl -fsS "http://127.0.0.1:${api_port}/health/ready" | grep -q '"migration":"compose-init"'

echo "question-brain migration smoke: compose config, Go boundary, content/cms migrations ok"
