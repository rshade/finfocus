'use strict';

const assert = require('node:assert/strict');
const vscode = require('vscode');
const { build, fakeRunner, fixtureUri } = require('./support');
const { DashboardTree } = require('../../src/editor/treeView');
const { DashboardStatusBar } = require('../../src/editor/statusBar');
const { SummaryView } = require('../../src/editor/summaryView');
const { ResourceActions } = require('../../src/editor/resourceActions');

const FRESH = '2999-01-01T00:00:00Z';
const WEB = { name: 'web', modifiedAt: FRESH, location: { file: 'index.js', line: 3, column: 13 } };
const WORKER = { name: 'worker', modifiedAt: FRESH, location: { file: 'index.js', line: 4, column: 16 }, projected: '$8.00' };
const ORPHAN = { name: 'orphan', modifiedAt: FRESH, projected: '$2.00' };

async function closeAll() {
  await vscode.commands.executeCommand('workbench.action.closeAllEditors');
}

function views(rows) {
  const parts = build({ runner: fakeRunner({ rows }) });
  const shown = [];
  const actions = new ResourceActions({ api: vscode, tracking: parts.tracking, present: (lines) => shown.push(lines), notify: (m) => shown.push(m) });
  const tree = new DashboardTree({ api: vscode, tracking: parts.tracking });
  const status = new DashboardStatusBar({ api: vscode, tracking: parts.tracking });
  const dispose = () => {
    tree.dispose();
    status.dispose();
    parts.annotations.dispose();
    parts.controller.dispose();
  };
  return { ...parts, actions, tree, status, shown, dispose };
}

suite('dashboard views', () => {
  teardown(closeAll);

  test('the tree lists every resource under its type and the status bar shows the CLI total', async () => {
    const v = views([WEB, WORKER, ORPHAN]);
    await v.controller.refresh();
    const [group] = await v.tree.getChildren();
    assert.equal(group.contextValue, 'group');
    const items = await v.tree.getChildren(group);
    assert.deepEqual(
      items.map((i) => [i.label, i.description]),
      [
        ['orphan', '$2.00 · freshness unknown'],
        ['web', '$61.32'],
        ['worker', '$8.00'],
      ],
    );
    assert.match(v.status.item.text, /\$1\.00/);
    assert.match(v.status.item.text, /no budget/i);
    assert.equal(v.status.item.command, 'finfocus.revealDashboard');
    v.dispose();
  });

  test('tree items carry an accessible label with the figures, location and stale reason', async () => {
    const v = views([WEB]);
    await v.controller.refresh();
    const editor = await vscode.window.showTextDocument(fixtureUri('index.js'));
    await editor.edit((edit) => edit.insert(new vscode.Position(0, 0), '// edit\n'));
    const [group] = await v.tree.getChildren();
    const [item] = await v.tree.getChildren(group);
    assert.match(item.accessibilityInformation.label, /web.*Projected \$61\.32.*index\.js:3.*Stale/s);
    v.dispose();
  });

  test('the tree and status bar change when the stale set changes', async () => {
    const v = views([WEB]);
    await v.controller.refresh();
    let fired = 0;
    v.tree.onDidChangeTreeData(() => (fired += 1));
    const editor = await vscode.window.showTextDocument(fixtureUri('index.js'));
    await editor.edit((edit) => edit.insert(new vscode.Position(0, 0), '// edit\n'));
    assert.ok(fired > 0);
    assert.match(v.status.item.tooltip, /1 stale/);
    v.dispose();
  });

  test('activating a resource opens its declaration; one without a location says so', async () => {
    const v = views([WEB, ORPHAN]);
    await v.controller.refresh();
    const web = v.tracking.view().result.rows.find((r) => r.urn.endsWith('::web'));
    const orphan = v.tracking.view().result.rows.find((r) => r.urn.endsWith('::orphan'));
    assert.equal(await v.actions.open(web.urn), true);
    assert.equal(vscode.window.activeTextEditor.selection.active.line, 2);
    assert.equal(await v.actions.open(orphan.urn), false);
    assert.match(v.shown.at(-1), /no recorded location/i);
    v.dispose();
  });

  test('the keyboard details command lists every hover field and the reason', async () => {
    const v = views([WEB]);
    await v.controller.refresh();
    const [row] = v.tracking.view().result.rows;
    await v.actions.details(row.urn);
    const lines = v.shown.at(-1);
    assert.ok(lines.includes('Projected $61.32 · active'));
    assert.ok(lines.includes('Declared at index.js:3'));
    v.dispose();
  });

  test('a summary card that fails to render leaves the tree and status bar intact', async () => {
    const v = views([WEB]);
    await v.controller.refresh();
    const broken = {
      webview: {
        options: {},
        cspSource: 'x',
        asWebviewUri: (u) => u,
        onDidReceiveMessage: () => ({ dispose() {} }),
        postMessage: async () => true,
        set html(_value) {
          throw new Error('webview unavailable');
        },
      },
      onDidDispose: () => ({ dispose() {} }),
      onDidChangeVisibility: () => ({ dispose() {} }),
      show() {},
    };
    const card = new SummaryView({ api: vscode, tracking: v.tracking, actions: v.actions, extensionUri: fixtureUri('') });
    assert.doesNotThrow(() => card.resolveWebviewView(broken));
    assert.equal(card.failed, true);
    const [group] = await v.tree.getChildren();
    assert.equal((await v.tree.getChildren(group)).length, 1);
    assert.match(v.status.item.text, /\$1\.00/);
    card.dispose();
    v.dispose();
  });

  test('the card html is built from escaped text and a strict content security policy', async () => {
    const v = views([{ name: '<img src=x onerror=alert(1)>', location: { file: 'index.js', line: 3 } }]);
    await v.controller.refresh();
    let html = '';
    const posted = [];
    const view = {
      webview: {
        options: {},
        cspSource: 'vscode-webview:',
        asWebviewUri: (u) => u,
        onDidReceiveMessage: () => ({ dispose() {} }),
        postMessage: async (m) => posted.push(m),
        set html(value) {
          html = value;
        },
      },
      onDidDispose: () => ({ dispose() {} }),
      onDidChangeVisibility: () => ({ dispose() {} }),
      show() {},
    };
    const card = new SummaryView({ api: vscode, tracking: v.tracking, actions: v.actions, extensionUri: vscode.Uri.file(require('node:path').resolve(__dirname, '../..')) });
    card.resolveWebviewView(view);
    assert.match(html, /Content-Security-Policy/);
    assert.match(html, /default-src 'none'/);
    assert.doesNotMatch(html, /<img src=x/);
    assert.doesNotMatch(html, /https?:\/\//);
    assert.ok(posted.some((m) => m.type === 'state' && JSON.stringify(m.state).includes('<img src=x onerror=alert(1)>')));
    assert.equal(card.failed, false);
    card.dispose();
    v.dispose();
  });
});

suite('incomplete results', () => {
  teardown(closeAll);

  async function withCompleteness(completeness) {
    const v = views([WEB, WORKER]);
    await v.controller.refresh();
    const { key, project, stack } = v.controller.currentContext;
    const { result } = v.tracking.view();
    await v.tracking.setResult({ key, project, stack, result, basis: 'state', completeness });
    return v;
  }

  test('the tree leads with a warning notice, then the groups', async () => {
    const v = await withCompleteness({ kind: 'incomplete', gaps: ['a', 'b'], total: 4 });
    const [notice, group] = await v.tree.getChildren();
    assert.equal(notice.label, 'Incomplete: 2 of 4 resources unpriced');
    assert.equal(notice.contextValue, 'notice');
    assert.equal(notice.command, undefined);
    assert.match(notice.accessibilityInformation.label, /incomplete/i);
    assert.equal(group.contextValue, 'group');
    v.dispose();
  });

  test('the status bar names it in words and in its accessible label', async () => {
    const v = await withCompleteness({ kind: 'incomplete', gaps: ['a', 'b'], total: 4 });
    assert.match(v.status.item.text, /incomplete/i);
    assert.match(v.status.item.accessibilityInformation.label, /incomplete: 2 of 4 resources unpriced/i);
    assert.match(String(v.status.item.tooltip), /2 of 4 resources unpriced/);
    v.dispose();
  });

  test('a complete or confirmed result changes neither the tree nor the status text', async () => {
    for (const completeness of [null, { kind: 'complete', gaps: [], total: 2 }, { kind: 'confirmed', gaps: ['a'], total: 2 }]) {
      const v = await withCompleteness(completeness);
      const [first] = await v.tree.getChildren();
      assert.equal(first.contextValue, 'group');
      assert.doesNotMatch(v.status.item.text, /incomplete/i);
      v.dispose();
    }
  });

  test('the card state carries the marker and its explanation', async () => {
    const v = await withCompleteness({ kind: 'incomplete', gaps: ['a', 'b'], total: 4 });
    const posted = [];
    const view = {
      webview: {
        options: {},
        cspSource: 'vscode-webview:',
        asWebviewUri: (u) => u,
        onDidReceiveMessage: () => ({ dispose() {} }),
        postMessage: async (m) => posted.push(m),
        set html(_) {},
      },
      onDidChangeVisibility: () => ({ dispose() {} }),
      onDidDispose: () => ({ dispose() {} }),
      visible: true,
    };
    const card = new SummaryView({ api: vscode, tracking: v.tracking, actions: v.actions, extensionUri: vscode.Uri.file(require('node:path').resolve(__dirname, '../..')) });
    card.resolveWebviewView(view);
    const state = posted.filter((m) => m.type === 'state').at(-1).state;
    assert.equal(state.completeness.kind, 'incomplete');
    assert.match(state.completeness.text, /2 of 4/);
    assert.ok(state.topFive.length > 0, 'the top five fills from projectedCost.monthlyCost (ISS-002)');
    card.dispose();
    v.dispose();
  });
});
