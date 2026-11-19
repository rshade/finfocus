'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const childProcess = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { Runner } = require('../../src/runner');
const { createShutdown } = require('../../src/shutdown');
const { createTree } = require('../../src/processTree');

const FIXTURE = path.join(__dirname, '..', 'fixtures', 'process-tree', 'parent.js');
const GRACE_MS = 600;
const CONFIRM_MS = 2000;

function isAlive(pid) {
  if (process.platform === 'linux') {
    try {
      const stat = fs.readFileSync(`/proc/${pid}/stat`, 'utf8');
      const state = stat.slice(stat.lastIndexOf(')') + 2, stat.lastIndexOf(')') + 3);
      return state !== 'Z' && state !== 'X';
    } catch {
      return false;
    }
  }
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

async function waitFor(condition, ms = 5000) {
  const deadline = Date.now() + ms;
  while (Date.now() < deadline) {
    if (condition()) return true;
    await new Promise((r) => setTimeout(r, 25));
  }
  return condition();
}

function readPids(dir) {
  const pids = {};
  for (const role of ['parent', 'child', 'grandchild']) {
    try {
      pids[role] = Number(fs.readFileSync(path.join(dir, `${role}.pid`), 'utf8'));
    } catch {
      continue;
    }
  }
  return pids;
}



async function launchWithEnv(mode) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'finfocus-tree-'));
  const runner = new Runner({
    isTrusted: () => true,
    env: { ...process.env, FIXTURE_PIDS: dir, FIXTURE_MODE: mode },
    shutdown: createShutdown({ tree: createTree(), graceMs: GRACE_MS, confirmMs: CONFIRM_MS }),
  });
  const handle = runner.start(process.execPath, [FIXTURE], dir, null, false);
  assert.ok(await waitFor(() => readPids(dir).grandchild !== undefined), 'the fixture started its grandchild');
  return { dir, handle, pids: readPids(dir), runner };
}

function cleanup(pids) {
  for (const pid of Object.values(pids)) {
    try {
      process.kill(pid, 'SIGKILL');
    } catch {
      continue;
    }
  }
}

test('the grandchild really is in its own process group inside the session', { skip: process.platform === 'win32' }, async () => {
  const { handle, pids } = await launchWithEnv('graceful');
  try {
    const group = (pid) => childProcess.execFileSync('ps', ['-o', 'pgid=,sid=', '-p', String(pid)], { encoding: 'utf8' }).trim().split(/\s+/).map(Number);
    const [parentGroup, parentSession] = group(pids.parent);
    const [grandGroup, grandSession] = group(pids.grandchild);
    assert.equal(parentSession, pids.parent, 'the runner made finfocus a session leader');
    assert.equal(grandSession, parentSession);
    assert.notEqual(grandGroup, parentGroup);
  } finally {
    await handle.stop();
    cleanup(pids);
  }
});

for (const mode of ['graceful', 'leave', 'ignore']) {
  test(`stop leaves no descendant running (${mode})`, async () => {
    const { handle, pids } = await launchWithEnv(mode);
    const started = Date.now();
    try {
      const result = await handle.stop();
      assert.equal(result.confirmed, true, JSON.stringify(result));
      const elapsed = Date.now() - started;
      assert.ok(elapsed < GRACE_MS + CONFIRM_MS + 1000, `took ${elapsed} ms`);
      for (const [role, pid] of Object.entries(pids)) {
        assert.equal(isAlive(pid), false, `${role} ${pid} survived`);
      }
      if (mode === 'ignore') assert.equal(result.forced, true, 'a process that ignores SIGTERM needs escalation');
      if (mode === 'graceful') assert.equal(result.forced, false);
    } finally {
      cleanup(pids);
    }
  });
}

test('a descendant left behind by a clean exit is swept', { skip: process.platform === 'win32' }, async () => {
  const { handle, pids } = await launchWithEnv('orphan-clean-exit');
  try {
    const outcome = await handle.done;
    assert.equal(outcome.kind, 'ok');
    for (const [role, pid] of Object.entries(pids)) {
      assert.equal(isAlive(pid), false, `${role} ${pid} survived a clean exit`);
    }
  } finally {
    cleanup(pids);
  }
});

test('an unrelated process survives', async () => {
  const bystander = childProcess.spawn(process.execPath, ['-e', 'setInterval(() => {}, 1e9)'], { detached: true, stdio: 'ignore' });
  const { handle, pids } = await launchWithEnv('ignore');
  try {
    await handle.stop();
    assert.equal(isAlive(bystander.pid), true);
  } finally {
    bystander.kill('SIGKILL');
    cleanup(pids);
  }
});

test('stop is idempotent and safe after the process exited', async () => {
  const { handle, pids } = await launchWithEnv('graceful');
  try {
    const first = handle.stop();
    assert.equal(await handle.stop(), await first);
    await handle.done;
    assert.equal((await handle.stop()).confirmed, true);
  } finally {
    cleanup(pids);
  }
});

test('after SIGKILL of the parent every descendant is still in its session and is swept', { skip: process.platform === 'win32' }, async () => {
  const { handle, pids } = await launchWithEnv('ignore');
  try {
    process.kill(pids.parent, 'SIGKILL');
    await handle.done;
    assert.equal(isAlive(pids.grandchild), false, 'done reaps the session after the leader died');
  } finally {
    cleanup(pids);
  }
});

test('a session that cannot be swept reports unconfirmed and names the survivors', async () => {
  const stubborn = {
    members: async () => [4242],
    killAll: () => {},
    taskkill: async () => false,
  };
  const shutdown = createShutdown({ tree: stubborn, graceMs: 20, confirmMs: 80, platform: 'linux' });
  const result = await shutdown.stop({ pid: 99999, exited: new Promise(() => {}), terminate: () => {} });
  assert.deepEqual(result, { confirmed: false, forced: true, survivors: [4242] });
});
