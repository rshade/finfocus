'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const { EventEmitter } = require('node:events');
const { PassThrough } = require('node:stream');
const { Runner, RunnerError, overviewArgs, buildEnv, redact, describeOutcome } = require('../../src/runner');

function fakeSpawn() {
  const calls = [];
  const spawn = (command, args, options) => {
    const child = new EventEmitter();
    child.stdout = new PassThrough();
    child.stderr = new PassThrough();
    child.pid = 4242;
    child.killed = false;
    child.kill = (signal) => {
      child.killed = true;
      child.lastSignal = signal;
      return true;
    };
    calls.push({ command, args, options, child });
    return child;
  };
  return { spawn, calls };
}

function finish(child, { stdout = '', stderr = '', code = 0, signal = null } = {}) {
  child.stdout.end(stdout);
  child.stderr.end(stderr);
  child.emit('close', code, signal);
}

function runner(overrides = {}) {
  const fake = fakeSpawn();
  const instance = new Runner({
    spawn: fake.spawn,
    isTrusted: () => true,
    env: { PATH: '/bin', HOME: '/home/u' },
    platform: 'linux',
    shutdown: { reap: async () => ({ confirmed: true, forced: false, survivors: [] }), stop: async () => ({ confirmed: true, forced: false, survivors: [] }) },
    ...overrides,
  });
  return { runner: instance, calls: fake.calls };
}

test('state-only and preview argument arrays match the contract', () => {
  assert.deepEqual(overviewArgs({ stack: 'dev', stateOnly: true }), [
    'overview', '--output', 'json', '--format', 'json', '--yes', '--state-only', '--stack', 'dev',
  ]);
  assert.deepEqual(overviewArgs({ stack: 'acme/shop/dev', stateOnly: false }), [
    'overview', '--output', 'json', '--format', 'json', '--yes', '--stack', 'acme/shop/dev',
  ]);
});

test('overview always passes the explicit stack, even a hostile-looking name, as one argument', () => {
  const { runner: r, calls } = runner();
  r.overview({ projectDir: '/work/a', stack: 'dev; rm -rf /', stateOnly: true, passphrase: null });
  const args = calls[0].args;
  assert.equal(args[args.indexOf('--stack') + 1], 'dev; rm -rf /');
  assert.equal(calls[0].options.shell, false);
});

test('overview runs in the project directory with no shell and an owned session on Unix', () => {
  const { runner: r, calls } = runner();
  r.overview({ projectDir: '/work/a', stack: 'dev', stateOnly: true, passphrase: null });
  assert.equal(calls[0].command, 'finfocus');
  assert.equal(calls[0].options.cwd, '/work/a');
  assert.equal(calls[0].options.detached, true);
  assert.equal(calls[0].options.windowsHide, true);
  assert.deepEqual(calls[0].options.stdio, ['ignore', 'pipe', 'pipe']);
});

test('on Windows the process is not detached, so taskkill can own its tree', () => {
  const { runner: r, calls } = runner({ platform: 'win32' });
  r.overview({ projectDir: 'C:\\work\\a', stack: 'dev', stateOnly: true, passphrase: null });
  assert.equal(calls[0].options.detached, false);
});

test('finfocus.path overrides the executable', () => {
  const { runner: r, calls } = runner({ finfocusPath: () => '/opt/finfocus' });
  r.overview({ projectDir: '/work/a', stack: 'dev', stateOnly: true, passphrase: null });
  assert.equal(calls[0].command, '/opt/finfocus');
});

test('an empty finfocus.path falls back to the PATH name', () => {
  const { runner: r, calls } = runner({ finfocusPath: () => '  ' });
  r.overview({ projectDir: '/work/a', stack: 'dev', stateOnly: true, passphrase: null });
  assert.equal(calls[0].command, 'finfocus');
});

test('the environment is inherited and the passphrase is absent unless entered', () => {
  const env = { PATH: '/bin', PULUMI_CONFIG_PASSPHRASE: 'from-shell' };
  assert.deepEqual(buildEnv(env, null), env);
  assert.equal(buildEnv(env, 'typed').PULUMI_CONFIG_PASSPHRASE, 'typed');
});

test('an empty passphrase is passed as an empty value, not dropped', () => {
  const built = buildEnv({ PATH: '/bin', PULUMI_CONFIG_PASSPHRASE: 'from-shell' }, '');
  assert.equal(Object.hasOwn(built, 'PULUMI_CONFIG_PASSPHRASE'), true);
  assert.equal(built.PULUMI_CONFIG_PASSPHRASE, '');
});

test('building the environment never mutates the inherited one', () => {
  const env = { PATH: '/bin' };
  buildEnv(env, 'secret');
  assert.deepEqual(env, { PATH: '/bin' });
});

test('each context passes only its own passphrase', () => {
  const { runner: r, calls } = runner();
  r.overview({ projectDir: '/work/a', stack: 'dev', stateOnly: true, passphrase: 'one' });
  r.overview({ projectDir: '/work/b', stack: 'dev', stateOnly: true, passphrase: null });
  assert.equal(calls[0].options.env.PULUMI_CONFIG_PASSPHRASE, 'one');
  assert.equal(Object.hasOwn(calls[1].options.env, 'PULUMI_CONFIG_PASSPHRASE'), false);
});

test('an untrusted workspace never spawns, for every kind of run', async () => {
  const { runner: r, calls } = runner({ isTrusted: () => false });
  assert.throws(() => r.overview({ projectDir: '/w', stack: 'dev', stateOnly: true, passphrase: null }), (e) => e instanceof RunnerError && e.code === 'untrusted');
  assert.throws(() => r.version(), (e) => e instanceof RunnerError && e.code === 'untrusted');
  assert.throws(() => r.discoverStacks('/w'), (e) => e instanceof RunnerError && e.code === 'untrusted');
  assert.equal(calls.length, 0);
});

test('trust is checked at each call, not captured at construction', () => {
  let trusted = false;
  const { runner: r, calls } = runner({ isTrusted: () => trusted });
  assert.throws(() => r.version(), RunnerError);
  trusted = true;
  r.version();
  assert.equal(calls.length, 1);
});

test('version runs finfocus --version and returns trimmed stdout', async () => {
  const { runner: r, calls } = runner();
  const handle = r.version();
  assert.deepEqual(calls[0].args, ['--version']);
  finish(calls[0].child, { stdout: 'finfocus version 0.5.8\n' });
  const outcome = await handle.done;
  assert.deepEqual(outcome, { kind: 'ok', stdout: 'finfocus version 0.5.8' });
});

test('discovery runs pulumi stack ls --json in the project and never selects a stack', async () => {
  const { runner: r, calls } = runner({ pulumiPath: () => '/usr/bin/pulumi' });
  const handle = r.discoverStacks('/work/a');
  assert.equal(calls[0].command, '/usr/bin/pulumi');
  assert.deepEqual(calls[0].args, ['stack', 'ls', '--json']);
  assert.equal(calls[0].options.cwd, '/work/a');
  assert.ok(!calls[0].args.includes('select'));
  finish(calls[0].child, { stdout: '[]' });
  assert.equal((await handle.done).kind, 'ok');
});

test('a clean exit returns stdout', async () => {
  const { runner: r, calls } = runner();
  const handle = r.overview({ projectDir: '/w', stack: 'dev', stateOnly: true, passphrase: null });
  finish(calls[0].child, { stdout: '{"ok":true}' });
  assert.deepEqual(await handle.done, { kind: 'ok', stdout: '{"ok":true}' });
});

test('a non-zero exit with an envelope reports the parsed failure', async () => {
  const { runner: r, calls } = runner();
  const handle = r.overview({ projectDir: '/w', stack: 'dev', stateOnly: true, passphrase: null });
  const envelope = JSON.stringify({ error_code: 'INTERNAL', message: 'locked (passphrase_required)', version: '0.5.8' });
  finish(calls[0].child, { stderr: envelope, code: 1 });
  const outcome = await handle.done;
  assert.equal(outcome.kind, 'failure');
  assert.equal(outcome.exitCode, 1);
  assert.equal(outcome.failure.category, 'passphrase_required');
});

test('a non-zero exit with no envelope keeps stderr for the output channel only', async () => {
  const { runner: r, calls } = runner();
  const handle = r.overview({ projectDir: '/w', stack: 'dev', stateOnly: true, passphrase: null });
  finish(calls[0].child, { stderr: 'panic: boom\n', code: 2 });
  const outcome = await handle.done;
  assert.equal(outcome.kind, 'failure');
  assert.equal(outcome.failure, null);
  assert.equal(outcome.stderr, 'panic: boom\n');
  assert.equal(describeOutcome(outcome), 'finfocus failed (exit 2); see output channel');
});

test('a failure message passes through exactly what finfocus reported', () => {
  const outcome = { kind: 'failure', exitCode: 1, stderr: '', failure: { message: 'no stack (config)', category: 'config', version: '', errorCode: '' } };
  assert.equal(describeOutcome(outcome), 'no stack (config)');
});

test('a spawn error is reported without the command line', async () => {
  const { runner: r, calls } = runner();
  const handle = r.overview({ projectDir: '/w', stack: 'dev', stateOnly: true, passphrase: 'hunter2' });
  const error = Object.assign(new Error('spawn finfocus ENOENT'), { code: 'ENOENT' });
  calls[0].child.emit('error', error);
  const outcome = await handle.done;
  assert.equal(outcome.kind, 'spawn_error');
  assert.equal(outcome.code, 'ENOENT');
  assert.doesNotMatch(describeOutcome(outcome), /overview|--stack|hunter2/);
  assert.match(describeOutcome(outcome), /ENOENT/);
});

test('a signal exit is its own outcome', async () => {
  const { runner: r, calls } = runner();
  const handle = r.overview({ projectDir: '/w', stack: 'dev', stateOnly: true, passphrase: null });
  finish(calls[0].child, { code: null, signal: 'SIGTERM' });
  assert.deepEqual(await handle.done, { kind: 'signalled', signal: 'SIGTERM', stderr: '' });
});

test('the handle exposes the real child pid and can signal it', () => {
  const { runner: r, calls } = runner();
  const handle = r.overview({ projectDir: '/w', stack: 'dev', stateOnly: true, passphrase: null });
  assert.equal(handle.pid, 4242);
  handle.signal('SIGTERM');
  assert.equal(calls[0].child.lastSignal, 'SIGTERM');
});

test('output is collected across chunks and the promise settles once', async () => {
  const { runner: r, calls } = runner();
  const handle = r.overview({ projectDir: '/w', stack: 'dev', stateOnly: true, passphrase: null });
  calls[0].child.stdout.write('{"a"');
  calls[0].child.stdout.write(':1}');
  finish(calls[0].child, {});
  assert.deepEqual(await handle.done, { kind: 'ok', stdout: '{"a":1}' });
  calls[0].child.emit('error', new Error('late'));
});

test('redact removes every non-empty secret and leaves empty ones alone', () => {
  assert.equal(redact('token hunter2 and hunter2', ['hunter2']), 'token [redacted] and [redacted]');
  assert.equal(redact('unchanged', ['']), 'unchanged');
  assert.equal(redact('a.b', ['a.b']), '[redacted]');
});

test('stderr in a failure outcome has the passphrase redacted', async () => {
  const { runner: r, calls } = runner();
  const handle = r.overview({ projectDir: '/w', stack: 'dev', stateOnly: true, passphrase: 'hunter2' });
  finish(calls[0].child, { stderr: 'bad passphrase hunter2\n', code: 3 });
  const outcome = await handle.done;
  assert.equal(outcome.stderr, 'bad passphrase [redacted]\n');
});

test('a passphrase echoed inside an envelope message is redacted too', async () => {
  const { runner: r, calls } = runner();
  const handle = r.overview({ projectDir: '/w', stack: 'dev', stateOnly: true, passphrase: 'hunter2' });
  finish(calls[0].child, { stderr: JSON.stringify({ message: 'wrong hunter2 (passphrase_required)' }), code: 1 });
  const outcome = await handle.done;
  assert.equal(outcome.failure.message, 'wrong [redacted] (passphrase_required)');
  assert.equal(outcome.failure.category, 'passphrase_required');
});
