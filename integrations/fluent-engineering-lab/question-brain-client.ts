/**
 * Tiny dependency-free Lab adapter. Keep the learner projection in the Lab;
 * this client only knows the versioned Question Brain HTTP contract.
 */

export type QuestionBrainConfig = {
  readonly enabled: boolean
  readonly baseURL: string
  readonly workspace: string
  readonly timeoutMs: number
}

export type QuestionBrainSearchResult = {
  readonly stable_key: string
  readonly slug: string
  readonly locale: string
  readonly prompt: string
  readonly short_answer: string | null
  readonly explanation: string | null
  readonly topic_key: string
  readonly topic_title: string
  readonly match_stages: readonly string[]
  readonly revision_id: string
  readonly content_hash: string
}

export type QuestionBrainSearchResponse = {
  readonly query: string
  readonly locale: string
  readonly topic_key: string
  readonly results: readonly QuestionBrainSearchResult[]
  readonly provenance: {
    readonly explainable: true
    readonly pipeline: readonly string[]
  }
}

const envNumber = (value: string | undefined, defaultValue: number): number => {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : defaultValue
}

export function questionBrainConfig(env: Record<string, string | undefined> = process.env): QuestionBrainConfig {
  return {
    enabled: env.QUESTION_BRAIN_READS === '1',
    baseURL: (env.QUESTION_BRAIN_BASE_URL ?? 'http://127.0.0.1:48127').replace(/\/$/, ''),
    workspace: env.QUESTION_BRAIN_WORKSPACE ?? 'fluent-interview',
    timeoutMs: envNumber(env.QUESTION_BRAIN_TIMEOUT_MS, 1200),
  }
}

export class QuestionBrainClient {
  public readonly config: QuestionBrainConfig

  constructor(config = questionBrainConfig()) {
    this.config = config
  }

  async search(input: {
    readonly query: string
    readonly locale?: 'en' | 'ru'
    readonly topicKey?: string
    readonly limit?: number
  }): Promise<QuestionBrainSearchResponse | null> {
    if (!this.config.enabled) return null
    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), this.config.timeoutMs)
    try {
      const response = await fetch(`${this.config.baseURL}/v1/search`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        signal: controller.signal,
        body: JSON.stringify({
          query: input.query,
          locale: input.locale ?? 'en',
          topic_key: input.topicKey ?? '',
          limit: input.limit ?? 24,
        }),
      })
      if (!response.ok) {
        throw new Error(`Question Brain search failed: ${response.status}`)
      }
      return (await response.json()) as QuestionBrainSearchResponse
    } finally {
      clearTimeout(timeout)
    }
  }

  async question(stableKey: string, locale: 'en' | 'ru' = 'en'): Promise<unknown | null> {
    if (!this.config.enabled) return null
    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), this.config.timeoutMs)
    try {
      const response = await fetch(
        `${this.config.baseURL}/v1/questions/${encodeURIComponent(stableKey)}?locale=${locale}`,
        { signal: controller.signal },
      )
      if (!response.ok) throw new Error(`Question Brain question read failed: ${response.status}`)
      return response.json()
    } finally {
      clearTimeout(timeout)
    }
  }
}
