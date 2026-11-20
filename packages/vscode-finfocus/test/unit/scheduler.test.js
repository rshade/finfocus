'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const { Scheduler } = require('../../src/scheduler');
const { FreshnessModel } = require('../../src/stale');

function overviewJson() {
  return JSON.stringify({
    metadata: { stackName: 'dev', generatedAt: '2026-11-17T00:00:00Z', isStateOnly: false },
    summary: { currency: 'USD', display: { projected: '$1.00', actual: '-', delta: '-' } },
    budgetDisplay: { healthDisplay: '', amountsDisplay: '', footerDisplay: '', details: [] },
    resources: [],
    errors: [],
  });
}

const ok = () => ({ kind: 'ok', stdout: overviewJson() });
const needsPassphrase = () => ({
  kind: 'failure',
  exitCode: 1,
  failure: { message: 'cannot read secrets (passphrase_required)', category: 'passphrase_required' },
  stderr: '',
});

/** A runner whose processes finish only when the test says so. */
function controlledRunner({ stopResult = { confirmed: true, forced: false, survivors: [] } } = {}) {
  const runs = [];
  const runner = {
    runs,
    running: 0,
    maxRunning: 0,
    overview(request) {
      let finish;
      const done = new Promise((resolve) => {
        finish = (outcome) => {
          if (run.active) {
            run.active = false;
            runner.running -= 1;
          }
          resolve(outcome);
        };
      });
      const run = {
        request,
        active: true,
        finish,
        stops: 0,
        handle: {
          pid: 100 + runs.length,
          signal: () => true,
          done,
          stop: async () => {
            run.stops += 1;
            const result = typeof stopResult === 'function' ? stopResult() : stopResult;
            if (result.confirmed) finish({ kind: 'signalled', signal: 'SIGTERM', stderr: '' });
            return result;
          },
        },
      };
      runner.running += 1;
      runner.maxRunning = Math.max(runner.maxRunning, runner.running);
      runs.push(run);
      return run.handle;
    },
  };
  return runner;
}

const tick = () => new Promise((resolve) => setImmediate(resolve));
const request = (extra = {}) => ({ key: 'k', projectDir: '/p', stack: 'dev', stateOnly: false, ...extra });

test('a result carries its request id, start time and the snapshot captured at the start', async () => {
  const model = new FreshnessModel();
  model.recordEdit('a.js', false);
  const snapshot = model.snapshot();
  const startedAt = new Date('2026-11-17T01:00:00Z');
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, now: () => startedAt });
  const pending = scheduler.run(request({ snapshot }));
  model.recordEdit('a.js', true);
  runner.runs[0].finish(ok());
  const outcome = await pending;
  assert.equal(outcome.kind, 'result');
  assert.equal(outcome.snapshot, snapshot, 'the snapshot is returned exactly as captured');
  assert.equal(outcome.snapshot.dirty.size, 0, 'edits after the start do not enter it');
  assert.equal(outcome.startedAt, startedAt);
  assert.equal(outcome.requestId, 1);
  assert.deepEqual(runner.runs[0].request, { projectDir: '/p', stack: 'dev', stateOnly: false, passphrase: null });
});

test('only one overview runs per context; the replacement waits for confirmed cleanup', async () => {
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner });
  const first = scheduler.run(request());
  const second = scheduler.run(request());
  await tick();
  assert.equal(runner.runs[0].stops, 1);
  assert.deepEqual(await first, { kind: 'superseded' });
  await tick();
  assert.equal(runner.runs.length, 2);
  runner.runs[1].finish(ok());
  assert.equal((await second).kind, 'result');
  assert.equal(runner.maxRunning, 1);
});

test('contexts run side by side', async () => {
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner });
  const a = scheduler.run(request({ key: 'a' }));
  const b = scheduler.run(request({ key: 'b' }));
  assert.equal(runner.runs.length, 2);
  runner.runs[0].finish(ok());
  runner.runs[1].finish(ok());
  assert.equal((await a).kind, 'result');
  assert.equal((await b).kind, 'result');
});

test('a result for a selection that is no longer current is dropped', async () => {
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner });
  let current = true;
  const pending = scheduler.run(request({ isCurrent: () => current }));
  current = false;
  runner.runs[0].finish(ok());
  assert.deepEqual(await pending, { kind: 'superseded' });
});

test('cancel invalidates the request first, stops the process, and drops its late output', async () => {
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner });
  const pending = scheduler.run(request());
  const stopped = await scheduler.cancel('k');
  assert.equal(stopped.confirmed, true);
  runner.runs[0].finish(ok());
  assert.deepEqual(await pending, { kind: 'superseded' });
  assert.equal(scheduler.stateOf('k'), 'idle');
});

test('a failed cleanup blocks replacement runs until a retry confirms it', async () => {
  let confirmed = false;
  const runner = controlledRunner({
    stopResult: () => ({ confirmed, forced: true, survivors: confirmed ? [] : [777] }),
  });
  const scheduler = new Scheduler({ runner });
  const first = scheduler.run(request());
  const replacement = await scheduler.run(request());
  assert.equal(replacement.kind, 'blocked');
  assert.match(replacement.message, /777/);
  assert.equal(scheduler.stateOf('k'), 'cleanupFailed');
  assert.equal((await scheduler.run(request())).kind, 'blocked');
  assert.equal(runner.runs.length, 1, 'no replacement process was started');
  assert.deepEqual(await Promise.race([first, tick().then(() => 'pending')]), 'pending');

  assert.equal((await scheduler.retryCleanup('k')).confirmed, false);
  confirmed = true;
  assert.equal((await scheduler.retryCleanup('k')).confirmed, true);
  const after = scheduler.run(request());
  await tick();
  runner.runs[1].finish(ok());
  assert.equal((await after).kind, 'result');
});

test('a failure and a refused start are reported, not thrown', async () => {
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner });
  const failed = scheduler.run(request());
  runner.runs[0].finish({ kind: 'failure', exitCode: 1, failure: { message: 'no stack (x)', category: 'x' }, stderr: '' });
  assert.deepEqual(await failed, { kind: 'failure', message: 'no stack (x)' });

  const { RunnerError } = require('../../src/runner');
  const untrusted = new Scheduler({
    runner: {
      overview() {
        throw new RunnerError('untrusted', 'finfocus does not run in an untrusted workspace');
      },
    },
  });
  assert.deepEqual(await untrusted.run(request()), {
    kind: 'failure',
    message: 'finfocus does not run in an untrusted workspace',
  });
});

test('a missing passphrase asks once per answer, passes an empty value, and retries the same request', async () => {
  const runner = controlledRunner();
  const asked = [];
  const scheduler = new Scheduler({
    runner,
    askPassphrase: async (query) => {
      asked.push({ key: query.key, retry: query.retry });
      return asked.length === 1 ? '' : 'right';
    },
  });
  const pending = scheduler.run(request());
  runner.runs[0].finish(needsPassphrase());
  await tick();
  assert.equal(runner.runs[1].request.passphrase, '', 'an empty passphrase is a real value');
  runner.runs[1].finish(needsPassphrase());
  await tick();
  assert.deepEqual(asked, [
    { key: 'k', retry: false },
    { key: 'k', retry: true },
  ]);
  assert.equal(runner.runs[2].request.passphrase, 'right');
  runner.runs[2].finish(ok());
  const outcome = await pending;
  assert.equal(outcome.kind, 'result');
  assert.equal(outcome.requestId, 1, 'the same request continues');
});

test('dismissing the prompt ends the run without a retry loop', async () => {
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, askPassphrase: async () => null });
  const pending = scheduler.run(request());
  runner.runs[0].finish(needsPassphrase());
  assert.deepEqual(await pending, { kind: 'dismissed' });
  assert.equal(runner.runs.length, 1);
  assert.equal(scheduler.stateOf('k'), 'idle');
});

test('a prompt for a superseded request is aborted and its late answer is discarded', async () => {
  const runner = controlledRunner();
  let release;
  let signal;
  const scheduler = new Scheduler({
    runner,
    askPassphrase: (query) => {
      signal = query.signal;
      return new Promise((resolve) => {
        release = resolve;
      });
    },
  });
  const first = scheduler.run(request());
  runner.runs[0].finish(needsPassphrase());
  await tick();
  assert.equal(scheduler.stateOf('k'), 'awaitingPassphrase');
  const second = scheduler.run(request());
  await tick();
  assert.equal(signal.aborted, true);
  release('late');
  assert.deepEqual(await first, { kind: 'superseded' });
  assert.equal(runner.runs.length, 2, 'the late answer started no process for the old request');
  assert.equal(runner.runs[1].request.passphrase, null, 'and was not stored');
  runner.runs[1].finish(ok());
  assert.equal((await second).kind, 'result');
});

test('credentials are per context and can be cleared', async () => {
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, askPassphrase: async () => 'secret' });
  const a = scheduler.run(request({ key: 'a' }));
  runner.runs[0].finish(needsPassphrase());
  await tick();
  runner.runs[1].finish(ok());
  await a;

  const b = scheduler.run(request({ key: 'b' }));
  assert.equal(runner.runs[2].request.passphrase, null, 'another context does not inherit it');
  runner.runs[2].finish(ok());
  await b;

  const again = scheduler.run(request({ key: 'a' }));
  assert.equal(runner.runs[3].request.passphrase, 'secret', 'the same context reuses it');
  runner.runs[3].finish(ok());
  await again;

  scheduler.clearCredentials('a');
  const cleared = scheduler.run(request({ key: 'a' }));
  assert.equal(runner.runs[4].request.passphrase, null);
  runner.runs[4].finish(ok());
  await cleared;
});

test('state changes are reported in order', async () => {
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner });
  const seen = [];
  scheduler.onState((key, state) => seen.push([key, state]));
  const pending = scheduler.run(request());
  runner.runs[0].finish(ok());
  await pending;
  assert.deepEqual(seen, [
    ['k', 'running'],
    ['k', 'idle'],
  ]);
});

test('closeAll cancels every context and reports an unconfirmed one', async () => {
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner });
  scheduler.run(request({ key: 'a' }));
  scheduler.run(request({ key: 'b' }));
  const results = await scheduler.closeAll();
  assert.deepEqual(results.map((r) => r.confirmed), [true, true]);
  assert.deepEqual(runner.runs.map((r) => r.stops), [1, 1]);
});

// ---- result reuse and completeness (FR-012a, FR-012b) ----

const fs = require('node:fs');
const path = require('node:path');
const { ResultCache, hashContents } = require('../../src/cache');

const fixtureText = (name) => fs.readFileSync(path.join(__dirname, '..', 'fixtures', 'completeness', name), 'utf8');
const COMPLETE = fixtureText('complete.json');
const PARTIAL = fixtureText('partial.json');
const STABLE = fixtureText('stable-gaps.json');
const okWith = (stdout) => ({ kind: 'ok', stdout });
const DNS = 'urn:pulumi:dev::shop::aws:ec2/instance:Instance::dns';

const signals = (extra = {}) => ({
  lastUpdate: '2026-11-18T03:41:07.000Z',
  resourceCount: 4,
  updateInProgress: false,
  stackConfigHash: hashContents('config'),
  projectConfigHash: '',
  finfocusVersion: '0.5.8',
  ...extra,
});

function cacheWithMemoryStore(now = new Date('2026-11-18T10:00:00Z')) {
  const files = new Map();
  const store = {
    async read(key) {
      return files.has(key) ? structuredClone(files.get(key)) : null;
    },
    async write(key, entry) {
      files.set(key, structuredClone(entry));
    },
    async remove(key) {
      files.delete(key);
    },
  };
  return { files, cache: new ResultCache({ store, ttlMs: 60 * 60 * 1000, now: () => now }) };
}

const cachedRequest = (extra = {}) => request({ stateOnly: true, cacheSignals: signals(), ...extra });

test('a cache hit starts no process and returns the stored result with its original start time', async () => {
  const { cache } = cacheWithMemoryStore();
  const producedAt = new Date('2026-11-18T09:30:00Z');
  await cache.save('k', signals(), { stdout: COMPLETE, producedAt, confirmedGaps: [] });
  const model = new FreshnessModel();
  model.recordEdit('a.js', false);
  const snapshot = model.snapshot();
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, cache });
  const outcome = await scheduler.run(cachedRequest({ snapshot }));
  assert.equal(runner.runs.length, 0);
  assert.equal(outcome.kind, 'result');
  assert.equal(outcome.fromCache, true);
  assert.deepEqual(outcome.startedAt, producedAt);
  assert.equal(outcome.snapshot, snapshot, 'edit marks are the request snapshot, as for a state-only refresh');
  assert.equal(outcome.completeness.kind, 'complete');
  assert.equal(scheduler.stateOf('k'), 'idle');
});

test('a hit with confirmed gaps reports them as confirmed', async () => {
  const { cache } = cacheWithMemoryStore();
  await cache.save('k', signals(), { stdout: STABLE, producedAt: new Date(), confirmedGaps: [DNS] });
  const outcome = await new Scheduler({ runner: controlledRunner(), cache }).run(cachedRequest());
  assert.equal(outcome.fromCache, true);
  assert.deepEqual(outcome.completeness, { kind: 'confirmed', gaps: [DNS], total: 4 });
});

test('a changed key signal runs finfocus and stores the new result', async () => {
  const { cache, files } = cacheWithMemoryStore();
  await cache.save('k', signals(), { stdout: COMPLETE, producedAt: new Date(), confirmedGaps: [] });
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, cache });
  const pending = scheduler.run(cachedRequest({ cacheSignals: signals({ lastUpdate: '2026-11-18T09:00:00.000Z' }) }));
  await tick();
  assert.equal(runner.runs.length, 1);
  runner.runs[0].finish(okWith(COMPLETE));
  const outcome = await pending;
  assert.equal(outcome.fromCache, false);
  assert.equal(files.get('k').lastUpdate, '2026-11-18T09:00:00.000Z');
});

test('an explicit refresh bypasses the cache and still refreshes the stored entry', async () => {
  const { cache, files } = cacheWithMemoryStore();
  await cache.save('k', signals(), { stdout: COMPLETE, producedAt: new Date('2026-11-18T01:00:00Z'), confirmedGaps: [] });
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, cache, now: () => new Date('2026-11-18T10:00:00Z') });
  const pending = scheduler.run(cachedRequest({ bypassCache: true }));
  await tick();
  assert.equal(runner.runs.length, 1);
  runner.runs[0].finish(okWith(COMPLETE));
  assert.equal((await pending).fromCache, false);
  assert.equal(files.get('k').producedAt, '2026-11-18T10:00:00.000Z');
});

test('a preview never reads or writes the cache', async () => {
  const { cache, files } = cacheWithMemoryStore();
  await cache.save('k', signals(), { stdout: COMPLETE, producedAt: new Date('2026-11-18T01:00:00Z'), confirmedGaps: [] });
  const before = structuredClone(files.get('k'));
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, cache });
  const pending = scheduler.run(cachedRequest({ stateOnly: false }));
  await tick();
  assert.equal(runner.runs.length, 1, 'a valid entry does not answer a preview');
  runner.runs[0].finish(okWith(COMPLETE));
  const outcome = await pending;
  assert.equal(outcome.completeness, null, 'completeness is judged on deployed-state loads only');
  assert.deepEqual(files.get('k'), before);
});

test('without cache signals the cache is neither read nor written', async () => {
  const { cache, files } = cacheWithMemoryStore();
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, cache });
  const pending = scheduler.run(request({ stateOnly: true }));
  runner.runs[0].finish(okWith(COMPLETE));
  await pending;
  assert.equal(files.size, 0);
});

test('a result with no unpriced rows is complete, needs one run, and is stored', async () => {
  const { cache, files } = cacheWithMemoryStore();
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, cache });
  const pending = scheduler.run(cachedRequest());
  await tick();
  runner.runs[0].finish(okWith(COMPLETE));
  const outcome = await pending;
  assert.equal(runner.runs.length, 1);
  assert.deepEqual(outcome.completeness, { kind: 'complete', gaps: [], total: 4 });
  assert.equal(files.get('k').stdout, COMPLETE);
});

test('unpriced rows with no baseline run once more; a clearing retry is complete and is the one kept', async () => {
  const { cache, files } = cacheWithMemoryStore();
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, cache });
  const pending = scheduler.run(cachedRequest());
  await tick();
  runner.runs[0].finish(okWith(PARTIAL));
  await tick();
  assert.equal(runner.runs.length, 2);
  runner.runs[1].finish(okWith(COMPLETE));
  const outcome = await pending;
  assert.equal(outcome.completeness.kind, 'complete');
  assert.equal(outcome.result.rows.length, 4);
  assert.equal(files.get('k').stdout, COMPLETE);
});

test('gap sets that differ across the two runs are incomplete, keep the fuller run, and are not stored', async () => {
  const { cache, files } = cacheWithMemoryStore();
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, cache });
  const pending = scheduler.run(cachedRequest());
  await tick();
  runner.runs[0].finish(okWith(PARTIAL));
  await tick();
  const lessPartial = JSON.stringify({
    ...JSON.parse(PARTIAL),
    resources: JSON.parse(PARTIAL).resources.map((row, i) =>
      i === 1 ? { ...row, display: { ...row.display, projected: '$5.00' } } : row,
    ),
  });
  runner.runs[1].finish(okWith(lessPartial));
  const outcome = await pending;
  assert.equal(outcome.completeness.kind, 'incomplete');
  assert.equal(outcome.completeness.gaps.length, 4);
  assert.equal(outcome.completeness.total, 6);
  assert.equal(files.size, 0, 'an incomplete result is never cached');
});

test('gaps both runs agree on are confirmed, stored with their baseline, and later loads need one run', async () => {
  const { cache, files } = cacheWithMemoryStore();
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, cache });
  const first = scheduler.run(cachedRequest());
  await tick();
  runner.runs[0].finish(okWith(STABLE));
  await tick();
  runner.runs[1].finish(okWith(STABLE));
  const outcome = await first;
  assert.equal(outcome.completeness.kind, 'confirmed');
  assert.deepEqual(files.get('k').confirmedGaps, [DNS]);

  const again = scheduler.run(cachedRequest({ bypassCache: true }));
  await tick();
  runner.runs[2].finish(okWith(STABLE));
  assert.equal((await again).completeness.kind, 'confirmed');
  assert.equal(runner.runs.length, 3, 'the baseline spares the retry');
});

test('the baseline survives a restart through the stored entry', async () => {
  const { cache } = cacheWithMemoryStore();
  await cache.save('k', signals({ lastUpdate: '2026-11-17T00:00:00.000Z' }), {
    stdout: STABLE, producedAt: new Date(), confirmedGaps: [DNS],
  });
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, cache });
  const pending = scheduler.run(cachedRequest());
  await tick();
  runner.runs[0].finish(okWith(STABLE));
  const outcome = await pending;
  assert.equal(runner.runs.length, 1);
  assert.equal(outcome.completeness.kind, 'confirmed');
});

test('a new gap beyond the baseline runs once more', async () => {
  const { cache } = cacheWithMemoryStore();
  await cache.save('k', signals({ lastUpdate: '2026-11-17T00:00:00.000Z' }), {
    stdout: STABLE, producedAt: new Date(), confirmedGaps: [DNS],
  });
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, cache });
  const pending = scheduler.run(cachedRequest());
  await tick();
  runner.runs[0].finish(okWith(PARTIAL));
  await tick();
  assert.equal(runner.runs.length, 2);
  runner.runs[1].finish(okWith(COMPLETE));
  await pending;
});

test('without a cache object the in-session baseline still spares repeat retries', async () => {
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner });
  const first = scheduler.run(request({ stateOnly: true }));
  runner.runs[0].finish(okWith(STABLE));
  await tick();
  runner.runs[1].finish(okWith(STABLE));
  assert.equal((await first).completeness.kind, 'confirmed');
  const second = scheduler.run(request({ stateOnly: true }));
  runner.runs[2].finish(okWith(STABLE));
  await second;
  assert.equal(runner.runs.length, 3);
});

test('the retry is a scheduler request: a newer request supersedes it and stops its process', async () => {
  const { cache } = cacheWithMemoryStore();
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, cache });
  const first = scheduler.run(cachedRequest());
  await tick();
  runner.runs[0].finish(okWith(PARTIAL));
  await tick();
  assert.equal(runner.runs.length, 2);
  const second = scheduler.run(cachedRequest({ bypassCache: true }));
  await tick();
  await tick();
  assert.equal(runner.runs[1].stops, 1);
  assert.deepEqual(await first, { kind: 'superseded' });
  await tick();
  runner.runs[2].finish(okWith(COMPLETE));
  assert.equal((await second).completeness.kind, 'complete');
});

test('cancel during the retry drops the result and writes nothing', async () => {
  const { cache, files } = cacheWithMemoryStore();
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, cache });
  const pending = scheduler.run(cachedRequest());
  await tick();
  runner.runs[0].finish(okWith(PARTIAL));
  await tick();
  await scheduler.cancel('k');
  runner.runs[1].finish(okWith(COMPLETE));
  assert.deepEqual(await pending, { kind: 'superseded' });
  assert.equal(files.size, 0);
});

test('a cache that fails to save does not fail the load', async () => {
  const cache = {
    enabled: true,
    lookup: async () => null,
    previous: async () => null,
    save: async () => {
      throw new Error('disk full');
    },
  };
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, cache });
  const pending = scheduler.run(cachedRequest());
  await tick();
  runner.runs[0].finish(okWith(COMPLETE));
  assert.equal((await pending).kind, 'result');
});

test('a failed run is a failure and is not cached', async () => {
  const { cache, files } = cacheWithMemoryStore();
  const runner = controlledRunner();
  const scheduler = new Scheduler({ runner, cache });
  const pending = scheduler.run(cachedRequest());
  await tick();
  runner.runs[0].finish({ kind: 'failure', exitCode: 1, failure: null, stderr: '' });
  assert.equal((await pending).kind, 'failure');
  assert.equal(files.size, 0);
});
