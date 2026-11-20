'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const { createTree, pidsWithOwner, OWNER_VAR, MAX_LISTING_BYTES } = require('../../src/processTree');

const TOKEN = 'a'.repeat(32);

const PS_OUTPUT = [
  '  PID TTY           TIME CMD',
  `  100 ??         0:01.10 /opt/finfocus overview PATH=/usr/bin HOME=/Users/u ${OWNER_VAR}=${TOKEN}`,
  `  101 ??         0:00.20 /opt/plugin --port 4000 ${OWNER_VAR}=${TOKEN} FOO=bar`,
  `  102 ??         0:00.30 /opt/other HOME=/Users/u ${OWNER_VAR}=${'b'.repeat(32)}`,
  '  103 ??         0:00.05 /usr/bin/ssh-agent HOME=/Users/u',
  `  777 ??         0:00.01 sweeper ${OWNER_VAR}=${TOKEN}`,
].join('\n');

test('only the processes carrying this run\'s token are members, and never the sweeper itself', () => {
  assert.deepEqual(pidsWithOwner(PS_OUTPUT, TOKEN, 777), [100, 101]);
  assert.deepEqual(pidsWithOwner(PS_OUTPUT, 'c'.repeat(32), 777), []);
  assert.deepEqual(pidsWithOwner('', TOKEN, 777), []);
});

test('a line that merely mentions the token without a pid column is ignored', () => {
  assert.deepEqual(pidsWithOwner(`note ${OWNER_VAR}=${TOKEN}`, TOKEN, 1), []);
});

const seenOptions = [];

function treeWith(output, code = 0) {
  const calls = [];
  const execFile = (command, args, options, callback) => {
    calls.push([command, args]);
    seenOptions.push(options);
    callback(code === 0 ? null : Object.assign(new Error('ps failed'), { code }), output);
  };
  return { tree: createTree({ platform: 'darwin', execFile }), calls };
}

test('on macOS the members come from ps with the environment, not from pgrep -s', async () => {
  const { tree, calls } = treeWith(PS_OUTPUT.replace('  777 ', `  ${process.pid} `));
  assert.deepEqual(await tree.members(100, TOKEN), [100, 101]);
  assert.deepEqual(calls, [['/bin/ps', ['-axEww']]]);
});

test('on macOS a ps that cannot run leaves the session unconfirmed instead of empty', async () => {
  const { tree } = treeWith('', 1);
  assert.deepEqual(await tree.members(100, TOKEN), [100]);
});

test('without a token the other-Unix path still asks pgrep', async () => {
  const { tree, calls } = treeWith('200\n201\n');
  assert.deepEqual(await tree.members(100), [200, 201]);
  assert.equal(calls[0][0], '/usr/bin/pgrep');
});

test('the process listing is read with a buffer far above the 1 MiB default', async () => {
  const { tree } = treeWith(PS_OUTPUT);
  await tree.members(100, TOKEN);
  assert.ok(MAX_LISTING_BYTES >= 64 * 1024 * 1024);
  assert.equal(seenOptions.at(-1).maxBuffer, MAX_LISTING_BYTES);
});

function execResult(code, stdout = '') {
  const calls = [];
  const execFile = (command, args, options, callback) => {
    calls.push([command, args]);
    callback(code === 0 ? null : Object.assign(new Error('failed'), { code }), stdout);
  };
  return { execFile, calls };
}

test('Windows has no session to list, so its members are always empty', async () => {
  const { execFile, calls } = execResult(0, 'ignored');
  const tree = createTree({ platform: 'win32', execFile });
  assert.deepEqual(await tree.members(4242), []);
  assert.deepEqual(calls, []);
});

test('taskkill ends the whole tree by pid and reports whether it succeeded', async () => {
  const ok = execResult(0);
  assert.equal(await createTree({ platform: 'win32', execFile: ok.execFile }).taskkill(4242), true);
  assert.deepEqual(ok.calls, [['taskkill', ['/PID', '4242', '/T', '/F']]]);
  assert.equal(await createTree({ platform: 'win32', execFile: execResult(128).execFile }).taskkill(4242), false);
});

test('a failed command with no numeric exit code still reads as a failure', async () => {
  const execFile = (command, args, options, callback) => callback(Object.assign(new Error('ENOENT'), { code: 'ENOENT' }), '');
  assert.equal(await createTree({ platform: 'win32', execFile }).taskkill(1), false);
});

test('killAll signals every pid and keeps going past one that already exited', () => {
  const killed = [];
  const kill = (pid, signal) => {
    if (pid === 2) throw Object.assign(new Error('ESRCH'), { code: 'ESRCH' });
    killed.push([pid, signal]);
  };
  createTree({ platform: 'linux', kill }).killAll([1, 2, 3]);
  assert.deepEqual(killed, [[1, 'SIGKILL'], [3, 'SIGKILL']]);
});

test('on Linux an entry that vanishes or is unreadable while scanning is skipped', async () => {
  const stat = (session) => `5 (cmd) S 1 ${session} ${session} 0 -1`;
  const fsApi = {
    readdir: async () => ['self', '10', '11', '12'],
    readFile: async (file) => {
      if (file === '/proc/11/stat') throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' });
      return file === '/proc/10/stat' ? stat(77) : stat(5);
    },
  };
  assert.deepEqual(await createTree({ platform: 'linux', fsApi }).members(77), [10]);
});
