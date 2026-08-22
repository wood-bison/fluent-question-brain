# Payload authoring surface (G4)

Payload CMS is a committed architectural choice, but it is intentionally not
installed as a half-configured second runtime in G1. The G4 implementation
will use the official Postgres adapter, versions/drafts, field localization,
and jobs for editorial workflows. It will write only the isolated `cms`
schema. A promote hook calls the Go command API; it never writes canonical
`content` tables directly.

Before this directory becomes a runnable service, the implementation must pin
the Payload/Next/Node versions, commit a lockfile, add an admin auth model,
and test a draft → review → promote → index flow against the Compose stack.

