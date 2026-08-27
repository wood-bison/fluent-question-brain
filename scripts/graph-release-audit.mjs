#!/usr/bin/env node

/**
 * W07 read-only graph release audit.
 *
 * The API is the only boundary this audit reads. It joins the immutable graph
 * release to accepted proposal evidence and the published catalog, then
 * fails closed on test provenance, stale/archived endpoints, cycles, unknown
 * kinds, duplicate edges, or missing reviewer evidence.
 */
import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import path from 'node:path';

const API_URL = (process.env.QUESTION_BRAIN_API_URL ?? 'http://127.0.0.1:48127').replace(/\/$/u, '');
const WORKSPACE = process.env.QUESTION_BRAIN_WORKSPACE ?? 'fluent-interview';
const output = path.resolve(process.env.GRAPH_AUDIT_JSON ?? 'docs/verification/two-audit-remediation/W07/graph-release-audit.json');
const markdownOutput = path.resolve(process.env.GRAPH_AUDIT_MD ?? 'docs/verification/two-audit-remediation/W07/graph-release-audit.md');
const checkOnly = process.argv.includes('--check');
const knownKinds = new Set(['prerequisite', 'related', 'contrast', 'follow_up', 'variant', 'duplicate', 'supersedes']);
const testMarker = /fixture|smoke|synthetic|\btest\b/iu;

async function getJson(route) {
  const response = await fetch(`${API_URL}${route}`);
  if (!response.ok) throw new Error(`${route}: HTTP ${response.status}`);
  return response.json();
}

function digest(value) {
  return crypto.createHash('sha256').update(JSON.stringify(value)).digest('hex');
}

function cycles(edges) {
  const adjacency = new Map();
  for (const edge of edges.filter((item) => item.kind === 'prerequisite')) {
    const list = adjacency.get(edge.from_stable_key) ?? [];
    list.push(edge.to_stable_key);
    adjacency.set(edge.from_stable_key, list);
  }
  const visiting = new Set();
  const visited = new Set();
  let count = 0;
  const visit = (node) => {
    if (visiting.has(node)) {
      count += 1;
      return;
    }
    if (visited.has(node)) return;
    visiting.add(node);
    for (const target of adjacency.get(node) ?? []) visit(target);
    visiting.delete(node);
    visited.add(node);
  };
  for (const node of adjacency.keys()) visit(node);
  return count;
}

const current = await getJson(`/v1/graph/releases/current?workspace=${encodeURIComponent(WORKSPACE)}`);
const releaseId = current.graph_release_id;
const release = await getJson(`/v1/graph/releases/${encodeURIComponent(releaseId)}`);
const proposalResponse = await getJson(`/v1/graph/proposals?workspace=${encodeURIComponent(WORKSPACE)}&status=accepted`);
const catalog = await getJson(`/v1/catalog?workspace=${encodeURIComponent(WORKSPACE)}&locale=en&limit=2000`);
const proposals = new Map((proposalResponse.proposals ?? []).map((proposal) => [proposal.id, proposal]));
const catalogByKey = new Map((catalog.questions ?? []).map((question) => [question.stable_key, question]));
const violations = [];
const warnings = [];
const seenEdges = new Set();
let orphaned = 0;
let stale = 0;
let archived = 0;
let invalidStatus = 0;
let missingEvidence = 0;
let testProvenance = 0;

if (current.contract_version !== 'question-brain.graph-edge.v1' || release.contract_version !== current.contract_version) violations.push('graph contract version mismatch');
if (current.status !== 'active' || release.status !== 'active') violations.push('current graph release is not active');
if (current.graph_release_id !== release.graph_release_id) violations.push('current and detail release IDs differ');
if (release.workspace_key !== WORKSPACE) violations.push('graph release workspace mismatch');
if (release.edge_count !== (release.edges ?? []).length) violations.push('edge_count does not equal returned edge length');

for (const edge of release.edges ?? []) {
  const edgeKey = `${edge.from_stable_key}|${edge.to_stable_key}|${edge.kind}`;
  if (seenEdges.has(edgeKey)) violations.push(`duplicate released edge ${edgeKey}`);
  seenEdges.add(edgeKey);
  if (!knownKinds.has(edge.kind)) violations.push(`unknown released edge kind ${edge.kind}`);
  const proposal = proposals.get(edge.proposal_id);
  if (!proposal) {
    violations.push(`released edge ${edge.proposal_id ?? edgeKey} has no accepted proposal evidence`);
  } else {
    if (proposal.status !== 'accepted') violations.push(`released proposal ${proposal.id} is not accepted`);
    if (proposal.kind !== edge.kind || proposal.from_stable_key !== edge.from_stable_key || proposal.to_stable_key !== edge.to_stable_key) violations.push(`released edge ${edgeKey} does not match proposal ${proposal.id}`);
    if (proposal.from_revision_id !== edge.from_revision_id || proposal.to_revision_id !== edge.to_revision_id) stale += 1;
    if (!proposal.rationale?.trim() || !proposal.decided_by?.trim() || !proposal.decided_at) missingEvidence += 1;
    if (testMarker.test(`${proposal.source ?? ''} ${proposal.decided_by ?? ''} ${proposal.rationale ?? ''}`)) testProvenance += 1;
    if (proposal.confidence === 1 && (!proposal.source?.trim() || !proposal.rationale?.trim() || !proposal.decided_by?.trim())) missingEvidence += 1;
  }
  const from = catalogByKey.get(edge.from_stable_key);
  const to = catalogByKey.get(edge.to_stable_key);
  if (!from || !to) {
    orphaned += 1;
  } else {
    if (from.status === 'archived' || to.status === 'archived') archived += 1;
    if (from.status !== 'published' || to.status !== 'published') invalidStatus += 1;
    if (from.revision_id !== edge.from_revision_id || to.revision_id !== edge.to_revision_id) stale += 1;
  }
}

if (orphaned) violations.push(`${orphaned} released edge endpoint is absent from published catalog`);
if (archived) violations.push(`${archived} released edge endpoint is archived`);
if (invalidStatus) violations.push(`${invalidStatus} released edge endpoint is not published`);
if (stale) violations.push(`${stale} released edge revision pin is stale`);
if (missingEvidence) violations.push(`${missingEvidence} released edge is missing reviewer evidence`);
if (testProvenance) violations.push(`${testProvenance} released edge contains test/fixture provenance`);
const cycleCount = cycles(release.edges ?? []);
if (cycleCount) violations.push(`${cycleCount} prerequisite cycle(s) detected in released graph`);
if ((proposalResponse.proposals ?? []).length < (release.edges ?? []).length) warnings.push('accepted proposal projection is smaller than release; inspect proposal query coverage');

const stable = {
  contractVersion: current.contract_version,
  workspace: WORKSPACE,
  releaseId,
  status: release.status,
  edgeCount: release.edge_count,
  edges: (release.edges ?? []).map((edge) => ({
    proposalId: edge.proposal_id,
    from: edge.from_stable_key,
    to: edge.to_stable_key,
    fromRevisionId: edge.from_revision_id,
    toRevisionId: edge.to_revision_id,
    kind: edge.kind,
  })),
  counts: { orphaned, stale, archived, invalidStatus, missingEvidence, testProvenance, cycleCount },
  violations,
};
const report = {
  reportVersion: 'question-graph-release-audit.v1',
  generatedAt: new Date().toISOString(),
  apiUrl: API_URL,
  workspace: WORKSPACE,
  releaseId,
  valid: violations.length === 0,
  summary: {
    edgeCount: release.edge_count ?? 0,
    acceptedProposalCount: proposalResponse.proposals?.length ?? 0,
    catalogCount: catalog.total ?? catalog.questions?.length ?? 0,
    orphaned,
    stale,
    archived,
    invalidStatus,
    missingEvidence,
    testProvenance,
    cycleCount,
  },
  violations,
  warnings,
  contentDigest: digest(stable),
};
const markdown = [
  `# W07 graph release audit`, '',
  `Release: \`${releaseId}\``, `Status: **${report.valid ? 'PASS' : 'FAIL'}**`, '',
  'Read-only audit of the active Question Brain graph release. Every edge is joined to accepted proposal evidence and the published catalog; no learner payloads are returned.', '',
  `- Edges: **${report.summary.edgeCount}**; accepted proposals: **${report.summary.acceptedProposalCount}**; catalog cards: **${report.summary.catalogCount}**.`,
  `- Test provenance: **${testProvenance}**; stale: **${stale}**; archived: **${archived}**; orphaned: **${orphaned}**; cycles: **${cycleCount}**.`,
  `- Violations: **${violations.length}**; warnings: **${warnings.length}**.`, '',
  '## Reproduction', '', '```bash',
  'cd /Users/sergeyzhechko/developer/fluent-interview',
  `QUESTION_BRAIN_API_URL=${API_URL} GRAPH_AUDIT_JSON=${output} GRAPH_AUDIT_MD=${markdownOutput} node fluent-question-brain/scripts/graph-release-audit.mjs`,
  `QUESTION_BRAIN_API_URL=${API_URL} GRAPH_AUDIT_JSON=${output} node fluent-question-brain/scripts/graph-release-audit.mjs --check`,
  '```', '', `Stable content digest: \`${report.contentDigest}\``, '',
].join('\n');

if (checkOnly) {
  try {
    const stored = JSON.parse(await fs.readFile(output, 'utf8'));
    const mismatches = [];
    if (stored.reportVersion !== report.reportVersion) mismatches.push('reportVersion mismatch');
    if (stored.contentDigest !== report.contentDigest) mismatches.push(`contentDigest: expected ${report.contentDigest}, found ${stored.contentDigest}`);
    mismatches.push(...violations);
    if (mismatches.length) {
      console.error(mismatches.join('\n'));
      process.exitCode = 1;
    } else {
      console.log(JSON.stringify({ valid: true, releaseId, summary: report.summary, contentDigest: report.contentDigest }, null, 2));
    }
  } catch (error) {
    console.error(`stored graph audit: ${error.message}`);
    process.exitCode = 1;
  }
} else {
  await fs.mkdir(path.dirname(output), { recursive: true });
  await fs.writeFile(output, `${JSON.stringify(report, null, 2)}\n`);
  await fs.writeFile(markdownOutput, markdown);
  console.log(JSON.stringify({ valid: report.valid, output, markdown: markdownOutput, releaseId, summary: report.summary, violations, warnings, contentDigest: report.contentDigest }, null, 2));
  if (!report.valid) process.exitCode = 1;
}
