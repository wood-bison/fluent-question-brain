-- G7 reviewed Question -> Capability bindings.
--
-- Curriculum placement (Program/Path/Domain) and station bindings are
-- separate decisions.  This migration gives station review its own durable
-- proposal, disposition, and immutable release boundary.  The legacy
-- content.question_capability table remains a compatibility projection; new
-- release rows carry the pins needed to prove which question and registries
-- were reviewed.

begin;

create table if not exists content.capability_binding_profile_config (
  profile_key text primary key,
  min_similarity numeric(6,5) not null,
  max_candidates integer not null default 3,
  revision text not null,
  updated_at timestamptz not null default now(),
  check (min_similarity >= 0 and min_similarity <= 1),
  check (max_candidates > 0 and max_candidates <= 20),
  check (length(trim(revision)) > 0)
);

insert into content.capability_binding_profile_config
  (profile_key, min_similarity, max_candidates, revision)
values ('semantic-neighbor-v1', 0.65, 3, 'g7-capability-neighbor-baseline-2026-08-25')
on conflict (profile_key) do nothing;

alter table content.question_capability
  add column if not exists role text not null default 'primary',
  add column if not exists provenance text not null default 'question-brain-legacy',
  add column if not exists confidence numeric(6,5),
  add column if not exists question_release_id text,
  add column if not exists capability_registry_release_id text,
  add column if not exists binding_release_id text,
  add column if not exists source_proposal_id uuid;

alter table content.question_capability
  drop constraint if exists question_capability_role_check;
alter table content.question_capability
  add constraint question_capability_role_check
  check (role in ('primary', 'prerequisite', 'follow_up', 'contrast', 'recall', 'supporting_evidence'));

alter table content.question_capability
  drop constraint if exists question_capability_confidence_check;
alter table content.question_capability
  add constraint question_capability_confidence_check
  check (confidence is null or (confidence >= 0 and confidence <= 1));

alter table content.question_capability
  drop constraint if exists question_capability_provenance_check;
alter table content.question_capability
  add constraint question_capability_provenance_check
  check (length(trim(provenance)) > 0);

create table if not exists content.question_capability_review (
  workspace_id uuid not null references content.workspace(id),
  revision_id uuid not null references content.question_revision(id) on delete cascade,
  question_release_id text not null,
  capability_registry_release_id text not null,
  disposition text not null,
  rationale text not null,
  source text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  primary key (workspace_id, revision_id, question_release_id, capability_registry_release_id),
  check (disposition in ('bound', 'theory_only', 'needs_new_capability', 'rejected')),
  check (length(trim(question_release_id)) > 0),
  check (length(trim(capability_registry_release_id)) > 0),
  check (length(trim(rationale)) > 0),
  check (length(trim(source)) > 0)
);

create index if not exists question_capability_review_workspace_disposition_idx
  on content.question_capability_review (workspace_id, disposition, updated_at desc);
create index if not exists question_capability_review_revision_idx
  on content.question_capability_review (revision_id, updated_at desc);

create or replace function content.validate_question_capability_review_workspace()
returns trigger
language plpgsql
as $$
declare
  revision_workspace uuid;
begin
  select q.workspace_id into revision_workspace
  from content.question_revision revision
  join content.question q on q.id = revision.question_id
  where revision.id = new.revision_id;
  if revision_workspace is null then
    raise exception 'question capability review revision does not exist';
  end if;
  if revision_workspace <> new.workspace_id then
    raise exception 'question capability review revision must belong to workspace';
  end if;
  return new;
end;
$$;

drop trigger if exists question_capability_review_workspace on content.question_capability_review;
create trigger question_capability_review_workspace
before insert or update of workspace_id, revision_id
on content.question_capability_review
for each row execute function content.validate_question_capability_review_workspace();

create table if not exists content.question_capability_binding_proposal (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid not null references content.workspace(id),
  revision_id uuid not null references content.question_revision(id) on delete cascade,
  path_key text not null references content.taxonomy_path(stable_key),
  capability_key text not null references content.taxonomy_capability(stable_key),
  role text not null default 'primary',
  provenance text not null,
  confidence numeric(6,5),
  evidence jsonb not null default '{}'::jsonb,
  question_release_id text not null,
  capability_registry_release_id text not null,
  status text not null default 'proposed',
  rationale text not null default '',
  source text not null,
  decided_at timestamptz,
  decided_by text,
  created_at timestamptz not null default now(),
  unique (workspace_id, revision_id, path_key, capability_key, role, capability_registry_release_id),
  check (role in ('primary', 'prerequisite', 'follow_up', 'contrast', 'recall', 'supporting_evidence')),
  check (confidence is null or (confidence >= 0 and confidence <= 1)),
  check (status in ('proposed', 'accepted', 'rejected')),
  check (length(trim(provenance)) > 0),
  check (length(trim(question_release_id)) > 0),
  check (length(trim(capability_registry_release_id)) > 0),
  check (length(trim(source)) > 0)
);

create index if not exists question_capability_binding_proposal_review_idx
  on content.question_capability_binding_proposal (workspace_id, status, capability_key, created_at);
create index if not exists question_capability_binding_proposal_revision_idx
  on content.question_capability_binding_proposal (revision_id, status);

create table if not exists content.question_capability_binding_release (
  binding_release_id text primary key,
  workspace_id uuid not null references content.workspace(id),
  question_release_id text not null,
  capability_registry_release_id text not null,
  status text not null default 'active',
  binding_count integer not null default 0,
  bound_count integer not null default 0,
  theory_only_count integer not null default 0,
  needs_new_capability_count integer not null default 0,
  rejected_count integer not null default 0,
  source_hash text not null,
  actor text not null,
  created_at timestamptz not null default now(),
  rolled_back_at timestamptz,
  rolled_back_by text,
  check (binding_release_id ~ '^question-capability-release-[a-f0-9]{16}$'),
  check (status in ('active', 'rolled_back')),
  check (length(trim(question_release_id)) > 0),
  check (length(trim(capability_registry_release_id)) > 0),
  check (length(source_hash) = 64),
  check (length(trim(actor)) > 0),
  check (binding_count >= 0),
  check (bound_count >= 0 and theory_only_count >= 0 and needs_new_capability_count >= 0 and rejected_count >= 0)
);

create unique index if not exists question_capability_binding_release_active_workspace_idx
  on content.question_capability_binding_release (workspace_id)
  where status = 'active';

create table if not exists content.question_capability_binding_release_item (
  binding_release_id text not null references content.question_capability_binding_release(binding_release_id),
  revision_id uuid not null references content.question_revision(id),
  path_key text not null references content.taxonomy_path(stable_key),
  capability_key text not null references content.taxonomy_capability(stable_key),
  role text not null,
  provenance text not null,
  confidence numeric(6,5),
  source_proposal_id uuid references content.question_capability_binding_proposal(id),
  created_at timestamptz not null default now(),
  primary key (binding_release_id, revision_id, path_key, capability_key, role),
  check (role in ('primary', 'prerequisite', 'follow_up', 'contrast', 'recall', 'supporting_evidence')),
  check (confidence is null or (confidence >= 0 and confidence <= 1)),
  check (length(trim(provenance)) > 0)
);

create index if not exists question_capability_binding_release_item_revision_idx
  on content.question_capability_binding_release_item (revision_id, capability_key);
create index if not exists question_capability_binding_release_item_capability_idx
  on content.question_capability_binding_release_item (capability_key, path_key);

create or replace function content.validate_question_capability_binding_release_item_workspace()
returns trigger
language plpgsql
as $$
declare
  release_workspace uuid;
  revision_workspace uuid;
begin
  select workspace_id into release_workspace
  from content.question_capability_binding_release
  where binding_release_id = new.binding_release_id;
  select q.workspace_id into revision_workspace
  from content.question_revision revision
  join content.question q on q.id = revision.question_id
  where revision.id = new.revision_id;
  if release_workspace is null or revision_workspace is null then
    raise exception 'capability binding release or question revision does not exist';
  end if;
  if release_workspace <> revision_workspace then
    raise exception 'released capability binding revision must belong to release workspace';
  end if;
  return new;
end;
$$;

drop trigger if exists question_capability_binding_release_item_workspace on content.question_capability_binding_release_item;
create trigger question_capability_binding_release_item_workspace
before insert or update of binding_release_id, revision_id
on content.question_capability_binding_release_item
for each row execute function content.validate_question_capability_binding_release_item_workspace();

alter table content.question_capability
  drop constraint if exists question_capability_binding_release_fk;
alter table content.question_capability
  add constraint question_capability_binding_release_fk
  foreign key (binding_release_id)
  references content.question_capability_binding_release(binding_release_id);

alter table content.question_capability
  drop constraint if exists question_capability_source_proposal_fk;
alter table content.question_capability
  add constraint question_capability_source_proposal_fk
  foreign key (source_proposal_id)
  references content.question_capability_binding_proposal(id);

create or replace function content.validate_question_capability_binding_workspace()
returns trigger
language plpgsql
as $$
declare
  revision_workspace uuid;
begin
  select q.workspace_id into revision_workspace
  from content.question_revision revision
  join content.question q on q.id = revision.question_id
  where revision.id = new.revision_id;
  if revision_workspace is null then
    raise exception 'question capability binding revision does not exist';
  end if;
  if revision_workspace <> new.workspace_id then
    raise exception 'question capability binding revision must belong to workspace';
  end if;
  return new;
end;
$$;

drop trigger if exists question_capability_binding_proposal_workspace on content.question_capability_binding_proposal;
create trigger question_capability_binding_proposal_workspace
before insert or update of workspace_id, revision_id
on content.question_capability_binding_proposal
for each row execute function content.validate_question_capability_binding_workspace();

commit;
