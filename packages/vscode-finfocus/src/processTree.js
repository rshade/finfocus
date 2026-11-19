'use strict';

const fs = require('node:fs');
const childProcess = require('node:child_process');

/**
 * Reads the owned process set. On Unix the unit is the session the extension
 * created for finfocus (`spawn` with `detached: true`): plugins call `setpgid`
 * but stay in it. Linux reads `/proc` directly, so a minimal container without
 * procps works; other Unix systems use `pgrep -s`. Windows has no session, so
 * its tree is ended through `taskkill /T`.
 */

/**
 * The session id from the contents of `/proc/<pid>/stat`, or null for a
 * zombie or a line that cannot be read. The command name sits in parentheses
 * and may itself contain spaces or parentheses, so fields count from the last
 * closing one: state, ppid, pgrp, session.
 *
 * @param {string} stat
 * @returns {number | null}
 */
function sessionOfStat(stat) {
  const end = stat.lastIndexOf(')');
  if (end < 0) return null;
  const fields = stat.slice(end + 1).trim().split(/\s+/);
  if (fields.length < 4 || fields[0] === 'Z' || fields[0] === 'X') return null;
  const session = Number(fields[3]);
  return Number.isInteger(session) ? session : null;
}

/**
 * @param {{
 *   platform?: NodeJS.Platform,
 *   fsApi?: Pick<typeof fs.promises, 'readdir' | 'readFile'>,
 *   execFile?: typeof childProcess.execFile,
 *   kill?: (pid: number, signal: NodeJS.Signals) => void,
 * }} [options]
 */
function createTree(options = {}) {
  const platform = options.platform ?? process.platform;
  const fsApi = options.fsApi ?? fs.promises;
  const execFile = options.execFile ?? childProcess.execFile;
  const kill = options.kill ?? ((pid, signal) => process.kill(pid, signal));

  /**
   * @param {string} command
   * @param {string[]} args
   * @returns {Promise<{code: number, stdout: string}>}
   */
  function run(command, args) {
    return new Promise((resolve) => {
      execFile(command, args, { windowsHide: true }, (error, stdout) => {
        const code = error === null ? 0 : typeof (/** @type {any} */ (error).code) === 'number' ? /** @type {any} */ (error).code : -1;
        resolve({ code, stdout: String(stdout ?? '') });
      });
    });
  }

  return {
    /**
     * Live members of a session.
     *
     * @param {number} sid
     * @returns {Promise<number[]>}
     */
    async members(sid) {
      if (platform === 'win32') return [];
      if (platform === 'linux') {
        /** @type {number[]} */
        const found = [];
        for (const entry of await fsApi.readdir('/proc')) {
          if (!/^\d+$/.test(entry)) continue;
          try {
            if (sessionOfStat(await fsApi.readFile(`/proc/${entry}/stat`, 'utf8')) === sid) found.push(Number(entry));
          } catch {
            continue;
          }
        }
        return found;
      }
      const { code, stdout } = await run('/usr/bin/pgrep', ['-s', String(sid)]);
      if (code !== 0) return [];
      return stdout.split('\n').filter((l) => /^\d+$/.test(l.trim())).map(Number);
    },

    /**
     * @param {number[]} pids
     */
    killAll(pids) {
      for (const pid of pids) {
        try {
          kill(pid, 'SIGKILL');
        } catch {
          continue;
        }
      }
    },

    /**
     * Ends a Windows process and everything it started.
     *
     * @param {number} pid
     * @returns {Promise<boolean>}
     */
    async taskkill(pid) {
      const { code } = await run('taskkill', ['/PID', String(pid), '/T', '/F']);
      return code === 0;
    },
  };
}

module.exports = { createTree, sessionOfStat };
