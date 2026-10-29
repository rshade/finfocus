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
  dispatch(k, extra = {}) { this.listeners.get(k)?.({ target: this, preventDefault() {}, ...extra }); }
  querySelector(selector) { return this.querySelectorAll(selector)[0]; }
  querySelectorAll(selector) {
    const match = n => selector[0] === '#' ? n.id === selector.slice(1) : selector[0] === '.' ? n.className?.split(' ').includes(selector.slice(1)) : n.tagName === selector;
    return this.children.filter(c => typeof c !== 'string').flatMap(c => [...(match(c) ? [c] : []), ...c.querySelectorAll(selector)]);
  }
}
const flush = () => new Promise(resolve => setImmediate(resolve));

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
  const mod = new vm.SourceTextModule(await readFile(new URL('../static/views/carbon.js', import.meta.url), 'utf8'), { context });
  await mod.link(() => app); await mod.evaluate();
  const calls = [];
  const api = { post(path, body, options) { return new Promise((resolve, reject) => calls.push({ path, body, options, resolve, reject })); } };
  const container = document.createElement('div'); document.append(container);
  const mounted = mod.namespace.mount(container, { api });
  const field = id => container.querySelector(`#${id}`);
  const submit = (values = {}) => {
    const defaults = { 'carbon-start': '2026-01-01', 'carbon-end': '2026-04-01', 'carbon-method': 'market-based', 'carbon-scope': 'total', 'carbon-accounts': '', 'carbon-regions': '', 'carbon-services': '' };
    for (const [id, value] of Object.entries({ ...defaults, ...values })) field(id).value = value;
    container.querySelector('form').dispatch('submit');
  };
  return { container, calls, mounted, field, submit };
}

const request = { start: '2026-01-01T00:00:00Z', end: '2026-04-01T00:00:00Z', method: 'CARBON_ACCOUNTING_METHOD_MARKET_BASED', scope: 'CARBON_EMISSIONS_SCOPE_TOTAL', regions: ['us-east-1'] };
const record = (overrides = {}) => ({ id: 'r1', provider: 'aws', source: 'aws-customer-carbon-footprint-tool', accountId: '111122223333', region: 'us-east-1', service: 'EC2', start: '2026-01-01T00:00:00Z', end: '2026-02-01T00:00:00Z', method: request.method, scope: request.scope, modelVersion: 'v3.0.0', value: 12.5, unit: 'gCO2e', retrievedAt: '2026-03-02T10:00:00Z', ...overrides });

test('carbon form posts the selectors and shows the server total without client arithmetic', async () => {
  const t = await setup();
  assert.equal(t.calls.length, 0);
  t.submit({ 'carbon-method': 'location-based', 'carbon-scope': '2', 'carbon-regions': ' us-east-1 , eu-west-1,, ', 'carbon-services': 'EC2' });
  assert.equal(t.calls[0].path, '/api/carbon/actual/query');
  assert.equal(JSON.stringify(t.calls[0].body), JSON.stringify({ start: '2026-01-01', end: '2026-04-01', method: 'location-based', scope: '2', regions: ['us-east-1', 'eu-west-1'], services: ['EC2'] }));
  assert.match(t.container.textContent, /Loading carbon report/);
  t.calls[0].resolve({ request, records: [record(), record({ id: 'r2', value: 0, region: '' })], totalGrams: 99.25 });
  await flush();
  const text = t.container.textContent;
  assert.match(text, /Sum of returned available records:99\.25 gCO2e/);
  assert.match(text, /2026-01-01T00:00:00Z to 2026-04-01T00:00:00Z \(exclusive\)/);
  assert.match(text, /aws-customer-carbon-footprint-tool/);
  assert.match(text, /v3\.0\.0/);
  assert.match(text, /2026-03-02T10:00:00Z/);
  assert.match(text, /2 records returned, 0 with missing values/);
  t.mounted.unmount();
});

test('carbon table distinguishes missing data from explicit zero', async () => {
  const t = await setup();
  t.submit();
  t.calls[0].resolve({ request, records: [record({ id: 'zero', value: 0 }), record({ id: 'gap', value: undefined })], totalGrams: 0 });
  await flush();
  const cells = t.container.querySelectorAll('.col-num');
  assert.equal(cells.length, 1);
  assert.equal(cells[0].textContent, '0 gCO2e');
  assert.equal(t.container.querySelectorAll('.carbon-missing').length, 1);
  assert.match(t.container.querySelector('.carbon-missing').textContent, /Missing/);
  assert.match(t.container.textContent, /2 records returned, 1 with missing values/);
  t.mounted.unmount();
});

test('carbon empty report keeps the query context and reports an unavailable total', async () => {
  const t = await setup();
  t.submit();
  t.calls[0].resolve({ request, records: [], totalGrams: null });
  await flush();
  const text = t.container.textContent;
  assert.match(text, /us-east-1/);
  assert.match(text, /The total is unavailable, not zero/);
  assert.match(text, /No emissions records match this query/);
  assert.equal(t.container.querySelector('.table-wrap').hidden, true);
  assert.equal(t.container.querySelector('.carbon-context').hidden, false);
  t.mounted.unmount();
});

test('carbon failure clears earlier results and shows the safe server message', async () => {
  const t = await setup();
  t.submit();
  t.calls[0].resolve({ request, records: [record()], totalGrams: 12.5 });
  await flush();
  t.submit();
  assert.equal(t.container.querySelector('.table-wrap').hidden, true);
  const message = 'Historical carbon reporting is disabled or unsupported. Install aws-ce and enable FINFOCUS_AWS_CARBON_ENABLED=true or plugins.aws-ce.carbon_enabled=true, then restart.';
  t.calls[1].reject(Object.assign(new Error(message), { status: 503, code: 'carbon_disabled' }));
  await flush();
  const alert = t.container.querySelector('#carbon-error');
  assert.equal(alert.hidden, false);
  assert.equal(alert.textContent, message);
  assert.equal(t.container.querySelector('.carbon-context').hidden, true);
  assert.doesNotMatch(t.container.textContent, /aws-customer-carbon-footprint-tool/);
  assert.match(t.container.textContent, /Unable to load the carbon report/);
  t.mounted.unmount();
});

test('carbon new query supersedes a pending one and unmount cancels it', async () => {
  const t = await setup();
  t.submit();
  const first = t.calls[0].options.signal;
  t.submit({ 'carbon-end': '2026-03-01' });
  assert.equal(first.aborted, true);
  t.calls[0].resolve({ request, records: [record({ id: 'stale' })], totalGrams: 1 });
  await flush();
  assert.doesNotMatch(t.container.textContent, /stale/);
  t.mounted.unmount();
  assert.equal(t.calls[1].options.signal.aborted, true);
});
