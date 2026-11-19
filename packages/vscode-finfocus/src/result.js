'use strict';

const defaultInventory = require('../contract/overview-fields.json');

/**
 * @typedef {{file: string, line: number, column?: number}} Location
 * @typedef {{projected: string, actual: string, delta: string, status: string}} RowDisplay
 * @typedef {{
 *   urn: string, type: string, status: string,
 *   monthlyCost: number | null, // from projectedCost.monthlyCost; null when finfocus gave none
 *   location: Location | null, modifiedAt: string | null, display: RowDisplay
 * }} OverviewRow
 * @typedef {{
 *   stack: string, generatedAt: string, stateOnly: boolean, currency: string,
 *   rows: OverviewRow[], summaryDisplay: {projected: string, actual: string, delta: string},
 *   budgetDisplay: {healthDisplay: string, amountsDisplay: string, footerDisplay: string, details: unknown[]},
 *   errors: {urn: string, message: string}[]
 * }} OverviewResult
 * @typedef {{message: string, category: string, version: string, errorCode: string}} RunFailure
 */

class ResultError extends Error {
  /**
   * @param {'malformed_json' | 'schema'} code
   * @param {string} message
   */
  constructor(code, message) {
    super(message);
    this.name = 'ResultError';
    this.code = code;
  }
}

/**
 * @param {unknown} value
 * @returns {string}
 */
function kindOf(value) {
  if (Array.isArray(value)) return 'array';
  if (value === null) return 'null';
  return typeof value;
}

/**
 * Walks a path such as "resources[].display.projected" and returns every leaf.
 * A segment ending in "[]" fans out over an array. With `skipMissing`, a node
 * that lacks the key is skipped instead of ending the walk (optional paths).
 *
 * @param {unknown} doc
 * @param {string} path
 * @param {boolean} skipMissing
 * @returns {unknown[] | null} null when a required node is missing
 */
function lookup(doc, path, skipMissing) {
  let current = [doc];
  for (const segment of path.split('.')) {
    const fan = segment.endsWith('[]');
    const key = fan ? segment.slice(0, -2) : segment;
    const next = [];
    for (const node of current) {
      if (node === null || typeof node !== 'object' || Array.isArray(node)) {
        if (skipMissing) continue;
        return null;
      }
      const record = /** @type {Record<string, unknown>} */ (node);
      if (!Object.hasOwn(record, key)) {
        if (skipMissing) continue;
        return null;
      }
      const child = record[key];
      if (fan) {
        if (!Array.isArray(child)) {
          if (skipMissing) continue;
          return null;
        }
        next.push(...child);
      } else {
        next.push(child);
      }
    }
    current = next;
  }
  return current;
}

/**
 * Checks a parsed document against the contract inventory.
 *
 * @param {unknown} doc
 * @param {{required: Record<string, string>, optional: Record<string, string>}} inventory
 */
function validateInventory(doc, inventory) {
  for (const [path, kind] of Object.entries(inventory.required)) {
    const leaves = lookup(doc, path, false);
    if (leaves === null) {
      throw new ResultError('schema', `finfocus output is missing "${path}"`);
    }
    for (const leaf of leaves) {
      if (kindOf(leaf) !== kind) {
        throw new ResultError('schema', `finfocus output has "${path}" as ${kindOf(leaf)}, expected ${kind}`);
      }
    }
  }
  for (const [path, kind] of Object.entries(inventory.optional)) {
    for (const leaf of lookup(doc, path, true) ?? []) {
      if (kindOf(leaf) !== kind) {
        throw new ResultError('schema', `finfocus output has "${path}" as ${kindOf(leaf)}, expected ${kind}`);
      }
    }
  }
}

/**
 * @param {unknown} raw
 * @returns {Location | null}
 */
function toLocation(raw) {
  if (raw === null || typeof raw !== 'object') return null;
  const { file, line, column } = /** @type {Record<string, unknown>} */ (raw);
  if (typeof file !== 'string' || file === '') return null;
  if (typeof line !== 'number' || !Number.isInteger(line) || line < 1) return null;
  /** @type {Location} */
  const location = { file, line };
  if (typeof column === 'number' && Number.isInteger(column) && column > 0) {
    location.column = column;
  }
  return location;
}

/**
 * @param {Record<string, any>} raw
 * @returns {OverviewRow}
 */
function toRow(raw) {
  return {
    urn: raw.urn,
    type: raw.type,
    status: raw.status,
    monthlyCost: typeof raw.projectedCost?.monthlyCost === 'number' ? raw.projectedCost.monthlyCost : null,
    location: toLocation(raw.location),
    modifiedAt: typeof raw.modifiedAt === 'string' ? raw.modifiedAt : null,
    display: raw.display,
  };
}

/**
 * Parses the stdout of `finfocus overview --output json`.
 *
 * @param {string} text
 * @param {{required: Record<string, string>, optional: Record<string, string>}} [inventory]
 * @returns {OverviewResult}
 */
function parseOverview(text, inventory = defaultInventory) {
  /** @type {any} */
  let doc;
  try {
    doc = JSON.parse(text);
  } catch {
    throw new ResultError('malformed_json', 'finfocus output is not valid JSON');
  }
  validateInventory(doc, inventory);
  return {
    stack: doc.metadata.stackName,
    generatedAt: doc.metadata.generatedAt,
    stateOnly: doc.metadata.isStateOnly === true,
    currency: doc.summary.currency,
    rows: doc.resources.map(toRow),
    summaryDisplay: doc.summary.display,
    budgetDisplay: doc.budgetDisplay,
    errors: doc.errors.map((/** @type {{urn: string, message: string}} */ e) => ({
      urn: e.urn,
      message: e.message,
    })),
  };
}

/**
 * Parses the ax-go failure envelope finfocus writes to stderr. Returns null
 * when stderr holds no envelope, so the caller reports the exit code instead.
 *
 * @param {string} text
 * @returns {RunFailure | null}
 */
function parseFailure(text) {
  /** @type {any} */
  let doc;
  try {
    doc = JSON.parse(text);
  } catch {
    return null;
  }
  if (doc === null || typeof doc !== 'object' || typeof doc.message !== 'string') return null;
  const match = /\(([a-z_]+)\)$/.exec(doc.message.trimEnd());
  return {
    message: doc.message,
    category: match ? match[1] : '',
    version: typeof doc.version === 'string' ? doc.version : '',
    errorCode: typeof doc.error_code === 'string' ? doc.error_code : '',
  };
}

module.exports = { ResultError, parseOverview, parseFailure, validateInventory };
