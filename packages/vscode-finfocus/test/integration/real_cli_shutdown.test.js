'use strict';

// Needs a built finfocus, a deployed Pulumi project, and installed plugins:
//   FINFOCUS_BINARY=<bin/finfocus> FINFOCUS_FIXTURE_PROJECT=<project dir>
// with FINFOCUS_HOME pointing at a home whose plugins directory has a plugin.
// It skips otherwise. Linux and macOS only: it reads session membership.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const { Runner } = require('../../src/runner');
const { createTree } = require('../../src/processTree');

const binary = process.env.FINFOCUS_BINARY;
const project = process.env.FINFOCUS_FIXTURE_PROJECT;
const enabled = Boolean(binary && project && fs.existsSync(project)) && process.platform !== 'win32';

function commandOf(pid) {
  try {
    return fs.readFileSync(`/proc/${pid}/cmdline`, 'utf8').split('\0')[0];
  } catch {
    return '';
  }
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
    const runner = new Runner({ isTrusted: () => true, finfocusPath: () => binary });
    const tree = createTree();
    const handle = runner.overview({ projectDir: project, stack: process.env.FINFOCUS_FIXTURE_STACK || 'dev', stateOnly: false, passphrase: null });
    const sid = handle.pid;
    const seen = new Map();
    const sawDescendants = await waitFor(async () => {
      for (const pid of await tree.members(sid)) if (pid !== sid) seen.set(pid, commandOf(pid));
      return [...seen.values()].some((c) => /finfocus-plugin|pulumi/.test(c));
    }, 90000);
    if (!sawDescendants) t.diagnostic('the run ended before a plugin or pulumi child appeared');
    t.diagnostic(`session ${sid}: ${[...seen].map(([p, c]) => `${p}=${c.split('/').pop()}`).join(' ')}`);

    const started = Date.now();
    if (mode === 'stop after the process was killed') process.kill(sid, 'SIGKILL');
    const result = await handle.stop();
    t.diagnostic(`stop took ${Date.now() - started} ms: ${JSON.stringify(result)}`);
    assert.equal(result.confirmed, true);
    assert.deepEqual(await tree.members(sid), []);
    for (const [pid, command] of seen) {
      let alive = false;
      try {
        const stat = fs.readFileSync(`/proc/${pid}/stat`, 'utf8');
        alive = !/^[ZX]/.test(stat.slice(stat.lastIndexOf(')') + 2));
      } catch {
        alive = false;
      }
      assert.equal(alive, false, `${pid} ${command} survived`);
    }
    assert.ok(sawDescendants, 'a plugin or pulumi child was observed before stopping');
  });
}
