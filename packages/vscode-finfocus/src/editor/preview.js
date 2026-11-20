'use strict';

const { relativeFile } = require('./staleTracking');

/**
 * @typedef {'saveAll' | 'runSaved' | 'cancel'} DirtyChoice
 * @typedef {{key: string, project: import('vscode').Uri, stack: string}} ContextRef
 */

const SAVE_ALL = 'Save all and run';
const RUN_SAVED = 'Run saved files';

/**
 * Prices local changes with a preview. A manual run offers to save unsaved
 * documents first; an on-save run captures only the file just saved and saves
 * nothing else. Pulumi reads saved content only, so what was unsaved stays
 * stale. A failed, cancelled, or superseded run changes no figure or mark.
 */
class PreviewRunner {
  /**
   * @param {{
   *   api: typeof import('vscode'),
   *   scheduler: InstanceType<typeof import('../scheduler').Scheduler>,
   *   tracking: InstanceType<typeof import('./staleTracking').StaleTracking>,
   *   annotations: InstanceType<typeof import('./annotations').AnnotationProvider>,
   *   selection: InstanceType<typeof import('../projects').Selection>,
   *   current: () => ContextRef | null,
   *   onSaveEnabled: () => boolean,
   *   report: (message: string) => void,
   *   chooseDirty?: (files: string[]) => Promise<DirtyChoice>,
   *   saveDocument?: (doc: import('vscode').TextDocument) => Thenable<boolean>,
   * }} deps
   */
  constructor(deps) {
    this.api = deps.api;
    this.scheduler = deps.scheduler;
    this.tracking = deps.tracking;
    this.annotations = deps.annotations;
    this.selection = deps.selection;
    this.current = deps.current;
    this.onSaveEnabled = deps.onSaveEnabled;
    this.report = deps.report;
    this.chooseDirty = deps.chooseDirty ?? ((files) => this.askDirty(files));
    this.saveDocument = deps.saveDocument ?? ((doc) => doc.save());
  }

  /**
   * The Run preview command.
   *
   * @returns {Promise<void>}
   */
  async runManual() {
    const context = this.current();
    if (context === null) {
      this.report('Select a Pulumi project and stack before running a preview.');
      return;
    }
    const dirty = this.api.workspace.textDocuments.filter(
      (doc) => doc.isDirty && relativeFile(context.project, doc.uri) !== null,
    );
    if (dirty.length > 0) {
      const choice = await this.chooseDirty(dirty.map((doc) => this.api.workspace.asRelativePath(doc.uri)));
      if (choice === 'cancel') return;
      if (choice === 'saveAll') {
        for (const doc of dirty) {
          if (!(await this.saveDocument(doc))) {
            this.report(`Could not save ${this.api.workspace.asRelativePath(doc.uri)}; no preview was run.`);
            return;
          }
        }
      }
    }
    await this.run(context);
  }

  /**
   * Runs after a save when on-save preview is enabled. Never saves anything.
   *
   * @param {import('vscode').TextDocument} doc
   * @returns {Promise<void>}
   */
  async runOnSave(doc) {
    if (!this.onSaveEnabled()) return;
    const context = this.current();
    if (context === null || relativeFile(context.project, doc.uri) === null) return;
    await this.run(context);
  }

  /**
   * @param {ContextRef} context
   * @returns {Promise<void>}
   */
  async run(context) {
    const epoch = this.selection.epoch;
    const snapshot = this.tracking.snapshotFor(context.key);
    const outcome = await this.api.window.withProgress(
      {
        location: this.api.ProgressLocation.Notification,
        title: `FinFocus: pricing a preview of ${context.stack}`,
        cancellable: true,
      },
      (_progress, token) => {
        const subscription = token.onCancellationRequested(() => void this.scheduler.cancel(context.key));
        return this.scheduler
          .run({
            key: context.key,
            projectDir: context.project.fsPath,
            stack: context.stack,
            stateOnly: false,
            snapshot,
            isCurrent: () => this.selection.isCurrent(epoch),
          })
          .finally(() => subscription.dispose());
      },
    );
    if (outcome.kind === 'failure' || outcome.kind === 'blocked') {
      this.report(outcome.message);
      return;
    }
    if (outcome.kind !== 'result') return;
    this.annotations.show(context.project, outcome.result.rows);
    await this.tracking.applyPreview({
      key: context.key,
      project: context.project,
      stack: context.stack,
      result: outcome.result,
      snapshot: /** @type {import('../stale').RequestSnapshot} */ (outcome.snapshot),
    });
  }

  /**
   * @param {string[]} files
   * @returns {Promise<DirtyChoice>}
   */
  async askDirty(files) {
    const list = files.length > 3 ? `${files.slice(0, 3).join(', ')} and ${files.length - 3} more` : files.join(', ');
    const picked = await this.api.window.showWarningMessage(
      `${files.length} unsaved file(s) in this project: ${list}. Pulumi previews saved files only.`,
      { modal: true },
      SAVE_ALL,
      RUN_SAVED,
    );
    if (picked === SAVE_ALL) return 'saveAll';
    if (picked === RUN_SAVED) return 'runSaved';
    return 'cancel';
  }
}

module.exports = { PreviewRunner };
