-- G9 canonical capability rename review.
--
-- Aliases and supersessions are historical identity facts, not learner graph
-- edges.  They are proposed and decided in Question Brain, then materialised
-- into the canonical registry tables in the same transaction as the review
-- decision.  Historical keys remain resolvable; new releases must use the
-- canonical key.

begin;

create table if not exists content.taxonomy_capability_alias_supersession_proposal (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid not null references content.workspace(id),
  action text not null,
  source_key text not null,
  canonical_key text not null references content.taxonomy_capability(stable_key),
  reason text not null,
  source text not null,
  provenance jsonb not null default '{}'::jsonb,
  status text not null default 'proposed',
  decided_at timestamptz,
  decided_by text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (workspace_id, action, source_key, canonical_key),
  check (action in ('alias', 'supersedes')),
  check (status in ('proposed', 'accepted', 'rejected')),
  check (source_key <> canonical_key),
  check (length(trim(source_key)) > 0),
  check (length(trim(canonical_key)) > 0),
  check (length(trim(reason)) > 0),
  check (length(trim(source)) > 0)
);

create index if not exists taxonomy_capability_alias_supersession_review_idx
  on content.taxonomy_capability_alias_supersession_proposal (workspace_id, status, created_at, id);

create index if not exists taxonomy_capability_alias_supersession_source_idx
  on content.taxonomy_capability_alias_supersession_proposal (source_key, canonical_key);

create or replace function content.validate_capability_alias_supersession_workspace()
returns trigger
language plpgsql
as $$
begin
  if not exists (select 1 from content.workspace where id = new.workspace_id) then
    raise exception 'capability alias proposal workspace does not exist';
  end if;
  if not exists (select 1 from content.taxonomy_capability where stable_key = new.canonical_key) then
    raise exception 'capability alias proposal canonical key does not exist: %', new.canonical_key;
  end if;
  return new;
end;
$$;

drop trigger if exists taxonomy_capability_alias_supersession_workspace
  on content.taxonomy_capability_alias_supersession_proposal;
create trigger taxonomy_capability_alias_supersession_workspace
before insert or update of workspace_id, canonical_key
on content.taxonomy_capability_alias_supersession_proposal
for each row execute function content.validate_capability_alias_supersession_workspace();

commit;
