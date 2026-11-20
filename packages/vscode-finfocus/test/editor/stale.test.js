'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vscode = require('vscode');
const { build, fakeRunner, fixtureUri } = require('./support');

const FUTURE = '2099-01-01T00:00:00Z';
const SCRATCH = 'scratch.js';
const budgetMs = 1000;
const timings = [];

function scratchPath() {
  return fixtureUri(SCRATCH).fsPath;
}

const row = (name, extra = {}) => ({
  name,
  location: { file: SCRATCH, line: 2, column: 1 },
  modifiedAt: FUTURE,
  ...extra,
});

function reasons(annotations) {
  const [annotation] = annotations.annotationsFor(fixtureUri(SCRATCH));
  return annotation === undefined ? null : annotations.reasonsFor(annotation);
}

/** Resolves with the elapsed ms once the predicate holds after a stale recompute. */
function until(tracking, predicate, limit = 5000) {
  const started = performance.now();
  return new Promise((resolve, reject) => {
    const check = () => {
      if (!predicate()) return false;
      subscription.dispose();
      clearTimeout(timer);
      resolve(performance.now() - started);
      return true;
    };
    const subscription = tracking.onDidChange(check);
    const timer = setTimeout(() => {
      subscription.dispose();
      reject(new Error('stale state did not change in time'));
    }, limit);
    check();
  });
}

async function loaded(rows, runnerOptions = {}) {
  const runner = fakeRunner({ rows, ...runnerOptions });
  const parts = build({ runner });
  await parts.controller.refresh();
  return { runner, ...parts };
}

function dispose({ annotations, controller, tracking }) {
  tracking.dispose();
  annotations.dispose();
  controller.dispose();
}

suite('stale tracking', () => {
  setup(async () => {
    fs.writeFileSync(scratchPath(), 'const a = 1;\nconst web = new Box("web");\nconst b = 2;\n');
    for (const name of ['Pulumi.dev.yaml', 'Pulumi.dev.yml', 'Pulumi.shop.yaml']) {
      fs.rmSync(path.join(path.dirname(scratchPath()), name), { force: true });
    }
    await new Promise((r) => setTimeout(r, 500));
  });
  teardown(async () => {
    await vscode.commands.executeCommand('workbench.action.revertAndCloseActiveEditor');
    await vscode.commands.executeCommand('workbench.action.closeAllEditors');
    fs.rmSync(scratchPath(), { force: true });
  });
  suiteTeardown(() => {
    console.log(`STALE_LATENCY_MS ${JSON.stringify(timings)}`);
    for (const name of ['Pulumi.dev.yaml', 'Pulumi.dev.yml', 'Pulumi.shop.yaml']) {
      fs.rmSync(path.join(path.dirname(scratchPath()), name), { force: true });
    }
  });

  test('a file older than modifiedAt starts fresh', async () => {
    const parts = await loaded([row('web')]);
    assert.deepEqual(reasons(parts.annotations), []);
    dispose(parts);
  });

  test('a file newer than modifiedAt, a bad timestamp, and a missing file are flagged at load', async () => {
    let parts = await loaded([row('web', { modifiedAt: '2000-01-01T00:00:00Z' })]);
    assert.deepEqual(reasons(parts.annotations), ['edited']);
    dispose(parts);
    parts = await loaded([row('web', { modifiedAt: 'garbage' })]);
    assert.deepEqual(reasons(parts.annotations), ['freshness unknown']);
    dispose(parts);
    fs.rmSync(scratchPath());
    parts = await loaded([row('web')]);
    assert.deepEqual(reasons(parts.annotations), ['freshness unknown']);
    dispose(parts);
  });

  test('a document that is already dirty is stale at load', async () => {
    const editor = await vscode.window.showTextDocument(fixtureUri(SCRATCH));
    await editor.edit((e) => e.insert(new vscode.Position(0, 0), '// dirty\n'));
    const parts = await loaded([row('web')]);
    assert.deepEqual(reasons(parts.annotations), ['unsaved']);
    dispose(parts);
  });

  test('an unsaved edit marks the row within the latency budget with no overview run', async () => {
    const parts = await loaded([row('web')]);
    const editor = await vscode.window.showTextDocument(fixtureUri(SCRATCH));
    const started = performance.now();
    await editor.edit((e) => e.insert(new vscode.Position(0, 0), '// x\n'));
    const waited = await until(parts.tracking, () => reasons(parts.annotations)?.[0] === 'unsaved');
    const latency = performance.now() - started;
    timings.push(['unsaved', Math.round(latency)]);
    assert.ok(waited <= budgetMs && latency <= budgetMs, `took ${latency} ms`);
    assert.equal(parts.runner.calls.overview, 1);
    dispose(parts);
  });

  test('a saved edit stays edited and starts no pricing', async () => {
    const parts = await loaded([row('web')]);
    const editor = await vscode.window.showTextDocument(fixtureUri(SCRATCH));
    await editor.edit((e) => e.insert(new vscode.Position(0, 0), '// x\n'));
    const started = performance.now();
    await editor.document.save();
    await until(parts.tracking, () => reasons(parts.annotations)?.[0] === 'edited');
    const latency = performance.now() - started;
    timings.push(['saved', Math.round(latency)]);
    assert.ok(latency <= budgetMs, `took ${latency} ms`);
    await new Promise((r) => setTimeout(r, 2500));
    assert.deepEqual(reasons(parts.annotations), ['edited'], 'the save echo from the watcher must not change it');
    assert.equal(parts.runner.calls.overview, 1);
    dispose(parts);
  });

  test('an external edit is marked from the file system', async () => {
    const parts = await loaded([row('web')]);
    const started = performance.now();
    fs.appendFileSync(scratchPath(), '// outside\n');
    await until(parts.tracking, () => reasons(parts.annotations)?.[0] === 'edited');
    timings.push(['external', Math.round(performance.now() - started)]);
    assert.ok(performance.now() - started <= budgetMs);
    dispose(parts);
  });

  test('a deleted or renamed declaring file marks the row', async () => {
    const parts = await loaded([row('web')]);
    fs.renameSync(scratchPath(), `${scratchPath()}.moved`);
    try {
      await until(parts.tracking, () => reasons(parts.annotations)?.[0] === 'edited');
    } finally {
      fs.rmSync(`${scratchPath()}.moved`, { force: true });
    }
    dispose(parts);
  });

  for (const [label, fileName, stack] of [
    ['.yaml', 'Pulumi.dev.yaml', 'dev'],
    ['.yml', 'Pulumi.dev.yml', 'dev'],
    ['a qualified stack name', 'Pulumi.shop.yaml', 'acme/fixture/shop'],
  ]) {
    test(`a stack settings change (${label}) marks every row as config changed`, async () => {
      const parts = await loaded([row('web'), { name: 'plain' }], { stacks: [{ name: stack, current: true }] });
      const started = performance.now();
      fs.writeFileSync(path.join(path.dirname(scratchPath()), fileName), 'config: {}\n');
      await until(parts.tracking, () => reasons(parts.annotations)?.includes('config changed'));
      timings.push([`config ${label}`, Math.round(performance.now() - started)]);
      dispose(parts);
    });
  }

  test('another stack\'s settings file does not mark this stack', async () => {
    const parts = await loaded([row('web')]);
    fs.writeFileSync(path.join(path.dirname(scratchPath()), 'Pulumi.shop.yaml'), 'config: {}\n');
    await new Promise((r) => setTimeout(r, 1500));
    assert.deepEqual(reasons(parts.annotations), []);
    dispose(parts);
  });

  test('an edit made while a refresh is running survives the refresh result', async () => {
    const runner = fakeRunner({ rows: [row('web')] });
    const parts = build({ runner });
    const editor = await vscode.window.showTextDocument(fixtureUri(SCRATCH));
    await parts.controller.refresh();
    const second = parts.controller.refresh();
    await editor.edit((e) => e.insert(new vscode.Position(0, 0), '// racing\n'));
    await second;
    await new Promise((r) => setTimeout(r, 100));
    assert.deepEqual(reasons(parts.annotations), ['unsaved']);
    dispose(parts);
  });
});
