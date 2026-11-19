'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { FileCacheStore } = require('../../src/editor/cacheStore');

function tempDir() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'finfocus-cache-'));
  return { dir, cleanup: () => fs.rmSync(dir, { recursive: true, force: true }) };
}

const entry = (extra = {}) => ({ version: 1, contextKey: 'k', stdout: '{}', ...extra });

test('an entry round-trips through the store, and a missing one reads as null', async (t) => {
  const { dir, cleanup } = tempDir();
  t.after(cleanup);
  const store = new FileCacheStore(dir);
  assert.equal(await store.read('file:///p|org/p/dev'), null);
  await store.write('file:///p|org/p/dev', entry());
  assert.deepEqual(await store.read('file:///p|org/p/dev'), entry());
});

test('file names are a hash of the key, so the key never reaches the file system as a path', async (t) => {
  const { dir, cleanup } = tempDir();
  t.after(cleanup);
  const store = new FileCacheStore(dir);
  await store.write('../../etc/passwd|stack', entry());
  const files = fs.readdirSync(path.join(dir, 'results'));
  assert.equal(files.length, 1);
  assert.match(files[0], /^[0-9a-f]{64}\.json$/);
});

test('the directory is created on first write, and the file is 0600 on Unix', { skip: process.platform === 'win32' }, async (t) => {
  const { dir, cleanup } = tempDir();
  t.after(cleanup);
  const store = new FileCacheStore(path.join(dir, 'storage', 'nested'));
  await store.write('k', entry());
  const [name] = fs.readdirSync(path.join(dir, 'storage', 'nested', 'results'));
  const mode = fs.statSync(path.join(dir, 'storage', 'nested', 'results', name)).mode & 0o777;
  assert.equal(mode, 0o600);
});

test('on Windows only the location is asserted: the file sits under the storage directory', { skip: process.platform !== 'win32' }, async (t) => {
  const { dir, cleanup } = tempDir();
  t.after(cleanup);
  const store = new FileCacheStore(dir);
  await store.write('k', entry());
  assert.equal(fs.readdirSync(path.join(dir, 'results')).length, 1);
});

test('a write replaces the entry atomically and leaves no temporary file', async (t) => {
  const { dir, cleanup } = tempDir();
  t.after(cleanup);
  const store = new FileCacheStore(dir);
  await store.write('k', entry({ stdout: 'one' }));
  await store.write('k', entry({ stdout: 'two' }));
  assert.equal((await store.read('k')).stdout, 'two');
  assert.deepEqual(fs.readdirSync(path.join(dir, 'results')).filter((n) => !n.endsWith('.json')), []);
});

test('a corrupt file is deleted and reads as null', async (t) => {
  const { dir, cleanup } = tempDir();
  t.after(cleanup);
  const store = new FileCacheStore(dir);
  await store.write('k', entry());
  const [name] = fs.readdirSync(path.join(dir, 'results'));
  fs.writeFileSync(path.join(dir, 'results', name), '{"version": 1, "stdo');
  assert.equal(await store.read('k'), null);
  assert.deepEqual(fs.readdirSync(path.join(dir, 'results')), []);
});

test('a half-written temporary file is never read as an entry', async (t) => {
  const { dir, cleanup } = tempDir();
  t.after(cleanup);
  const store = new FileCacheStore(dir);
  fs.mkdirSync(path.join(dir, 'results'), { recursive: true });
  fs.writeFileSync(path.join(dir, 'results', 'abc.json.tmp-123'), '{"version":1}');
  assert.equal(await store.read('k'), null);
});

test('remove deletes an entry and ignores a missing one', async (t) => {
  const { dir, cleanup } = tempDir();
  t.after(cleanup);
  const store = new FileCacheStore(dir);
  await store.write('k', entry());
  await store.remove('k');
  await store.remove('k');
  assert.equal(await store.read('k'), null);
});

test('prune removes entries untouched for the given age, and stale temporary files, and keeps newer ones', async (t) => {
  const { dir, cleanup } = tempDir();
  t.after(cleanup);
  const store = new FileCacheStore(dir);
  await store.write('old', entry());
  await store.write('new', entry());
  const results = path.join(dir, 'results');
  const files = fs.readdirSync(results);
  const oldName = files.find((n) => n.startsWith(require('node:crypto').createHash('sha256').update('old').digest('hex')));
  const tmp = path.join(results, 'x.json.tmp-1');
  fs.writeFileSync(tmp, '{}');
  const past = new Date(Date.now() - 31 * 24 * 60 * 60 * 1000);
  fs.utimesSync(path.join(results, oldName), past, past);
  fs.utimesSync(tmp, past, past);
  await store.prune(30 * 24 * 60 * 60 * 1000);
  assert.equal(await store.read('old'), null);
  assert.notEqual(await store.read('new'), null);
  assert.equal(fs.existsSync(tmp), false);
});

test('prune on a directory that does not exist yet does nothing', async (t) => {
  const { dir, cleanup } = tempDir();
  t.after(cleanup);
  await assert.doesNotReject(new FileCacheStore(path.join(dir, 'nope')).prune(1000));
});
