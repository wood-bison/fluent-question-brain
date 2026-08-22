-- QF0 release boundary. A published revision can still be a development
-- fixture; publication is not the same as learner eligibility.
alter table content.question
  add column if not exists content_kind text not null default 'production';

alter table content.question
  drop constraint if exists question_content_kind_check;
alter table content.question
  add constraint question_content_kind_check
  check (content_kind in ('production', 'fixture', 'migration', 'example'));

-- Existing local smoke records predate the explicit field. Keep this migration
-- deterministic and narrow; future authoring writes the field directly.
update content.question
set content_kind = 'fixture'
where stable_key like 'g4.%'
   or stable_key = 'g5.rollback-smoke';

create index if not exists question_workspace_release_kind_idx
  on content.question (workspace_id, status, content_kind, updated_at desc);
