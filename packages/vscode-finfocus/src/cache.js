'use strict';

const crypto = require('node:crypto');
const { parseOverview, ResultError } = require('./result');

const ENTRY_VERSION = 1;

/**
 * @typedef {{
 *   lastUpdate?: string, resourceCount?: number, updateInProgress?: boolean,
 *   stackConfigHash: string, projectConfigHash: string, finfocusVersion: string,
 * }} CacheSignals
 * @typedef {{
 *   version: number, contextKey: string, lastUpdate: string, resourceCount: number | null,
 *   stackConfigHash: string, projectConfigHash: string, finfocusVersion: string,
 *   stateOnly: true, storedAt: string, producedAt: string, confirmedGaps: string[], stdout: string,
 * }} CacheEntry
 * @typedef {{
 *   read: (key: string) => Promise<CacheEntry | null>,
 *   write: (key: string, entry: CacheEntry) => Promise<void>,
 *   remove: (key: string) => Promise<void>,
 * }} CacheStorePort
 * @typedef {{
 *   result: import('./result').OverviewResult, producedAt: Date, confirmedGaps: string[],
 * }} CacheHit
 */

const KEY_FIELDS = /** @type {const} */ (['lastUpdate', 'resourceCount', 'stackConfigHash', 'projectConfigHash', 'finfocusVersion']);

/**
 * SHA-256 of a file's text, or '' for an absent file.
 *
 * @param {string | null | undefined} text
 * @returns {string}
 */
function hashContents(text) {
  if (text === null || text === undefined) return '';
  return crypto.createHash('sha256').update(text).digest('hex');
}

/**
 * Reuses the last deployed-state result while nothing it depends on changed
 * (FR-012a). The cache holds finfocus's output verbatim; it never receives a
 * passphrase or an environment, and a failing store only makes it a miss.
 */
class ResultCache {
  /**
   * @param {{
   *   store: CacheStorePort, ttlMs: number | (() => number), now?: () => Date,
   *   isTrusted?: () => boolean, parse?: typeof parseOverview,
   * }} options
   */
  constructor(options) {
    this.store = options.store;
    this.limit = options.ttlMs;
    this.now = options.now ?? (() => new Date());
    this.isTrusted = options.isTrusted ?? (() => true);
    this.parse = options.parse ?? parseOverview;
  }

  /** @returns {number} the time limit in milliseconds, read each time so a setting change applies */
  get ttlMs() {
    return typeof this.limit === 'function' ? this.limit() : this.limit;
  }

  /** @returns {boolean} */
  get enabled() {
    return this.ttlMs > 0;
  }

  /**
   * A stack with no update, or one being updated, has nothing stable to key on.
   *
   * @param {Partial<CacheSignals> | null | undefined} signals
   * @returns {signals is CacheSignals & {lastUpdate: string}}
   */
  static eligible(signals) {
    if (signals === null || signals === undefined) return false;
    return typeof signals.lastUpdate === 'string' && signals.lastUpdate !== '' && signals.updateInProgress !== true;
  }

  /**
   * @param {string} contextKey
   * @param {CacheSignals} signals
   * @returns {Promise<CacheHit | null>}
   */
  async lookup(contextKey, signals) {
    if (!this.enabled || !this.isTrusted() || !ResultCache.eligible(signals)) return null;
    const entry = await this.load(contextKey);
    if (entry === null) return null;
    if (!this.matches(entry, signals)) return null;
    if (this.now().getTime() - Date.parse(entry.storedAt) > this.ttlMs) return null;
    try {
      return {
        result: this.parse(entry.stdout),
        producedAt: new Date(entry.producedAt),
        confirmedGaps: entry.confirmedGaps,
      };
    } catch (error) {
      if (!(error instanceof ResultError)) throw error;
      await this.discard(contextKey);
      return null;
    }
  }

  /**
   * The last stored entry, valid or not; its gaps are the baseline.
   *
   * @param {string} contextKey
   * @returns {Promise<{confirmedGaps: string[]} | null>}
   */
  async previous(contextKey) {
    if (!this.isTrusted()) return null;
    const entry = await this.load(contextKey);
    return entry === null ? null : { confirmedGaps: entry.confirmedGaps };
  }

  /**
   * @param {string} contextKey
   * @param {Partial<CacheSignals>} signals
   * @param {{stdout: string, producedAt: Date, confirmedGaps: string[]}} fields
   * @returns {Promise<void>}
   */
  async save(contextKey, signals, fields) {
    if (!this.enabled || !this.isTrusted() || !ResultCache.eligible(signals)) return;
    /** @type {CacheEntry} */
    const entry = {
      version: ENTRY_VERSION,
      contextKey,
      lastUpdate: signals.lastUpdate,
      resourceCount: typeof signals.resourceCount === 'number' ? signals.resourceCount : null,
      stackConfigHash: signals.stackConfigHash,
      projectConfigHash: signals.projectConfigHash,
      finfocusVersion: signals.finfocusVersion,
      stateOnly: true,
      storedAt: this.now().toISOString(),
      producedAt: fields.producedAt.toISOString(),
      confirmedGaps: [...fields.confirmedGaps],
      stdout: fields.stdout,
    };
    try {
      await this.store.write(contextKey, entry);
    } catch {
      // A cache that cannot be written is only a slower load.
    }
  }

  /**
   * @param {string} contextKey
   * @returns {Promise<CacheEntry | null>}
   */
  async load(contextKey) {
    let entry;
    try {
      entry = await this.store.read(contextKey);
    } catch {
      return null;
    }
    if (entry === null) return null;
    if (!isShapedEntry(entry)) {
      await this.discard(contextKey);
      return null;
    }
    return entry;
  }

  /** @param {string} contextKey */
  async discard(contextKey) {
    try {
      await this.store.remove(contextKey);
    } catch {
      // Nothing to do: the next write replaces it.
    }
  }

  /**
   * @param {CacheEntry} entry
   * @param {CacheSignals} signals
   * @returns {boolean}
   */
  matches(entry, signals) {
    const wanted = { ...signals, resourceCount: typeof signals.resourceCount === 'number' ? signals.resourceCount : null };
    return KEY_FIELDS.every((field) => entry[field] === wanted[field]);
  }
}

/**
 * @param {any} entry
 * @returns {entry is CacheEntry}
 */
function isShapedEntry(entry) {
  return (
    entry !== null &&
    typeof entry === 'object' &&
    entry.version === ENTRY_VERSION &&
    typeof entry.stdout === 'string' &&
    typeof entry.lastUpdate === 'string' &&
    Number.isFinite(Date.parse(entry.storedAt)) &&
    Number.isFinite(Date.parse(entry.producedAt)) &&
    Array.isArray(entry.confirmedGaps) &&
    entry.confirmedGaps.every((/** @type {unknown} */ g) => typeof g === 'string')
  );
}

module.exports = { ResultCache, hashContents, ENTRY_VERSION };
