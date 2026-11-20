'use strict';

/**
 * Bounded shutdown of one owned finfocus process and everything it started.
 * Graceful first (SIGTERM to finfocus, which closes its plugins), then every
 * member of its session is killed, and cleanup is confirmed by looking again.
 * Confirmation also runs after a clean exit, so a descendant a clean exit left
 * behind is swept.
 *
 * @typedef {{confirmed: boolean, forced: boolean, survivors: number[]}} StopResult
 * @typedef {ReturnType<typeof import('./processTree').createTree>} Tree
 */

const DEFAULT_GRACE_MS = 5000;
const DEFAULT_CONFIRM_MS = 2000;
const POLL_MS = 25;

/**
 * @param {{
 *   tree: Tree,
 *   platform?: NodeJS.Platform,
 *   graceMs?: number,
 *   confirmMs?: number,
 * }} options
 */
function createShutdown(options) {
  const { tree } = options;
  const platform = options.platform ?? process.platform;
  const graceMs = options.graceMs ?? DEFAULT_GRACE_MS;
  const confirmMs = options.confirmMs ?? DEFAULT_CONFIRM_MS;

  /** @param {number} ms */
  const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

  /**
   * @param {Promise<unknown>} promise
   * @param {number} ms
   * @returns {Promise<boolean>} true when the promise settled first
   */
  async function within(promise, ms) {
    /** @type {NodeJS.Timeout | undefined} */
    let timer;
    const timeout = new Promise((resolve) => {
      timer = setTimeout(() => resolve(false), ms);
    });
    const settled = await Promise.race([promise.then(() => true, () => true), timeout]);
    clearTimeout(timer);
    return settled;
  }

  /**
   * Kills every member of the session and waits, up to the confirmation
   * deadline, until none is left.
   *
   * @param {number} sid
   * @param {string | null} owner
   * @returns {Promise<{confirmed: boolean, killed: boolean, survivors: number[]}>}
   */
  async function sweep(sid, owner) {
    let members = await tree.members(sid, owner);
    if (members.length === 0) return { confirmed: true, killed: false, survivors: [] };
    const deadline = Date.now() + confirmMs;
    while (members.length > 0 && Date.now() < deadline) {
      tree.killAll(members);
      await sleep(POLL_MS);
      members = await tree.members(sid, owner);
    }
    return { confirmed: members.length === 0, killed: true, survivors: members };
  }

  return {
    /**
     * Runs after every exit.
     *
     * @param {number | undefined} pid
     * @param {string | null} [owner]
     * @returns {Promise<StopResult>}
     */
    async reap(pid, owner = null) {
      if (pid === undefined || platform === 'win32') return { confirmed: true, forced: false, survivors: [] };
      const result = await sweep(pid, owner);
      return { confirmed: result.confirmed, forced: result.killed, survivors: result.survivors };
    },

    /**
     * @param {{pid: number | undefined, exited: Promise<unknown>, terminate: () => void, owner?: string | null}} target
     * @returns {Promise<StopResult>}
     */
    async stop({ pid, exited, terminate, owner = null }) {
      if (pid === undefined) return { confirmed: true, forced: false, survivors: [] };
      if (platform === 'win32') {
        // taskkill finds nothing when finfocus already died on its own; core's
        // kill-on-close job object has then ended every descendant, so the
        // exit of the process is what confirms the stop.
        await tree.taskkill(pid);
        const gone = await within(exited, confirmMs);
        return { confirmed: gone, forced: true, survivors: gone ? [] : [pid] };
      }
      terminate();
      const graceful = await within(exited, graceMs);
      const result = await sweep(pid, owner);
      return { confirmed: result.confirmed, forced: !graceful || result.killed, survivors: result.survivors };
    },
  };
}

module.exports = { createShutdown, DEFAULT_GRACE_MS, DEFAULT_CONFIRM_MS };
