import path from 'node:path'
import { fileURLToPath } from 'node:url'

import { postgresAdapter } from '@payloadcms/db-postgres'
import { lexicalEditor } from '@payloadcms/richtext-lexical'
import { buildConfig } from 'payload'

import { Questions } from './src/collections/Questions'
import { Users } from './src/collections/Users'

const filename = fileURLToPath(import.meta.url)
const dirname = path.dirname(filename)

export default buildConfig({
  admin: {
    user: Users.slug,
    importMap: {
      baseDir: path.resolve(dirname, 'src'),
    },
  },
  collections: [Users, Questions],
  editor: lexicalEditor(),
  localization: {
    locales: ['en', 'ru'],
    defaultLocale: 'ru',
    fallback: true,
  },
  db: postgresAdapter({
    pool: {
      connectionString:
        process.env.PAYLOAD_DATABASE_URL ??
        'postgres://question_brain:question_brain@localhost:55437/question_brain?sslmode=disable',
    },
    schemaName: 'cms',
    push: false,
  }),
  secret: process.env.PAYLOAD_SECRET ?? '',
  typescript: {
    outputFile: path.resolve(dirname, 'src/payload-types.ts'),
  },
  graphQL: {
    disable: true,
  },
  telemetry: false,
})
