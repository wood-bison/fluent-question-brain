-- G2 ingestion/reconciliation state. Import runs are append-only evidence;
-- missing source cards are archived, never physically deleted.

create table if not exists content.import_run (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid not null references content.workspace(id),
  source_system text not null,
  source_root text not null,
  mode text not null,
  status text not null default 'running',
  totals jsonb not null default '{}'::jsonb,
  started_at timestamptz not null default now(),
  completed_at timestamptz,
  check (mode in ('dry_run', 'reconcile', 'single_file')),
  check (status in ('running', 'succeeded', 'failed'))
);

create table if not exists content.import_item (
  id uuid primary key default gen_random_uuid(),
  run_id uuid not null references content.import_run(id) on delete cascade,
  source_ref text not null,
  stable_key text,
  content_hash text,
  action text not null,
  question_id uuid references content.question(id) on delete set null,
  error text,
  created_at timestamptz not null default now(),
  unique (run_id, source_ref),
  check (action in ('created', 'updated', 'unchanged', 'invalid', 'would_create', 'would_update', 'would_skip'))
);

create index if not exists import_item_source_idx
  on content.import_item (source_ref, created_at desc);
create index if not exists import_run_workspace_idx
  on content.import_run (workspace_id, started_at desc);
