-- Taxonomy v1: an additive curriculum crosswalk for Fluent Engineering Lab.
--
-- Track/Group/Topic remain the legacy content placement fields.  This
-- migration deliberately does not backfill or rewrite any question payloads,
-- revisions, hashes, or topic bindings.  New curriculum mappings are
-- authored explicitly and may be released independently of the legacy graph.

create table if not exists content.taxonomy_program (
  stable_key text primary key,
  title text not null,
  created_at timestamptz not null default now(),
  check (stable_key ~ '^program[.][a-z0-9]+(?:-[a-z0-9]+)*$'),
  check (length(trim(title)) > 0)
);

create table if not exists content.taxonomy_path (
  stable_key text primary key,
  program_key text not null references content.taxonomy_program(stable_key),
  title text not null,
  created_at timestamptz not null default now(),
  check (stable_key ~ '^path[.][a-z0-9]+(?:-[a-z0-9]+)*$'),
  check (length(trim(title)) > 0)
);

create table if not exists content.taxonomy_domain (
  stable_key text primary key,
  title text not null,
  shared boolean not null default true,
  created_at timestamptz not null default now(),
  check (stable_key ~ '^domain[.][a-z0-9]+(?:-[a-z0-9]+)*$'),
  check (length(trim(title)) > 0)
);

-- Capabilities are intentionally not seeded from legacy Topics.  A
-- capability is a reviewed Lab station, not a synonym for a source topic.
create table if not exists content.taxonomy_capability (
  stable_key text primary key,
  domain_key text not null references content.taxonomy_domain(stable_key),
  title text not null,
  created_at timestamptz not null default now(),
  check (stable_key ~ '^capability[.][a-z0-9]+(?:-[a-z0-9]+)*[.][a-z0-9]+(?:[.-][a-z0-9]+)*$'),
  check (length(trim(title)) > 0)
);

-- Explicit aliases close the 132 registry rows vs 134 raw payload labels
-- discrepancy without mutating historical revision hashes.  canonical_key is
-- text because legacy topic keys predate the curriculum registry tables.
create table if not exists content.taxonomy_alias (
  kind text not null,
  alias text not null,
  canonical_key text not null,
  reason text not null,
  created_at timestamptz not null default now(),
  primary key (kind, alias),
  check (kind in ('topic', 'program', 'path', 'domain', 'stage', 'capability')),
  check (length(trim(alias)) > 0),
  check (length(trim(canonical_key)) > 0),
  check (length(trim(reason)) > 0)
);

-- A mapping is revision-scoped and many-to-many.  The Lab must consume only
-- accepted rows from a released revision; proposed mappings are review debt.
create table if not exists content.question_capability (
  revision_id uuid not null references content.question_revision(id) on delete cascade,
  path_key text not null references content.taxonomy_path(stable_key),
  capability_key text not null references content.taxonomy_capability(stable_key),
  mapping_state text not null default 'proposed',
  mapping_version text not null default 'question-brain.taxonomy.v1',
  source text not null default 'question-brain-editorial',
  created_at timestamptz not null default now(),
  primary key (revision_id, path_key, capability_key),
  check (mapping_state in ('proposed', 'accepted', 'rejected')),
  check (mapping_version = 'question-brain.taxonomy.v1'),
  check (length(trim(source)) > 0)
);

create index if not exists taxonomy_path_program_idx
  on content.taxonomy_path (program_key, stable_key);
create index if not exists taxonomy_capability_domain_idx
  on content.taxonomy_capability (domain_key, stable_key);
create index if not exists question_capability_revision_idx
  on content.question_capability (revision_id, mapping_state);
create index if not exists question_capability_capability_idx
  on content.question_capability (capability_key, mapping_state);

insert into content.taxonomy_program (stable_key, title)
values ('program.backend-engineer', 'Backend Engineer')
on conflict (stable_key) do nothing;

insert into content.taxonomy_path (stable_key, program_key, title)
values
  ('path.nodejs-typescript', 'program.backend-engineer', 'Node.js + TypeScript'),
  ('path.java-spring', 'program.backend-engineer', 'Java + Spring'),
  ('path.dotnet-csharp', 'program.backend-engineer', '.NET + C#'),
  ('path.go', 'program.backend-engineer', 'Go'),
  ('path.frontend', 'program.backend-engineer', 'Frontend'),
  ('path.system-design', 'program.backend-engineer', 'System Design'),
  ('path.algorithms', 'program.backend-engineer', 'Algorithms'),
  ('path.behavioral', 'program.backend-engineer', 'Behavioral')
on conflict (stable_key) do nothing;

insert into content.taxonomy_domain (stable_key, title, shared)
values
  ('domain.runtime', 'Runtime', true),
  ('domain.http-api', 'HTTP/API', true),
  ('domain.data-postgresql', 'Data/PostgreSQL', true),
  ('domain.distributed-systems', 'Distributed Systems', true),
  ('domain.os-networking', 'OS/Networking', true),
  ('domain.testing', 'Testing', true),
  ('domain.delivery-observability', 'Delivery/Observability', true)
on conflict (stable_key) do nothing;

insert into content.taxonomy_alias (kind, alias, canonical_key, reason)
values
  ('topic', 'Distributed Systems / Resilience', 'topic.distributed-systems-resilience', 'legacy separator alias'),
  ('topic', 'Go / Channels & Select', 'topic.go-channels-select', 'legacy capitalization alias'),
  ('topic', 'Go / Sync Patterns', 'topic.go-sync-patterns', 'legacy conjunction alias'),
  ('stage', 'stage.runtime', 'domain.runtime', 'deprecated Lab stage-to-domain alias'),
  ('stage', 'stage.http-api', 'domain.http-api', 'deprecated Lab stage-to-domain alias'),
  ('stage', 'stage.data-postgresql', 'domain.data-postgresql', 'deprecated Lab stage-to-domain alias'),
  ('stage', 'stage.distributed-systems', 'domain.distributed-systems', 'deprecated Lab stage-to-domain alias'),
  ('stage', 'stage.os-networking', 'domain.os-networking', 'deprecated Lab stage-to-domain alias'),
  ('stage', 'stage.testing', 'domain.testing', 'deprecated Lab stage-to-domain alias'),
  ('stage', 'stage.delivery-observability', 'domain.delivery-observability', 'deprecated Lab stage-to-domain alias')
on conflict (kind, alias) do nothing;
