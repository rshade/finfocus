'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const vscode = require('vscode');
const { build, fakeRunner, fixtureUri, hangingHandle, overviewJson, handle } = require('./support');
const { createPassphraseAsker } = require('../../src/editor/passphrase');

const SCRATCH = 'scratch.js';
const scratch = () => fixtureUri(SCRATCH);
const row = (name, extra = {}) => ({ name, location: { file: SCRATCH, line: 2, column: 1 }, modifiedAt: '2099-01-01T00:00:00Z', ...extra });

function reasons(annotations) {
  const [annotation] = annotations.annotationsFor(scratch());
  return annotation === undefined ? null : annotations.reasonsFor(annotation);
}

function text(annotations) {
  return annotations.annotationsFor(scratch())[0]?.text;
}

async function loaded(options = {}) {
  const { rows = [row('web')], onOverview, ...rest } = options;
  const runner = fakeRunner({ rows, onOverview });
  const parts = build({ runner, ...rest });
  await parts.controller.refresh();
  return { runner, ...parts };
}

function done(parts) {
  parts.tracking.dispose();
  parts.annotations.dispose();
  return parts.controller.dispose();
}

async function dirtyEdit() {
  const editor = await vscode.window.showTextDocument(scratch());
  await editor.edit((e) => e.insert(new vscode.Position(0, 0), '// edit\n'));
  return editor;
}

const settle = (ms = 300) => new Promise((r) => setTimeout(r, ms));

suite('preview', () => {
  setup(async () => {
    fs.writeFileSync(scratch().fsPath, 'const a = 1;\nconst web = new Box("web");\nconst b = 2;\n');
    await settle(500);
  });
  teardown(async () => {
    await vscode.commands.executeCommand('workbench.action.revertAndCloseActiveEditor');
    await vscode.commands.executeCommand('workbench.action.closeAllEditors');
    fs.rmSync(scratch().fsPath, { force: true });
  });

  test('Save all and run saves first, captures the snapshot after, and clears the covered mark', async () => {
    const parts = await loaded({
      rows: [row('web', { projected: '$1.00' })],
      preview: { chooseDirty: async () => 'saveAll' },
    });
    await dirtyEdit();
    await settle(100);
    assert.deepEqual(reasons(parts.annotations), ['unsaved']);
    await parts.preview.runManual();
    assert.equal(vscode.workspace.textDocuments.find((d) => d.uri.toString() === scratch().toString()).isDirty, false);
    assert.equal(parts.runner.calls.overview, 2);
    assert.equal(parts.runner.calls.requests[1].stateOnly, false);
    await settle(200);
    assert.deepEqual(reasons(parts.annotations), []);
    await done(parts);
  });

  test('Run saved files leaves the dirty resources stale', async () => {
    const parts = await loaded({ preview: { chooseDirty: async () => 'runSaved' } });
    await dirtyEdit();
    await parts.preview.runManual();
    assert.equal(parts.runner.calls.overview, 2);
    await settle(200);
    assert.deepEqual(reasons(parts.annotations), ['unsaved']);
    await done(parts);
  });

  test('Cancel at the dirty prompt runs nothing', async () => {
    const parts = await loaded({ preview: { chooseDirty: async () => 'cancel' } });
    await dirtyEdit();
    await parts.preview.runManual();
    assert.equal(parts.runner.calls.overview, 1);
    await done(parts);
  });

  test('a failed save aborts the run and says which file', async () => {
    const parts = await loaded({ preview: { chooseDirty: async () => 'saveAll', saveDocument: async () => false } });
    await dirtyEdit();
    await parts.preview.runManual();
    assert.equal(parts.runner.calls.overview, 1);
    assert.match(parts.reports[0], /scratch\.js/);
    await done(parts);
  });

  test('a clean project runs without asking', async () => {
    let asked = false;
    const parts = await loaded({ preview: { chooseDirty: async () => ((asked = true), 'cancel') } });
    await parts.preview.runManual();
    assert.equal(asked, false);
    assert.equal(parts.runner.calls.overview, 2);
    await done(parts);
  });

  test('on-save is off by default and starts no process', async () => {
    const parts = await loaded();
    const doc = await vscode.workspace.openTextDocument(scratch());
    await parts.preview.runOnSave(doc);
    assert.equal(parts.runner.calls.overview, 1);
    await done(parts);
  });

  test('on-save, when on, prices the saved file and saves no other document', async () => {
    let asked = false;
    const parts = await loaded({ preview: { onSave: true, chooseDirty: async () => ((asked = true), 'saveAll') } });
    const other = await vscode.workspace.openTextDocument(fixtureUri('index.js'));
    const otherEditor = await vscode.window.showTextDocument(other);
    await otherEditor.edit((e) => e.insert(new vscode.Position(0, 0), '// other\n'));
    const editor = await vscode.window.showTextDocument(scratch());
    await editor.edit((e) => e.insert(new vscode.Position(0, 0), '// s\n'));
    await editor.document.save();
    await settle(1000); // the file watcher reports the save late on Windows; a run that already started would treat it as a new edit
    await parts.preview.runOnSave(editor.document);
    assert.equal(asked, false, 'on-save never offers to save');
    assert.equal(parts.runner.calls.overview, 2);
    assert.equal(other.isDirty, true, 'another dirty document was not saved');
    await settle(200);
    assert.deepEqual(reasons(parts.annotations), []);
    await vscode.window.showTextDocument(other);
    await vscode.commands.executeCommand('workbench.action.revertAndCloseActiveEditor');
    await done(parts);
  });

  test('an edit during the run keeps its mark even when saved before the result', async () => {
    const hang = [];
    const parts = await loaded({
      onOverview: (request, n) => {
        if (n === 1) return handle({ kind: 'ok', stdout: overviewJson([row('web')]) });
        const h = hangingHandle();
        hang.push(h);
        return h;
      },
      preview: { chooseDirty: async () => 'runSaved' },
    });
    const editor = await vscode.window.showTextDocument(scratch());
    const running = parts.preview.runManual();
    await settle(200);
    await editor.edit((e) => e.insert(new vscode.Position(0, 0), '// during\n'));
    await editor.document.save();
    hang[0].complete({ kind: 'ok', stdout: overviewJson([row('web')]) });
    await running;
    await settle(200);
    assert.notDeepEqual(reasons(parts.annotations), []);
    await done(parts);
  });

  test('cancelling the progress notification stops the process and keeps prior figures and marks', async () => {
    const hangs = [];
    const parts = await loaded({
      rows: [row('web', { projected: '$5.00' })],
      onOverview: (request, n) => {
        if (n === 1) return handle({ kind: 'ok', stdout: overviewJson([row('web', { projected: '$5.00' })]) });
        const h = hangingHandle();
        hangs.push(h);
        return h;
      },
    });
    const editor = await vscode.window.showTextDocument(scratch());
    await editor.edit((e) => e.insert(new vscode.Position(0, 0), '// x\n'));
    await editor.document.save();
    await settle(100);
    const running = parts.preview.runManual();
    await settle(200);
    assert.equal(hangs.length, 1);
    await parts.controller.cancelActive();
    await running;
    assert.equal(hangs[0].stops, 1);
    assert.equal(text(parts.annotations), '$5.00');
    assert.deepEqual(reasons(parts.annotations), ['edited']);
    await done(parts);
  });

  test('a failed preview keeps the previous figures and stale reasons and reports the CLI message', async () => {
    const parts = await loaded({
      onOverview: (request, n) =>
        n === 1
          ? handle({ kind: 'ok', stdout: overviewJson([row('web', { projected: '$5.00' })]) })
          : handle({ kind: 'failure', exitCode: 1, failure: { message: 'preview failed (preview_failed)', category: 'preview_failed' }, stderr: '' }),
    });
    const editor = await vscode.window.showTextDocument(scratch());
    await editor.edit((e) => e.insert(new vscode.Position(0, 0), '// x\n'));
    await editor.document.save();
    await settle(100);
    await parts.preview.runManual();
    assert.deepEqual(parts.reports, ['preview failed (preview_failed)']);
    assert.equal(text(parts.annotations), '$5.00');
    assert.deepEqual(reasons(parts.annotations), ['edited']);
    await done(parts);
  });
});

suite('passphrase prompt', () => {
  function apiReturning(values, seen) {
    return new Proxy(vscode, {
      get(target, prop) {
        if (prop === 'window') {
          return new Proxy(target.window, {
            get(win, key) {
              if (key === 'showInputBox') {
                return async (options, token) => {
                  seen.push({ options, token });
                  const value = values.shift();
                  if (value === 'WAIT') {
                    await new Promise((resolve) => token.onCancellationRequested(resolve));
                    return undefined;
                  }
                  return value;
                };
              }
              const v = win[key];
              return typeof v === 'function' ? v.bind(win) : v;
            },
          });
        }
        return target[prop];
      },
    });
  }

  const query = (retry = false) => ({ key: '["p","dev"]', retry, signal: new AbortController().signal });

  test('the box masks input, names the stack, and returns what was typed', async () => {
    const seen = [];
    const ask = createPassphraseAsker({ api: apiReturning(['hunter2'], seen), labelFor: () => 'dev' });
    assert.equal(await ask(query()), 'hunter2');
    assert.equal(seen[0].options.password, true);
    assert.equal(seen[0].options.ignoreFocusOut, true);
    assert.match(seen[0].options.title, /dev/);
  });

  test('an empty passphrase is returned as an empty string, not as a dismissal', async () => {
    const ask = createPassphraseAsker({ api: apiReturning([''], []), labelFor: () => 'dev' });
    assert.equal(await ask(query()), '');
  });

  test('dismissing the box returns null', async () => {
    const ask = createPassphraseAsker({ api: apiReturning([undefined], []), labelFor: () => 'dev' });
    assert.equal(await ask(query()), null);
  });

  test('a retry prompt says the passphrase was not accepted', async () => {
    const seen = [];
    const ask = createPassphraseAsker({ api: apiReturning(['x'], seen), labelFor: () => 'dev' });
    await ask(query(true));
    assert.match(seen[0].options.prompt, /not accepted/);
  });

  test('aborting the request closes the box', async () => {
    const seen = [];
    const controller = new AbortController();
    const ask = createPassphraseAsker({ api: apiReturning(['WAIT'], seen), labelFor: () => 'dev' });
    const pending = ask({ key: 'k', retry: false, signal: controller.signal });
    controller.abort();
    assert.equal(await pending, null);
  });

  test('wrong, then right: the scheduler asks again and the second answer runs', async () => {
    const answers = ['wrong', 'right'];
    const runner = fakeRunner({
      rows: [row('web')],
      onOverview: (request, n) =>
        n === 1
          ? handle({ kind: 'ok', stdout: overviewJson([row('web')]) })
          : request.passphrase === 'right'
            ? handle({ kind: 'ok', stdout: overviewJson([row('web')]) })
            : handle({ kind: 'failure', exitCode: 1, failure: { message: 'x (passphrase_required)', category: 'passphrase_required' }, stderr: '' }),
    });
    const parts = build({ runner, askPassphrase: async () => answers.shift() });
    await parts.controller.refresh();
    const outcome = await parts.scheduler.run({ key: parts.controller.currentContext.key, projectDir: fixtureUri('').fsPath, stack: 'dev', stateOnly: false });
    assert.equal(outcome.kind, 'result');
    assert.deepEqual(answers, []);
    parts.tracking.dispose();
    parts.annotations.dispose();
    await parts.controller.dispose();
  });
});
