'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const inspector = require('node:inspector');
const os = require('node:os');
const path = require('node:path');
const { monitorEventLoopDelay } = require('node:perf_hooks');
const vscode = require('vscode');
const { Runner } = require('../../src/runner');
const { Scheduler } = require('../../src/scheduler');
const { build, fixtureUri } = require('./support');
const { SOURCE_FILE, RESOURCE_COUNT, buildSource } = require('../fixtures/performance/fixture');

const binary = process.env.FINFOCUS_BINARY;
const project = process.env.FINFOCUS_FIXTURE_PROJECT;
const stack = process.env.FINFOCUS_FIXTURE_STACK || 'dev';
const WORKLOAD_SECONDS = Number(process.env.FINFOCUS_PERF_SECONDS ?? 30);
const PAUSE_LIMIT_MS = 100;
const TYPING_INTERVAL_MS = 40;
const REPEAT_LOADS = 4;
const SRC_DIR = path.resolve(__dirname, '..', '..', 'src') + path.sep;

const results = { stateOnlyLoadsMs: [], preview: {}, workload: {}, profile: {} };

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const sourcePath = () => fixtureUri(SOURCE_FILE).fsPath;
const post = (session, method, params) => new Promise((resolve, reject) => session.post(method, params, (e, r) => (e ? reject(e) : resolve(r))));

function realRunner() {
  return new Runner({ isTrusted: () => true, finfocusPath: () => binary, env: { ...process.env, PULUMI_CONFIG_PASSPHRASE: '' } });
}

function newScheduler() {
  return new Scheduler({ runner: realRunner(), askPassphrase: async () => '' });
}

async function typeFor(editor, seconds) {
  const histogram = monitorEventLoopDelay({ resolution: 5 });
  histogram.enable();
  const latencies = [];
  const end = performance.now() + seconds * 1000;
  let edits = 0;
  while (performance.now() < end) {
    const started = performance.now();
    await editor.edit((e) => e.insert(new vscode.Position(1 + (edits % RESOURCE_COUNT), 0), 'x'));
    latencies.push(performance.now() - started);
    edits += 1;
    await sleep(TYPING_INTERVAL_MS);
  }
  histogram.disable();
  latencies.sort((a, b) => a - b);
  return {
    edits,
    maxLoopDelayMs: histogram.max / 1e6,
    p99LoopDelayMs: histogram.percentile(99) / 1e6,
    maxEditMs: latencies[latencies.length - 1],
    p99EditMs: latencies[Math.min(latencies.length - 1, Math.floor(latencies.length * 0.99))],
  };
}

/** Self time per function from a CPU profile, split into this extension's source and everything else. */
function summarize(profile) {
  const byId = new Map(profile.nodes.map((n) => [n.id, n]));
  const selfMs = new Map();
  profile.samples.forEach((id, i) => selfMs.set(id, (selfMs.get(id) ?? 0) + (profile.timeDeltas[i] ?? 0) / 1000));
  let total = 0;
  let extension = 0;
  const functions = new Map();
  for (const [id, ms] of selfMs) {
    const { callFrame } = byId.get(id);
    total += ms;
    const file = callFrame.url.startsWith('file://') ? decodeURIComponent(callFrame.url.slice(7)) : callFrame.url;
    if (file.startsWith(SRC_DIR)) {
      extension += ms;
      const key = `${path.relative(SRC_DIR, file)}:${callFrame.functionName || '(anonymous)'}`;
      functions.set(key, (functions.get(key) ?? 0) + ms);
    }
  }
  const top = [...functions].sort((a, b) => b[1] - a[1]).slice(0, 8).map(([name, ms]) => ({ name, selfMs: Math.round(ms * 10) / 10 }));
  return { totalSampledMs: Math.round(total), extensionSelfMs: Math.round(extension * 10) / 10, topExtensionFunctions: top };
}

suite('performance against the real CLI (T082)', function () {
  this.timeout((WORKLOAD_SECONDS * 2 + 240) * 1000);

  suiteSetup(function () {
    if (!binary || !project || !fs.existsSync(project)) this.skip();
  });
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
    console.log(`PERF_REAL_RESULT ${JSON.stringify(results)}`);
    if (process.env.FINFOCUS_PERF_OUT) {
      const host = { cpu: os.cpus()[0]?.model, cpus: os.cpus().length, platform: os.platform(), arch: os.arch(), vscode: vscode.version };
      fs.writeFileSync(process.env.FINFOCUS_PERF_OUT, JSON.stringify({ ...results, host }, null, 2));
    }
  });

  test('the first state-only load is reported apart from the repeats', async () => {
    for (let n = 0; n <= REPEAT_LOADS; n += 1) {
      const started = performance.now();
      const outcome = await newScheduler().run({ key: `perf-${n}`, projectDir: project, stack, stateOnly: true });
      results.stateOnlyLoadsMs.push(Math.round(performance.now() - started));
      assert.equal(outcome.kind, 'result');
    }
  });

  test('typing during a real preview adds no pause over the limit', async () => {
    const editor = await vscode.window.showTextDocument(fixtureUri(SOURCE_FILE));
    const baseline = await typeFor(editor, WORKLOAD_SECONDS);
    await vscode.commands.executeCommand('workbench.action.files.revert');

    const parts = build({ runner: realRunner() });
    const session = new inspector.Session();
    session.connect();
    await post(session, 'Profiler.enable');
    await post(session, 'Profiler.start');

    const previewStarted = performance.now();
    const preview = newScheduler()
      .run({ key: 'perf-preview', projectDir: project, stack, stateOnly: false })
      .then((outcome) => {
        results.preview = { kind: outcome.kind, ms: Math.round(performance.now() - previewStarted), rows: outcome.result?.rows.length ?? null, message: outcome.kind === 'failure' ? outcome.message : null };
        if (outcome.kind === 'result') parts.annotations.show(vscode.Uri.file(project), outcome.result.rows);
        return outcome;
      });
    const withExtension = await typeFor(editor, WORKLOAD_SECONDS);
    const outcome = await preview;

    const { profile } = await post(session, 'Profiler.stop');
    session.disconnect();
    if (process.env.FINFOCUS_PERF_PROFILE) fs.writeFileSync(process.env.FINFOCUS_PERF_PROFILE, JSON.stringify(profile));
    results.profile = summarize(profile);
    parts.tracking.dispose();
    parts.annotations.dispose();
    parts.controller.dispose();

    results.workload = { seconds: WORKLOAD_SECONDS, baseline, withExtension, previewRanWithinWindow: results.preview.ms <= WORKLOAD_SECONDS * 1000 };
    assert.equal(outcome.kind, 'result', 'the real preview must complete');
    assert.ok(results.workload.previewRanWithinWindow, 'the typing window must cover the whole preview');
    const added = withExtension.maxLoopDelayMs - baseline.maxLoopDelayMs;
    assert.ok(added <= PAUSE_LIMIT_MS, `extension added ${added.toFixed(1)} ms to the longest pause`);
  });
});
