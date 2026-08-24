import type { Payload } from 'payload'

// Local-development convenience: skip the login form on a loopback-only stack.
//
// This is deliberately opt-in through one loud environment variable rather than
// inferred from NODE_ENV, because this Compose stack runs NODE_ENV=production
// even on a laptop — inferring would have turned auth off exactly where it
// still matters. Off unless QB_CMS_DEV_LOGIN is set.
const enabled = process.env.QB_CMS_DEV_LOGIN === '1' || process.env.QB_CMS_DEV_LOGIN === 'true'

export const devLogin = {
  enabled,
  email: process.env.QB_CMS_DEV_EMAIL ?? 'dev@local.dev',
  password: process.env.QB_CMS_DEV_PASSWORD ?? 'local-dev-password',
}

// Creates the dev account on boot so the credentials autoLogin submits actually
// resolve. Idempotent: an existing account is left untouched, including its
// password, so a locally changed password is never silently reset.
export async function seedDevUser(payload: Payload): Promise<void> {
  if (!devLogin.enabled) return

  payload.logger.warn(
    `QB_CMS_DEV_LOGIN is on: the admin auto-signs in as ${devLogin.email}. ` +
      'Intended for the loopback-bound local stack only — never enable it on a reachable deployment.',
  )

  const existing = await payload.find({
    collection: 'users',
    where: { email: { equals: devLogin.email } },
    limit: 1,
    depth: 0,
  })
  if (existing.totalDocs > 0) return

  await payload.create({
    collection: 'users',
    data: { email: devLogin.email, password: devLogin.password },
  })
  payload.logger.info(`Seeded local development admin ${devLogin.email}.`)
}
