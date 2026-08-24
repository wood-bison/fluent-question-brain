import * as migration_20260822_101040_initial from './20260822_101040_initial';
import * as migration_20260824_path_taxonomy from './20260824_path_taxonomy';
import * as migration_20260824_published_questions_view from './20260824_published_questions_view';

export const migrations = [
  {
    up: migration_20260822_101040_initial.up,
    down: migration_20260822_101040_initial.down,
    name: '20260822_101040_initial'
  },
  {
    up: migration_20260824_path_taxonomy.up,
    down: migration_20260824_path_taxonomy.down,
    name: '20260824_path_taxonomy'
  },
  {
    up: migration_20260824_published_questions_view.up,
    down: migration_20260824_published_questions_view.down,
    name: '20260824_published_questions_view'
  },
];
