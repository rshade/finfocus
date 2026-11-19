'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const vscode = require('vscode');
const { build, fakeRunner, apiWith, fixtureUri, overviewJson, handle } = require('./support');
const { ResultCache } = require('../../src/cache');
const { FileCacheStore } = require('../../src/editor/cacheStore');
const { DashboardTree } = require('../../src/editor/treeView');

const FRESH = '2999-01-01T00:00:00Z';
const WEB = { name: 'web', modifiedAt: FRESH, location: { file: 'index.js', line: 3, column: 13 } };
const WORKER = { name: 'worker', modifiedAt: FRESH, location: { file: 'index.js', line: 4, column: 16 }, projected: '$8.00' };
const GAP_A = { name: 'gap-a', modifiedAt: FRESH, projected: '-' };
const GAP_B = { name: 'gap-b', modifiedAt: FRESH, projected: '-' };

const stacks = (lastUpdate = '2026-11-18T03:41:07.000Z') => [
  { name: 'dev', current: true, lastUpdate, updateInProgress: false, resourceCount: 2 },
];

function tempDir() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'finfocus-editor-cache-'));
}

function entryFiles(dir) {
  const results = path.join(dir, 'results');
  return fs.existsSync(results) ? fs.readdirSync(results).filter((name) => name.endsWith('.json')) : [];
}

/** One extension-host lifetime: a fresh runner, scheduler and controller over a shared store directory. */
function session({ dir, rows = [WEB, WORKER], stackList = stacks(), clock, api = vscode, onOverview = null, trustedFor = () => true }) {
  const runner = fakeRunner({ rows, stacks: stackList, onOverview });
  const store = new FileCacheStore(dir);
  const reads = { count: 0 };
  const read = store.read.bind(store);
  store.read = async (key) => {
    reads.count += 1;
    return read(key);
  };
  const cache = new ResultCache({ store, ttlMs: 60 * 60 * 1000, now: clock, isTrusted: trustedFor });
  const parts = build({ runner, api, cache });
  const tree = new DashboardTree({ api: vscode, tracking: parts.tracking });
  const dispose = () => {
    tree.dispose();
    parts.annotations.dispose();
    return parts.controller.dispose();
  };
  return { ...parts, runner, tree, reads, dispose };
}

suite('result cache in the editor', () => {
  let dir;
  setup(() => {
    dir = tempDir();
  });
  teardown(async () => {
    await vscode.commands.executeCommand('workbench.action.closeAllEditors');
    fs.rmSync(dir, { recursive: true, force: true });
  });

  test('a reopened window paints from the stored entry and starts no finfocus overview', async () => {
    const first = session({ dir });
    await first.controller.refresh();
    assert.equal(first.runner.calls.overview, 1);
    assert.equal(entryFiles(dir).length, 1);
    await first.dispose();

    const second = session({ dir });
    await second.controller.refresh();
    assert.equal(second.runner.calls.overview, 0, 'no overview process on a hit');
    assert.equal(second.runner.calls.stacks, 1, 'discovery still supplies the change signals');
    const view = second.tracking.view();
    assert.deepEqual(view.result.rows.map((r) => r.display.projected).sort(), ['$61.32', '$8.00']);
    const [group] = await second.tree.getChildren();
    assert.equal((await second.tree.getChildren(group)).length, 2);
    await second.dispose();
  });

  test('a new stack update invalidates the entry', async () => {
    const first = session({ dir });
    await first.controller.refresh();
    await first.dispose();

    const second = session({ dir, stackList: stacks('2026-11-18T09:00:00.000Z') });
    await second.controller.refresh();
    assert.equal(second.runner.calls.overview, 1);
    await second.dispose();
  });

  test('an update in progress is neither served from nor written to the cache', async () => {
    const running = [{ name: 'dev', current: true, lastUpdate: '2026-11-18T03:41:07.000Z', updateInProgress: true }];
    const first = session({ dir, stackList: running });
    await first.controller.refresh();
    assert.equal(entryFiles(dir).length, 0);
    await first.dispose();
  });

  test('a changed stack settings file invalidates the entry', async () => {
    const settingsFile = fixtureUri('Pulumi.dev.yaml');
    const write = (text) => vscode.workspace.fs.writeFile(settingsFile, Buffer.from(text));
    try {
      await write('config:\n  fixture:size: small\n');
      const first = session({ dir });
      await first.controller.refresh();
      await first.dispose();

      await write('config:\n  fixture:size: large\n');
      const second = session({ dir });
      await second.controller.refresh();
      assert.equal(second.runner.calls.overview, 1);
      await second.dispose();
    } finally {
      await vscode.workspace.fs.delete(settingsFile, { useTrash: false }).then(undefined, () => undefined);
    }
  });

  test('an expired entry runs finfocus again', async () => {
    let now = new Date('2026-11-18T10:00:00Z');
    const first = session({ dir, clock: () => now });
    await first.controller.refresh();
    await first.dispose();

    now = new Date('2026-11-18T11:30:00Z');
    const second = session({ dir, clock: () => now });
    await second.controller.refresh();
    assert.equal(second.runner.calls.overview, 1);
    await second.dispose();
  });

  test('an explicit refresh bypasses a valid entry and stores the new result', async () => {
    const first = session({ dir });
    await first.controller.refresh();
    await first.dispose();

    const second = session({ dir, rows: [WEB] });
    await second.controller.refresh({ bypassCache: true });
    assert.equal(second.runner.calls.overview, 1);
    assert.equal(second.tracking.view().result.rows.length, 1);
    await second.dispose();

    const third = session({ dir });
    await third.controller.refresh();
    assert.equal(third.runner.calls.overview, 0);
    assert.equal(third.tracking.view().result.rows.length, 1, 'the bypassing run refreshed the stored entry');
    await third.dispose();
  });

  test('an untrusted workspace never reads or shows an entry', async () => {
    const first = session({ dir });
    await first.controller.refresh();
    await first.dispose();

    const untrusted = session({ dir, api: apiWith({ trusted: false }), trustedFor: () => false });
    await untrusted.controller.refresh();
    assert.deepEqual(untrusted.runner.calls, { version: 0, stacks: 0, overview: 0, requests: [] });
    assert.equal(untrusted.reads.count, 0);
    assert.equal(untrusted.tracking.view(), null);
    await untrusted.dispose();
  });

  test('a corrupt entry is deleted, treated as a miss, and replaced', async () => {
    const first = session({ dir });
    await first.controller.refresh();
    await first.dispose();
    const [name] = entryFiles(dir);
    fs.writeFileSync(path.join(dir, 'results', name), '{"version":1,"stdout":"{broken');

    const second = session({ dir });
    await second.controller.refresh();
    assert.equal(second.runner.calls.overview, 1);
    assert.equal(entryFiles(dir).length, 1);
    await second.dispose();

    const third = session({ dir });
    await third.controller.refresh();
    assert.equal(third.runner.calls.overview, 0, 'the replacement is valid');
    await third.dispose();
  });

  test('an incomplete result shows the marker and leaves no entry on disk', async () => {
    const outcomes = [[GAP_A, GAP_B, WEB], [GAP_A, WEB, WORKER]];
    const s = session({
      dir,
      onOverview: (_request, call) => handle({ kind: 'ok', stdout: overviewJson(outcomes[call - 1]) }),
    });
    await s.controller.refresh();
    assert.equal(s.runner.calls.overview, 2, 'differing gaps run once more');
    assert.equal(entryFiles(dir).length, 0);
    const [notice] = await s.tree.getChildren();
    assert.equal(notice.contextValue, 'notice');
    assert.match(notice.label, /^Incomplete: 1 of 3 resources unpriced$/);
    await s.dispose();
  });

  test('gaps both runs agree on are stored, and the next window needs no second run', async () => {
    const rows = [GAP_A, WEB];
    const first = session({ dir, rows });
    await first.controller.refresh();
    assert.equal(first.runner.calls.overview, 2);
    assert.equal(entryFiles(dir).length, 1);
    const [group] = await first.tree.getChildren();
    assert.equal(group.contextValue, 'group', 'confirmed gaps are not called incomplete');
    await first.dispose();

    const second = session({ dir, rows });
    await second.controller.refresh();
    assert.equal(second.runner.calls.overview, 0);
    await second.dispose();
  });
});
