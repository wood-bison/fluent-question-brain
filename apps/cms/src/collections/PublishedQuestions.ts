import type { CollectionConfig } from 'payload'

const readOnly = { readOnly: true } as const

// The released bank, surfaced in the admin for reading only.
//
// Backed by the `cms.published_questions` view over the canonical content
// schema, so there is nothing here to keep in sync — the rows are the Go
// service's rows. Every write path is denied: Payload cannot create, update or
// delete, and a view would reject the statement anyway.
//
// Authoring still happens in the `questions` collection, which promotes through
// the API. This collection exists so you can see what is actually published.
export const PublishedQuestions: CollectionConfig = {
  slug: 'published-questions',
  labels: {
    singular: 'Опубликованный вопрос',
    plural: 'Опубликованные вопросы',
  },
  admin: {
    useAsTitle: 'promptRu',
    defaultColumns: ['id', 'promptRu', 'track', 'level', 'topic'],
    description:
      'Живой срез канонического банка (только чтение). Данные принадлежат Go-сервису; правки делаются в разделе Questions и публикуются через промоушен.',
    pagination: { defaultLimit: 50 },
  },
  access: {
    read: ({ req }) => Boolean(req.user),
    create: () => false,
    update: () => false,
    delete: () => false,
  },
  // The view carries no version history of its own; revisions live in the
  // canonical schema and are addressed by contentHash / revisionNo below.
  versions: false,
  // Edit locks exist to stop two people editing one document. Nothing here is
  // editable, and the lock table has no relation column for this collection
  // (it predates it, and the adapter runs with push disabled), so asking for
  // locks would only produce a failing query on every document open.
  lockDocuments: false,
  fields: [
    // Custom text ID: the stable graph key is already unique and readable,
    // which beats a synthetic integer the view cannot supply anyway.
    { name: 'id', type: 'text', label: 'Стабильный ключ', admin: readOnly },
    { name: 'promptRu', type: 'textarea', label: 'Вопрос (RU)', admin: readOnly },
    { name: 'promptEn', type: 'textarea', label: 'Вопрос (EN)', admin: readOnly },
    {
      type: 'row',
      fields: [
        { name: 'track', type: 'text', label: 'Трек', index: true, admin: { ...readOnly, width: '25%' } },
        { name: 'level', type: 'text', label: 'Уровень', index: true, admin: { ...readOnly, width: '25%' } },
        { name: 'company', type: 'text', label: 'Компания', index: true, admin: { ...readOnly, width: '25%' } },
        { name: 'group', type: 'text', label: 'Группа', admin: { ...readOnly, width: '25%' } },
      ],
    },
    { name: 'topic', type: 'text', label: 'Тема', index: true, admin: readOnly },
    { name: 'title', type: 'text', label: 'Заголовок', admin: readOnly },
    { name: 'shortAnswerRu', type: 'textarea', label: 'Короткий ответ (RU)', admin: readOnly },
    { name: 'shortAnswerEn', type: 'textarea', label: 'Короткий ответ (EN)', admin: readOnly },
    { name: 'explanationRu', type: 'textarea', label: 'Разбор (RU)', admin: readOnly },
    { name: 'explanationEn', type: 'textarea', label: 'Разбор (EN)', admin: readOnly },
    {
      type: 'row',
      fields: [
        { name: 'slug', type: 'text', label: 'Слаг', admin: { ...readOnly, width: '25%' } },
        { name: 'status', type: 'text', label: 'Статус', admin: { ...readOnly, width: '25%' } },
        { name: 'revisionNo', type: 'number', label: 'Ревизия', admin: { ...readOnly, width: '15%' } },
        { name: 'contentHash', type: 'text', label: 'Хеш содержимого', admin: { ...readOnly, width: '35%' } },
      ],
    },
  ],
}
