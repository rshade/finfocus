'use strict';

const {
  discoverProjects,
  resolveProject,
  chooseStack,
  contextKey,
  parseStackList,
  stackSettingsFiles,
  ProjectError,
} = require('../projects');
const { describeOutcome, RunnerError } = require('../runner');
const { hashContents } = require('../cache');
const { Readiness, withGuidance } = require('./readiness');

const PROJECT_GLOB = '**/Pulumi.{yaml,yml}';
const EXCLUDE_GLOB = '**/{node_modules,.git,vendor,dist,build,out,target}/**';

/**
 * Chooses the project and stack, loads the deployed state, and hands the rows
 * to the annotation provider. Nothing here runs before the workspace is
 * trusted or in a workspace with no Pulumi project.
 */
class Controller {
  /**
   * @param {{
   *   api: typeof import('vscode'),
   *   runner: InstanceType<typeof import('../runner').Runner>,
   *   scheduler: InstanceType<typeof import('../scheduler').Scheduler>,
   *   annotations: InstanceType<typeof import('./annotations').AnnotationProvider>,
   *   selection: InstanceType<typeof import('../projects').Selection>,
   *   minFinfocusVersion: string,
   *   output: {appendLine: (line: string) => void},
   *   tracking: InstanceType<typeof import('./staleTracking').StaleTracking>,
   * }} deps
   */
  constructor(deps) {
    this.api = deps.api;
    this.runner = deps.runner;
    this.scheduler = deps.scheduler;
    this.annotations = deps.annotations;
    this.selection = deps.selection;
    this.minFinfocusVersion = deps.minFinfocusVersion;
    this.output = deps.output;
    this.tracking = deps.tracking;
    /** @type {Map<string, string>} */
    this.explicitStacks = new Map();
    /** @type {string | null} */
    this.explicitProject = null;
    this.readiness = new Readiness({
      runner: this.runner,
      minFinfocusVersion: this.minFinfocusVersion,
      report: (message) => this.report(message),
      log: (line) => this.output.appendLine(line),
    });
    /** @type {string | null} */
    this.activeKey = null;
    /** @type {{key: string, project: import('vscode').Uri, stack: string} | null} */
    this.currentContext = null;
    /** @type {Map<string, string>} context key to project URI, for removal */
    this.known = new Map();
    this.disposed = false;
  }

  /**
   * Discovers, selects, and loads. Safe to call again for a refresh: it reruns
   * discovery so an external stack selection is picked up.
   *
   * `bypassCache` is set for the explicit Refresh command and for a changed
   * finfocus path: the stored result is not used, but the new one is stored.
   *
   * @param {{promptProject?: boolean, promptStack?: boolean, bypassCache?: boolean}} [options]
   * @returns {Promise<void>}
   */
  async refresh(options = {}) {
    if (this.disposed || !this.api.workspace.isTrusted) return;
    const projects = await this.findProjects();
    await this.dropRemoved(projects);
    if (projects.length === 0) {
      this.annotations.clear();
      return;
    }
    const project = await this.pickProject(projects, options.promptProject === true);
    if (project === null) return;
    if (this.selection.project !== project) this.annotations.clear();
    this.selection.selectProject(project);
    if (!(await this.readiness.ready())) return;

    const projectUri = this.api.Uri.parse(project);
    const picked = await this.pickStack(project, projectUri, options.promptStack === true);
    if (picked === null) return;
    const { name: stack, entry } = picked;
    const epoch = this.selection.selectStack(project, stack);
    const key = contextKey(project, stack);
    if (this.activeKey !== null && this.activeKey !== key) {
      const stopped = await this.scheduler.cancel(this.activeKey);
      if (stopped !== null && !stopped.confirmed) this.report('The previous run could not be stopped; wait for it or reload the window.');
    }
    this.activeKey = key;

    const outcome = await this.scheduler.run({
      key,
      projectDir: projectUri.fsPath,
      stack,
      stateOnly: true,
      isCurrent: () => this.selection.isCurrent(epoch) && !this.disposed,
      cacheSignals: await this.cacheSignals(projectUri, stack, entry),
      bypassCache: options.bypassCache === true,
    });
    if (outcome.kind === 'superseded' || outcome.kind === 'dismissed' || this.disposed) return;
    if (outcome.kind === 'failure' || outcome.kind === 'blocked') {
      this.report(withGuidance(outcome.message));
      return;
    }
    this.known.set(key, project);
    this.currentContext = { key, project: projectUri, stack };
    this.annotations.show(projectUri, outcome.result.rows);
    await this.tracking.setResult({
      key,
      project: projectUri,
      stack,
      result: outcome.result,
      basis: 'state',
      completeness: outcome.completeness,
    });
  }

  /**
   * What the result cache keys on besides the context: the stack's update
   * signals from discovery, the hashes of its settings file and the project's
   * finfocus configuration, and the finfocus version. A file that cannot be
   * read hashes as absent, which only costs a miss.
   *
   * @param {import('vscode').Uri} projectUri
   * @param {string} stack
   * @param {import('../projects').StackEntry | null} entry
   * @returns {Promise<import('../cache').CacheSignals>}
   */
  async cacheSignals(projectUri, stack, entry) {
    const settings = [];
    for (const name of stackSettingsFiles(stack)) {
      const text = await this.readText(this.api.Uri.joinPath(projectUri, name));
      if (text !== null) settings.push(`${name}\n${text}`);
    }
    const projectConfig = await this.readText(this.api.Uri.joinPath(projectUri, '.finfocus', 'config.hujson'));
    return {
      lastUpdate: entry?.lastUpdate,
      resourceCount: entry?.resourceCount,
      updateInProgress: entry?.updateInProgress,
      stackConfigHash: settings.length === 0 ? '' : hashContents(settings.join('\n---\n')),
      projectConfigHash: hashContents(projectConfig),
      finfocusVersion: this.readiness.version,
    };
  }

  /**
   * @param {import('vscode').Uri} uri
   * @returns {Promise<string | null>}
   */
  async readText(uri) {
    try {
      return Buffer.from(await this.api.workspace.fs.readFile(uri)).toString('utf8');
    } catch {
      return null;
    }
  }

  /**
   * A removed project stops its run, forgets its credentials and marks, and
   * clears what it showed.
   *
   * @param {string[]} projects
   * @returns {Promise<void>}
   */
  async dropRemoved(projects) {
    for (const [key, project] of [...this.known]) {
      if (projects.includes(project)) continue;
      this.known.delete(key);
      await this.scheduler.cancel(key);
      this.scheduler.clearCredentials(key);
      this.tracking.drop(key);
      this.selection.invalidateProject(project);
      if (this.activeKey === key) {
        this.activeKey = null;
        this.currentContext = null;
        this.annotations.clear();
      }
    }
  }

  /**
   * The text shown for a context key in a prompt.
   *
   * @param {string} key
   * @returns {string}
   */
  labelFor(key) {
    try {
      return String(JSON.parse(key)[1]);
    } catch {
      return 'this stack';
    }
  }

  /** Stops the active context's run, for the cancel command. */
  async cancelActive() {
    if (this.activeKey !== null) await this.scheduler.cancel(this.activeKey);
  }

  /** @returns {Promise<void>} */
  async chooseProject() {
    this.explicitProject = null;
    await this.refresh({ promptProject: true });
  }

  /** @returns {Promise<void>} */
  async chooseStackExplicitly() {
    await this.refresh({ promptStack: true });
  }

  /**
   * @returns {Promise<string[]>}
   */
  async findProjects() {
    const files = await this.api.workspace.findFiles(PROJECT_GLOB, EXCLUDE_GLOB);
    const folders = (this.api.workspace.workspaceFolders ?? []).map((f) => f.uri.toString());
    return discoverProjects(
      files.map((f) => f.toString()),
      folders,
    );
  }

  /**
   * @param {string[]} projects
   * @param {boolean} forcePick
   * @returns {Promise<string | null>}
   */
  async pickProject(projects, forcePick) {
    const active = this.api.window.activeTextEditor?.document.uri.toString() ?? null;
    const choice = resolveProject({ projects, activeFile: forcePick ? null : active, explicit: this.explicitProject });
    if (choice.kind === 'selected' && !(forcePick && projects.length > 1)) return choice.project;
    if (projects.length === 0) return null;
    const labels = new Map(projects.map((p) => [this.api.workspace.asRelativePath(this.api.Uri.parse(p)), p]));
    const picked = await this.api.window.showQuickPick([...labels.keys()], { placeHolder: 'Select the Pulumi project' });
    if (picked === undefined) return null;
    const project = labels.get(picked) ?? null;
    this.explicitProject = project;
    return project;
  }

  /**
   * @param {string} project
   * @param {import('vscode').Uri} projectUri
   * @param {boolean} forcePick
   * @returns {Promise<{name: string, entry: import('../projects').StackEntry | null} | null>}
   */
  async pickStack(project, projectUri, forcePick) {
    const stacks = await this.listStacks(projectUri);
    if (stacks === null) return null;
    const explicit = this.explicitStacks.get(project) ?? null;
    const choice = chooseStack(stacks, explicit);
    if (choice.kind === 'none') {
      this.report('This Pulumi project has no stacks. Create one with `pulumi stack init`.');
      return null;
    }
    const found = (/** @type {string} */ name) => ({ name, entry: stacks.find((s) => s.name === name) ?? null });
    if (choice.kind === 'selected' && !forcePick) return found(choice.stack);
    const picked = await this.api.window.showQuickPick(
      stacks.map((s) => s.name),
      { placeHolder: 'Select the Pulumi stack' },
    );
    if (picked === undefined) return null;
    this.explicitStacks.set(project, picked);
    return found(picked);
  }

  /**
   * @param {import('vscode').Uri} projectUri
   * @returns {Promise<import('../projects').StackEntry[] | null>}
   */
  async listStacks(projectUri) {
    try {
      const outcome = await this.runner.discoverStacks(projectUri.fsPath).done;
      if (outcome.kind !== 'ok') {
        this.report(
          outcome.kind === 'spawn_error' ? 'Could not run pulumi; install it or set finfocus.pulumiPath to its location.' : describeOutcome(outcome),
        );
        return null;
      }
      return parseStackList(outcome.stdout);
    } catch (error) {
      if (error instanceof ProjectError || error instanceof RunnerError) {
        this.report(error.message);
        return null;
      }
      throw error;
    }
  }

  /** Allows one new readiness attempt after a setting that locates finfocus changes. */
  settingsChanged() {
    this.readiness.settingsChanged();
  }

  /**
   * @param {string} message
   */
  report(message) {
    this.output.appendLine(message);
    void this.api.window.showWarningMessage(`FinFocus: ${message}`);
  }

  /**
   * Stops every owned process to a confirmed end and forgets credentials.
   *
   * @returns {Promise<void>}
   */
  async dispose() {
    this.disposed = true;
    const stopped = await this.scheduler.closeAll();
    for (const result of stopped) {
      if (!result.confirmed) this.output.appendLine(`A finfocus process could not be stopped: ${result.survivors.join(', ')}`);
    }
  }
}

module.exports = { Controller };
