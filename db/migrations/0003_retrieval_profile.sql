-- G3 retrieval profile. The deterministic hash provider is a local pipeline
-- probe, not a claim of semantic quality; production providers get a new
-- immutable profile and a measured backfill.

insert into content.embedding_profile
  (profile_key, provider, model, purpose, dimensions, distance_metric, active)
values
  ('semantic-dev-hash-v1', 'local', 'sha256-token-v1', 'question-retrieval', 1024, 'cosine', true)
on conflict (profile_key) do update set
  provider = excluded.provider,
  model = excluded.model,
  purpose = excluded.purpose,
  dimensions = excluded.dimensions,
  distance_metric = excluded.distance_metric,
  active = excluded.active;

create index if not exists question_embedding_profile_locale_idx
  on content.question_embedding (profile_key, locale_id, created_at desc);
