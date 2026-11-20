import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

class Node {
  constructor(tag, doc) { this.tagName = tag; this.doc = doc; this.children = []; this.listeners = new Map(); this.attrs = {}; this.dataset = {}; this.hidden = false; this.value = ''; this.open = false; }
  append(...children) { for (const child of children) { if (typeof child !== 'string') child.parent = this; this.children.push(child); } }
  replaceChildren(...children) { this.children = []; this.append(...children); }
  set textContent(text) { this.text = String(text); this.children = []; }
  get textContent() { return (this.text ?? '') + this.children.map(c => typeof c === 'string' ? c : c.textContent).join(''); }
  setAttribute(k, v) { this.attrs[k] = String(v); if (k === 'id') this.id = v; if (k === 'class') this.className = v; }
  getAttribute(k) { return this.attrs[k]; }
  addEventListener(k, fn) { this.listeners.set(k, fn); }
  dispatch(k, extra = {}) { return this.listeners.get(k)?.({ target: this, preventDefault() {}, ...extra }); }
  querySelector(selector) { return this.querySelectorAll(selector)[0]; }
  querySelectorAll(selector) {
    const match = n => selector[0] === '#' ? n.id === selector.slice(1) : selector[0] === '.' ? n.className?.split(' ').includes(selector.slice(1)) : n.tagName === selector;
    return this.children.filter(c => typeof c !== 'string').flatMap(c => [...(match(c) ? [c] : []), ...c.querySelectorAll(selector)]);
  }
}
const flush = () => new Promise(resolve => setImmediate(resolve));

function apiError(status, code, message) {
  return Object.assign(new Error(message), { status, code });
}

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
  const mod = new vm.SourceTextModule(await readFile(new URL('../static/views/cluster.js', import.meta.url), 'utf8'), { context });
  await mod.link(() => app); await mod.evaluate();
  const calls = [];
  const navigated = [];
  const api = { post(path, body, options) { return new Promise((resolve, reject) => calls.push({ path, body, options, resolve, reject })); } };
  const container = document.createElement('div'); document.append(container);
  const mounted = mod.namespace.mount(container, { api, navigate: name => navigated.push(name) });
  const field = id => container.querySelector(`#${id}`);
  field('cluster-group-by').value = 'namespace'; // a browser select starts on its first option
  const submit = async (values = {}) => {
    for (const [id, value] of Object.entries(values)) field(id).value = value;
    field('cluster-group-by').dispatch('change');
    container.querySelector('form').dispatch('submit');
    await flush(); await flush();
  };
  const answer = async (call, doc) => { call.resolve(doc); await flush(); await flush(); };
  const fail = async (call, err) => { call.reject(err); await flush(); await flush(); };
  const text = selector => container.querySelector(selector).textContent;
  const rows = selector => container.querySelector(selector).querySelector('tbody').querySelectorAll('tr').map(tr => tr.children.map(c => c.textContent));
  return { container, calls, navigated, mounted, field, submit, answer, fail, text, rows };
}

const display = (extra = {}) => ({
  total: '$300.00 USD', idle: '$60.00 USD', idleShare: '20.0%', mode: 'run-rate (monthly, 730 h)',
  policy: 'built-in defaults · abc123def456',
  groups: [
    { key: 'payments', cpu: '$120.00 USD', memory: '$120.00 USD', total: '$240.00 USD', share: '80.0%' },
    { key: '__idle__', cpu: '$30.00 USD', memory: '$30.00 USD', total: '$60.00 USD', share: '20.0%' },
  ],
  capacity: [{ capacityType: 'unlabeled', total: '$300.00 USD', share: '100.0%', idle: '$60.00 USD' }],
  warnings: [], incompleteCauses: [], unpriced: [],
  ...extra,
});
const report = (extra = {}, displayExtra = {}) => ({
  status: 'ok', mode: 'run-rate', period: 'monthly', currency: 'USD', group_by: 'namespace',
  total: 300, idle: 60, incomplete: false,
  groups: [{ key: 'payments', notes: ['3 pods'] }, { key: '__idle__' }],
  capacity: [{ capacity_type: 'unlabeled', nodes: 2 }],
  display: display(displayExtra),
  ...extra,
});

test('a query sends only the fields that are set and renders the server strings in order', async () => {
  const t = await setup();
  assert.equal(t.calls.length, 0, 'nothing is sent before Run');
  t.field('cluster-policy-file').files = [{ text: async () => '{ "idle": "share" }' }];
  await t.submit({ 'cluster-context': ' prod ', 'cluster-selector': 'team=payments, ,tier=web', 'cluster-group-by': 'label', 'cluster-label-key': ' app ', 'cluster-from': '2026-11-01' });
  assert.equal(t.calls[0].path, '/api/cluster/query');
  assert.equal(JSON.stringify(t.calls[0].body), JSON.stringify({
    groupBy: 'label:app', context: 'prod', from: '2026-11-01', selector: ['team=payments', 'tier=web'], policy: '{ "idle": "share" }',
  }));
  assert.equal(t.field('cluster-status').dataset.state, 'loading');

  await t.answer(t.calls[0], report({ group_by: 'label:app' }));
  assert.equal(t.text('.totals-bar'), 'Total: $300.00 USDIdle: $60.00 USDIdle share: 20.0%Mode: run-rate (monthly, 730 h)Policy: built-in defaults · abc123def456');
  assert.equal(t.text('caption'), 'Cost by label:app');
  assert.deepEqual(t.rows('.cluster-table'), [
    ['payments', '$120.00 USD', '$120.00 USD', '$240.00 USD', '80.0%', '3 pods'],
    ['__idle__', '$30.00 USD', '$30.00 USD', '$60.00 USD', '20.0%', ''],
  ]);
  assert.equal(t.container.querySelector('.cluster-capacity-table').parent.hidden, true, 'only unlabeled capacity hides the capacity table');
  assert.equal(t.container.querySelector('.cluster-warnings').hidden, true);
  assert.equal(t.container.querySelector('.cluster-incomplete').hidden, true);
  assert.equal(t.text('#cluster-status'), '2 groups allocated.');
  assert.equal(t.container.querySelector('.cluster-table').getAttribute('aria-busy'), 'false');
});

test('the default query sends only the group-by', async () => {
  const t = await setup();
  await t.submit();
  assert.equal(JSON.stringify(t.calls[0].body), JSON.stringify({ groupBy: 'namespace' }));
  assert.equal(t.field('cluster-label-key').parent.hidden, true, 'label key appears only for label grouping');
});

test('a scoped, incomplete report shows scope, labeled capacity, warnings, and causes', async () => {
  const t = await setup();
  await t.submit({ 'cluster-namespace': 'payments' });
  await t.answer(t.calls[0], report(
    { incomplete: true, capacity: [{ capacity_type: 'spot', nodes: 1, notes: ['interruptible'] }, { capacity_type: 'on-demand', nodes: 2 }] },
    {
      idle: undefined, idleShare: undefined, scope: 'namespace',
      capacity: [
        { capacityType: 'spot', total: '$100.00 USD', share: '33.3%' },
        { capacityType: 'on-demand', total: '$200.00 USD', share: '66.7%', idle: '$10.00 USD' },
      ],
      warnings: ['node n1: no usage'], incompleteCauses: ['cpu usage for 1 pod'], unpriced: ['node n2: no price'],
    }));
  assert.equal(t.text('.totals-bar'), 'Total: $300.00 USDIdle: omittedScoped by: namespaceMode: run-rate (monthly, 730 h)Policy: built-in defaults · abc123def456');
  assert.equal(t.container.querySelector('.cluster-capacity-table').parent.hidden, false);
  assert.deepEqual(t.rows('.cluster-capacity-table'), [
    ['spot', '1', '$100.00 USD', '33.3%', '-', 'interruptible'],
    ['on-demand', '2', '$200.00 USD', '66.7%', '$10.00 USD', ''],
  ]);
  assert.equal(t.text('.cluster-warnings'), 'Warningsnode n1: no usage');
  assert.equal(t.text('.cluster-incomplete'), 'Incomplete reportThe usage source reported partial data:cpu usage for 1 podCould not be priced:node n2: no price');
  assert.equal(t.text('#cluster-status'), '2 groups allocated. The report is incomplete.');
});

test('an incomplete report with no causes points at the warnings', async () => {
  const t = await setup();
  await t.submit();
  await t.answer(t.calls[0], report({ incomplete: true }, { warnings: ['usage window truncated'] }));
  assert.equal(t.text('.cluster-incomplete'), 'Incomplete reportThe warnings above explain what is missing.');
});

test('the unavailable answer is an explanation naming the capability, not an error', async () => {
  const t = await setup();
  await t.submit();
  await t.answer(t.calls[0], {
    status: 'unavailable', code: 'cluster_plugin_missing', capability: 'usage_stats',
    error: 'no installed plugin provides usage_stats; install one with: finfocus plugin install kubernetes',
  });
  const panel = t.container.querySelector('.cluster-unavailable');
  assert.equal(panel.hidden, false);
  assert.equal(panel.querySelector('code').textContent, 'usage_stats');
  assert.ok(panel.textContent.includes('install one with: finfocus plugin install kubernetes'));
  assert.equal(t.field('cluster-error').hidden, true, 'no error banner');
  assert.equal(t.field('cluster-status').dataset.state, 'unavailable');
  assert.equal(t.container.querySelector('.totals-bar').hidden, true);
  panel.querySelector('a').dispatch('click');
  assert.deepEqual(t.navigated, ['plugins']);
});

test('a 400 shows the message as written and reveals only the named plugin choice', async () => {
  const t = await setup();
  assert.equal(t.field('cluster-usage-source').parent.hidden, true);
  assert.equal(t.field('cluster-allocator').parent.hidden, true);
  await t.submit();
  const message = 'several usage sources can serve run-rate usage (kubernetes, prometheus); choose one with --usage-source';
  await t.fail(t.calls[0], apiError(400, 'invalid_cluster_query', message));
  assert.equal(t.text('#cluster-status'), message);
  assert.equal(t.field('cluster-status').dataset.state, 'invalid');
  assert.equal(t.field('cluster-error').hidden, true);
  assert.equal(t.field('cluster-usage-source').parent.hidden, false);
  assert.equal(t.field('cluster-allocator').parent.hidden, true);

  await t.submit({ 'cluster-usage-source': 'prometheus' });
  assert.equal(t.calls[1].body.usageSource, 'prometheus');
  await t.fail(t.calls[1], apiError(400, 'invalid_cluster_query', 'several plugins provide allocation (a, b); choose one with --allocator'));
  assert.equal(t.field('cluster-allocator').parent.hidden, false);
  assert.equal(t.field('cluster-usage-source').parent.hidden, false, 'a revealed field stays');
});

test('a server failure uses the error banner and clears earlier results', async () => {
  const t = await setup();
  await t.submit();
  await t.answer(t.calls[0], report());
  await t.submit();
  assert.equal(t.container.querySelector('.totals-bar').hidden, true, 'a new run clears the previous report');
  await t.fail(t.calls[1], apiError(502, 'cluster_unavailable', 'cluster data unavailable'));
  assert.equal(t.field('cluster-error').hidden, false);
  assert.equal(t.text('#cluster-error'), 'cluster data unavailable');
  assert.equal(t.text('#cluster-status'), 'Unable to run the allocation.');
});

test('a superseded or unmounted query is ignored', async () => {
  const t = await setup();
  await t.submit();
  await t.submit();
  assert.equal(t.calls[0].options.signal.aborted, true);
  await t.answer(t.calls[0], report({ group_by: 'stale' }));
  assert.equal(t.container.querySelector('.totals-bar').hidden, true);
  t.mounted.unmount();
  assert.equal(t.calls[1].options.signal.aborted, true);
  await t.answer(t.calls[1], report());
  assert.equal(t.container.querySelector('.totals-bar').hidden, true);
});

test('the policy panel loads on open with the chosen allocator and policy text', async () => {
  const t = await setup();
  const panel = t.container.querySelector('.cluster-policy');
  panel.dispatch('toggle');
  assert.equal(t.calls.length, 0, 'closing or a closed panel sends nothing');
  t.field('cluster-policy-file').files = [{ text: async () => '{}' }];
  panel.open = true; panel.dispatch('toggle');
  await flush(); await flush();
  assert.equal(t.calls[0].path, '/api/cluster/policy');
  assert.equal(JSON.stringify(t.calls[0].body), JSON.stringify({ policy: '{}' }));
  await t.answer(t.calls[0], {
    status: 'ok', source: 'request policy', digest: 'abc123def4567890', effective: { idle: 'share', weights: { cpu: 0.5 } },
    display: { source: 'request policy', digest: 'abc123def456' },
  });
  assert.equal(panel.querySelector('dl').textContent, 'Sourcerequest policyDigestabc123def456');
  assert.equal(panel.querySelector('pre').textContent, '{\n  "idle": "share",\n  "weights": {\n    "cpu": 0.5\n  }\n}');
  assert.equal(panel.querySelector('.cluster-policy-status').textContent, 'Effective policy loaded.');
});

test('the policy panel explains unavailable and shows a 400 as written', async () => {
  const t = await setup();
  const panel = t.container.querySelector('.cluster-policy');
  panel.open = true; panel.dispatch('toggle');
  await flush(); await flush();
  await t.answer(t.calls[0], { status: 'unavailable', code: 'cluster_plugin_missing', capability: 'allocation', error: 'no installed plugin provides allocation' });
  assert.equal(panel.querySelector('.cluster-policy-status').textContent, 'The policy is unavailable: no installed plugin provides allocation');
  assert.equal(panel.querySelector('pre').hidden, true);

  panel.dispatch('toggle');
  await flush(); await flush();
  await t.fail(t.calls[1], apiError(400, 'invalid_cluster_query', 'policy: unknown field "idel"'));
  assert.equal(panel.querySelector('.cluster-policy-status').textContent, 'policy: unknown field "idel"');
  assert.equal(t.field('cluster-error').hidden, true, 'a policy failure stays in the panel');
});
