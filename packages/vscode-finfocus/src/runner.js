'use strict';

const childProcess = require('node:child_process');
const crypto = require('node:crypto');
const { parseFailure } = require('./result');
const { createTree, OWNER_VAR } = require('./processTree');
const { createShutdown } = require('./shutdown');

const PASSPHRASE_VAR = 'PULUMI_CONFIG_PASSPHRASE';

/**
 * @typedef {import('./result').RunFailure} RunFailure
 * @typedef {{kind: 'ok', stdout: string}
 *   | {kind: 'failure', exitCode: number, failure: RunFailure | null, stderr: string}
 *   | {kind: 'signalled', signal: string, stderr: string}
 *   | {kind: 'spawn_error', code: string, stderr: string}} Outcome
 * @typedef {import('./shutdown').StopResult} StopResult
 * @typedef {{
 *   pid: number | undefined,
 *   signal: (signal: NodeJS.Signals) => boolean,
 *   done: Promise<Outcome>,
 *   stop: () => Promise<StopResult>,
 * }} RunHandle
 */

class RunnerError extends Error {
  /**
   * @param {'untrusted'} code
   * @param {string} message
   */
  constructor(code, message) {
    super(message);
    this.name = 'RunnerError';
    this.code = code;
  }
}

/**
 * @param {{stack: string, stateOnly: boolean}} request
 * @returns {string[]}
 */
function overviewArgs({ stack, stateOnly }) {
  const args = ['overview', '--output', 'json', '--format', 'json', '--yes'];
  if (stateOnly) args.push('--state-only');
  args.push('--stack', stack);
  return args;
}

/**
 * The inherited environment plus the passphrase entered for this context.
 * An empty string is a real value; null means none was entered.
 *
 * @param {NodeJS.ProcessEnv} base
 * @param {string | null} passphrase
 * @param {string | null} [owner] the token that marks every descendant of this run
 * @returns {NodeJS.ProcessEnv}
 */
function buildEnv(base, passphrase, owner = null) {
  const env = { ...base };
  if (passphrase !== null) env[PASSPHRASE_VAR] = passphrase;
  if (owner !== null) env[OWNER_VAR] = owner;
  return env;
}

/**
 * @param {string} text
 * @param {string[]} secrets
 * @returns {string}
 */
function redact(text, secrets) {
  let out = text;
  for (const secret of secrets) {
    if (secret !== '') out = out.split(secret).join('[redacted]');
  }
  return out;
}

/**
 * The one-line text shown for an outcome. A reported failure passes through as
 * finfocus wrote it; nothing else here carries a command line or environment
 * value.
 *
 * @param {Outcome} outcome
 * @returns {string}
 */
function describeOutcome(outcome) {
  switch (outcome.kind) {
    case 'ok':
      return '';
    case 'failure':
      return outcome.failure?.message ?? `finfocus failed (exit ${outcome.exitCode}); see output channel`;
    case 'signalled':
      return `finfocus was stopped by ${outcome.signal}`;
    case 'spawn_error':
      return `Could not start finfocus (${outcome.code})`;
  }
}

/**
 * Starts the finfocus and pulumi executables in the workspace host. Every
 * start goes through one trust check and uses an argument array with no shell.
 */
class Runner {
  /**
   * @param {{
   *   isTrusted: () => boolean,
   *   spawn?: typeof childProcess.spawn,
   *   env?: NodeJS.ProcessEnv,
   *   platform?: NodeJS.Platform,
   *   finfocusPath?: () => string,
   *   pulumiPath?: () => string,
   *   shutdown?: ReturnType<typeof createShutdown>,
   * }} options
   */
  constructor(options) {
    this.isTrusted = options.isTrusted;
    this.spawn = options.spawn ?? childProcess.spawn;
    this.env = options.env ?? process.env;
    this.platform = options.platform ?? process.platform;
    this.finfocusPath = options.finfocusPath ?? (() => '');
    this.pulumiPath = options.pulumiPath ?? (() => '');
    this.shutdown = options.shutdown ?? createShutdown({ tree: createTree({ platform: this.platform }), platform: this.platform });
  }

  /** @returns {RunHandle} */
  version() {
    return this.start(this.executable(this.finfocusPath, 'finfocus'), ['--version'], process.cwd(), null, true);
  }

  /**
   * @param {string} projectDir
   * @returns {RunHandle}
   */
  discoverStacks(projectDir) {
    return this.start(this.executable(this.pulumiPath, 'pulumi'), ['stack', 'ls', '--json'], projectDir, null, false);
  }

  /**
   * @param {{projectDir: string, stack: string, stateOnly: boolean, passphrase: string | null}} request
   * @returns {RunHandle}
   */
  overview({ projectDir, stack, stateOnly, passphrase }) {
    return this.start(
      this.executable(this.finfocusPath, 'finfocus'),
      overviewArgs({ stack, stateOnly }),
      projectDir,
      passphrase,
      false,
    );
  }

  /**
   * @param {() => string} configured
   * @param {string} fallback
   * @returns {string}
   */
  executable(configured, fallback) {
    const value = configured().trim();
    return value === '' ? fallback : value;
  }

  /**
   * @param {string} command
   * @param {string[]} args
   * @param {string} cwd
   * @param {string | null} passphrase
   * @param {boolean} trimStdout
   * @returns {RunHandle}
   */
  start(command, args, cwd, passphrase, trimStdout) {
    if (!this.isTrusted()) {
      throw new RunnerError('untrusted', 'finfocus does not run in an untrusted workspace');
    }
    const owner = this.platform === 'darwin' ? crypto.randomBytes(16).toString('hex') : null;
    const child = this.spawn(command, args, {
      cwd,
      env: buildEnv(this.env, passphrase, owner),
      shell: false,
      detached: this.platform !== 'win32',
      windowsHide: true,
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    const secrets = passphrase === null ? [] : [passphrase];
    const closed = collect(child, secrets, trimStdout);
    const pid = child.pid;
    const done = closed.then(async (outcome) => {
      await this.shutdown.reap(pid, owner);
      return outcome;
    });
    /** @type {Promise<StopResult> | null} */
    let stopping = null;
    return {
      pid,
      signal: (signal) => child.kill(signal),
      done,
      stop: () => {
        stopping ??= this.shutdown.stop({ pid, exited: closed, terminate: () => child.kill('SIGTERM'), owner }).then((result) => {
          if (!result.confirmed) stopping = null;
          return result;
        });
        return stopping;
      },
    };
  }
}

/**
 * @param {import('node:child_process').ChildProcess} child
 * @param {string[]} secrets
 * @param {boolean} trimStdout
 * @returns {Promise<Outcome>}
 */
function collect(child, secrets, trimStdout) {
  return new Promise((resolve) => {
    let stdout = '';
    let stderr = '';
    let settled = false;
    const settle = (/** @type {Outcome} */ outcome) => {
      if (settled) return;
      settled = true;
      resolve(outcome);
    };
    child.stdout?.setEncoding('utf8');
    child.stderr?.setEncoding('utf8');
    child.stdout?.on('data', (chunk) => {
      stdout += chunk;
    });
    child.stderr?.on('data', (chunk) => {
      stderr += chunk;
    });
    child.on('error', (error) => {
      settle({ kind: 'spawn_error', code: /** @type {NodeJS.ErrnoException} */ (error).code ?? 'UNKNOWN', stderr: '' });
    });
    child.on('close', (code, signal) => {
      const safeStderr = redact(stderr, secrets);
      if (signal !== null) {
        settle({ kind: 'signalled', signal, stderr: safeStderr });
      } else if (code === 0) {
        settle({ kind: 'ok', stdout: trimStdout ? stdout.trim() : stdout });
      } else {
        const failure = parseFailure(stderr);
        settle({
          kind: 'failure',
          exitCode: code ?? 1,
          failure: failure === null ? null : { ...failure, message: redact(failure.message, secrets) },
          stderr: safeStderr,
        });
      }
    });
  });
}

module.exports = { Runner, RunnerError, overviewArgs, buildEnv, redact, describeOutcome };
