'use strict';

const vscode = require('vscode');
const { Runner } = require('./runner');
const { Scheduler } = require('./scheduler');
const { ResultCache } = require('./cache');
const { FileCacheStore } = require('./editor/cacheStore');
const { Selection } = require('./projects');
const { AnnotationProvider } = require('./editor/annotations');
const { Controller } = require('./editor/controller');
const { StaleStore } = require('./stale');
const { StaleTracking } = require('./editor/staleTracking');
const { PreviewRunner } = require('./editor/preview');
const { createPassphraseAsker } = require('./editor/passphrase');
const { DashboardTree } = require('./editor/treeView');
const { DashboardStatusBar, REVEAL_COMMAND } = require('./editor/statusBar');
const { SummaryView, VIEW_ID } = require('./editor/summaryView');
const { ResourceActions } = require('./editor/resourceActions');
const manifest = require('../package.json');

const CACHE_PRUNE_MS = 30 * 24 * 60 * 60 * 1000;

/** @type {InstanceType<typeof Controller> | null} */
let controller = null;

/**
 * @param {import('vscode').ExtensionContext} context
 */
function activate(context) {
  const settings = () => vscode.workspace.getConfiguration('finfocus');
  const runner = new Runner({
    isTrusted: () => vscode.workspace.isTrusted,
    finfocusPath: () => settings().get('path', ''),
    pulumiPath: () => settings().get('pulumiPath', ''),
  });
  const output = vscode.window.createOutputChannel('FinFocus');
  const annotations = new AnnotationProvider(vscode);
  const tracking = new StaleTracking({ api: vscode, store: new StaleStore(), annotations });
  /** @type {InstanceType<typeof Controller> | null} */
  let active = null;
  const cacheStore = new FileCacheStore(context.globalStorageUri.fsPath);
  const cache = new ResultCache({
    store: cacheStore,
    ttlMs: () => Math.max(0, Number(settings().get('cache.ttlMinutes', 60)) || 0) * 60 * 1000,
    isTrusted: () => vscode.workspace.isTrusted,
  });
  if (vscode.workspace.isTrusted) void cacheStore.prune(CACHE_PRUNE_MS).catch(() => undefined);
  const scheduler = new Scheduler({
    runner,
    cache,
    askPassphrase: createPassphraseAsker({ api: vscode, labelFor: (key) => active?.labelFor(key) ?? 'this stack' }),
  });
  const selection = new Selection();
  active = controller = new Controller({
    api: vscode,
    runner,
    scheduler,
    annotations,
    selection,
    minFinfocusVersion: manifest.minFinfocusVersion,
    output,
    tracking,
  });
  const ctl = active;
  const preview = new PreviewRunner({
    api: vscode,
    scheduler,
    tracking,
    annotations,
    selection,
    current: () => ctl.currentContext,
    onSaveEnabled: () => settings().get('previewOnSave', false),
    report: (message) => ctl.report(message),
  });
  const actions = new ResourceActions({ api: vscode, tracking });
  const tree = new DashboardTree({ api: vscode, tracking });
  const resources = vscode.window.createTreeView('finfocus.resources', { treeDataProvider: tree });
  const status = new DashboardStatusBar({ api: vscode, tracking });
  const card = new SummaryView({
    api: vscode,
    tracking,
    actions,
    extensionUri: context.extensionUri,
    log: (message) => output.appendLine(message),
  });
  /** @param {unknown} target a tree node, a resource URN, or nothing for the selection */
  const resourceUrn = (target) => {
    if (typeof target === 'string') return target;
    const node = /** @type {{id?: string} | undefined} */ (target) ?? resources.selection[0];
    return node?.id;
  };
  const refresh = () => ctl.refresh();
  const projectWatcher = vscode.workspace.createFileSystemWatcher('**/Pulumi.{yaml,yml}');

  context.subscriptions.push(
    output,
    annotations,
    tracking,
    projectWatcher,
    tree,
    resources,
    status,
    card,
    vscode.window.registerWebviewViewProvider(VIEW_ID, card),
    projectWatcher.onDidCreate(refresh),
    projectWatcher.onDidDelete(refresh),
    vscode.workspace.onDidChangeWorkspaceFolders(refresh),
    vscode.workspace.onDidGrantWorkspaceTrust(refresh),
    vscode.workspace.onDidChangeConfiguration((event) => {
      if (!event.affectsConfiguration('finfocus.path') && !event.affectsConfiguration('finfocus.pulumiPath')) return;
      ctl.settingsChanged();
      void ctl.refresh({ bypassCache: true });
    }),
    vscode.workspace.onDidSaveTextDocument((doc) => preview.runOnSave(doc)),
    vscode.commands.registerCommand('finfocus.refresh', () => ctl.refresh({ bypassCache: true })),
    vscode.commands.registerCommand('finfocus.selectProject', () => ctl.chooseProject()),
    vscode.commands.registerCommand('finfocus.selectStack', () => ctl.chooseStackExplicitly()),
    vscode.commands.registerCommand('finfocus.runPreview', () => preview.runManual()),
    vscode.commands.registerCommand('finfocus.cancelPreview', () => ctl.cancelActive()),
    vscode.commands.registerCommand('finfocus.openResource', (target) => {
      const urn = resourceUrn(target);
      return urn === undefined ? undefined : actions.open(urn);
    }),
    vscode.commands.registerCommand('finfocus.showResourceDetails', (target) => {
      const urn = resourceUrn(target);
      return urn === undefined ? undefined : actions.details(urn);
    }),
    vscode.commands.registerCommand(REVEAL_COMMAND, () => card.reveal()),
  );

  void refresh();
  return { controller: ctl, annotations, tracking, scheduler, preview };
}

/**
 * Stops every owned process to a confirmed end before the host exits.
 *
 * @returns {Promise<void>}
 */
async function deactivate() {
  const closing = controller?.dispose();
  controller = null;
  await closing;
}

module.exports = { activate, deactivate };
