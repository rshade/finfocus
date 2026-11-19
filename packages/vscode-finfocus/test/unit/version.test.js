'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const { checkVersion } = require('../../src/version');

test('equal and newer versions pass, including a git-describe build suffix', () => {
  assert.deepEqual(checkVersion('finfocus version v0.5.8', '0.5.8'), { kind: 'ok' });
  assert.deepEqual(checkVersion('finfocus version v0.5.8-10-ga3d8d714-dirty', '0.5.8'), { kind: 'ok' });
  assert.deepEqual(checkVersion('finfocus version v1.0.0', '0.5.8'), { kind: 'ok' });
  assert.deepEqual(checkVersion('finfocus version v0.6.0', '0.5.8'), { kind: 'ok' });
});

test('an older version names both versions', () => {
  assert.deepEqual(checkVersion('finfocus version v0.5.7', '0.5.8'), {
    kind: 'too_old',
    found: '0.5.7',
    required: '0.5.8',
  });
  assert.equal(checkVersion('v0.4.99', '0.5.0').kind, 'too_old');
});

test('a dev or unparsable version is allowed but reported', () => {
  assert.deepEqual(checkVersion('finfocus version dev\n', '0.5.8'), { kind: 'unknown', output: 'finfocus version dev' });
  assert.equal(checkVersion('', '0.5.8').kind, 'unknown');
});

const loadReadiness = () => require('../../src/editor/readiness').Readiness;

function versionRunner(outcomes) {
  const calls = { version: 0 };
  const runner = {
    calls,
    version: () => {
      const outcome = outcomes[Math.min(calls.version, outcomes.length - 1)];
      calls.version += 1;
      return { done: Promise.resolve(outcome) };
    },
  };
  return runner;
}

function readinessFor(outcomes, minFinfocusVersion = '0.5.8') {
  const Readiness = loadReadiness();
  const runner = versionRunner(outcomes);
  const reports = [];
  const logs = [];
  const readiness = new Readiness({
    runner,
    minFinfocusVersion,
    report: (message) => reports.push(message),
    log: (line) => logs.push(line),
  });
  return { readiness, runner, reports, logs };
}

const OK = { kind: 'ok', stdout: 'finfocus version v0.5.8' };
const MISSING = { kind: 'spawn_error', code: 'ENOENT', stderr: '' };

test('a supported version is ready, silent, and checked once', async () => {
  const { readiness, runner, reports } = readinessFor([OK]);
  assert.equal(await readiness.ready(), true);
  assert.equal(await readiness.ready(), true);
  assert.equal(runner.calls.version, 1);
  assert.deepEqual(reports, []);
});

test('concurrent callers share one version run', async () => {
  const { readiness, runner } = readinessFor([OK]);
  const answers = await Promise.all([readiness.ready(), readiness.ready(), readiness.ready()]);
  assert.deepEqual(answers, [true, true, true]);
  assert.equal(runner.calls.version, 1);
});

test('a missing binary reports install and path guidance once and is not retried', async () => {
  const { readiness, runner, reports } = readinessFor([MISSING]);
  assert.equal(await readiness.ready(), false);
  assert.equal(await readiness.ready(), false);
  assert.equal(await readiness.ready(), false);
  assert.equal(runner.calls.version, 1);
  assert.equal(reports.length, 1);
  assert.match(reports[0], /install/i);
  assert.match(reports[0], /finfocus\.path/);
});

test('a settings change allows one new attempt', async () => {
  const { readiness, runner, reports } = readinessFor([MISSING, OK]);
  assert.equal(await readiness.ready(), false);
  readiness.settingsChanged();
  assert.equal(await readiness.ready(), true);
  assert.equal(await readiness.ready(), true);
  assert.equal(runner.calls.version, 2);
  assert.equal(reports.length, 1);
});

test('a settings change that still fails reports again, once', async () => {
  const { readiness, runner, reports } = readinessFor([MISSING]);
  await readiness.ready();
  readiness.settingsChanged();
  await readiness.ready();
  await readiness.ready();
  assert.equal(runner.calls.version, 2);
  assert.equal(reports.length, 2);
});

test('an old version names the found and the required version and is not retried', async () => {
  const { readiness, runner, reports } = readinessFor([{ kind: 'ok', stdout: 'finfocus version v0.5.7' }]);
  assert.equal(await readiness.ready(), false);
  assert.equal(await readiness.ready(), false);
  assert.equal(runner.calls.version, 1);
  assert.equal(reports.length, 1);
  assert.match(reports[0], /0\.5\.8/);
  assert.match(reports[0], /0\.5\.7/);
});

test('a dev or unparsable version is ready with a log line and no warning', async () => {
  const { readiness, reports, logs } = readinessFor([{ kind: 'ok', stdout: 'finfocus version dev\n' }]);
  assert.equal(await readiness.ready(), true);
  assert.deepEqual(reports, []);
  assert.equal(logs.length, 1);
  assert.match(logs[0], /finfocus version dev/);
});

test('a version command that exits non-zero reports the sanitized reason once', async () => {
  const failed = { kind: 'failure', exitCode: 1, failure: { message: 'plugin directory unreadable', category: '', version: '', errorCode: '' }, stderr: '' };
  const { readiness, runner, reports } = readinessFor([failed]);
  assert.equal(await readiness.ready(), false);
  assert.equal(await readiness.ready(), false);
  assert.equal(runner.calls.version, 1);
  assert.deepEqual(reports, ['plugin directory unreadable']);
});

test('reports never carry the stderr text or a command line', async () => {
  const leaky = { kind: 'failure', exitCode: 2, failure: null, stderr: 'token=SECRET-VALUE /usr/bin/finfocus --version' };
  const { readiness, reports, logs } = readinessFor([leaky]);
  await readiness.ready();
  for (const text of [...reports, ...logs]) {
    assert.doesNotMatch(text, /SECRET-VALUE/);
    assert.doesNotMatch(text, /--version/);
  }
});
