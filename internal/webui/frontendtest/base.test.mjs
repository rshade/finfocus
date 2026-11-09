import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import vm from 'node:vm';

const flush = () => new Promise((resolve) => setImmediate(resolve));
const staticDir = new URL('../static/', import.meta.url);

async function setup(prefix) {
  const node = () => ({
    children: [], append() {}, replaceChildren() {}, querySelector: () => null,
    querySelectorAll: () => [], addEventListener() {}, setAttribute() {}, focus() {},
  });
  const document = {
    createElement: node,
    getElementById: (id) => (id === 'view' || id === 'main' ? node() : id === 'connection-status' ? { textContent: '', dataset: {} } : id === 'view-announcer' ? { textContent: '' } : null),
    querySelector: (selector) => (selector === 'meta[name="finfocus-base"]' && prefix !== undefined ? { content: prefix } : null),
    querySelectorAll: () => [],
    addEventListener() {},
  };
  class EventSource {
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSED = 2;
    static all = [];
    constructor(url) {
      this.url = url;
      this.listeners = new Map();
      this.readyState = 0;
      EventSource.all.push(this);
    }
    addEventListener(name, fn) { this.listeners.set(name, fn); }
    close() { this.readyState = 2; }
  }
  const fetches = [];
  const context = vm.createContext({
    document,
    window: { addEventListener() {} },
    location: { hash: '#/overview' },
    history: { replaceState() {} },
    console,
    EventSource,
    MessageEvent: class MessageEvent {},
    AbortController,
    setTimeout: () => 1,
    clearTimeout() {},
    fetch: (url) => {
      fetches.push(url);
      return Promise.resolve({ ok: true, status: 200, text: async () => '{}' });
    },
  });
  const app = new vm.SourceTextModule(await readFile(new URL('app.js', staticDir), 'utf8'), {
    context,
    importModuleDynamically: async () => {
      const mod = new vm.SyntheticModule(['mount', 'probeLogs'], function () { this.setExport('mount', () => {}); this.setExport('probeLogs', async () => false); }, { context });
      await mod.link(() => {});
      await mod.evaluate();
      return mod;
    },
  });
  await app.link(() => {});
  await app.evaluate();
  await flush();
  return { ns: app.namespace, EventSource, fetches };
}

test('withBase prefixes api requests when the page declares a stack prefix', async () => {
  const t = await setup('/s/dev');
  await t.ns.api.get('/api/config');
  await t.ns.api.post('/api/config', {});
  assert.deepEqual(t.fetches.slice(-2), ['/s/dev/api/config', '/s/dev/api/config']);
  assert.equal(t.ns.base(), '/s/dev');
  assert.equal(t.ns.withBase('/static/zerolog-viewer/index.js'), '/s/dev/static/zerolog-viewer/index.js');
});

test('the overview stream and its session check use the prefix', async () => {
  const t = await setup('/s/dev');
  const before = t.EventSource.all.length;
  const stream = new t.ns.OverviewStream('/api/overview/stream', { textContent: '', dataset: {} });
  stream.start();
  const source = t.EventSource.all.at(-1);
  assert.equal(t.EventSource.all.length, before + 1);
  assert.equal(source.url, '/s/dev/api/overview/stream');
  source.readyState = 2;
  source.listeners.get('error')({});
  assert.equal(t.fetches.at(-1), '/s/dev/api/overview/budget');
});

test('with no prefix every URL is unchanged', async () => {
  for (const prefix of [undefined, '']) {
    const t = await setup(prefix);
    await t.ns.api.get('/api/config');
    assert.equal(t.fetches.at(-1), '/api/config');
    assert.equal(t.ns.base(), '');
    const stream = new t.ns.OverviewStream('/api/overview/stream', { textContent: '', dataset: {} });
    stream.start();
    assert.equal(t.EventSource.all.at(-1).url, '/api/overview/stream');
  }
});

test('a prefix that is not a stack path is ignored', async () => {
  for (const prefix of ['//evil.example', 'https://evil.example', '/s/', '/s/a b', '/other', '/s/dev/']) {
    const t = await setup(prefix);
    assert.equal(t.ns.base(), '', prefix);
  }
});

async function* walk(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const url = new URL(entry.name + (entry.isDirectory() ? '/' : ''), dir);
    if (entry.isDirectory()) yield* walk(url);
    else yield url;
  }
}

test('no network call or import takes an absolute /api or /static literal', async () => {
  const absolute = /\b(fetch|EventSource|Source|import)\(\s*['"`]\//;
  const offenders = [];
  for await (const url of walk(staticDir)) {
    if (!url.pathname.endsWith('.js')) continue;
    const lines = (await readFile(url, 'utf8')).split('\n');
    lines.forEach((line, i) => {
      if (absolute.test(line)) offenders.push(`${url.pathname.split('/static/')[1]}:${i + 1}: ${line.trim()}`);
    });
  }
  assert.deepEqual(offenders, []);
});

test('every dynamic import is relative to its module or goes through withBase', async () => {
  const offenders = [];
  for await (const url of walk(staticDir)) {
    if (!url.pathname.endsWith('.js')) continue;
    const lines = (await readFile(url, 'utf8')).split('\n');
    lines.forEach((line, i) => {
      for (const match of line.matchAll(/\bimport\(\s*([^)]{0,40})/g)) {
        if (!/^(['"]\.\.?\/|withBase\()/.test(match[1])) offenders.push(`${url.pathname.split('/static/')[1]}:${i + 1}: ${line.trim()}`);
      }
    });
  }
  assert.deepEqual(offenders, []);
});

test('the shell references assets relatively and declares an empty prefix', async () => {
  const html = await readFile(new URL('index.html', staticDir), 'utf8');
  assert.doesNotMatch(html, /(?:href|src)="\/(?:static|api)\//);
  assert.match(html, /<meta name="finfocus-base" content="">/);
});
