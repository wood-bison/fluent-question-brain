#!/usr/bin/env node

/**
 * Build the W05 domain-separated mapping release from the immutable 2026-08-25
 * canonical manifest. This is a deterministic, answer-free transformation:
 * only explicit path.algorithms/path.behavioral domain coordinates change.
 * It never reads or rewrites Question Brain payloads.
 */
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const input = path.join(root, 'releases/curriculum-mapping-2026-08-25-canonical.json');
const output = path.join(root, 'releases/curriculum-mapping-2026-08-27-domain-separated.json');
const source = 'question-brain-editorial-topic-registry-v1/domain-separated-2026-08-27';

const manifest = JSON.parse(await fs.readFile(input, 'utf8'));
if (manifest.contract_version !== 'question-brain.curriculum-mapping.v1') {
  throw new Error(`unexpected contract_version: ${manifest.contract_version}`);
}
if (manifest.taxonomy_version !== 'question-brain.taxonomy.v1') {
  throw new Error(`unexpected taxonomy_version: ${manifest.taxonomy_version}`);
}
if (!Array.isArray(manifest.entries) || manifest.entries.length === 0) {
  throw new Error('canonical manifest has no entries');
}

const counts = { algorithms: 0, behavioral: 0, unchanged: 0 };
const entries = manifest.entries.map((entry) => {
  const next = { ...entry, source };
  if (entry.path_key === 'path.algorithms') {
    next.domain_key = 'domain.algorithms';
    counts.algorithms += 1;
  } else if (entry.path_key === 'path.behavioral') {
    next.domain_key = 'domain.behavioral';
    counts.behavioral += 1;
  } else {
    counts.unchanged += 1;
  }
  return next;
});
const result = {
  ...manifest,
  source,
  entries,
};
await fs.writeFile(output, `${JSON.stringify(result, null, 2)}\n`);
console.log(JSON.stringify({ input, output, entryCount: entries.length, counts }, null, 2));
