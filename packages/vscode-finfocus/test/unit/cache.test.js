'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { ResultCache, hashContents } = require('../../src/cache');
const { parseOverview } = require('../../src/result');

const stdout = fs.readFileSync(path.join(__dirname, '..', 'fixtures', 'completeness', 'complete.json'), 'utf8');
const KEY = 'file:///work/shop|org/shop/dev';
const NOW = new Date('2026-11-18T10:00:00Z');

const signals = (extra = {}) => ({
  lastUpdate: '2026-11-18T03:41:07.000Z',
  resourceCount: 65,
  updateInProgress: false,
  stackConfigHash: hashContents('config: a'),
  projectConfigHash: '',
  finfocusVersion: '0.5.8',
  ...extra,
});

function memoryStore() {
  const files = new Map();
  return {
    files,
    async read(key) {
      return files.has(key) ? structuredClone(files.get(key)) : null;
    },
    async write(key, entry) {
      files.set(key, structuredClone(entry));
    },
    async remove(key) {
      files.delete(key);
    },
  };
}

function cache(extra = {}) {
  const store = memoryStore();
  let now = NOW;
  const instance = new ResultCache({ store, ttlMs: 60 * 60 * 1000, now: () => now, ...extra });
  return {
    store,
    cache: instance,
    advance: (ms) => {
      now = new Date(now.getTime() + ms);
    },
  };
}

const save = (c, extra = {}) =>
  c.cache.save(KEY, signals(), { stdout, producedAt: NOW, confirmedGaps: [], ...extra });

test('a saved result is returned for the same key signals, parsed, with its original start time', async () => {
  const c = cache();
  await save(c);
  const hit = await c.cache.lookup(KEY, signals());
  assert.notEqual(hit, null);
  assert.equal(hit.result.stack, 'dev');
  assert.deepEqual(hit.producedAt, NOW);
  assert.deepEqual(hit.confirmedGaps, []);
});

test('each signal changing is a miss', async () => {
  const changes = {
    lastUpdate: '2026-11-18T05:00:00.000Z',
    resourceCount: 66,
    stackConfigHash: hashContents('config: b'),
    projectConfigHash: hashContents('{}'),
    finfocusVersion: '0.5.9',
  };
  for (const [field, value] of Object.entries(changes)) {
    const c = cache();
    await save(c);
    assert.equal(await c.cache.lookup(KEY, signals({ [field]: value })), null, field);
  }
});

test('another context key is a miss', async () => {
  const c = cache();
  await save(c);
  assert.equal(await c.cache.lookup('file:///work/other|org/shop/dev', signals()), null);
});

test('an entry expires after the time limit and lookups before it still hit', async () => {
  const c = cache();
  await save(c);
  c.advance(59 * 60 * 1000);
  assert.notEqual(await c.cache.lookup(KEY, signals()), null);
  c.advance(2 * 60 * 1000);
  assert.equal(await c.cache.lookup(KEY, signals()), null);
});

test('a time limit of zero turns the cache off for reads and writes', async () => {
  const c = cache({ ttlMs: 0 });
  assert.equal(c.cache.enabled, false);
  await save(c);
  assert.equal(c.store.files.size, 0);
  assert.equal(await c.cache.lookup(KEY, signals()), null);
});

test('a stack with no last update, or an update in progress, is never cached', async () => {
  for (const bad of [{ lastUpdate: undefined }, { lastUpdate: '' }, { updateInProgress: true }]) {
    const c = cache();
    await c.cache.save(KEY, signals(bad), { stdout, producedAt: NOW, confirmedGaps: [] });
    assert.equal(c.store.files.size, 0, JSON.stringify(bad));
    await save(c);
    assert.equal(await c.cache.lookup(KEY, signals(bad)), null, JSON.stringify(bad));
  }
});

test('an entry whose stdout fails the parser is removed and counts as a miss', async () => {
  const c = cache();
  await save(c, { stdout: '{"not": "an overview"}' });
  assert.equal(await c.cache.lookup(KEY, signals()), null);
  assert.equal(c.store.files.size, 0);
});

test('an entry the store cannot shape (wrong version or missing fields) is removed', async () => {
  for (const damaged of [{ version: 99 }, { stdout: undefined }, { storedAt: 'yesterday' }]) {
    const c = cache();
    await save(c);
    c.store.files.set(KEY, { ...c.store.files.get(KEY), ...damaged });
    assert.equal(await c.cache.lookup(KEY, signals()), null, JSON.stringify(damaged));
    assert.equal(c.store.files.size, 0, JSON.stringify(damaged));
  }
});

test('a stored entry holds only the documented fields, never a passphrase or environment', async () => {
  const c = cache();
  await c.cache.save(KEY, signals(), {
    stdout,
    producedAt: NOW,
    confirmedGaps: [],
    passphrase: 'hunter2',
    env: { PULUMI_CONFIG_PASSPHRASE: 'hunter2' },
  });
  const entry = c.store.files.get(KEY);
  assert.deepEqual(Object.keys(entry).sort(), [
    'confirmedGaps', 'contextKey', 'finfocusVersion', 'lastUpdate', 'producedAt', 'projectConfigHash',
    'resourceCount', 'stackConfigHash', 'stateOnly', 'stdout', 'storedAt', 'version',
  ]);
  assert.equal(JSON.stringify(entry).includes('hunter2'), false);
  assert.equal(entry.stateOnly, true);
});

test('previous() returns the last entry for the baseline even when it is no longer valid', async () => {
  const c = cache();
  await save(c, { confirmedGaps: ['urn:a'] });
  c.advance(5 * 60 * 60 * 1000);
  assert.equal(await c.cache.lookup(KEY, signals()), null);
  assert.deepEqual((await c.cache.previous(KEY))?.confirmedGaps, ['urn:a']);
});

test('previous() is null when nothing is stored and after a corrupt entry is removed', async () => {
  const c = cache();
  assert.equal(await c.cache.previous(KEY), null);
  await save(c, { stdout: 'not json' });
  await c.cache.lookup(KEY, signals());
  assert.equal(await c.cache.previous(KEY), null);
});

test('hashContents is stable, content-sensitive, and empty for absent files', () => {
  assert.equal(hashContents('a'), hashContents('a'));
  assert.notEqual(hashContents('a'), hashContents('b'));
  assert.equal(hashContents(null), '');
  assert.match(hashContents('a'), /^[0-9a-f]{64}$/);
});

test('the stored stdout is the exact text finfocus printed', async () => {
  const c = cache();
  await save(c);
  assert.equal(c.store.files.get(KEY).stdout, stdout);
  assert.doesNotThrow(() => parseOverview(c.store.files.get(KEY).stdout));
});

test('a store that throws on read is a miss, and one that throws on write does not fail the caller', async () => {
  const failing = {
    async read() { throw new Error('disk gone'); },
    async write() { throw new Error('disk full'); },
    async remove() { throw new Error('nope'); },
  };
  const instance = new ResultCache({ store: failing, ttlMs: 1000, now: () => NOW });
  assert.equal(await instance.lookup(KEY, signals()), null);
  assert.equal(await instance.previous(KEY), null);
  await assert.doesNotReject(instance.save(KEY, signals(), { stdout, producedAt: NOW, confirmedGaps: [] }));
});

test('nothing is read or written while the workspace is untrusted (FR-022)', async () => {
  let trusted = true;
  const c = cache({ isTrusted: () => trusted });
  await save(c);
  trusted = false;
  let reads = 0;
  const read = c.store.read;
  c.store.read = async (key) => {
    reads += 1;
    return read(key);
  };
  assert.equal(await c.cache.lookup(KEY, signals()), null);
  assert.equal(await c.cache.previous(KEY), null);
  c.store.files.clear();
  await save(c);
  assert.equal(reads, 0);
  assert.equal(c.store.files.size, 0);
  trusted = true;
  await save(c);
  assert.equal(c.store.files.size, 1);
});
