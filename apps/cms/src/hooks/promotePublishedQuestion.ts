import type { CollectionAfterChangeHook } from 'payload'

type Locale = 'en' | 'ru'
type Localized<T> = T | Partial<Record<Locale, T>>

const locales: Locale[] = ['en', 'ru']

const localizedValue = <T>(value: unknown, locale: Locale): T | undefined => {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return (value as Record<string, T | undefined>)[locale]
  }
  return value as T | undefined
}

const nonEmpty = (...values: Array<unknown>): string => {
  for (const value of values) {
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return ''
}

const sectionsFor = (value: unknown): Array<{ title: string; body: string }> => {
  if (!Array.isArray(value)) return []
  return value.flatMap((entry) => {
    if (!entry || typeof entry !== 'object') return []
    const record = entry as Record<string, unknown>
    const title = typeof record.title === 'string' ? record.title.trim() : ''
    const body = typeof record.body === 'string' ? record.body.trim() : ''
    return title && body ? [{ title, body }] : []
  })
}

export const promotePublishedQuestion: CollectionAfterChangeHook = async ({ doc, req }) => {
  if (doc._status !== 'published') return doc

  // Payload passes the document in the locale selected for this write. When
  // the admin/API submits `locale=all`, localized fields are objects keyed by
  // locale and we can promote both languages in one transaction. We avoid a
  // second read here: afterChange runs inside Payload's write transaction and
  // a nested findByID can legitimately observe no row yet.
  const full = doc as unknown as Record<string, unknown>

  const values = Object.fromEntries(
    locales.map((locale) => {
      const title = nonEmpty(localizedValue<string>(full.title, locale))
      const question = nonEmpty(localizedValue<string>(full.question, locale))
      const shortAnswer = nonEmpty(localizedValue<string>(full.shortAnswer, locale))
      const explanation = nonEmpty(localizedValue<string>(full.explanation, locale))
      const sections = sectionsFor(localizedValue<unknown>(full.sections, locale))
      return [locale, { title, question, shortAnswer, explanation, sections }]
    }),
  ) as Record<Locale, { title: string; question: string; shortAnswer: string; explanation: string; sections: Array<{ title: string; body: string }> }>

  const english = values.en
  const russian = values.ru
  const englishQuestion = nonEmpty(english.question, russian.question)
  const englishTitle = nonEmpty(english.title, russian.title, full.stableKey)
  if (!englishQuestion) {
    throw new Error('A published question must have an English or Russian question prompt')
  }

  const sections = [
    { title: 'Question', body: englishQuestion },
    { title: 'Core Idea', body: nonEmpty(english.shortAnswer, russian.shortAnswer) },
    { title: 'Explanation', body: nonEmpty(english.explanation, russian.explanation) },
    { title: 'Question (RU)', body: russian.question },
    { title: 'Core Idea (RU)', body: russian.shortAnswer },
    { title: 'Russian Explanation', body: russian.explanation },
    ...english.sections,
  ].filter((section) => section.body)

  const body = {
    workspace_key: nonEmpty(full.workspaceKey, 'fluent-interview'),
    workspace_name: nonEmpty(full.workspaceName, 'Fluent Interview'),
    source_ref: `payload://question/${String(full.id)}`,
    stable_key: String(full.stableKey ?? ''),
    slug: String(full.slug ?? ''),
    title: englishTitle,
    track: String(full.track ?? ''),
    topic: String(full.topic ?? ''),
    scope: String(full.scope ?? ''),
    lang: 'en+ru',
    priority: String(full.priority ?? ''),
    group: String(full.group ?? ''),
    level: String(full.level ?? ''),
    question: englishQuestion,
    sections,
  }

  const promoteURL = process.env.QUESTION_BRAIN_PROMOTE_URL
  const token = process.env.QUESTION_BRAIN_INTERNAL_TOKEN
  if (!promoteURL || !token) {
    throw new Error('QUESTION_BRAIN_PROMOTE_URL and QUESTION_BRAIN_INTERNAL_TOKEN are required to publish')
  }
  const actor = req.user && typeof req.user.email === 'string' ? req.user.email : 'payload-cms'
  const response = await fetch(promoteURL, {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
      'x-question-brain-token': token,
      'x-question-brain-actor': actor,
    },
    body: JSON.stringify(body),
  })
  if (!response.ok) {
    const detail = await response.text()
    throw new Error(`Question Brain promote failed (${response.status}): ${detail.slice(0, 500)}`)
  }

  req.payload.logger.info({
    msg: 'question promoted to canonical Question Brain API',
    stableKey: body.stable_key,
    actor,
  })
  return doc
}
