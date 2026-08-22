-- G1 canonical storage contract.
-- The migration is intentionally explicit and boring: no CMS-generated DDL
-- is allowed to own the content schema.

create extension if not exists pgcrypto;
create extension if not exists vector;
create extension if not exists pg_trgm;
create extension if not exists unaccent;

create schema if not exists content;

create table if not exists content.workspace (
  id uuid primary key default gen_random_uuid(),
  stable_key text not null unique,
  display_name text not null,
  default_locale text not null default 'en',
  created_at timestamptz not null default now(),
  check (length(trim(stable_key)) > 0),
  check (length(trim(display_name)) > 0)
);

create table if not exists content.question (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid not null references content.workspace(id),
  stable_key text not null,
  slug text not null,
  status text not null default 'draft',
  current_revision_id uuid,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (workspace_id, stable_key),
  unique (workspace_id, slug),
  check (status in ('draft', 'in_review', 'published', 'archived'))
);

create table if not exists content.question_revision (
  id uuid primary key default gen_random_uuid(),
  question_id uuid not null references content.question(id) on delete cascade,
  revision_no integer not null,
  content_hash text not null,
  normalized_payload jsonb not null,
  source_system text not null,
  source_ref text,
  authored_by text,
  created_at timestamptz not null default now(),
  unique (question_id, revision_no),
  unique (question_id, content_hash),
  check (revision_no > 0),
  check (length(content_hash) = 64)
);

alter table content.question
  drop constraint if exists question_current_revision_fk;
alter table content.question
  add constraint question_current_revision_fk
  foreign key (current_revision_id)
  references content.question_revision(id)
  deferrable initially deferred;

create table if not exists content.question_locale (
  id uuid primary key default gen_random_uuid(),
  revision_id uuid not null references content.question_revision(id) on delete cascade,
  locale text not null,
  prompt text not null,
  short_answer text,
  explanation text,
  body jsonb not null default '{}'::jsonb,
  search_document tsvector not null default ''::tsvector,
  created_at timestamptz not null default now(),
  unique (revision_id, locale),
  check (length(trim(locale)) > 0),
  check (length(trim(prompt)) > 0)
);

create or replace function content.refresh_question_locale_search_document()
returns trigger
language plpgsql
as $$
begin
  new.search_document := to_tsvector(
    'simple',
    unaccent(concat_ws(' ', new.prompt, new.short_answer, new.explanation))
  );
  return new;
end;
$$;

drop trigger if exists question_locale_search_document on content.question_locale;
create trigger question_locale_search_document
before insert or update of prompt, short_answer, explanation
on content.question_locale
for each row execute function content.refresh_question_locale_search_document();

create table if not exists content.topic (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid not null references content.workspace(id),
  stable_key text not null,
  title text not null,
  parent_id uuid references content.topic(id),
  created_at timestamptz not null default now(),
  unique (workspace_id, stable_key)
);

create table if not exists content.question_topic (
  question_id uuid not null references content.question(id) on delete cascade,
  topic_id uuid not null references content.topic(id) on delete cascade,
  relation text not null default 'primary',
  created_at timestamptz not null default now(),
  primary key (question_id, topic_id),
  check (relation in ('primary', 'secondary', 'coverage'))
);

create table if not exists content.question_edge (
  id uuid primary key default gen_random_uuid(),
  from_question_id uuid not null references content.question(id) on delete cascade,
  to_question_id uuid not null references content.question(id) on delete cascade,
  kind text not null,
  weight numeric(6,5),
  created_at timestamptz not null default now(),
  unique (from_question_id, to_question_id, kind),
  check (from_question_id <> to_question_id),
  check (kind in ('prerequisite', 'related', 'contrast', 'follow_up', 'example_of')),
  check (weight is null or (weight >= 0 and weight <= 1))
);

create table if not exists content.embedding_profile (
  profile_key text primary key,
  provider text not null,
  model text not null,
  purpose text not null,
  dimensions integer not null,
  distance_metric text not null default 'cosine',
  active boolean not null default false,
  created_at timestamptz not null default now(),
  check (dimensions > 0),
  check (distance_metric in ('cosine', 'l2', 'inner_product'))
);

insert into content.embedding_profile
  (profile_key, provider, model, purpose, dimensions, active)
values
  ('semantic-v1', 'pending', 'pending', 'question-retrieval', 1024, false)
on conflict (profile_key) do nothing;

create table if not exists content.question_embedding (
  id uuid primary key default gen_random_uuid(),
  locale_id uuid not null references content.question_locale(id) on delete cascade,
  profile_key text not null references content.embedding_profile(profile_key),
  content_hash text not null,
  embedding vector(1024) not null,
  created_at timestamptz not null default now(),
  unique (locale_id, profile_key, content_hash),
  check (length(content_hash) = 64),
  check (vector_dims(embedding) = 1024)
);

create table if not exists content.duplicate_candidate (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid not null references content.workspace(id),
  left_revision_id uuid not null references content.question_revision(id) on delete cascade,
  right_revision_id uuid not null references content.question_revision(id) on delete cascade,
  exact_score numeric(8,7),
  semantic_score numeric(8,7),
  decision text not null default 'open',
  decided_by text,
  decided_at timestamptz,
  created_at timestamptz not null default now(),
  unique (left_revision_id, right_revision_id),
  check (left_revision_id <> right_revision_id),
  check (decision in ('open', 'keep_separate', 'merge', 'not_duplicate'))
);

create table if not exists content.placement_decision (
  id uuid primary key default gen_random_uuid(),
  revision_id uuid not null references content.question_revision(id) on delete cascade,
  topic_id uuid not null references content.topic(id) on delete cascade,
  decision text not null default 'proposed',
  evidence jsonb not null default '{}'::jsonb,
  decided_by text,
  decided_at timestamptz,
  created_at timestamptz not null default now(),
  unique (revision_id, topic_id),
  check (decision in ('proposed', 'accepted', 'rejected'))
);

create table if not exists content.audit_event (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid references content.workspace(id),
  aggregate_type text not null,
  aggregate_id uuid,
  event_type text not null,
  actor text,
  correlation_id text,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create table if not exists content.outbox_event (
  id uuid primary key default gen_random_uuid(),
  aggregate_type text not null,
  aggregate_id uuid not null,
  event_type text not null,
  idempotency_key text not null unique,
  payload jsonb not null,
  attempts integer not null default 0,
  available_at timestamptz not null default now(),
  claimed_at timestamptz,
  published_at timestamptz,
  last_error text,
  created_at timestamptz not null default now(),
  check (attempts >= 0)
);

create index if not exists question_workspace_status_idx
  on content.question (workspace_id, status, updated_at desc);
create index if not exists question_revision_hash_idx
  on content.question_revision (content_hash);
create index if not exists question_locale_search_idx
  on content.question_locale using gin (search_document);
create index if not exists question_locale_trgm_idx
  on content.question_locale using gin (prompt gin_trgm_ops);
create index if not exists question_topic_topic_idx
  on content.question_topic (topic_id, question_id);
create index if not exists question_edge_from_idx
  on content.question_edge (from_question_id, kind);
create index if not exists question_edge_to_idx
  on content.question_edge (to_question_id, kind);
create index if not exists outbox_available_idx
  on content.outbox_event (available_at, created_at)
  where published_at is null;

-- HNSW is intentionally not created in G1. The index is a benchmark result,
-- not a schema assumption. Add it in a later migration after recall@k evidence.
