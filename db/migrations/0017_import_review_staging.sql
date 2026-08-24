-- G6 import review boundary. A source card is staged before a release can
-- publish it; candidate rows are auditable and never learner-visible.

create table if not exists content.import_review_stage (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid not null references content.workspace(id),
  run_id uuid references content.import_run(id) on delete set null,
  source_system text not null,
  source_ref text not null,
  stable_key text not null,
  content_hash text not null,
  normalized_payload jsonb not null,
  status text not null default 'staged',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (workspace_id, source_ref, content_hash),
  check (length(trim(source_system)) > 0),
  check (length(trim(source_ref)) > 0),
  check (length(trim(stable_key)) > 0),
  check (length(content_hash) = 64),
  check (status in ('staged', 'cleared', 'blocked', 'published', 'discarded'))
);

create index if not exists import_review_stage_workspace_status_idx
  on content.import_review_stage (workspace_id, status, updated_at desc);
create index if not exists import_review_stage_stable_idx
  on content.import_review_stage (workspace_id, stable_key, content_hash);

create table if not exists content.import_review_candidate (
  id uuid primary key default gen_random_uuid(),
  stage_id uuid not null references content.import_review_stage(id) on delete cascade,
  related_revision_id uuid not null references content.question_revision(id) on delete cascade,
  candidate_type text not null,
  exact_score numeric(8,7),
  lexical_score numeric(8,7),
  semantic_score numeric(8,7),
  embedding_profile text,
  evidence jsonb not null default '{}'::jsonb,
  decision text not null default 'open',
  decided_by text,
  decided_at timestamptz,
  created_at timestamptz not null default now(),
  unique (stage_id, related_revision_id, candidate_type),
  check (candidate_type in ('exact_duplicate', 'semantic_neighbor', 'lexical_neighbor')),
  check (decision in ('open', 'not_duplicate', 'keep_separate', 'merge')),
  check (exact_score is null or (exact_score >= 0 and exact_score <= 1)),
  check (lexical_score is null or (lexical_score >= 0 and lexical_score <= 1)),
  check (semantic_score is null or (semantic_score >= 0 and semantic_score <= 1))
);

create index if not exists import_review_candidate_stage_decision_idx
  on content.import_review_candidate (stage_id, decision, candidate_type);
create index if not exists import_review_candidate_revision_idx
  on content.import_review_candidate (related_revision_id, candidate_type);

create table if not exists content.duplicate_profile_config (
  profile_key text primary key references content.embedding_profile(profile_key),
  lexical_threshold numeric(8,7) not null,
  semantic_threshold numeric(8,7) not null,
  max_candidates integer not null default 25,
  calibration_revision text not null,
  created_at timestamptz not null default now(),
  check (lexical_threshold >= 0 and lexical_threshold <= 1),
  check (semantic_threshold >= 0 and semantic_threshold <= 1),
  check (max_candidates between 1 and 100),
  check (length(trim(calibration_revision)) > 0)
);

insert into content.duplicate_profile_config
  (profile_key, lexical_threshold, semantic_threshold, max_candidates, calibration_revision)
values ('semantic-v1', 0.55, 0.80, 25, 'g6-calibration-baseline-2026-08-25')
on conflict (profile_key) do nothing;

create or replace function content.validate_import_review_candidate_workspace()
returns trigger
language plpgsql
as $$
declare
  stage_workspace uuid;
  revision_workspace uuid;
begin
  select workspace_id into stage_workspace
  from content.import_review_stage
  where id = new.stage_id;
  select q.workspace_id into revision_workspace
  from content.question_revision revision
  join content.question q on q.id = revision.question_id
  where revision.id = new.related_revision_id;
  if stage_workspace is null or revision_workspace is null then
    raise exception 'import review stage or related revision does not exist';
  end if;
  if stage_workspace <> revision_workspace then
    raise exception 'import review candidate crosses workspace boundary';
  end if;
  return new;
end;
$$;

drop trigger if exists import_review_candidate_workspace on content.import_review_candidate;
create trigger import_review_candidate_workspace
before insert or update of stage_id, related_revision_id
on content.import_review_candidate
for each row execute function content.validate_import_review_candidate_workspace();
