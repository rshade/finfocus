import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

class Node {
  constructor(tag) { this.tagName = tag; this.children = []; this.attrs = {}; this.dataset = {}; this.hidden = false; }
  append(...children) { for (const child of children) this.children.push(child); }
  replaceChildren(...children) { this.children = []; this.append(...children); }
  set textContent(text) { this.text = String(text); this.children = []; }
  get textContent() { return (this.text ?? '') + this.children.map(c => typeof c === 'string' ? c : c.textContent).join(''); }
  setAttribute(k, v) { this.attrs[k] = String(v); if (k === 'class') this.className = v; }
  addEventListener() {}
}

class FakeSource {
  static all = [];
  constructor(url) { this.url = url; this.listeners = new Map(); this.readyState = 0; this.closed = false; FakeSource.all.push(this); }
  addEventListener(name, fn) { this.listeners.set(name, fn); }
  close() { this.closed = true; this.readyState = 2; }
  emit(name, payload) { this.listeners.get(name)?.({ data: JSON.stringify(payload) }); }
}

const viewURL = new URL('../static/views/logs.js', import.meta.url);
const notFound = () => Object.assign(new Error('not found'), { status: 404 });

async function setup({ failure } = {}) {
  FakeSource.all = [];
  const context = vm.createContext({ console });
  const app = new vm.SyntheticModule(['el'], function () {
    this.setExport('el', (tag, attrs = {}, ...children) => {
      const node = new Node(tag);
      for (const [k, v] of Object.entries(attrs)) {
        if (k === 'text') node.textContent = v; else if (k === 'class') node.className = v; else node.setAttribute(k, v);
      }
      node.append(...children.flat());
      return node;
    });
  }, { context });
  await app.link(() => {}); await app.evaluate();
  const mod = new vm.SourceTextModule(await readFile(viewURL, 'utf8'), { context });
  await mod.link(() => app); await mod.evaluate();

  const pushed = [];
  const state = { destroyed: false };
  const created = [];
  const viewer = { setSources: names => { state.sources = names; }, push: e => pushed.push(e), clear: () => {}, destroy: () => { state.destroyed = true; } };
  const api = { get: async () => { if (failure) throw failure; return { enabled: true, sources: [{ name: 'core', kind: 'core' }, { name: 'aws-public', kind: 'plugin' }] }; } };
  const timers = [];
  const deps = {
    EventSource: FakeSource,
    createViewer: async (host, options) => { created.push(options); return viewer; },
    setTimeout: (fn, ms) => { timers.push({ fn, ms }); return timers.length; },
    clearTimeout: () => {},
  };
  return { mod, api, created, pushed, state, timers, deps, container: new Node('div') };
}

test('probe leaves the nav entry hidden on 404 and shows it on 200', async () => {
  const hidden = await setup({ failure: notFound() });
  const link404 = new Node('a'); link404.hidden = true;
  assert.equal(await hidden.mod.namespace.probeLogs(hidden.api, link404), false);
  assert.equal(link404.hidden, true);

  const shown = await setup();
  const link200 = new Node('a'); link200.hidden = true;
  assert.equal(await shown.mod.namespace.probeLogs(shown.api, link200), true);
  assert.equal(link200.hidden, false);
});

test('probe and mount stay idle when status says logs are off', async () => {
  const off = await setup();
  off.api.get = async () => ({ enabled: false, capacity: 0, retained: 0, dropped: 0, sources: [] });
  const link = new Node('a');
  link.hidden = true;
  assert.equal(await off.mod.namespace.probeLogs(off.api, link), false);
  assert.equal(link.hidden, true);

  await off.mod.namespace.mount(off.container, { api: off.api }, off.deps);
  assert.equal(FakeSource.all.length, 0);
  const status = off.container.children.find((child) => child.className === 'logs-status');
  assert.equal(status.textContent, 'Logs are not enabled for this session.');
});

test('mount forwards snapshot, log and gap events to the viewer', async () => {
  const t = await setup();
  await t.mod.namespace.mount(t.container, { api: t.api }, t.deps);
  const source = FakeSource.all[0];
  assert.equal(source.url, '/api/logs/stream');
  source.emit('snapshot', { entries: [{ seq: 1, message: 'a' }, { seq: 2, message: 'b' }] });
  source.emit('log', { seq: 3, message: 'c' });
  source.emit('gap', { missed: 7 });
  assert.equal(t.pushed.length, 4);
  assert.equal(t.pushed.slice(0, 3).map(e => e.message).join(','), 'a,b,c');
  assert.equal(t.pushed[3].message, '7 log lines were dropped from the buffer');
});

test('entries already shown are not pushed again after a reconnect', async () => {
  const t = await setup();
  await t.mod.namespace.mount(t.container, { api: t.api }, t.deps);
  FakeSource.all[0].emit('log', { seq: 5, message: 'five' });
  FakeSource.all[0].readyState = 2;
  FakeSource.all[0].listeners.get('error')();
  t.timers[0].fn();
  FakeSource.all[1].emit('snapshot', { entries: [{ seq: 4, message: 'old' }, { seq: 5, message: 'five' }, { seq: 6, message: 'six' }] });
  assert.equal(t.pushed.map(e => e.message).join(','), 'five,six');
});

test('a recreated stream that skips evicted entries reports the gap once', async () => {
  const t = await setup();
  await t.mod.namespace.mount(t.container, { api: t.api }, t.deps);
  FakeSource.all[0].emit('log', { seq: 5, message: 'five' });
  FakeSource.all[0].readyState = 2;
  FakeSource.all[0].listeners.get('error')();
  t.timers[0].fn();
  FakeSource.all[1].emit('snapshot', { entries: [{ seq: 9, message: 'nine' }] });
  const warnings = t.pushed.filter(e => e.level === 'warn');
  assert.equal(warnings.length, 1);
  assert.equal(warnings[0].message, '3 log lines were dropped from the buffer');
  assert.equal(t.pushed.map(e => e.message).includes('nine'), true);
});

test('a gap of one line uses the singular and a zero gap is silent', async () => {
  const t = await setup();
  await t.mod.namespace.mount(t.container, { api: t.api }, t.deps);
  const source = FakeSource.all[0];
  source.emit('log', { seq: 1, message: 'one' });
  source.emit('gap', { missed: 1 });
  source.emit('gap', { missed: 0 });
  const warnings = t.pushed.filter(e => e.level === 'warn');
  assert.equal(warnings.length, 1);
  assert.equal(warnings[0].message, '1 log line was dropped from the buffer');
});

test('the first connection reports no gap even when the first seq is high', async () => {
  const t = await setup();
  await t.mod.namespace.mount(t.container, { api: t.api }, t.deps);
  FakeSource.all[0].emit('snapshot', { entries: [{ seq: 900, message: 'late' }] });
  assert.equal(t.pushed.filter(e => e.level === 'warn').length, 0);
});

test('reconnect uses capped exponential backoff and resets on open', async () => {
  const t = await setup();
  await t.mod.namespace.mount(t.container, { api: t.api }, t.deps);
  const delays = [];
  for (let i = 0; i < 7; i++) {
    const source = FakeSource.all.at(-1);
    source.readyState = 2;
    source.listeners.get('error')();
    delays.push(t.timers.at(-1).ms);
    t.timers.at(-1).fn();
  }
  assert.equal(delays.join(','), '1000,2000,4000,8000,15000,15000,15000');
  FakeSource.all.at(-1).listeners.get('open')();
  FakeSource.all.at(-1).readyState = 2;
  FakeSource.all.at(-1).listeners.get('error')();
  assert.equal(t.timers.at(-1).ms, 1000);
});

test('unmount closes the stream and destroys the viewer', async () => {
  const t = await setup();
  const mounted = await t.mod.namespace.mount(t.container, { api: t.api }, t.deps);
  mounted.unmount();
  assert.equal(FakeSource.all[0].closed, true);
  assert.equal(t.state.destroyed, true);
});

test('mount says logs are off when status is 404', async () => {
  const t = await setup({ failure: notFound() });
  await t.mod.namespace.mount(t.container, { api: t.api }, t.deps);
  assert.equal(FakeSource.all.length, 0);
  assert.ok(t.container.textContent.includes('not enabled'));
});

test('viewer is created with follow on and the sources from status', async () => {
  const t = await setup();
  await t.mod.namespace.mount(t.container, { api: t.api }, t.deps);
  assert.equal(t.created[0].follow, true);
  assert.equal('followTail' in t.created[0], false);
  assert.equal(t.created[0].sources.join(','), 'core,aws-public');
});

test('a snapshot with new sources refreshes the viewer sources', async () => {
  const t = await setup();
  await t.mod.namespace.mount(t.container, { api: t.api }, t.deps);
  FakeSource.all[0].emit('snapshot', { entries: [], sources: [{ name: 'core' }, { name: 'aws-public' }, { name: 'kubernetes' }] });
  assert.equal(t.state.sources.join(','), 'core,aws-public,kubernetes');
});
