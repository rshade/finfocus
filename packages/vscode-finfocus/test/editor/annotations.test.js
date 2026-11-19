'use strict';

const assert = require('node:assert/strict');
const childProcess = require('node:child_process');
const fs = require('node:fs');
const vscode = require('vscode');
const { build, fakeRunner, apiWith, fixtureUri } = require('./support');
const { openDeclaration } = require('../../src/editor/navigation');
const { Runner } = require('../../src/runner');
const { Scheduler } = require('../../src/scheduler');
const { AnnotationProvider } = require('../../src/editor/annotations');

const WEB = { name: 'web', location: { file: 'index.js', line: 3, column: 13 } };
const WORKER = { name: 'worker', location: { file: 'index.js', line: 4, column: 16 }, projected: '$8.00' };

async function closeAll() {
  await vscode.commands.executeCommand('workbench.action.closeAllEditors');
}

suite('annotations', () => {
  teardown(closeAll);

  test('a deployed state-only load annotates declarations with the CLI strings', async () => {
    const runner = fakeRunner({ rows: [WEB, WORKER] });
    const { annotations, controller } = build({ runner });
    await vscode.window.showTextDocument(fixtureUri('index.js'));
    await controller.refresh();
    const placed = annotations.annotationsFor(fixtureUri('index.js'));
    assert.deepEqual(
      placed.map((a) => [a.line, a.text]),
      [
        [3, '$61.32'],
        [4, '$8.00'],
      ],
    );
    assert.equal(runner.calls.overview, 1);
    annotations.dispose();
    controller.dispose();
  });

  test('closing and reopening a document keeps its annotations', async () => {
    const { annotations, controller } = build({ runner: fakeRunner({ rows: [WEB] }) });
    await controller.refresh();
    await vscode.window.showTextDocument(fixtureUri('index.js'));
    await closeAll();
    await vscode.window.showTextDocument(fixtureUri('index.js'));
    assert.equal(annotations.annotationsFor(fixtureUri('index.js')).length, 1);
    annotations.dispose();
    controller.dispose();
  });

  test('inserting lines above a declaration moves its annotation', async () => {
    const { annotations, controller } = build({ runner: fakeRunner({ rows: [WEB] }) });
    const editor = await vscode.window.showTextDocument(fixtureUri('index.js'));
    await controller.refresh();
    await editor.edit((edit) => edit.insert(new vscode.Position(0, 0), '// one\n// two\n'));
    assert.equal(annotations.annotationsFor(fixtureUri('index.js'))[0].line, 5);
    await vscode.commands.executeCommand('undo');
    assert.equal(annotations.annotationsFor(fixtureUri('index.js'))[0].line, 3);
    annotations.dispose();
    controller.dispose();
  });

  test('an explicit refresh replaces the annotations', async () => {
    const runner = fakeRunner({ rows: [WEB] });
    const { annotations, controller } = build({ runner });
    await controller.refresh();
    runner.rows = [WORKER];
    await controller.refresh();
    const placed = annotations.annotationsFor(fixtureUri('index.js'));
    assert.deepEqual(
      placed.map((a) => a.line),
      [4],
    );
    annotations.dispose();
    controller.dispose();
  });

  test('rows with no or unsafe location get no annotation', async () => {
    const rows = [
      { name: 'none' },
      { name: 'escape', location: { file: '../secret.js', line: 1 } },
      { name: 'absolute', location: { file: '/etc/passwd', line: 1 } },
    ];
    const { annotations, controller } = build({ runner: fakeRunner({ rows }) });
    await controller.refresh();
    assert.equal(annotations.annotationsFor(fixtureUri('index.js')).length, 0);
    annotations.dispose();
    controller.dispose();
  });

  test('source navigation opens the declaration and clamps a stale position', async () => {
    const root = vscode.workspace.workspaceFolders[0].uri;
    const row = (location) => ({ urn: 'u', location, display: {} });
    assert.equal(await openDeclaration(vscode, root, row({ file: 'index.js', line: 4, column: 3 })), true);
    assert.equal(vscode.window.activeTextEditor.selection.active.line, 3);
    assert.equal(await openDeclaration(vscode, root, row({ file: 'index.js', line: 9999 })), true);
    assert.ok(vscode.window.activeTextEditor.selection.active.line < 20);
    assert.equal(await openDeclaration(vscode, root, row({ file: '../outside.js', line: 1 })), false);
    assert.equal(await openDeclaration(vscode, root, row({ file: 'missing.js', line: 1 })), false);
    assert.equal(await openDeclaration(vscode, root, row(null)), false);
  });

  test('an untrusted workspace starts no subprocess', async () => {
    const runner = fakeRunner({ rows: [WEB] });
    const api = apiWith({ trusted: false });
    const { annotations, controller } = build({ runner, api });
    await controller.refresh();
    assert.deepEqual([runner.calls.version, runner.calls.stacks, runner.calls.overview], [0, 0, 0]);
    annotations.dispose();
    controller.dispose();
  });

  test('a workspace with no Pulumi project starts no subprocess', async () => {
    const runner = fakeRunner({ rows: [WEB] });
    const api = apiWith({ projectFiles: [] });
    const { annotations, controller } = build({ runner, api });
    await controller.refresh();
    assert.deepEqual([runner.calls.version, runner.calls.stacks, runner.calls.overview], [0, 0, 0]);
    annotations.dispose();
    controller.dispose();
  });
});

suite('annotations against the real CLI (T043)', () => {
  const binary = process.env.FINFOCUS_BINARY;
  const project = process.env.FINFOCUS_FIXTURE_PROJECT;

  test('every display string, count and missing location matches the CLI JSON', async function () {
    if (!binary || !project || !fs.existsSync(project)) this.skip();
    const stack = process.env.FINFOCUS_FIXTURE_STACK || 'dev';
    const args = ['overview', '--output', 'json', '--format', 'json', '--yes', '--state-only', '--stack', stack];
    const cli = JSON.parse(childProcess.execFileSync(binary, args, { cwd: project, encoding: 'utf8' }));

    const runner = new Runner({ isTrusted: () => true, finfocusPath: () => binary });
    const outcome = await new Scheduler({ runner }).run({ key: 'real', projectDir: project, stack, stateOnly: true });
    assert.equal(outcome.kind, 'result');

    const annotations = new AnnotationProvider(vscode);
    const projectUri = vscode.Uri.file(project);
    annotations.show(projectUri, outcome.result.rows);

    const expected = new Map();
    let withoutLocation = 0;
    for (const resource of cli.resources) {
      if (!resource.location) {
        withoutLocation += 1;
        continue;
      }
      const key = `${resource.location.file}:${resource.location.line}`;
      expected.set(key, [...(expected.get(key) ?? []), resource.display.projected]);
    }
    assert.ok(withoutLocation > 0, 'the fixture has a resource with no location');
    assert.ok([...expected.values()].some((v) => v.length > 1), 'the fixture has a shared declaration line');

    const placed = new Map();
    for (const file of new Set([...expected.keys()].map((k) => k.split(':')[0]))) {
      for (const a of annotations.annotationsFor(vscode.Uri.joinPath(projectUri, ...file.split('/')))) {
        placed.set(`${file}:${a.line}`, a);
      }
    }
    assert.deepEqual([...placed.keys()].sort(), [...expected.keys()].sort());
    for (const [key, strings] of expected) {
      const annotation = placed.get(key);
      assert.equal(annotation.count, strings.length, key);
      assert.equal(annotation.text, strings.length === 1 ? strings[0] : `${strings.length} resources`, key);
    }
    assert.equal(
      [...placed.values()].reduce((sum, a) => sum + a.count, 0),
      cli.resources.length - withoutLocation,
    );
    annotations.dispose();
  });
});
