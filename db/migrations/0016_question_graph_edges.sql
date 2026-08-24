-- G5 reviewed question graph. Legacy content.question_edge remains readable;
-- these revision-aware tables are the only source for new released relations.

create table if not exists content.question_edge_proposal (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid not null references content.workspace(id),
  from_revision_id uuid not null references content.question_revision(id),
  to_revision_id uuid not null references content.question_revision(id),
  kind text not null,
  status text not null default 'proposed',
  confidence numeric(6,5),
  rationale text not null default '',
  source text not null default 'question-brain-editorial',
  created_at timestamptz not null default now(),
  decided_at timestamptz,
  decided_by text,
  unique (workspace_id, from_revision_id, to_revision_id, kind),
  check (from_revision_id <> to_revision_id),
  check (kind in ('prerequisite', 'related', 'contrast', 'follow_up', 'variant', 'duplicate', 'supersedes')),
  check (status in ('proposed', 'accepted', 'rejected', 'superseded')),
  check (confidence is null or (confidence >= 0 and confidence <= 1)),
  check (length(trim(source)) > 0)
);

create index if not exists question_edge_proposal_workspace_status_idx
  on content.question_edge_proposal (workspace_id, status, kind, created_at);
create index if not exists question_edge_proposal_from_idx
  on content.question_edge_proposal (from_revision_id, kind, status);
create index if not exists question_edge_proposal_to_idx
  on content.question_edge_proposal (to_revision_id, kind, status);

create or replace function content.validate_question_edge_proposal_workspace()
returns trigger
language plpgsql
as $$
declare
  from_workspace uuid;
  to_workspace uuid;
begin
  select q.workspace_id into from_workspace
  from content.question_revision revision
  join content.question q on q.id = revision.question_id
  where revision.id = new.from_revision_id;
  select q.workspace_id into to_workspace
  from content.question_revision revision
  join content.question q on q.id = revision.question_id
  where revision.id = new.to_revision_id;
  if from_workspace is null or to_workspace is null then
    raise exception 'question edge endpoint revision does not exist';
  end if;
  if from_workspace <> new.workspace_id or to_workspace <> new.workspace_id then
    raise exception 'question edge endpoints must belong to proposal workspace';
  end if;
  if new.from_revision_id = new.to_revision_id then
    raise exception 'question edge cannot point to itself';
  end if;
  return new;
end;
$$;

drop trigger if exists question_edge_proposal_workspace on content.question_edge_proposal;
create trigger question_edge_proposal_workspace
before insert or update of workspace_id, from_revision_id, to_revision_id
on content.question_edge_proposal
for each row execute function content.validate_question_edge_proposal_workspace();

create table if not exists content.question_graph_release (
  graph_release_id text primary key,
  workspace_id uuid not null references content.workspace(id),
  question_release_id text not null,
  status text not null default 'active',
  edge_count integer not null default 0,
  source_hash text not null,
  actor text not null,
  created_at timestamptz not null default now(),
  rolled_back_at timestamptz,
  rolled_back_by text,
  check (graph_release_id ~ '^question-graph-release-[a-f0-9]{16}$'),
  check (status in ('active', 'rolled_back')),
  check (edge_count >= 0),
  check (length(trim(question_release_id)) > 0),
  check (length(trim(source_hash)) = 64),
  check (length(trim(actor)) > 0)
);

create unique index if not exists question_graph_release_active_workspace_idx
  on content.question_graph_release (workspace_id)
  where status = 'active';

create table if not exists content.question_edge_release (
  graph_release_id text not null references content.question_graph_release(graph_release_id),
  proposal_id uuid references content.question_edge_proposal(id),
  from_revision_id uuid not null references content.question_revision(id),
  to_revision_id uuid not null references content.question_revision(id),
  kind text not null,
  confidence numeric(6,5),
  rationale text not null default '',
  created_at timestamptz not null default now(),
  primary key (graph_release_id, from_revision_id, to_revision_id, kind),
  check (from_revision_id <> to_revision_id),
  check (kind in ('prerequisite', 'related', 'contrast', 'follow_up', 'variant', 'duplicate', 'supersedes')),
  check (confidence is null or (confidence >= 0 and confidence <= 1))
);

create index if not exists question_edge_release_from_idx
  on content.question_edge_release (from_revision_id, kind);
create index if not exists question_edge_release_to_idx
  on content.question_edge_release (to_revision_id, kind);

create or replace function content.validate_question_edge_release_workspace()
returns trigger
language plpgsql
as $$
declare
  release_workspace uuid;
  from_workspace uuid;
  to_workspace uuid;
  proposal_from uuid;
  proposal_to uuid;
begin
  select workspace_id into release_workspace
  from content.question_graph_release
  where graph_release_id = new.graph_release_id;
  select q.workspace_id into from_workspace
  from content.question_revision revision
  join content.question q on q.id = revision.question_id
  where revision.id = new.from_revision_id;
  select q.workspace_id into to_workspace
  from content.question_revision revision
  join content.question q on q.id = revision.question_id
  where revision.id = new.to_revision_id;
  if release_workspace is null or from_workspace is null or to_workspace is null then
    raise exception 'released question edge endpoint or release does not exist';
  end if;
  if release_workspace <> from_workspace or release_workspace <> to_workspace then
    raise exception 'released question edge endpoints must belong to release workspace';
  end if;
  if new.proposal_id is not null then
    select from_revision_id, to_revision_id into proposal_from, proposal_to
    from content.question_edge_proposal
    where id = new.proposal_id;
    if proposal_from is null or proposal_to is null
       or proposal_from <> new.from_revision_id or proposal_to <> new.to_revision_id then
      raise exception 'released question edge does not match its proposal';
    end if;
  end if;
  return new;
end;
$$;

drop trigger if exists question_edge_release_workspace on content.question_edge_release;
create trigger question_edge_release_workspace
before insert or update of graph_release_id, proposal_id, from_revision_id, to_revision_id
on content.question_edge_release
for each row execute function content.validate_question_edge_release_workspace();
