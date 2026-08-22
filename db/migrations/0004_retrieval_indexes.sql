-- G3 benchmark winner for the current 2.4k-vector development corpus.
-- A new embedding profile gets a separately benchmarked index; this index is
-- intentionally named with the profile so it cannot silently mix models.

create index if not exists question_embedding_hnsw_dev
  on content.question_embedding using hnsw (embedding vector_cosine_ops)
  with (m = 16, ef_construction = 64);
