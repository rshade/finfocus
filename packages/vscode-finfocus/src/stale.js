'use strict';

/**
 * Staleness of annotated figures. Everything here is plain data so it can be
 * tested without an editor. Marking never starts a run; only a covering
 * preview that nothing edited during clears a mark.
 *
 * @typedef {import('./result').OverviewRow} OverviewRow
 * @typedef {'state' | 'preview'} Basis
 * @typedef {{generations: Map<string, number>, dirty: Set<string>, configGeneration: number}} RequestSnapshot
 */

const REASONS = Object.freeze({
  edited: 'edited',
  unsaved: 'unsaved',
  config: 'config changed',
  unknown: 'freshness unknown',
});

/**
 * Per-context freshness state. Files are project-relative paths, the same
 * strings a row's location carries.
 */
class FreshnessModel {
  constructor() {
    /** @type {Map<string, number>} */
    this.generations = new Map();
    /** @type {Set<string>} */
    this.dirty = new Set();
    /** @type {Set<string>} */
    this.marked = new Set();
    this.configGeneration = 0;
    this.configMarked = false;
  }

  /**
   * An editor change to a document. Every edit bumps the file's generation.
   *
   * @param {string} file
   * @param {boolean} dirty whether the document now differs from disk
   */
  recordEdit(file, dirty) {
    this.bump(file);
    this.marked.add(file);
    if (dirty) this.dirty.add(file);
    else this.dirty.delete(file);
  }

  /**
   * A save writes the buffer to disk. It is not a new edit, so the generation
   * and the edit mark stay.
   *
   * @param {string} file
   */
  recordSave(file) {
    this.dirty.delete(file);
  }

  /**
   * A change on disk that did not come through the editor, including a
   * rename or deletion.
   *
   * @param {string} file
   */
  recordExternalChange(file) {
    this.bump(file);
    this.marked.add(file);
  }

  recordConfigChange() {
    this.configGeneration += 1;
    this.configMarked = true;
  }

  /**
   * @param {string} file
   */
  bump(file) {
    this.generations.set(file, (this.generations.get(file) ?? 0) + 1);
  }

  /**
   * What a request captures when it starts.
   *
   * @returns {RequestSnapshot}
   */
  snapshot() {
    return {
      generations: new Map(this.generations),
      dirty: new Set(this.dirty),
      configGeneration: this.configGeneration,
    };
  }

  /**
   * A state-only result prices nothing that was edited, so it clears no mark.
   * Present so the rule has one named place.
   */
  applyState() {}

  /**
   * A successful preview clears a file's mark when its rows are in the result
   * and the file was clean at request start, is clean now, and was not edited
   * since. The config mark clears when no settings change happened since.
   *
   * @param {{location: {file: string} | null}[]} rows
   * @param {RequestSnapshot} snapshot
   */
  applyPreview(rows, snapshot) {
    const covered = new Set();
    for (const row of rows) {
      if (row.location !== null) covered.add(row.location.file);
    }
    for (const file of covered) {
      const unchanged = (this.generations.get(file) ?? 0) === (snapshot.generations.get(file) ?? 0);
      if (unchanged && !snapshot.dirty.has(file) && !this.dirty.has(file)) this.marked.delete(file);
    }
    if (snapshot.configGeneration === this.configGeneration) this.configMarked = false;
  }

  /**
   * The reasons each row's figures may be out of date, keyed by URN. A row
   * absent from the result is fresh.
   *
   * @param {OverviewRow[] | {urn: string, location: {file: string} | null, modifiedAt: string | null}[]} rows
   * @param {Basis} basis `state` compares file times with `modifiedAt`; a
   *   preview was priced from current files, so it does not
   * @param {Map<string, number | null>} mtimes file modification times in ms; null when unreadable
   * @returns {Map<string, string[]>}
   */
  staleSet(rows, basis, mtimes) {
    /** @type {Map<string, string[]>} */
    const result = new Map();
    for (const row of rows) {
      const reasons = this.reasonsFor(row, basis, mtimes);
      if (reasons.length > 0) result.set(row.urn, reasons);
    }
    return result;
  }

  /**
   * @param {{location: {file: string} | null, modifiedAt: string | null}} row
   * @param {Basis} basis
   * @param {Map<string, number | null>} mtimes
   * @returns {string[]}
   */
  reasonsFor(row, basis, mtimes) {
    /** @type {string[]} */
    const reasons = [];
    const file = row.location?.file ?? null;
    if (file !== null) {
      if (this.dirty.has(file)) reasons.push(REASONS.unsaved);
      else if (this.marked.has(file)) reasons.push(REASONS.edited);
    }
    if (this.configMarked) reasons.push(REASONS.config);
    if (reasons.length > 0) return reasons;
    if (file === null) return [REASONS.unknown];
    if (basis === 'preview') return [];

    const modified = row.modifiedAt === null ? Number.NaN : Date.parse(row.modifiedAt);
    const mtime = mtimes.get(file) ?? null;
    if (Number.isNaN(modified) || mtime === null) return [REASONS.unknown];
    return mtime > modified ? [REASONS.edited] : [];
  }
}

/** One FreshnessModel per context key; never shared across contexts. */
class StaleStore {
  constructor() {
    /** @type {Map<string, FreshnessModel>} */
    this.models = new Map();
  }

  /**
   * @param {string} key
   * @returns {FreshnessModel}
   */
  model(key) {
    let model = this.models.get(key);
    if (model === undefined) {
      model = new FreshnessModel();
      this.models.set(key, model);
    }
    return model;
  }

  /** @param {string} key */
  drop(key) {
    this.models.delete(key);
  }

  /**
   * Context keys are `JSON.stringify([projectUri, stackRef])`.
   *
   * @param {string} project
   * @returns {string[]}
   */
  keysForProject(project) {
    return [...this.models.keys()].filter((key) => {
      try {
        return JSON.parse(key)[0] === project;
      } catch {
        return false;
      }
    });
  }
}

module.exports = { FreshnessModel, StaleStore, REASONS };
