'use strict';

const { isStackSettingsFile } = require('../projects');

const SAVE_ECHO_MS = 2000;

/**
 * @typedef {import('../result').OverviewRow} OverviewRow
 * @typedef {import('../result').OverviewResult} OverviewResult
 * @typedef {import('../stale').Basis} Basis
 */

/**
 * The project-relative, forward-slash path of a document inside a project, or
 * null when it is elsewhere (another scheme or authority, or outside).
 *
 * @param {{scheme: string, authority: string, path: string}} project
 * @param {{scheme: string, authority: string, path: string}} uri
 * @returns {string | null}
 */
function relativeFile(project, uri) {
  if (uri.scheme !== project.scheme || uri.authority !== project.authority) return null;
  const root = project.path.endsWith('/') ? project.path : `${project.path}/`;
  if (!uri.path.startsWith(root)) return null;
  const file = uri.path.slice(root.length);
  return file === '' || file.split('/').includes('..') ? null : file;
}

/**
 * Marks annotated figures stale from document edits, saves, file-system
 * changes and stack settings changes. It never starts a run: it only records
 * what changed and repaints. Each context keeps its own model.
 */
class StaleTracking {
  /**
   * @param {{
   *   api: typeof import('vscode'),
   *   store: InstanceType<typeof import('../stale').StaleStore>,
   *   annotations: InstanceType<typeof import('./annotations').AnnotationProvider>,
   *   now?: () => number,
   * }} deps
   */
  constructor(deps) {
    this.api = deps.api;
    this.store = deps.store;
    this.annotations = deps.annotations;
    this.now = deps.now ?? (() => Date.now());
    /** @type {Map<string, {project: import('vscode').Uri, stack: string, result: OverviewResult, rows: OverviewRow[], basis: Basis, mtimes: Map<string, number | null>, completeness: import('../completeness').Completeness | null}>} */
    this.contexts = new Map();
    /** @type {string | null} */
    this.activeKey = null;
    /** @type {Map<string, number>} */
    this.recentSaves = new Map();
    /** @type {import('vscode').FileSystemWatcher | null} */
    this.watcher = null;
    /** @type {string | null} */
    this.watchedProject = null;
    this.changed = new this.api.EventEmitter();
    /** Fires with the context key whenever a stale set was recomputed. */
    this.onDidChange = this.changed.event;
    this.disposables = [
      this.changed,
      this.api.workspace.onDidChangeTextDocument((e) => this.onEdit(e.document)),
      this.api.workspace.onDidSaveTextDocument((doc) => this.onSave(doc)),
    ];
  }

  /**
   * Records a result for a context and recomputes what is stale. A document
   * that was already dirty counts as edited, since the result cannot have
   * priced it.
   *
   * `completeness` is the scheduler's judgment of a deployed-state load; a
   * preview passes none.
   *
   * @param {{
   *   key: string, project: import('vscode').Uri, stack: string, result: OverviewResult, basis: Basis,
   *   completeness?: import('../completeness').Completeness | null,
   * }} input
   * @returns {Promise<void>}
   */
  async setResult({ key, project, stack, result, basis, completeness = null }) {
    const rows = result.rows;
    const model = this.store.model(key);
    for (const doc of this.api.workspace.textDocuments) {
      const file = relativeFile(project, doc.uri);
      if (file !== null && doc.isDirty && !model.dirty.has(file)) model.recordEdit(file, true);
    }
    if (basis === 'state') model.applyState();
    const mtimes = await this.statFiles(project, rows);
    this.contexts.set(key, { project, stack, result, rows, basis, mtimes, completeness });
    this.setActive(key);
    this.refresh(key);
  }

  /**
   * What a request captures when it starts, after any requested saves.
   *
   * @param {string} key
   * @returns {import('../stale').RequestSnapshot}
   */
  snapshotFor(key) {
    return this.store.model(key).snapshot();
  }

  /**
   * A successful preview: clears the marks it covers, then shows the result.
   *
   * @param {{key: string, project: import('vscode').Uri, stack: string, result: OverviewResult, snapshot: import('../stale').RequestSnapshot}} input
   * @returns {Promise<void>}
   */
  async applyPreview({ key, project, stack, result, snapshot }) {
    this.store.model(key).applyPreview(result.rows, snapshot);
    await this.setResult({ key, project, stack, result, basis: 'preview' });
  }

  /**
   * The shown context's result and stale reasons, or null before any result.
   *
   * @returns {{key: string, project: import('vscode').Uri, stack: string, result: OverviewResult, basis: Basis, stale: Map<string, string[]>, completeness: import('../completeness').Completeness | null} | null}
   */
  view() {
    if (this.activeKey === null) return null;
    const context = this.contexts.get(this.activeKey);
    if (context === undefined) return null;
    const stale = this.store.model(this.activeKey).staleSet(context.rows, context.basis, context.mtimes);
    return {
      key: this.activeKey,
      project: context.project,
      stack: context.stack,
      result: context.result,
      basis: context.basis,
      stale,
      completeness: context.completeness,
    };
  }

  /**
   * @param {string} key
   */
  setActive(key) {
    this.activeKey = key;
    const context = this.contexts.get(key);
    if (context !== undefined) this.watch(context.project);
  }

  /**
   * Forgets a context, for a removed project or a stack that no longer exists.
   *
   * @param {string} key
   */
  drop(key) {
    this.contexts.delete(key);
    this.store.drop(key);
    if (this.activeKey === key) this.activeKey = null;
  }

  /**
   * @param {import('vscode').Uri} project
   * @param {OverviewRow[]} rows
   * @returns {Promise<Map<string, number | null>>}
   */
  async statFiles(project, rows) {
    const files = new Set(rows.flatMap((r) => (r.location === null ? [] : [r.location.file])));
    /** @type {Map<string, number | null>} */
    const mtimes = new Map();
    await Promise.all(
      [...files].map(async (file) => {
        try {
          const stat = await this.api.workspace.fs.stat(this.api.Uri.joinPath(project, ...file.split('/')));
          mtimes.set(file, stat.mtime);
        } catch {
          mtimes.set(file, null);
        }
      }),
    );
    return mtimes;
  }

  /**
   * @param {import('vscode').TextDocument} doc
   */
  onEdit(doc) {
    for (const key of this.keysFor(doc.uri)) {
      const target = this.locate(key, doc.uri);
      if (target === null) continue;
      const { context, file } = target;
      const model = this.store.model(key);
      if (isStackSettingsFile(file, context.stack) && !file.includes('/')) model.recordConfigChange();
      else model.recordEdit(file, doc.isDirty);
      this.refresh(key);
    }
  }

  /**
   * @param {import('vscode').TextDocument} doc
   */
  onSave(doc) {
    for (const key of this.keysFor(doc.uri)) {
      const target = this.locate(key, doc.uri);
      if (target === null) continue;
      const { context, file } = target;
      this.recentSaves.set(`${key}\u0000${file}`, this.now());
      this.store.model(key).recordSave(file);
      this.refresh(key);
    }
  }

  /**
   * A file-system event for a path. The watcher also reports the editor's own
   * saves, which were already counted as edits, so one such echo is ignored.
   *
   * @param {import('vscode').Uri} uri
   */
  onFileChanged(uri) {
    for (const key of this.keysFor(uri)) {
      const target = this.locate(key, uri);
      if (target === null) continue;
      const { context, file } = target;
      const saved = this.recentSaves.get(`${key}\u0000${file}`);
      this.recentSaves.delete(`${key}\u0000${file}`);
      if (saved !== undefined && this.now() - saved < SAVE_ECHO_MS) continue;
      const model = this.store.model(key);
      if (isStackSettingsFile(file, context.stack) && !file.includes('/')) model.recordConfigChange();
      else model.recordExternalChange(file);
      this.refresh(key);
    }
  }

  /**
   * @param {string} key
   * @param {import('vscode').Uri} uri
   * @returns {{context: NonNullable<ReturnType<StaleTracking['contexts']['get']>>, file: string} | null}
   */
  locate(key, uri) {
    const context = this.contexts.get(key);
    if (context === undefined) return null;
    const file = relativeFile(context.project, uri);
    return file === null ? null : { context, file };
  }

  /**
   * @param {import('vscode').Uri} uri
   * @returns {string[]}
   */
  keysFor(uri) {
    return [...this.contexts.entries()]
      .filter(([, context]) => relativeFile(context.project, uri) !== null)
      .map(([key]) => key);
  }

  /**
   * @param {string} key
   */
  refresh(key) {
    const context = this.contexts.get(key);
    if (context === undefined) return;
    const stale = this.store.model(key).staleSet(context.rows, context.basis, context.mtimes);
    if (key === this.activeKey) this.annotations.setFreshness(stale);
    this.changed.fire(key);
  }

  /**
   * @param {import('vscode').Uri} project
   */
  watch(project) {
    const id = project.toString();
    if (this.watchedProject === id) return;
    this.watcher?.dispose();
    this.watchedProject = id;
    const watcher = this.api.workspace.createFileSystemWatcher(new this.api.RelativePattern(project, '**/*'));
    watcher.onDidChange((uri) => this.onFileChanged(uri));
    watcher.onDidCreate((uri) => this.onFileChanged(uri));
    watcher.onDidDelete((uri) => this.onFileChanged(uri));
    this.watcher = watcher;
  }

  dispose() {
    this.watcher?.dispose();
    for (const d of this.disposables) d.dispose();
    this.contexts.clear();
  }
}

module.exports = { StaleTracking, relativeFile };
