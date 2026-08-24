-- Reviewed capability inventory for the first executable Lab crosswalk.
--
-- These rows correspond to authored Task Runtime/Lab stations that already
-- have a release contract. They are not generated from legacy Track/Group/
-- Topic values. Additional stations must be reviewed and added explicitly.

insert into content.taxonomy_capability (stable_key, domain_key, title)
values
  ('capability.runtime.fluent-calculator', 'domain.runtime', 'Fluent calculator'),
  ('capability.runtime.deferred', 'domain.runtime', 'Deferred promise settlement'),
  ('capability.runtime.node-event-loop-001', 'domain.runtime', 'Node.js event loop ordering'),
  ('capability.runtime.node-cpu-bound-002', 'domain.runtime', 'CPU-bound work and worker threads'),
  ('capability.runtime.node-streams-003', 'domain.runtime', 'Streams and backpressure'),
  ('capability.runtime.node-memory-004', 'domain.runtime', 'Memory retention and garbage collection'),
  ('capability.runtime.node-concurrency-012', 'domain.runtime', 'Bounded concurrency'),
  ('capability.runtime.dotnet-cancellation-001', 'domain.runtime', '.NET cancellation boundary'),
  ('capability.distributed-systems.rate-limiter', 'domain.distributed-systems', 'Rate limiting and bounded state'),
  ('capability.http-api.rate-limiter', 'domain.http-api', 'Rate limiting API contract'),
  ('capability.http-api.node-auth-015', 'domain.http-api', 'Authentication and authorization'),
  ('capability.data-postgresql.pg-indexes-008', 'domain.data-postgresql', 'PostgreSQL query planning'),
  ('capability.data-postgresql.pg-locks-016', 'domain.data-postgresql', 'PostgreSQL row locks'),
  ('capability.distributed-systems.node-idempotency-013', 'domain.distributed-systems', 'Idempotent message delivery'),
  ('capability.delivery-observability.node-cache-014', 'domain.delivery-observability', 'Cache invalidation and stampede control')
on conflict (stable_key) do update
set domain_key = excluded.domain_key,
    title = excluded.title;
