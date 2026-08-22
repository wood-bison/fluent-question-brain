-- G4 authoring boundary.
-- Payload owns only its draft/version tables in this schema. It must never
-- write the canonical content schema directly; the promote hook calls the Go
-- API, which remains the single published-content writer.
create schema if not exists cms;
