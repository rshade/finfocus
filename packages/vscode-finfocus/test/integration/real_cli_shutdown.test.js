'use strict';

// Needs a built finfocus, a deployed Pulumi project, and installed plugins:
//   FINFOCUS_BINARY=<bin/finfocus> FINFOCUS_FIXTURE_PROJECT=<project dir>
// with FINFOCUS_HOME pointing at a home whose plugins directory has a plugin.
// It skips otherwise. Linux reads /proc, macOS reads `ps` with the runner's
// environment token, and Windows walks parent pids through CIM.
const test = require('node:test');
const assert = require('node:assert/strict');
const childProcess = require('node:child_process');
const fs = require('node:fs');
const { Runner } = require('../../src/runner');
const { createTree, OWNER_VAR } = require('../../src/processTree');

const binary = process.env.FINFOCUS_BINARY;
const project = process.env.FINFOCUS_FIXTURE_PROJECT;
const enabled = Boolean(binary && project && fs.existsSync(project));
const windows = process.platform === 'win32';

function psOutput(args) {
  try {
    return childProcess.execFileSync('ps', args, { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 });
  } catch {
    return '';
  }
}

function commandOf(pid) {
  if (process.platform === 'linux') {
    try {
      return fs.readFileSync(`/proc/${pid}/cmdline`, 'utf8').split('\0')[0];
    } catch {
      return '';
    }
  }
  return psOutput(['-o', 'command=', '-p', String(pid)]).trim().split(/\s+/)[0] ?? '';
}

/** Whether a process is running; a zombie awaiting its parent is gone. */
function isAlive(pid) {
  if (windows) {
    const out = childProcess.spawnSync('tasklist', ['/FI', `PID eq ${pid}`, '/NH', '/FO', 'CSV'], { encoding: 'utf8' }).stdout ?? '';
    return out.includes(`"${pid}"`);
  }
  const stat = psOutput(['-o', 'stat=', '-p', String(pid)]).trim();
  return stat !== '' && !stat.startsWith('Z');
}

/** Windows has no session: every process under `root`, by parent pid, with its image name. */
function windowsDescendants(root) {
  const script = 'Get-CimInstance Win32_Process | Select-Object ProcessId,ParentProcessId,Name | ConvertTo-Json -Compress';
  const out = childProcess.spawnSync('powershell', ['-NoProfile', '-Command', script], { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 }).stdout ?? '';
  let rows = [];
  try {
    rows = [].concat(JSON.parse(out));
  } catch {
    return new Map();
  }
  const found = new Map();
  const queue = [root];
  while (queue.length > 0) {
    const parent = queue.shift();
    for (const row of rows) {
      if (row.ParentProcessId === parent && !found.has(row.ProcessId)) {
        found.set(row.ProcessId, row.Name);
        queue.push(row.ProcessId);
      }
    }
  }
  return found;
}

async function waitFor(fn, ms) {
  const deadline = Date.now() + ms;
  for (;;) {
    const value = await fn();
    if (value || Date.now() > deadline) return value;
    await new Promise((r) => setTimeout(r, 100));
  }
}

for (const mode of ['stop mid-preview', 'stop after the process was killed']) {
  test(`real finfocus: ${mode} leaves no plugin or pulumi descendant`, { skip: !enabled }, async (t) => {
    let owner = null;
    const spawn = (command, args, options) => {
      owner = options.env[OWNER_VAR] ?? null;
      return childProcess.spawn(command, args, options);
    };
    const runner = new Runner({ isTrusted: () => true, finfocusPath: () => binary, spawn });
    const tree = createTree();
    const handle = runner.overview({ projectDir: project, stack: process.env.FINFOCUS_FIXTURE_STACK || 'dev', stateOnly: false, passphrase: null });
    const sid = handle.pid;
    const seen = new Map();
    const sawDescendants = await waitFor(async () => {
      if (windows) {
        for (const [pid, name] of windowsDescendants(sid)) seen.set(pid, name);
      } else {
        for (const pid of await tree.members(sid, owner)) if (pid !== sid) seen.set(pid, commandOf(pid));
      }
      return [...seen.values()].some((c) => /finfocus-plugin|pulumi/.test(c));
    }, 90000);
    if (!sawDescendants) t.diagnostic('the run ended before a plugin or pulumi child appeared');
    t.diagnostic(`session ${sid}: ${[...seen].map(([p, c]) => `${p}=${c.split('/').pop()}`).join(' ')}`);

    const started = Date.now();
    if (mode === 'stop after the process was killed') process.kill(sid, windows ? undefined : 'SIGKILL');
    const result = await handle.stop();
    t.diagnostic(`stop took ${Date.now() - started} ms: ${JSON.stringify(result)}`);
    assert.equal(result.confirmed, true);
    if (!windows) assert.deepEqual(await tree.members(sid, owner), []);
    for (const [pid, command] of seen) {
      assert.equal(isAlive(pid), false, `${pid} ${command} survived`);
    }
    assert.ok(sawDescendants, 'a plugin or pulumi child was observed before stopping');
  });
}
