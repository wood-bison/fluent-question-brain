-- Curriculum mapping release v1.
--
-- This table is deliberately separate from question_revision and from the
-- many-to-many question_capability relation.  It records one explicit
-- Program/Path/Domain crosswalk decision for a pinned revision.  An
-- `unmapped` row is an audit decision, not a curriculum assignment; its
-- canonical keys remain NULL until an editor supplies them.

create table if not exists content.question_curriculum_mapping (
  revision_id uuid primary key references content.question_revision(id) on delete cascade,
  program_key text references content.taxonomy_program(stable_key),
  path_key text references content.taxonomy_path(stable_key),
  domain_key text references content.taxonomy_domain(stable_key),
  capability_key text references content.taxonomy_capability(stable_key),
  mapping_state text not null default 'unmapped',
  mapping_version text not null default 'question-brain.taxonomy.v1',
  source text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  check (mapping_state in ('unmapped', 'proposed', 'accepted', 'rejected')),
  check (mapping_version = 'question-brain.taxonomy.v1'),
  check (length(trim(source)) > 0),
  check (
    mapping_state = 'unmapped'
    or (program_key is not null and path_key is not null and domain_key is not null)
  ),
  check (capability_key is null or (path_key is not null and domain_key is not null))
);

create index if not exists question_curriculum_mapping_path_idx
  on content.question_curriculum_mapping (path_key, domain_key, mapping_state);
create index if not exists question_curriculum_mapping_capability_idx
  on content.question_curriculum_mapping (capability_key, mapping_state);
