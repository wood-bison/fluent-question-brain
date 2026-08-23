-- Promote the real multilingual retrieval profile (QB-BUG-2): bge-m3 served
-- by the local Ollama instance. bge-m3 is multilingual and produces exactly
-- 1024 dimensions, so the existing vector(1024) column and CHECK constraint
-- stay valid and no table migration is required.
--
-- The deterministic hash profile is retired to inactive but kept as a row so
-- its stored vectors remain addressable for rollback comparisons.

update content.embedding_profile
set provider = 'ollama',
    model = 'bge-m3',
    purpose = 'question-retrieval',
    dimensions = 1024,
    distance_metric = 'cosine'
where profile_key = 'semantic-v1';

update content.embedding_profile
set active = false
where profile_key = 'semantic-dev-hash-v1';

update content.embedding_profile
set active = true
where profile_key = 'semantic-v1';
