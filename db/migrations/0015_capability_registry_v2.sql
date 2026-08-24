-- Capability registry v2: additive identity and many-to-many domains.
--
-- The old taxonomy_capability.domain_key and stable_key remain readable for
-- historical releases. New releases must use taxonomy_capability_domain and
-- lifecycle=active rows from the reviewed registry. No question revision,
-- runtime manifest, or learner evidence is rewritten here.

begin;

alter table content.taxonomy_capability
  add column if not exists display_slug text,
  add column if not exists lifecycle text not null default 'active',
  add column if not exists updated_at timestamptz not null default now();

alter table content.taxonomy_capability
  alter column domain_key drop not null;

alter table content.taxonomy_capability
  drop constraint if exists taxonomy_capability_lifecycle_check;

alter table content.taxonomy_capability
  add constraint taxonomy_capability_lifecycle_check
  check (lifecycle in ('active', 'deprecated', 'retired'));

update content.taxonomy_capability
set display_slug = regexp_replace(stable_key, '^capability[.][^.]+[.]', ''),
    updated_at = now()
where display_slug is null;

alter table content.taxonomy_capability
  drop constraint if exists taxonomy_capability_display_slug_check;

alter table content.taxonomy_capability
  add constraint taxonomy_capability_display_slug_check
  check (display_slug is null or display_slug ~ '^[a-z0-9]+(?:[.-][a-z0-9]+)*$');

create table if not exists content.taxonomy_capability_domain (
  capability_key text not null references content.taxonomy_capability(stable_key) on delete cascade,
  domain_key text not null references content.taxonomy_domain(stable_key),
  role text not null default 'primary',
  created_at timestamptz not null default now(),
  primary key (capability_key, domain_key),
  check (role in ('primary', 'secondary'))
);

create index if not exists taxonomy_capability_domain_lookup_idx
  on content.taxonomy_capability_domain (domain_key, capability_key);

insert into content.taxonomy_capability_domain (capability_key, domain_key, role)
select stable_key, domain_key, 'primary'
from content.taxonomy_capability
where domain_key is not null
on conflict (capability_key, domain_key) do nothing;

create table if not exists content.taxonomy_capability_alias (
  alias_key text primary key,
  canonical_key text not null references content.taxonomy_capability(stable_key),
  reason text not null,
  created_at timestamptz not null default now(),
  check (length(trim(alias_key)) > 0),
  check (alias_key <> canonical_key),
  check (length(trim(reason)) > 0)
);

create index if not exists taxonomy_capability_alias_canonical_idx
  on content.taxonomy_capability_alias (canonical_key);

create table if not exists content.taxonomy_capability_supersedes (
  superseded_key text primary key references content.taxonomy_capability(stable_key),
  canonical_key text not null references content.taxonomy_capability(stable_key),
  reason text not null,
  created_at timestamptz not null default now(),
  check (superseded_key <> canonical_key),
  check (length(trim(reason)) > 0)
);

-- A supersedes edge must point forward to a terminal canonical key. Foreign
-- keys reject dangling rows; this trigger rejects a cycle before it can make
-- historical resolution ambiguous.
create or replace function content.reject_capability_supersedes_cycle()
returns trigger
language plpgsql
as $$
declare
  has_cycle boolean;
begin
  with recursive chain(key) as (
    select new.canonical_key
    union
    select s.canonical_key
    from content.taxonomy_capability_supersedes s
    join chain c on s.superseded_key = c.key
  )
  select exists (select 1 from chain where key = new.superseded_key)
    into has_cycle;
  if has_cycle then
    raise exception 'capability supersedes cycle: % -> %', new.superseded_key, new.canonical_key;
  end if;
  return new;
end;
$$;

drop trigger if exists taxonomy_capability_supersedes_cycle on content.taxonomy_capability_supersedes;
create trigger taxonomy_capability_supersedes_cycle
before insert or update on content.taxonomy_capability_supersedes
for each row execute function content.reject_capability_supersedes_cycle();

-- The reviewed registry preserves stable old rows for historical evidence and
-- introduces task-sequence-free canonical identities for new releases.
insert into content.taxonomy_capability
  (stable_key, domain_key, title, display_slug, lifecycle)
values
  ('capability.nodejs.event-loop-ordering', 'domain.runtime', 'Node.js event loop ordering', 'event-loop-ordering', 'active'),
  ('capability.nodejs.cpu-bound-work', 'domain.runtime', 'CPU-bound work and worker threads', 'cpu-bound-work', 'active'),
  ('capability.nodejs.streams-backpressure', 'domain.runtime', 'Streams and backpressure', 'streams-backpressure', 'active'),
  ('capability.nodejs.memory-retention', 'domain.runtime', 'Memory retention and garbage collection', 'memory-retention', 'active'),
  ('capability.nodejs.bounded-concurrency', 'domain.runtime', 'Bounded concurrency', 'bounded-concurrency', 'active'),
  ('capability.dotnet.cancellation-boundary', 'domain.runtime', '.NET cancellation boundary', 'cancellation-boundary', 'active'),
  ('capability.http-api.authentication-authorization', 'domain.http-api', 'Authentication and authorization', 'authentication-authorization', 'active'),
  ('capability.postgresql.query-planning', 'domain.data-postgresql', 'PostgreSQL query planning', 'query-planning', 'active'),
  ('capability.postgresql.row-locks', 'domain.data-postgresql', 'PostgreSQL row locks', 'row-locks', 'active'),
  ('capability.distributed-systems.idempotent-delivery', 'domain.distributed-systems', 'Idempotent message delivery', 'idempotent-delivery', 'active'),
  ('capability.delivery-observability.cache-invalidation', 'domain.delivery-observability', 'Cache invalidation and stampede control', 'cache-invalidation', 'active')
on conflict (stable_key) do update
set domain_key = excluded.domain_key,
    title = excluded.title,
    display_slug = excluded.display_slug,
    lifecycle = 'active',
    updated_at = now();

insert into content.taxonomy_capability_domain (capability_key, domain_key, role)
select stable_key, domain_key, 'primary'
from content.taxonomy_capability
where stable_key in (
  'capability.nodejs.event-loop-ordering',
  'capability.nodejs.cpu-bound-work',
  'capability.nodejs.streams-backpressure',
  'capability.nodejs.memory-retention',
  'capability.nodejs.bounded-concurrency',
  'capability.dotnet.cancellation-boundary',
  'capability.http-api.authentication-authorization',
  'capability.postgresql.query-planning',
  'capability.postgresql.row-locks',
  'capability.distributed-systems.idempotent-delivery',
  'capability.delivery-observability.cache-invalidation'
)
on conflict (capability_key, domain_key) do nothing;

update content.taxonomy_capability
set lifecycle = 'deprecated', updated_at = now()
where stable_key in (
  'capability.runtime.node-event-loop-001',
  'capability.runtime.node-cpu-bound-002',
  'capability.runtime.node-streams-003',
  'capability.runtime.node-memory-004',
  'capability.runtime.node-concurrency-012',
  'capability.runtime.dotnet-cancellation-001',
  'capability.http-api.node-auth-015',
  'capability.data-postgresql.pg-indexes-008',
  'capability.data-postgresql.pg-locks-016',
  'capability.distributed-systems.node-idempotency-013',
  'capability.delivery-observability.node-cache-014'
);

insert into content.taxonomy_capability_alias (alias_key, canonical_key, reason)
values
  ('capability.runtime.node-event-loop-001', 'capability.nodejs.event-loop-ordering', 'remove task sequence from observable skill key'),
  ('capability.runtime.node-cpu-bound-002', 'capability.nodejs.cpu-bound-work', 'remove task sequence from observable skill key'),
  ('capability.runtime.node-streams-003', 'capability.nodejs.streams-backpressure', 'remove task sequence from observable skill key'),
  ('capability.runtime.node-memory-004', 'capability.nodejs.memory-retention', 'remove task sequence from observable skill key'),
  ('capability.runtime.node-concurrency-012', 'capability.nodejs.bounded-concurrency', 'remove task sequence from observable skill key'),
  ('capability.runtime.dotnet-cancellation-001', 'capability.dotnet.cancellation-boundary', 'remove task sequence from observable skill key'),
  ('capability.http-api.node-auth-015', 'capability.http-api.authentication-authorization', 'remove implementation name and task sequence'),
  ('capability.data-postgresql.pg-indexes-008', 'capability.postgresql.query-planning', 'remove task sequence from observable skill key'),
  ('capability.data-postgresql.pg-locks-016', 'capability.postgresql.row-locks', 'remove task sequence from observable skill key'),
  ('capability.distributed-systems.node-idempotency-013', 'capability.distributed-systems.idempotent-delivery', 'remove implementation name and task sequence'),
  ('capability.delivery-observability.node-cache-014', 'capability.delivery-observability.cache-invalidation', 'remove implementation name and task sequence')
on conflict (alias_key) do update
set canonical_key = excluded.canonical_key,
    reason = excluded.reason;

insert into content.taxonomy_capability_supersedes (superseded_key, canonical_key, reason)
select alias_key, canonical_key, reason
from content.taxonomy_capability_alias
where alias_key like 'capability.%'
on conflict (superseded_key) do update
set canonical_key = excluded.canonical_key,
    reason = excluded.reason;

commit;
