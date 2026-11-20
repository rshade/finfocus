'use strict';

const vscode = require('vscode');
const { Selection } = require('../../src/projects');
const { Scheduler } = require('../../src/scheduler');
const { AnnotationProvider } = require('../../src/editor/annotations');
const { Controller } = require('../../src/editor/controller');
const { PreviewRunner } = require('../../src/editor/preview');
const { StaleStore } = require('../../src/stale');
const { StaleTracking } = require('../../src/editor/staleTracking');

const URN = 'urn:pulumi:dev::fixture::pulumi-nodejs:dynamic:Resource::';

function overviewJson(rows) {
  return JSON.stringify({
    metadata: { stackName: 'dev', generatedAt: '2026-11-17T00:00:00Z', isStateOnly: true },
    summary: { currency: 'USD', display: { projected: '$1.00', actual: '-', delta: '-' } },
    budgetDisplay: { healthDisplay: '', amountsDisplay: '', footerDisplay: '', details: [] },
    resources: rows.map((r) => ({
      urn: `${URN}${r.name}`,
      type: 'pulumi-nodejs:dynamic:Resource',
      status: 'active',
      projectedCost: { monthlyCost: 1, currency: 'USD' },
      ...(r.location ? { location: r.location } : {}),
      ...(r.modifiedAt === undefined ? {} : { modifiedAt: r.modifiedAt }),
      display: { projected: r.projected ?? '$61.32', actual: '-', delta: '-', status: 'active' },
    })),
    errors: [],
  });
}

function handle(outcome) {
  return { pid: 1, signal: () => true, done: Promise.resolve(outcome), stop: async () => ({ confirmed: true, forced: false, survivors: [] }) };
}

/** A runner that spawns nothing and counts what it was asked to start. */
function fakeRunner({ rows, stacks = [{ name: 'dev', current: true }], onOverview = null }) {
  const calls = { version: 0, stacks: 0, overview: 0, requests: [] };
  const runner = {
    calls,
    rows,
    version: () => {
      calls.version += 1;
      return handle({ kind: 'ok', stdout: 'finfocus version v0.5.8' });
    },
    discoverStacks: () => {
      calls.stacks += 1;
      return handle({ kind: 'ok', stdout: JSON.stringify(stacks) });
    },
    overview: (request) => {
      calls.overview += 1;
      calls.requests.push(request);
      if (onOverview !== null) return onOverview(request, calls.overview);
      return handle({ kind: 'ok', stdout: overviewJson(runner.rows) });
    },
  };
  return runner;
}

/** The real API with trust and project discovery overridden. */
function apiWith({ trusted = true, projectFiles } = {}) {
  return new Proxy(vscode, {
    get(target, prop) {
      if (prop === 'workspace') {
        return new Proxy(target.workspace, {
          get(ws, key) {
            if (key === 'isTrusted') return trusted;
            if (key === 'findFiles' && projectFiles !== undefined) return async () => projectFiles;
            const value = ws[key];
            return typeof value === 'function' ? value.bind(ws) : value;
          },
        });
      }
      return target[prop];
    },
  });
}

function build({ runner, api = vscode, askPassphrase, cache = null, output = { appendLine: () => {} }, preview: previewOptions = {} }) {
  const annotations = new AnnotationProvider(api);
  const tracking = new StaleTracking({ api, store: new StaleStore(), annotations });
  const scheduler = new Scheduler({ runner, askPassphrase, cache });
  const selection = new Selection();
  const controller = new Controller({
    api,
    runner,
    scheduler,
    annotations,
    selection,
    minFinfocusVersion: '0.5.8',
    output,
    tracking,
  });
  const reports = [];
  const preview = new PreviewRunner({
    api,
    scheduler,
    tracking,
    annotations,
    selection,
    current: () => controller.currentContext,
    onSaveEnabled: () => previewOptions.onSave ?? false,
    report: (message) => reports.push(message),
    chooseDirty: previewOptions.chooseDirty,
    saveDocument: previewOptions.saveDocument,
  });
  return { annotations, controller, tracking, scheduler, preview, reports };
}

function fixtureUri(file) {
  return vscode.Uri.joinPath(vscode.workspace.workspaceFolders[0].uri, file);
}

/** A process that ends only when stopped, like a long preview. */
function hangingHandle() {
  let finish;
  const done = new Promise((resolve) => {
    finish = resolve;
  });
  const handle = {
    pid: 2,
    stops: 0,
    complete: (outcome) => finish(outcome),
    signal: () => true,
    done,
    stop: async () => {
      handle.stops += 1;
      finish({ kind: 'signalled', signal: 'SIGTERM', stderr: '' });
      return { confirmed: true, forced: false, survivors: [] };
    },
  };
  return handle;
}

module.exports = { build, fakeRunner, apiWith, fixtureUri, hangingHandle, overviewJson, handle, URN };
