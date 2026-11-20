'use strict';

const { describeOutcome, RunnerError } = require('../runner');
const { checkVersion } = require('../version');

const INSTALL_GUIDANCE =
  'finfocus was not found. Install it (https://github.com/rshade/finfocus/releases) and put it on PATH, or set finfocus.path to its location.';

const PLUGIN_GUIDANCE = 'Run `finfocus plugin list` to check them, or `finfocus plugin install <name>` to add one.';

/**
 * Keeps the CLI's own message and adds the plugin hint when that message is
 * about plugins. Core gives plugin failures no separate category, so the text
 * is the only signal; nothing else from the run is read.
 *
 * @param {string} message
 * @returns {string}
 */
function withGuidance(message) {
  return /plugin/i.test(message) ? `${message} ${PLUGIN_GUIDANCE}` : message;
}

/**
 * Decides once per settings generation whether the finfocus executable can be
 * used. A failure is reported once and remembered, so a missing or too old
 * binary is not started again on every refresh until a setting changes.
 */
class Readiness {
  /**
   * @param {{
   *   runner: {version: () => {done: Promise<import('../runner').Outcome>}},
   *   minFinfocusVersion: string,
   *   report: (message: string) => void,
   *   log: (line: string) => void,
   * }} deps
   */
  constructor(deps) {
    this.runner = deps.runner;
    this.minFinfocusVersion = deps.minFinfocusVersion;
    this.report = deps.report;
    this.log = deps.log;
    /** @type {Promise<boolean> | null} */
    this.pending = null;
    /** The first line of `finfocus --version` once a check passed, for the cache key. */
    this.version = '';
  }

  /** @returns {Promise<boolean>} */
  ready() {
    if (this.pending === null) this.pending = this.check();
    return this.pending;
  }

  /** Allows one new attempt after `finfocus.path` or a similar setting changes. */
  settingsChanged() {
    this.pending = null;
    this.version = '';
  }

  /** @returns {Promise<boolean>} */
  async check() {
    try {
      const outcome = await this.runner.version().done;
      if (outcome.kind === 'spawn_error') return this.fail(INSTALL_GUIDANCE);
      if (outcome.kind !== 'ok') return this.fail(describeOutcome(outcome));
      const check = checkVersion(outcome.stdout, this.minFinfocusVersion);
      if (check.kind === 'too_old') {
        return this.fail(`FinFocus ${check.required} or newer is required; found ${check.found}. Upgrade finfocus or set finfocus.path.`);
      }
      if (check.kind === 'unknown') this.log(`Could not read the finfocus version from "${check.output}"; continuing.`);
      this.version = outcome.stdout.trim().split('\n')[0];
      return true;
    } catch (error) {
      if (error instanceof RunnerError) return this.fail(error.message);
      throw error;
    }
  }

  /**
   * @param {string} message
   * @returns {false}
   */
  fail(message) {
    this.report(message);
    return false;
  }
}

module.exports = { Readiness, withGuidance };
