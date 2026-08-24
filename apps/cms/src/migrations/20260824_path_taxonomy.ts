import { MigrateDownArgs, MigrateUpArgs, sql } from '@payloadcms/db-postgres'

/** Additive editorial fields for the Question Brain taxonomy v1 crosswalk. */
export async function up({ db }: MigrateUpArgs): Promise<void> {
  await db.execute(sql`
    ALTER TABLE "cms"."questions"
      ADD COLUMN IF NOT EXISTS "program_key" varchar,
      ADD COLUMN IF NOT EXISTS "path_key" varchar,
      ADD COLUMN IF NOT EXISTS "domain_key" varchar,
      ADD COLUMN IF NOT EXISTS "capability_key" varchar,
      ADD COLUMN IF NOT EXISTS "mapping_state" varchar;
    ALTER TABLE "cms"."_questions_v"
      ADD COLUMN IF NOT EXISTS "version_program_key" varchar,
      ADD COLUMN IF NOT EXISTS "version_path_key" varchar,
      ADD COLUMN IF NOT EXISTS "version_domain_key" varchar,
      ADD COLUMN IF NOT EXISTS "version_capability_key" varchar,
      ADD COLUMN IF NOT EXISTS "version_mapping_state" varchar;
  `)
}

export async function down({ db }: MigrateDownArgs): Promise<void> {
  await db.execute(sql`
    ALTER TABLE "cms"."questions"
      DROP COLUMN IF EXISTS "program_key",
      DROP COLUMN IF EXISTS "path_key",
      DROP COLUMN IF EXISTS "domain_key",
      DROP COLUMN IF EXISTS "capability_key",
      DROP COLUMN IF EXISTS "mapping_state";
    ALTER TABLE "cms"."_questions_v"
      DROP COLUMN IF EXISTS "version_program_key",
      DROP COLUMN IF EXISTS "version_path_key",
      DROP COLUMN IF EXISTS "version_domain_key",
      DROP COLUMN IF EXISTS "version_capability_key",
      DROP COLUMN IF EXISTS "version_mapping_state";
  `)
}
