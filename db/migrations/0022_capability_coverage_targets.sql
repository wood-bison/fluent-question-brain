-- Capability coverage target release.
--
-- This is an additive policy overlay on the immutable Question -> Capability
-- binding release. It never copies or infers a Path/Capability/role relation,
-- and it does not modify the current question or binding releases.

begin;

create table if not exists content.capability_coverage_target_release (
  coverage_target_release_id text primary key,
  workspace_id uuid not null references content.workspace(id),
  question_release_id text not null,
  capability_registry_release_id text not null,
  binding_release_id text not null references content.question_capability_binding_release(binding_release_id),
  status text not null default 'active',
  minimum_coverage_score_bps integer not null,
  source text not null,
  source_hash text not null,
  actor text not null,
  created_at timestamptz not null default now(),
  rolled_back_at timestamptz,
  rolled_back_by text,
  check (coverage_target_release_id ~ '^capability-coverage-target-release-[a-f0-9]{16}$'),
  check (status in ('active', 'rolled_back')),
  check (minimum_coverage_score_bps between 9000 and 10000),
  check (length(trim(question_release_id)) > 0),
  check (length(trim(capability_registry_release_id)) > 0),
  check (length(trim(source)) > 0),
  check (source_hash ~ '^[a-f0-9]{64}$'),
  check (length(trim(actor)) > 0)
);

create unique index if not exists capability_coverage_target_release_active_workspace_idx
  on content.capability_coverage_target_release (workspace_id)
  where status = 'active';

create or replace function content.validate_capability_coverage_target_release()
returns trigger
language plpgsql
as $$
declare
  binding content.question_capability_binding_release%rowtype;
begin
  select * into binding
  from content.question_capability_binding_release
  where binding_release_id = new.binding_release_id;

  if binding.binding_release_id is null then
    raise exception 'referenced capability binding release does not exist';
  end if;
  if binding.workspace_id <> new.workspace_id then
    raise exception 'coverage target and binding releases must belong to the same workspace';
  end if;
  if binding.question_release_id <> new.question_release_id then
    raise exception 'coverage target and binding releases must pin the same question release';
  end if;
  if binding.capability_registry_release_id <> new.capability_registry_release_id then
    raise exception 'coverage target and binding releases must pin the same capability registry release';
  end if;
  return new;
end;
$$;

drop trigger if exists capability_coverage_target_release_guard on content.capability_coverage_target_release;
create trigger capability_coverage_target_release_guard
before insert or update of workspace_id, question_release_id, capability_registry_release_id, binding_release_id
on content.capability_coverage_target_release
for each row execute function content.validate_capability_coverage_target_release();

create table if not exists content.capability_coverage_target (
  coverage_target_release_id text not null references content.capability_coverage_target_release(coverage_target_release_id),
  path_key text not null references content.taxonomy_path(stable_key),
  capability_key text not null references content.taxonomy_capability(stable_key),
  mandatory boolean not null,
  minimum_primary_questions integer not null,
  minimum_supporting_prompts integer not null,
  rationale text not null,
  created_at timestamptz not null default now(),
  primary key (coverage_target_release_id, path_key, capability_key),
  check (minimum_primary_questions >= 0),
  check (minimum_supporting_prompts >= 0),
  check (not mandatory or minimum_primary_questions > 0),
  check (length(trim(rationale)) > 0)
);

create table if not exists content.question_coverage_classification (
  coverage_target_release_id text not null references content.capability_coverage_target_release(coverage_target_release_id),
  revision_id uuid not null references content.question_revision(id),
  stable_key text not null,
  content_hash text not null,
  disposition text not null,
  rationale text not null,
  created_at timestamptz not null default now(),
  primary key (coverage_target_release_id, revision_id),
  unique (coverage_target_release_id, stable_key),
  check (stable_key ~ '^question[.]'),
  check (content_hash ~ '^[a-f0-9]{64}$'),
  check (disposition in ('core', 'supplemental', 'quarantined')),
  check (length(trim(rationale)) > 0)
);

create index if not exists question_coverage_classification_disposition_idx
  on content.question_coverage_classification (coverage_target_release_id, disposition, stable_key);

create or replace function content.validate_question_coverage_classification()
returns trigger
language plpgsql
as $$
declare
  release_workspace uuid;
  release_binding_id text;
  revision_workspace uuid;
  revision_stable_key text;
  revision_content_hash text;
  binding_count integer;
begin
  select workspace_id, binding_release_id
    into release_workspace, release_binding_id
  from content.capability_coverage_target_release
  where coverage_target_release_id = new.coverage_target_release_id;

  select question.workspace_id, question.stable_key, revision.content_hash
    into revision_workspace, revision_stable_key, revision_content_hash
  from content.question_revision revision
  join content.question question on question.id = revision.question_id
  where revision.id = new.revision_id;

  if release_workspace is null or revision_workspace is null then
    raise exception 'coverage release or question revision does not exist';
  end if;
  if release_workspace <> revision_workspace then
    raise exception 'coverage classification revision must belong to release workspace';
  end if;
  if revision_stable_key <> new.stable_key or revision_content_hash <> new.content_hash then
    raise exception 'coverage classification must pin current immutable question identity';
  end if;

  select count(*) into binding_count
  from content.question_capability_binding_release_item item
  where item.binding_release_id = release_binding_id
    and item.revision_id = new.revision_id;

  if new.disposition = 'core' and binding_count = 0 then
    raise exception 'core coverage classification requires a reviewed capability binding';
  end if;
  if new.disposition = 'quarantined' and binding_count > 0 then
    raise exception 'quarantined coverage classification cannot remain learner-bound';
  end if;
  return new;
end;
$$;

drop trigger if exists question_coverage_classification_guard on content.question_coverage_classification;
create trigger question_coverage_classification_guard
before insert or update
on content.question_coverage_classification
for each row execute function content.validate_question_coverage_classification();

commit;
