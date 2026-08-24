import { MigrateUpArgs, MigrateDownArgs, sql } from '@payloadcms/db-postgres'

// A read-only window from the CMS onto the canonical, Go-owned content schema.
//
// The single-writer rule this stack is built on constrains writes, not reads:
// Payload must never write `content`, but there is no reason an editor cannot
// see what is published. Without this view the admin shows only its own drafts
// (two rows) while the released bank (1392 cards) stays invisible.
//
// It is a VIEW, not a table, so it cannot drift from the canonical rows and
// cannot be written through. The collection that maps to it also denies
// create/update/delete, so both layers have to fail before a write is possible.
export async function up({ db }: MigrateUpArgs): Promise<void> {
  await db.execute(sql`
    CREATE VIEW "cms"."published_questions" AS
    SELECT
      q.stable_key                                        AS id,
      q.slug                                              AS slug,
      q.status                                            AS status,
      q.level                                             AS level,
      COALESCE(NULLIF(q.company, 'unclassified'), '')     AS company,
      COALESCE(qr.normalized_payload->>'track', '')       AS track,
      COALESCE(qr.normalized_payload->>'topic', '')       AS topic,
      COALESCE(qr.normalized_payload->>'group', '')       AS "group",
      COALESCE(qr.normalized_payload->>'title', '')       AS title,
      qr.content_hash                                     AS content_hash,
      qr.revision_no                                      AS revision_no,
      COALESCE((
        SELECT ql.prompt FROM content.question_locale ql
        WHERE ql.revision_id = qr.id AND ql.locale = 'ru' LIMIT 1
      ), '')                                              AS prompt_ru,
      COALESCE((
        SELECT ql.prompt FROM content.question_locale ql
        WHERE ql.revision_id = qr.id AND ql.locale = 'en' LIMIT 1
      ), '')                                              AS prompt_en,
      COALESCE((
        SELECT ql.short_answer FROM content.question_locale ql
        WHERE ql.revision_id = qr.id AND ql.locale = 'ru' LIMIT 1
      ), '')                                              AS short_answer_ru,
      COALESCE((
        SELECT ql.short_answer FROM content.question_locale ql
        WHERE ql.revision_id = qr.id AND ql.locale = 'en' LIMIT 1
      ), '')                                              AS short_answer_en,
      COALESCE((
        SELECT ql.explanation FROM content.question_locale ql
        WHERE ql.revision_id = qr.id AND ql.locale = 'ru' LIMIT 1
      ), '')                                              AS explanation_ru,
      COALESCE((
        SELECT ql.explanation FROM content.question_locale ql
        WHERE ql.revision_id = qr.id AND ql.locale = 'en' LIMIT 1
      ), '')                                              AS explanation_en,
      q.created_at                                        AS created_at,
      q.updated_at                                        AS updated_at
    FROM content.question q
    JOIN content.question_revision qr ON qr.id = q.current_revision_id
    WHERE q.status = 'published'
      AND q.content_kind = 'production';
  `)
}

export async function down({ db }: MigrateDownArgs): Promise<void> {
  await db.execute(sql`DROP VIEW IF EXISTS "cms"."published_questions";`)
}
