'use strict';

const defaultInventory = require('../contract/overview-fields.json');

/**
 * @typedef {'complete' | 'confirmed' | 'incomplete'} CompletenessKind
 * @typedef {{kind: CompletenessKind, gaps: string[], total: number}} Completeness
 * @typedef {{kind: CompletenessKind, gaps: string[], keep: 'first' | 'second'}} Settled
 * @typedef {{unpriced: {path: string, value: string, exemptStatuses: string[]}}} Placeholders
 */

/**
 * URNs of rows with no price: the contract's placeholder in
 * `display.projected`, on a row whose raw status is not exempt (a deleting row
 * is expected to have none). The JSON does not say whether such a gap is a type
 * with no cost data or a failed price, so callers judge it by stability.
 *
 * @param {import('./result').OverviewResult} result
 * @param {{placeholders: Placeholders}} [inventory]
 * @returns {string[]}
 */
function gapsOf(result, inventory = defaultInventory) {
  const { value, exemptStatuses } = inventory.placeholders.unpriced;
  return result.rows
    .filter((row) => !exemptStatuses.includes(row.status) && row.display.projected === value)
    .map((row) => row.urn)
    .sort();
}

/**
 * @param {string[]} gaps
 * @param {string[] | null} baseline gaps an earlier run confirmed, or null
 * @returns {boolean}
 */
function needsRetry(gaps, baseline) {
  if (gaps.length === 0) return false;
  if (baseline === null) return true;
  return gaps.some((urn) => !baseline.includes(urn));
}

/**
 * @param {string[]} a
 * @param {string[]} b
 * @returns {boolean}
 */
function sameSet(a, b) {
  return a.length === b.length && a.every((urn) => b.includes(urn));
}

/**
 * Decides what a state-only load's gaps mean. `second` is null when no retry
 * was needed. Equal sets across two runs, or gaps inside the baseline, are
 * confirmed; differing sets mean a slow or failing plugin, and the run with
 * fewer gaps is kept (the later one on a tie).
 *
 * @param {{first: string[], second: string[] | null, baseline: string[] | null}} runs
 * @returns {Settled}
 */
function settle({ first, second, baseline }) {
  if (second === null) {
    if (first.length === 0) return { kind: 'complete', gaps: [], keep: 'first' };
    return { kind: 'confirmed', gaps: first, keep: 'first' };
  }
  const keep = second.length <= first.length ? 'second' : 'first';
  const kept = keep === 'second' ? second : first;
  if (kept.length === 0) return { kind: 'complete', gaps: [], keep };
  const insideBaseline = baseline !== null && kept.every((urn) => baseline.includes(urn));
  if (sameSet(first, second) || insideBaseline) return { kind: 'confirmed', gaps: kept, keep };
  return { kind: 'incomplete', gaps: kept, keep };
}

module.exports = { gapsOf, needsRetry, settle };
