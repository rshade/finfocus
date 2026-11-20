'use strict';

const assert = require('node:assert/strict');
const vscode = require('vscode');
const { build, fakeRunner, apiWith, fixtureUri, handle } = require('./support');

const MISSING = { kind: 'spawn_error', code: 'ENOENT', stderr: '' };

function capture() {
  const lines = [];
  return { lines, output: { appendLine: (line) => lines.push(line) } };
}

function withVersion(runner, outcomes) {
  let calls = 0;
  runner.version = () => {
    runner.calls.version += 1;
    const outcome = outcomes[Math.min(calls, outcomes.length - 1)];
    calls += 1;
    return handle(outcome);
  };
  return runner;
}

function dispose({ annotations, controller, tracking }) {
  tracking.dispose();
  annotations.dispose();
  controller.dispose();
}

suite('first-run readiness', () => {
  test('a missing finfocus gives install and path guidance, then no further attempts', async () => {
    const { lines, output } = capture();
    const runner = withVersion(fakeRunner({ rows: [] }), [MISSING]);
    const parts = build({ runner, output });
    await parts.controller.refresh();
    await parts.controller.refresh();
    await parts.controller.refresh();
    assert.equal(runner.calls.version, 1);
    assert.equal(runner.calls.stacks, 0);
    assert.equal(runner.calls.overview, 0);
    assert.equal(lines.filter((line) => /finfocus\.path/.test(line)).length, 1);
    assert.match(lines.join('\n'), /install/i);
    dispose(parts);
  });

  test('changing the setting retries once and loads', async () => {
    const { lines, output } = capture();
    const runner = withVersion(fakeRunner({ rows: [{ name: 'web', location: { file: 'index.js', line: 1, column: 1 } }] }), [
      MISSING,
      { kind: 'ok', stdout: 'finfocus version v0.5.8' },
    ]);
    const parts = build({ runner, output });
    await parts.controller.refresh();
    assert.equal(runner.calls.overview, 0);
    parts.controller.settingsChanged();
    await parts.controller.refresh();
    assert.equal(runner.calls.version, 2);
    assert.equal(runner.calls.overview, 1);
    assert.equal(lines.length, 1);
    dispose(parts);
  });

  test('an old finfocus names the version it needs and starts no run', async () => {
    const { lines, output } = capture();
    const runner = withVersion(fakeRunner({ rows: [] }), [{ kind: 'ok', stdout: 'finfocus version v0.5.1' }]);
    const parts = build({ runner, output });
    await parts.controller.refresh();
    await parts.controller.refresh();
    assert.equal(runner.calls.version, 1);
    assert.equal(runner.calls.overview, 0);
    assert.match(lines.join('\n'), /0\.5\.8/);
    assert.match(lines.join('\n'), /0\.5\.1/);
    dispose(parts);
  });

  test('a development build is allowed, logged, and loads', async () => {
    const { lines, output } = capture();
    const runner = withVersion(fakeRunner({ rows: [{ name: 'web', location: { file: 'index.js', line: 1, column: 1 } }] }), [
      { kind: 'ok', stdout: 'finfocus version dev' },
    ]);
    const parts = build({ runner, output });
    await parts.controller.refresh();
    assert.equal(runner.calls.overview, 1);
    assert.equal(lines.length, 1);
    assert.match(lines[0], /finfocus version dev/);
    dispose(parts);
  });

  test('missing plugins point to plugin installation with the CLI reason kept', async () => {
    const { lines, output } = capture();
    const failed = {
      kind: 'failure',
      exitCode: 1,
      failure: { message: 'no cost plugins are installed', category: 'plugin', version: '1', errorCode: 'NO_PLUGINS' },
      stderr: '',
    };
    const runner = fakeRunner({ rows: [], onOverview: () => handle(failed) });
    const parts = build({ runner, output });
    await parts.controller.refresh();
    const text = lines.join('\n');
    assert.match(text, /no cost plugins are installed/);
    assert.match(text, /finfocus plugin install/);
    assert.equal(parts.annotations.annotationsFor(fixtureUri('index.js')).length, 0);
    dispose(parts);
  });

  test('a missing pulumi points to finfocus.pulumiPath and starts no overview', async () => {
    const { lines, output } = capture();
    const runner = fakeRunner({ rows: [] });
    runner.discoverStacks = () => {
      runner.calls.stacks += 1;
      return handle(MISSING);
    };
    const parts = build({ runner, output });
    await parts.controller.refresh();
    assert.equal(runner.calls.overview, 0);
    assert.match(lines.join('\n'), /finfocus\.pulumiPath/);
    dispose(parts);
  });

  test('a project with no stacks says how to create one', async () => {
    const { lines, output } = capture();
    const runner = fakeRunner({ rows: [], stacks: [] });
    const parts = build({ runner, output });
    await parts.controller.refresh();
    assert.equal(runner.calls.overview, 0);
    assert.match(lines.join('\n'), /pulumi stack init/);
    dispose(parts);
  });

  test('a malformed stack listing is reported, not thrown, and starts no overview', async () => {
    const { lines, output } = capture();
    const runner = fakeRunner({ rows: [] });
    runner.discoverStacks = () => handle({ kind: 'ok', stdout: '{"not": "a list"' });
    const parts = build({ runner, output });
    await assert.doesNotReject(parts.controller.refresh());
    assert.equal(runner.calls.overview, 0);
    assert.equal(lines.length, 1);
    dispose(parts);
  });

  test('an untrusted workspace starts nothing and says nothing', async () => {
    const { lines, output } = capture();
    const runner = fakeRunner({ rows: [] });
    const parts = build({ runner, api: apiWith({ trusted: false }), output });
    await parts.controller.refresh();
    assert.deepEqual(runner.calls, { version: 0, stacks: 0, overview: 0, requests: [] });
    assert.deepEqual(lines, []);
    dispose(parts);
  });

  test('a workspace with no Pulumi project is silent and starts nothing', async () => {
    const { lines, output } = capture();
    const runner = fakeRunner({ rows: [] });
    const parts = build({ runner, api: apiWith({ projectFiles: [] }), output });
    await parts.controller.refresh();
    assert.deepEqual(runner.calls, { version: 0, stacks: 0, overview: 0, requests: [] });
    assert.deepEqual(lines, []);
    dispose(parts);
  });

  test('a Pulumi.yaml only under node_modules is ignored: nothing runs or shows', async () => {
    const { lines, output } = capture();
    const runner = fakeRunner({ rows: [] });
    const nested = vscode.Uri.joinPath(vscode.workspace.workspaceFolders[0].uri, 'node_modules', 'dep', 'Pulumi.yaml');
    const parts = build({ runner, api: apiWith({ projectFiles: [nested] }), output });
    await parts.controller.refresh();
    assert.deepEqual(runner.calls, { version: 0, stacks: 0, overview: 0, requests: [] });
    assert.deepEqual(lines, []);
    assert.equal(parts.annotations.annotationsFor(fixtureUri('index.js')).length, 0);
    dispose(parts);
  });
});
