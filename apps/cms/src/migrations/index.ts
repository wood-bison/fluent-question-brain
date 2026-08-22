import * as migration_20260822_101040_initial from './20260822_101040_initial';

export const migrations = [
  {
    up: migration_20260822_101040_initial.up,
    down: migration_20260822_101040_initial.down,
    name: '20260822_101040_initial'
  },
];
