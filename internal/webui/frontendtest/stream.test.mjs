import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

const flush = () => new Promise((resolve) => setImmediate(resolve));

async function setup() {
  const document = {
    createElement: () => ({
      id: '',
      children: [],
      append() {},
      replaceChildren() {},
      querySelector: () => null,
      addEventListener() {},
      setAttribute() {},
      focus() {},
    }),
    getElementById: () => null,
    querySelector: () => null,
    querySelectorAll: () => [],
    addEventListener() {},
  };
  document.createElement = (tag) => ({
    tag,
    id: '',
    children: [],
    append() {},
    replaceChildren() {},
    querySelector: () => null,
    querySelectorAll: () => [],
    addEventListener() {},
    setAttribute() {},
    focus() {},
  });
  const view = document.createElement('div');
  view.id = 'view';
  const statusNode = { textContent: '', dataset: {} };
  const main = document.createElement('main');
  main.id = 'main';
  document.getElementById = (id) => (id === 'view' ? view : id === 'main' ? main : id === 'connection-status' ? statusNode : null);
  class EventSource {
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSED = 2;
    static all = [];
    constructor() {
      this.listeners = new Map();
      this.readyState = EventSource.CONNECTING;
      EventSource.all.push(this);
    }
    addEventListener(name, fn) { this.listeners.set(name, fn); }
    close() { this.closed = true; this.readyState = EventSource.CLOSED; }
  }
  class MessageEvent {}
  let timerId = 0;
  const timers = new Map();
  const fetches = [];
  const context = vm.createContext({
    document,
    window: { addEventListener() {} },
    location: { hash: '#/overview' },
    history: { replaceState() {} },
    console,
    EventSource,
    MessageEvent,
    AbortController,
    setTimeout: (fn) => { const id = ++timerId; timers.set(id, fn); return id; },
    clearTimeout: (id) => timers.delete(id),
    fetch: (url, init) => {
      const pending = deferred();
      fetches.push({ url, init, ...pending });
      return pending.promise;
    },
  });
  const app = new vm.SourceTextModule(await readFile(new URL('../static/app.js', import.meta.url), 'utf8'), {
    context,
    importModuleDynamically: async () => {
      const mod = new vm.SyntheticModule(['mount'], function () { this.setExport('mount', () => {}); }, { context });
      await mod.link(() => {});
      await mod.evaluate();
      return mod;
    },
  });
  await app.link(() => {});
  await app.evaluate();
  await flush();
  const status = { textContent: '', dataset: {} };
  return { EventSource, MessageEvent, timers, fetches, status, OverviewStream: app.namespace.OverviewStream, context };
}

function connectionError(source) {
  source.readyState = source.constructor.CLOSED;
  source.listeners.get('error')({});
}

test('a confirmed 401 stops overview stream retries', async () => {
  const t = await setup();
  const stream = new t.OverviewStream('/api/overview/stream', t.status);
  const before = t.EventSource.all.length;
  stream.start();
  const source = t.EventSource.all.at(-1);
  connectionError(source);
  assert.equal(t.fetches.length, 1);
  assert.equal(t.fetches[0].url, '/api/overview/budget');
  assert.equal(t.fetches[0].init.credentials, 'same-origin');
  assert.equal(t.fetches[0].init.headers, undefined);
  t.fetches[0].resolve({ status: 401, ok: false });
  await flush();
  assert.equal(t.timers.size, 0);
  assert.equal(source.closed, true);
  assert.match(t.status.textContent, /terminal/i);
  stream.start();
  assert.equal(t.EventSource.all.length, before + 1);
});

test('a transient overview stream failure keeps bounded backoff and can recover', async () => {
  const t = await setup();
  const stream = new t.OverviewStream('/api/overview/stream', t.status);
  stream.start();
  const source = t.EventSource.all.at(-1);
  connectionError(source);
  t.fetches[0].resolve({ status: 200, ok: true });
  await flush();
  assert.equal(t.timers.size, 1);
  assert.doesNotMatch(t.status.textContent, /terminal/i);
  const before = t.EventSource.all.length;
  for (const callback of [...t.timers.values()]) callback();
  assert.equal(t.EventSource.all.length, before + 1);
  const next = t.EventSource.all.at(-1);
  next.listeners.get('open')();
  assert.equal(t.status.dataset.state, 'live');
  assert.equal(source.closed, true);
  assert.ok(!next.closed);
});

test('disposing the overview stream cancels a pending authentication check', async () => {
  const t = await setup();
  const stream = new t.OverviewStream('/api/overview/stream', t.status);
  stream.start();
  const source = t.EventSource.all.at(-1);
  connectionError(source);
  assert.equal(t.fetches[0].init.signal.aborted, false);
  const before = t.EventSource.all.length;
  stream.close();
  assert.equal(t.fetches[0].init.signal.aborted, true);
  const abort = new DOMException('aborted', 'AbortError');
  t.fetches[0].reject(abort);
  await flush();
  assert.equal(t.timers.size, 0);
  assert.equal(t.EventSource.all.length, before);
  assert.equal(source.closed, true);
});

test('a source that reopens while the session check is pending is not restarted', async () => {
  const t = await setup();
  const stream = new t.OverviewStream('/api/overview/stream', t.status);
  stream.start();
  const source = t.EventSource.all.at(-1);
  source.readyState = t.EventSource.CONNECTING;
  source.listeners.get('error')({});
  assert.equal(t.fetches.length, 1);
  source.readyState = 1;
  source.listeners.get('open')();
  const before = t.EventSource.all.length;
  t.fetches[0].resolve({ status: 200, ok: true });
  await flush();
  assert.equal(t.timers.size, 0);
  assert.equal(t.status.dataset.state, 'live');
  assert.equal(source.closed, undefined);
  assert.equal(t.EventSource.all.length, before);
});

test('a stale 401 after the source reopened does not close the live stream', async () => {
  const t = await setup();
  const stream = new t.OverviewStream('/api/overview/stream', t.status);
  stream.start();
  const source = t.EventSource.all.at(-1);
  source.readyState = t.EventSource.CONNECTING;
  source.listeners.get('error')({});
  source.readyState = 1;
  source.listeners.get('open')();
  t.fetches[0].resolve({ status: 401, ok: false });
  await flush();
  assert.equal(source.closed, undefined);
  assert.equal(t.status.dataset.state, 'live');
  assert.doesNotMatch(t.status.textContent, /terminal/i);
});

test('a hung session check times out and falls back to bounded backoff', async () => {
  const t = await setup();
  const stream = new t.OverviewStream('/api/overview/stream', t.status);
  stream.start();
  const source = t.EventSource.all.at(-1);
  connectionError(source);
  assert.equal(t.fetches.length, 1);
  assert.equal(t.timers.size, 1);
  const [timeout] = [...t.timers.values()];
  timeout();
  await flush();
  assert.equal(t.fetches[0].init.signal.aborted, true);
  assert.equal(t.status.dataset.state, 'down');
  assert.doesNotMatch(t.status.textContent, /terminal/i);
  assert.equal(t.timers.size, 1);
  const before = t.EventSource.all.length;
  for (const callback of [...t.timers.values()]) callback();
  assert.equal(t.EventSource.all.length, before + 1);
});

test('a second connection error while the session check is pending does not start another check', async () => {
  const t = await setup();
  const stream = new t.OverviewStream('/api/overview/stream', t.status);
  stream.start();
  const source = t.EventSource.all.at(-1);
  connectionError(source);
  connectionError(source);
  assert.equal(t.fetches.length, 1);
});
