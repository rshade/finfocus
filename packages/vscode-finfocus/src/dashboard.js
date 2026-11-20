'use strict';

const { declarationTarget, resourceName, UNPRICED_MARKERS } = require('./annotations');
const { REASONS } = require('./stale');

const TOP_COUNT = 5;
const UNPRICED_LABEL = 'Unpriced';

/**
 * Pure projection from an overview result to what the card, tree and status
 * bar show. It formats no money and sums nothing: every figure is a display
 * string finfocus produced, and the top five are chosen by comparing the
 * numeric monthly cost it reported.
 *
 * @typedef {import('./result').OverviewResult} OverviewResult
 * @typedef {import('./result').OverviewRow} OverviewRow
 * @typedef {{
 *   urn: string, name: string, type: string, status: string, projected: string,
 *   openable: boolean, reasons: string[], error: string | null, detail: string[]
 * }} ResourceItem
 * @typedef {{label: string, unpriced: boolean, items: ResourceItem[]}} ResourceGroup
 * @typedef {{
 *   stack: string, stateOnly: boolean,
 *   summary: {projected: string, actual: string, delta: string},
 *   budget: {configured: boolean, health: string, amounts: string, footer: string, text: string},
 *   freshness: {stale: number, unknown: number, text: string},
 *   completeness: {kind: import('./completeness').CompletenessKind, text: string, help: string},
 *   topFive: ResourceItem[], groups: ResourceGroup[], total: number,
 *   errors: {urn: string, message: string}[]
 * }} Dashboard
 */

/**
 * @param {string} a
 * @param {string} b
 * @returns {number}
 */
function compareText(a, b) {
  return a < b ? -1 : a > b ? 1 : 0;
}

/**
 * @param {OverviewRow} row
 * @returns {boolean}
 */
function isUnpriced(row) {
  return UNPRICED_MARKERS.has(row.display.projected);
}

/**
 * @param {OverviewRow} row
 * @returns {string}
 */
function figures(row) {
  const { projected, actual, delta, status } = row.display;
  const parts = [`Projected ${projected}`];
  if (actual !== '-') parts.push(`Actual ${actual}`);
  if (delta !== '-') parts.push(`Delta ${delta}`);
  parts.push(status);
  return parts.join(' · ');
}

/**
 * Every line the hover shows, plus the declaration, stale reasons and error,
 * so the keyboard command offers what a pointer user sees.
 *
 * @param {OverviewRow} row
 * @param {string[]} reasons
 * @param {string | null} error
 * @returns {string[]}
 */
function detailLines(row, reasons, error) {
  const target = declarationTarget(row);
  const lines = [resourceName(row.urn), row.type, figures(row)];
  lines.push(target === null ? 'No recorded location' : `Declared at ${target.file}:${target.line}`);
  if (reasons.length > 0) lines.push(`Stale: ${reasons.join(', ')}`);
  if (error !== null) lines.push(`Error: ${error}`);
  return lines;
}

/**
 * @param {OverviewRow} row
 * @param {Map<string, string[]>} stale
 * @param {Map<string, string>} errors
 * @returns {ResourceItem}
 */
function toItem(row, stale, errors) {
  const reasons = stale.get(row.urn) ?? [];
  const error = errors.get(row.urn) ?? null;
  return {
    urn: row.urn,
    name: resourceName(row.urn),
    type: row.type,
    status: row.display.status,
    projected: row.display.projected,
    openable: declarationTarget(row) !== null,
    reasons,
    error,
    detail: detailLines(row, reasons, error),
  };
}

/**
 * @param {OverviewResult['budgetDisplay']} display
 * @returns {Dashboard['budget']}
 */
function toBudget(display) {
  const { healthDisplay: health, amountsDisplay: amounts, footerDisplay: footer } = display;
  const configured = health !== '' || amounts !== '' || footer !== '';
  if (!configured) return { configured, health, amounts, footer, text: 'No budget configured' };
  const detail = [amounts, footer].filter((part) => part !== '').join(' · ');
  return { configured, health, amounts, footer, text: `Budget ${health}${detail === '' ? '' : `: ${detail}`}` };
}

/**
 * @param {number} count
 * @returns {string}
 */
function resources(count) {
  return `${count} ${count === 1 ? 'resource' : 'resources'}`;
}

const INCOMPLETE_HELP =
  'finfocus returned no price for these resources, and a second run disagreed about which. The total may be too low. Run Refresh costs to try again.';

/**
 * What the card, status bar and tree say about unpriced rows. Only an
 * unconfirmed result is called incomplete (FR-012b); a confirmed one is a note.
 *
 * @param {import('./completeness').Completeness | null | undefined} completeness
 * @returns {Dashboard['completeness']}
 */
function toCompleteness(completeness) {
  if (completeness === null || completeness === undefined || completeness.gaps.length === 0) {
    return { kind: 'complete', text: '', help: '' };
  }
  const gaps = completeness.gaps.length;
  if (completeness.kind === 'incomplete') {
    return {
      kind: 'incomplete',
      text: `Incomplete: ${gaps} of ${resources(completeness.total)} unpriced`,
      help: INCOMPLETE_HELP,
    };
  }
  return { kind: 'confirmed', text: `${resources(gaps)} unpriced`, help: '' };
}

/**
 * @param {Map<string, string[]>} stale
 * @returns {Dashboard['freshness']}
 */
function toFreshness(stale) {
  let unknown = 0;
  let count = 0;
  for (const reasons of stale.values()) {
    if (reasons.every((r) => r === REASONS.unknown)) unknown += 1;
    else count += 1;
  }
  const parts = [];
  if (count > 0) parts.push(`${count} stale`);
  if (unknown > 0) parts.push(`${unknown} freshness unknown`);
  return { stale: count, unknown, text: parts.length === 0 ? 'No stale resources' : parts.join(' · ') };
}

/**
 * @param {{
 *   result: OverviewResult, stale: Map<string, string[]>,
 *   completeness?: import('./completeness').Completeness | null,
 * }} input
 * @returns {Dashboard}
 */
function buildDashboard({ result, stale, completeness = null }) {
  const errors = new Map(result.errors.map((e) => [e.urn, e.message]));
  const ordered = [...result.rows].sort((a, b) => compareText(a.urn, b.urn));
  const items = new Map(ordered.map((row) => [row.urn, toItem(row, stale, errors)]));

  const topFive = ordered
    .filter((row) => !isUnpriced(row) && row.monthlyCost !== null)
    .sort((a, b) => /** @type {number} */ (b.monthlyCost) - /** @type {number} */ (a.monthlyCost) || compareText(a.urn, b.urn))
    .slice(0, TOP_COUNT)
    .map((row) => /** @type {ResourceItem} */ (items.get(row.urn)));

  /** @type {Map<string, ResourceItem[]>} */
  const byType = new Map();
  /** @type {ResourceItem[]} */
  const unpriced = [];
  for (const row of ordered) {
    const item = /** @type {ResourceItem} */ (items.get(row.urn));
    if (isUnpriced(row) || errors.has(row.urn)) {
      unpriced.push(item);
    } else {
      byType.set(row.type, [...(byType.get(row.type) ?? []), item]);
    }
  }
  /** @type {ResourceGroup[]} */
  const groups = [...byType.keys()]
    .sort(compareText)
    .map((type) => ({ label: type, unpriced: false, items: sortByName(byType.get(type) ?? []) }));
  if (unpriced.length > 0) groups.push({ label: UNPRICED_LABEL, unpriced: true, items: sortByName(unpriced) });

  return {
    stack: result.stack,
    stateOnly: result.stateOnly,
    summary: result.summaryDisplay,
    budget: toBudget(result.budgetDisplay),
    freshness: toFreshness(stale),
    completeness: toCompleteness(completeness),
    topFive,
    groups,
    total: result.rows.length,
    errors: result.errors,
  };
}

/**
 * @param {ResourceItem[]} items
 * @returns {ResourceItem[]}
 */
function sortByName(items) {
  return [...items].sort((a, b) => compareText(a.name, b.name) || compareText(a.urn, b.urn));
}

module.exports = { buildDashboard };
