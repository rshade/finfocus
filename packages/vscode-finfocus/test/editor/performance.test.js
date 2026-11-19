'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const { monitorEventLoopDelay } = require('node:perf_hooks');
const vscode = require('vscode');
const { build, fakeRunner, fixtureUri } = require('./support');
const { SOURCE_FILE, RESOURCE_COUNT, buildSource, buildRows } = require('../fixtures/performance/fixture');

const WARM_LOADS = 5;
const WARM_LOAD_LIMIT_MS = 20000;
const PAUSE_LIMIT_MS = 100;
const WORKLOAD_SECONDS = Number(process.env.FINFOCUS_PERF_SECONDS ?? 60);
const TYPING_INTERVAL_MS = 40;

const results = { loads: [], workload: {} };

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

function sourcePath() {
  return fixtureUri(SOURCE_FILE).fsPath;
}

function dispose({ annotations, controller, tracking }) {
  tracking.dispose();
  annotations.dispose();
  controller.dispose();
}

/**
 * Types into the document for the whole duration and samples event-loop delay,
 * the stall any other extension would see from this host.
 */
async function runWorkload(editor, seconds) {
  const histogram = monitorEventLoopDelay({ resolution: 5 });
  histogram.enable();
  const editLatencies = [];
  const end = performance.now() + seconds * 1000;
  let edits = 0;
  while (performance.now() < end) {
    const line = 1 + (edits % RESOURCE_COUNT);
    const started = performance.now();
    await editor.edit((e) => e.insert(new vscode.Position(line, 0), 'x'));
    editLatencies.push(performance.now() - started);
    edits += 1;
    await sleep(TYPING_INTERVAL_MS);
  }
  histogram.disable();
  const sorted = [...editLatencies].sort((a, b) => a - b);
  return {
    edits,
    maxLoopDelayMs: histogram.max / 1e6,
    p99LoopDelayMs: histogram.percentile(99) / 1e6,
    maxEditMs: sorted[sorted.length - 1],
    p99EditMs: sorted[Math.min(sorted.length - 1, Math.floor(sorted.length * 0.99))],
  };
}

suite('performance with 300 resources', function () {
  this.timeout((WORKLOAD_SECONDS * 2 + 60) * 1000);

  setup(async () => {
    fs.writeFileSync(sourcePath(), buildSource());
    await sleep(500);
  });
  teardown(async () => {
    await vscode.commands.executeCommand('workbench.action.revertAndCloseActiveEditor');
    await vscode.commands.executeCommand('workbench.action.closeAllEditors');
    fs.unlinkSync(sourcePath());
  });
  suiteTeardown(() => {
    console.log(`PERF_RESULT ${JSON.stringify(results)}`);
    if (process.env.FINFOCUS_PERF_OUT) {
      const host = { cpu: os.cpus()[0]?.model, cpus: os.cpus().length, platform: os.platform(), arch: os.arch(), vscode: vscode.version };
      fs.writeFileSync(process.env.FINFOCUS_PERF_OUT, JSON.stringify({ ...results, host }, null, 2));
    }
  });

  test(`five warm loads each finish within ${WARM_LOAD_LIMIT_MS / 1000} s and show the CLI strings`, async () => {
    const expected = new Map(buildRows().map((r) => [r.location.line, `$${r.projected.slice(1)}`]));
    for (let n = 0; n < WARM_LOADS; n += 1) {
      const runner = fakeRunner({ rows: buildRows() });
      const parts = build({ runner });
      const started = performance.now();
      await parts.controller.refresh();
      const placed = parts.annotations.annotationsFor(fixtureUri(SOURCE_FILE));
      const elapsed = performance.now() - started;
      results.loads.push(Math.round(elapsed));
      assert.equal(placed.length, RESOURCE_COUNT);
      assert.ok(elapsed <= WARM_LOAD_LIMIT_MS, `load ${n} took ${elapsed} ms`);
      for (const annotation of placed) assert.equal(annotation.text, expected.get(annotation.line), `line ${annotation.line}`);
      dispose(parts);
    }
  });

  test('typing for the workload adds no pause over the limit and starts no pricing', async () => {
    const editor = await vscode.window.showTextDocument(fixtureUri(SOURCE_FILE));
    const baseline = await runWorkload(editor, WORKLOAD_SECONDS);
    await vscode.commands.executeCommand('workbench.action.files.revert');

    const runner = fakeRunner({ rows: buildRows() });
    const parts = build({ runner });
    await parts.controller.refresh();
    const withExtension = await runWorkload(editor, WORKLOAD_SECONDS);
    dispose(parts);

    results.workload = { seconds: WORKLOAD_SECONDS, baseline, withExtension };
    assert.equal(runner.calls.overview, 1, 'edits must not start a pricing run');
    const added = withExtension.maxLoopDelayMs - baseline.maxLoopDelayMs;
    assert.ok(added <= PAUSE_LIMIT_MS, `extension added ${added.toFixed(1)} ms to the longest pause`);
  });
});
