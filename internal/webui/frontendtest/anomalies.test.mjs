import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

class Node {
  constructor(tag, doc) { this.tagName = tag; this.doc = doc; this.children = []; this.listeners = new Map(); this.attrs = {}; this.dataset = {}; this.hidden = false; this.value = ''; }
  append(...children) { for (const child of children) { if (typeof child !== 'string') child.parent = this; this.children.push(child); } }
  replaceChildren(...children) { this.children = []; this.append(...children); }
  set textContent(text) { this.text = String(text); this.children = []; }
  get textContent() { return (this.text ?? '') + this.children.map(c => typeof c === 'string' ? c : c.textContent).join(''); }
  setAttribute(k, v) { this.attrs[k] = String(v); if (k === 'id') this.id = v; if (k === 'class') this.className = v; }
  addEventListener(k, fn) { this.listeners.set(k, fn); }
  querySelector(selector) { return this.querySelectorAll(selector)[0]; }
  querySelectorAll(selector) {
    const match = n => selector[0] === '#' ? n.id === selector.slice(1) : selector[0] === '.' ? n.className?.split(' ').includes(selector.slice(1)) : n.tagName === selector;
    return this.children.filter(c => typeof c !== 'string').flatMap(c => [...(match(c) ? [c] : []), ...c.querySelectorAll(selector)]);
  }
}
const flush = () => new Promise(resolve => setImmediate(resolve));
const viewURL = new URL('../static/views/anomalies.js', import.meta.url);

async function setup() {
  const document = new Node('document'); document.doc = document; document.createElement = tag => new Node(tag, document);
  const context = vm.createContext({ document, AbortController, console });
  const app = new vm.SyntheticModule(['el'], function () {
    this.setExport('el', (tag, attrs = {}, ...children) => {
      const node = document.createElement(tag);
      for (const [k, v] of Object.entries(attrs)) {
        if (k === 'text') node.textContent = v; else if (k === 'class') node.className = v; else if (k === 'dataset') Object.assign(node.dataset, v); else node.setAttribute(k, v);
      }
      node.append(...children.flat());
      return node;
    });
  }, { context });
  await app.link(() => {}); await app.evaluate();
  const mod = new vm.SourceTextModule(await readFile(viewURL, 'utf8'), { context });
  await mod.link(() => app); await mod.evaluate();
  const calls = [];
  const api = { get(path, options) { return new Promise((resolve, reject) => calls.push({ path, options, resolve, reject })); } };
  const handlers = new Map();
  const stream = {
    on(name, fn) { if (!handlers.has(name)) handlers.set(name, new Set()); handlers.get(name).add(fn); return () => handlers.get(name).delete(fn); },
    emit(name, payload) { for (const fn of [...(handlers.get(name) ?? [])]) fn(payload); },
    count(name) { return handlers.get(name)?.size ?? 0; },
  };
  const container = document.createElement('div'); document.append(container);
  const mounted = mod.namespace.mount(container, { api, stream });
  return { container, calls, mounted, stream };
}

const anomaly = (overrides = {}) => ({
  resource: 'urn:pulumi:prod::app::aws:ec2/instance:Instance::web', type: 'aws:ec2/instance:Instance',
  period: { start: '2026-10-01T00:00:00Z', end: '2026-10-11T00:00:00Z' },
  baseline: 720, observed: 1080, deviation: 360, percentDrift: 50, severity: 'warning', method: 'drift', currency: 'USD',
  display: { baseline: '$720.00', observed: '$1,080.00', deviation: '+$360.00', drift: '+50.0%', period: 'Oct 1 – Oct 11, 2026' },
  ...overrides,
});
const page = (anomalies, considered = 10) => ({ considered, summary: `${anomalies.length} of ${considered} resources`, anomalies });
const cells = row => row.querySelectorAll('td').map(c => c.textContent);

test('anomalies view fetches on mount and shows a loading state', async () => {
  const t = await setup();
  assert.equal(t.calls.length, 1);
  assert.equal(t.calls[0].path, '/api/overview/anomalies');
  assert.match(t.container.textContent, /Loading anomalies/);
  assert.equal(t.container.querySelector('.table-wrap').hidden, true);
  t.mounted.unmount();
});

test('anomalies table renders the server strings verbatim and does no arithmetic', async () => {
  const t = await setup();
  t.calls[0].resolve(page([
    anomaly(),
    anomaly({ resource: 'urn::db', type: 'aws:rds/instance:Instance', currency: 'EUR', display: { baseline: '€1.00', observed: 'obs-text', deviation: 'dev-text', drift: 'drift-text', period: '' } }),
  ], 7));
  await flush();
  const rows = t.container.querySelectorAll('tbody')[0].querySelectorAll('tr');
  assert.equal(rows.length, 2);
  assert.deepEqual(cells(rows[0]), ['urn:pulumi:prod::app::aws:ec2/instance:Instance::web', 'aws:ec2/instance:Instance', 'Oct 1 – Oct 11, 2026', '$720.00', '$1,080.00', '+$360.00', '+50.0%', 'USD']);
  assert.deepEqual(cells(rows[1]).slice(3, 7), ['€1.00', 'obs-text', 'dev-text', 'drift-text']);
  assert.equal(cells(rows[1])[2], '—', 'a missing period renders a dash');
  assert.match(t.container.textContent, /2 of 7 resources/);
  assert.equal(t.container.querySelector('.table-wrap').hidden, false);
  t.mounted.unmount();
});

test('anomalies numeric columns are marked for right alignment', async () => {
  const t = await setup();
  t.calls[0].resolve(page([anomaly()]));
  await flush();
  const row = t.container.querySelectorAll('tbody')[0].querySelectorAll('tr')[0];
  const numeric = row.querySelectorAll('.col-num');
  assert.deepEqual(numeric.map(c => c.textContent), ['$720.00', '$1,080.00', '+$360.00', '+50.0%']);
  const headerCells = t.container.querySelectorAll('th').filter(h => h.className === 'col-num');
  assert.deepEqual(headerCells.map(h => h.textContent), ['Baseline', 'Observed', 'Deviation', 'Drift %']);
  t.mounted.unmount();
});

test('anomalies empty result says nothing drifted and keeps the considered count', async () => {
  const t = await setup();
  t.calls[0].resolve(page([], 49));
  await flush();
  assert.match(t.container.textContent, /No anomalies found \(0 of 49 resources\)/);
  assert.equal(t.container.querySelector('.table-wrap').hidden, true);
  assert.equal(t.container.querySelector('.error-banner').hidden, true);
  t.mounted.unmount();
});

test('anomalies not-ready answer waits quietly and the ready event loads the list', async () => {
  const t = await setup();
  t.calls[0].reject(Object.assign(new Error('overview data is loading'), { status: 503, code: 'not_ready' }));
  await flush();
  assert.match(t.container.textContent, /Waiting for stack data/);
  assert.equal(t.container.querySelector('.error-banner').hidden, true);
  assert.equal(t.container.querySelector('.table-wrap').hidden, true);
  t.stream.emit('ready', {});
  assert.equal(t.calls.length, 2);
  t.calls[1].resolve(page([anomaly()]));
  await flush();
  assert.match(t.container.textContent, /1 of 10 resources/);
  t.mounted.unmount();
});

test('anomalies failure shows an alert and does not keep the old table as current', async () => {
  const t = await setup();
  t.calls[0].resolve(page([anomaly()]));
  await flush();
  t.stream.emit('ready', {});
  t.calls[1].reject(Object.assign(new Error('overview anomalies unavailable'), { status: 500 }));
  await flush();
  const alert = t.container.querySelector('.error-banner');
  assert.equal(alert.hidden, false);
  assert.equal(alert.attrs.role, 'alert');
  assert.equal(alert.textContent, 'overview anomalies unavailable');
  assert.equal(t.container.querySelector('.table-wrap').hidden, true);
  assert.match(t.container.textContent, /Unable to load anomalies/);
  t.mounted.unmount();
});

test('anomalies refresh on ready and on a ready snapshot only', async () => {
  const t = await setup();
  t.calls[0].resolve(page([]));
  await flush();
  t.stream.emit('snapshot', { ready: false });
  assert.equal(t.calls.length, 1);
  t.stream.emit('snapshot', { ready: true });
  assert.equal(t.calls.length, 2);
  t.stream.emit('ready', {});
  assert.equal(t.calls.length, 3);
  t.mounted.unmount();
});

test('anomalies ignore a response from a superseded request', async () => {
  const t = await setup();
  t.stream.emit('ready', {});
  assert.equal(t.calls.length, 2);
  assert.equal(t.calls[0].options.signal.aborted, true, 'the older request is aborted');
  t.calls[1].resolve(page([anomaly({ resource: 'urn::new' })]));
  await flush();
  t.calls[0].resolve(page([anomaly({ resource: 'urn::stale' })]));
  await flush();
  const text = t.container.textContent;
  assert.match(text, /urn::new/);
  assert.doesNotMatch(text, /urn::stale/);
  t.mounted.unmount();
});

test('anomalies unmount unsubscribes and drops a late response', async () => {
  const t = await setup();
  assert.equal(t.stream.count('ready'), 1);
  assert.equal(t.stream.count('snapshot'), 1);
  t.mounted.unmount();
  assert.equal(t.stream.count('ready'), 0);
  assert.equal(t.stream.count('snapshot'), 0);
  t.calls[0].resolve(page([anomaly()]));
  await flush();
  assert.doesNotMatch(t.container.textContent, /urn:pulumi/);
});

test('anomalies page keeps the accessible table structure', async () => {
  const t = await setup();
  t.calls[0].resolve(page([anomaly()]));
  await flush();
  assert.equal(t.container.querySelector('caption').textContent, 'Cost anomalies');
  const headers = t.container.querySelectorAll('th');
  assert.deepEqual(headers.map(h => h.textContent), ['Resource', 'Type', 'Period', 'Baseline', 'Observed', 'Deviation', 'Drift %', 'Currency']);
  assert.ok(headers.every(h => h.attrs.scope === 'col'));
  assert.equal(t.container.querySelector('.table-wrap').attrs.tabindex, '0');
  assert.ok(t.container.querySelectorAll('p').some(p => p.attrs.role === 'status'));
  t.mounted.unmount();
});

test('anomalies view builds nodes from text only and formats nothing itself', async () => {
  const source = await readFile(viewURL, 'utf8');
  assert.doesNotMatch(source, /innerHTML|insertAdjacentHTML|outerHTML/);
  assert.doesNotMatch(source, /toFixed|toLocale|Intl\.|Math\.|parseFloat|Number\(/);
});
