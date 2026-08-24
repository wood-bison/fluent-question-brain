import type { CollectionConfig } from 'payload'

import { promotePublishedQuestion } from '../hooks/promotePublishedQuestion'

export const Questions: CollectionConfig = {
  slug: 'questions',
  admin: {
    useAsTitle: 'stableKey',
    defaultColumns: ['stableKey', 'title', '_status', 'updatedAt'],
    description:
      'Draft and review questions here. Publishing sends one canonical payload to the Go Question Brain API; it never writes content tables directly.',
  },
  access: {
    create: ({ req }) => Boolean(req.user),
    delete: ({ req }) => Boolean(req.user),
    read: ({ req }) => Boolean(req.user),
    update: ({ req }) => Boolean(req.user),
  },
  hooks: {
    afterChange: [promotePublishedQuestion],
  },
  versions: {
    drafts: {
      autosave: {
        interval: 750,
      },
      validate: true,
    },
    maxPerDoc: 50,
  },
  fields: [
    {
      name: 'stableKey',
      type: 'text',
      required: true,
      unique: true,
      admin: {
        description: 'Stable graph identity, for example question.q001. Never rename after publish.',
      },
    },
    {
      name: 'slug',
      type: 'text',
      required: true,
      unique: true,
    },
    {
      name: 'workspaceKey',
      type: 'text',
      defaultValue: 'fluent-interview',
      required: true,
    },
    {
      name: 'workspaceName',
      type: 'text',
      defaultValue: 'Fluent Interview',
      required: true,
    },
    {
      name: 'title',
      type: 'text',
      localized: true,
      required: true,
    },
    {
      name: 'track',
      type: 'text',
    },
    {
      name: 'topic',
      type: 'text',
    },
    {
      name: 'scope',
      type: 'text',
    },
    {
      name: 'priority',
      type: 'text',
    },
    {
      name: 'group',
      type: 'text',
    },
    {
      name: 'level',
      type: 'text',
    },
    {
      name: 'programKey',
      type: 'text',
      admin: {
        description: 'Optional curriculum program key, for example program.backend-engineer.',
      },
    },
    {
      name: 'pathKey',
      type: 'text',
      admin: {
        description: 'Optional explicit Lab path key. Do not infer it from Track.',
      },
    },
    {
      name: 'domainKey',
      type: 'text',
      admin: {
        description: 'Optional shared Lab domain key. Deprecated stage_key maps here.',
      },
    },
    {
      name: 'capabilityKey',
      type: 'text',
      admin: {
        description: 'Optional reviewed Lab capability key; Topic is not a capability.',
      },
    },
    {
      name: 'mappingState',
      type: 'select',
      defaultValue: 'proposed',
      options: [
        { label: 'Proposed', value: 'proposed' },
        { label: 'Accepted', value: 'accepted' },
        { label: 'Rejected', value: 'rejected' },
      ],
    },
    {
      name: 'question',
      type: 'textarea',
      localized: true,
      required: true,
    },
    {
      name: 'shortAnswer',
      type: 'textarea',
      localized: true,
    },
    {
      name: 'explanation',
      type: 'textarea',
      localized: true,
    },
    {
      name: 'sections',
      type: 'json',
      localized: true,
      admin: {
        description:
          'Optional JSON array of {"title":"...","body":"..."} sections. The promote hook adds the localized question, core idea, and explanation sections.',
      },
    },
  ],
}
