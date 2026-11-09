import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';
import { linkView } from './view_linker.mjs';

class Node {
  constructor(tag, doc) {
    this.tagName = tag;
    this.doc = doc;
    this.children = [];
    this.listeners = new Map();
    this.attrs = {};
    this.dataset = {};
    this.hidden = false;
    this.value = '';
  }

  append(...children) {
    for (const child of children) {
      if (child === undefined || child === null) continue;
      if (typeof child !== 'string') child.parent = this;
      this.children.push(child);
    }
  }

  replaceChildren(...children) {
    for (const child of this.children) if (typeof child !== 'string') child.parent = null;
    this.children = [];
    this.append(...children);
  }

  set textContent(text) {
    this.text = String(text);
    this.children = [];
  }

  get textContent() {
    return (this.text ?? '') + this.children.map((child) => (typeof child === 'string' ? child : child.textContent)).join('');
  }

  setAttribute(key, value) {
    this.attrs[key] = String(value);
    if (key === 'id') this.id = value;
    if (key === 'value') this.value = String(value);
    if (key === 'class') this.className = value;
  }

  removeAttribute(key) {
    delete this.attrs[key];
  }

  addEventListener(key, fn) {
    this.listeners.set(key, fn);
  }

  dispatch(key) {
    this.listeners.get(key)?.({ target: this, preventDefault() {} });
  }

  focus() {
    this.doc.activeElement = this;
  }

  showModal() {
    this.open = true;
  }

  close() {
    this.open = false;
    this.dispatch('close');
  }

  get isConnected() {
    return this === this.doc || Boolean(this.parent?.isConnected);
  }

  querySelector(selector) {
    return this.querySelectorAll(selector)[0];
  }

  querySelectorAll(selector) {
    const match = (node) => (selector[0] === '#'
      ? node.id === selector.slice(1)
      : selector[0] === '.'
        ? node.className?.split(' ').includes(selector.slice(1))
        : node.tagName === selector);
    return this.children.filter((child) => typeof child !== 'string').flatMap((child) => [
      ...(match(child) ? [child] : []),
      ...child.querySelectorAll(selector),
    ]);
  }
}

const flush = () => new Promise((resolve) => setImmediate(resolve));

function elFactory(document) {
  return (tag, attrs = {}, ...children) => {
    const node = document.createElement(tag);
    for (const [key, value] of Object.entries(attrs)) {
      if (value === undefined || value === null) continue;
      if (key === 'text') node.textContent = value;
      else if (key === 'class') node.className = value;
      else if (key === 'dataset') Object.assign(node.dataset, value);
      else node.setAttribute(key, value);
    }
    node.append(...children.flat());
    return node;
  };
}

async function setup() {
  const document = new Node('document');
  document.doc = document;
  document.createElement = (tag) => new Node(tag, document);
  document.createElementNS = (_ns, tag) => new Node(tag, document);
  const context = vm.createContext({
    document,
    AbortController,
    URLSearchParams,
    console,
    setTimeout,
    clearTimeout,
    Date,
    Math,
    Number,
    encodeURIComponent,
    Set,
  });
  const app = new vm.SyntheticModule(['el'], function exportEl() {
    this.setExport('el', elFactory(document));
  }, { context });
  await app.link(() => {});
  await app.evaluate();
  const mod = new vm.SourceTextModule(await readFile(new URL('../static/views/cost.js', import.meta.url), 'utf8'), { context });
  await linkView(mod, app, context, 'cost');
  await mod.evaluate();
  const calls = [];
  function request(path, body, options) {
    let resolve;
    let reject;
    const promise = new Promise((yes, no) => {
      resolve = yes;
      reject = no;
    });
    calls.push({ path, body, options, resolve, reject });
    return promise;
  }
  const api = {
    post: (path, body, options) => request(path, body, options),
    get: (path, options) => request(path, null, options),
  };
  const listeners = new Map();
  const stream = {
    on(name, fn) {
      const list = listeners.get(name) ?? [];
      list.push(fn);
      listeners.set(name, list);
      return () => listeners.set(name, list.filter((item) => item !== fn));
    },
  };
  const container = document.createElement('div');
  document.append(container);
  const mounted = mod.namespace.mount(container, {
    api,
    stream: { on: stream.on },
  });
  return {
    container,
    calls,
    mounted,
    document,
    emit: (name, data) => (listeners.get(name) ?? []).forEach((fn) => fn(data)),
  };
}

const costPage = (text) => ({
  rows: [{ id: text, type: 'aws:ec2:Instance', costDisplay: '$12.34', currency: 'USD', canDetail: true }],
  results: [],
  totalDisplay: '$12.34',
  summary: { currency: 'USD' },
  page: 1,
  totalPages: 1,
});

function historyCall(t) {
  return t.calls.find((call) => call.path.startsWith('/api/history/view'));
}

function reject(call, message, status, code, extra = {}) {
  call.reject(Object.assign(new Error(message), { status, code, ...extra }));
}

function formOf(node) {
  let form = node;
  while (form && form.tagName !== 'form') form = form.parent;
  return form;
}

const viewBody = {
  status: 'ok',
  budget: 120,
  currency_warning: 'Two currencies are stored.',
  snapshots: [
    { version: 35, timestamp: '2026-09-01T00:00:00Z', total_monthly: 8, currency: 'USD' },
    { version: 36, timestamp: '2026-10-01T00:00:00Z', total_monthly: 12.5, currency: 'USD' },
  ],
  annotations: [
    { version: 35, kind: 'update', message: 'baseline' },
    { version: 36, kind: 'update', message: 'resized' },
  ],
  display: {
    budget: 'BUDGET-TEXT',
    snapshots: [
      { version: 35, date: '2026-09-01', total: 'EIGHT' },
      { version: 36, date: '2026-10-01', total: 'TWELVE' },
    ],
  },
};

test('cost history panel follows the pager and keeps the cost query first', async () => {
  const t = await setup();
  assert.equal(t.calls[0].path, '/api/cost/actual/query');
  assert.equal(t.calls[1].path, '/api/history/view');
  const kids = t.container.children;
  const pager = kids.findIndex((node) => node.className === 'pager');
  const panel = kids.findIndex((node) => node.className === 'history-panel');
  const dialog = kids.findIndex((node) => node.tagName === 'dialog');
  assert.ok(pager < panel && panel < dialog);
  assert.equal(t.container.querySelector('#cost-history-panel-title').textContent, 'Cost history');
  assert.match(t.container.querySelector('#history-from').parent.textContent, /From/);
  assert.equal(t.container.querySelector('.history-diff').hidden, true);
  t.mounted.unmount();
  assert.equal(t.calls[1].options.signal.aborted, true);
});

test('no history is a hint and hides the chart and diff', async () => {
  const t = await setup();
  historyCall(t).resolve({ status: 'no_history' });
  await flush();
  assert.match(t.container.querySelector('#history-status').textContent, /finfocus cost history collect/);
  assert.equal(t.container.querySelector('.history-chart'), undefined);
  assert.equal(t.container.querySelector('.history-diff').hidden, true);
  assert.equal(t.container.querySelector('#history-alert').hidden, true);
  t.mounted.unmount();
});

test('display strings, budget line, annotations, and increase wording come from the server', async () => {
  const t = await setup();
  historyCall(t).resolve(viewBody);
  await flush();
  const diff = t.calls.at(-1);
  assert.equal(diff.path, '/api/history/diff?from=v35&to=v36');
  diff.resolve({
    status: 'ok',
    unchanged_count: 1,
    added: [],
    removed: [{ urn: 'urn:old', type: 'aws:s3:Bucket', monthly_cost: 2, delta: -2 }],
    changed: [{ urn: 'urn:res', type: 'aws:ec2:Instance', monthly_cost: 12.5, delta: 4.5 }],
    display: {
      delta: '+FOUR',
      from: 'FROM-TEXT',
      to: 'TO-TEXT',
      unchangedTotal: 'UNCHANGED-TEXT',
      added: [],
      removed: [{ monthlyCost: 'TWO', delta: '-TWO' }],
      changed: [{ monthlyCost: 'TWELVE', delta: '+FOUR' }],
    },
  });
  await flush();
  const text = t.container.querySelector('.history-panel').textContent;
  assert.match(text, /BUDGET-TEXT/);
  assert.match(text, /Budget line: BUDGET-TEXT/);
  assert.match(text, /EIGHT/);
  assert.match(text, /TWELVE/);
  assert.match(text, /1\. v35 update: baseline/);
  assert.match(text, /2\. v36 update: resized/);
  assert.match(text, /Two currencies are stored/);
  assert.match(text, /\+FOUR increase/);
  assert.match(text, /-TWO decrease/);
  assert.match(text, /FROM-TEXT → TO-TEXT/);
  assert.match(text, /Unchanged: 1 \(UNCHANGED-TEXT\)/);
  assert.match(text, /Cost history series/);
  assert.doesNotMatch(text, /12\.5|\$8|4\.5/);
  assert.equal(t.container.querySelector('.history-chart').attrs['aria-hidden'], 'true');
  assert.equal(t.container.querySelector('.history-series').parent.attrs.tabindex, '0');
  const section = t.container.querySelector('.history-panel');
  const range = t.container.querySelector('#history-from').parent.parent;
  const slot = t.container.querySelector('.history-series-slot');
  const diffSection = t.container.querySelector('.history-diff');
  assert.ok(section.children.indexOf(range) < section.children.indexOf(slot));
  assert.ok(section.children.indexOf(slot) < section.children.indexOf(diffSection));
  assert.equal(t.container.querySelector('#history-diff-from').attrs.list, 'history-diff-options');
  t.mounted.unmount();
});

test('an empty snapshot range is an empty state, not an error', async () => {
  const t = await setup();
  historyCall(t).resolve({ status: 'ok', snapshots: [], display: { snapshots: [] } });
  await flush();
  assert.equal(t.container.querySelector('#history-status').textContent, 'No snapshots in this range.');
  assert.equal(t.container.querySelector('.history-series'), undefined);
  assert.equal(t.calls.some((call) => call.path.startsWith('/api/history/diff')), false);
  t.mounted.unmount();
});

test('a bad history query is shown beside the control and a 502 stays on the alert line', async () => {
  const t = await setup();
  reject(historyCall(t), 'snapshot version 9 not found', 400, 'invalid_history_query', { data: { path: '/secret/history.db' } });
  await flush();
  const field = t.container.querySelector('#history-query-error');
  assert.equal(field.hidden, false);
  assert.equal(field.textContent, 'snapshot version 9 not found');
  assert.equal(t.container.querySelector('#history-status').hidden, true);
  assert.equal(t.container.querySelector('#history-status').textContent, '');
  assert.equal(t.container.querySelector('#history-alert').hidden, true);
  assert.equal(t.container.querySelector('#history-from').attrs['aria-describedby'], 'history-query-error');
  assert.doesNotMatch(t.container.textContent, /secret|\[object Object\]/);
  const from = t.container.querySelector('#history-from');
  from.value = '2026-09-01';
  t.container.querySelector('#history-currency').value = '   ';
  let form = from.parent;
  while (form && form.tagName !== 'form') form = form.parent;
  form.dispatch('submit');
  const again = t.calls.at(-1);
  assert.match(again.path, /\/api\/history\/view\?from=2026-09-01$/);
  reject(again, 'cost history unavailable', 502, 'history_unavailable', { data: { path: '/secret/history.db' } });
  await flush();
  assert.equal(t.container.querySelector('#history-alert').hidden, false);
  assert.equal(t.container.querySelector('#history-alert').textContent, 'cost history unavailable');
  assert.equal(t.container.querySelector('#history-status').textContent, 'Cost history could not be loaded.');
  assert.equal(t.container.querySelector('.history-diff').hidden, true);
  assert.equal(t.container.querySelector('#history-query-error').hidden, true);
  assert.doesNotMatch(t.container.textContent, /secret|\[object Object\]/);
  t.mounted.unmount();
});

test('history retries a 503 when the cost view becomes ready and not after success', async () => {
  const t = await setup();
  reject(historyCall(t), 'cost history is loading', 503, 'not_ready');
  await flush();
  assert.equal(t.container.querySelector('#history-alert').textContent, 'cost history is loading');
  t.emit('ready', {});
  const views = t.calls.filter((call) => call.path.startsWith('/api/history/view'));
  assert.equal(views.length, 2);
  views[1].resolve({ status: 'no_history' });
  await flush();
  t.emit('ready', {});
  assert.equal(t.calls.filter((call) => call.path.startsWith('/api/history/view')).length, 2);
  t.mounted.unmount();
});

test('a recovered diff clears its failure and does not reload on the next ready', async () => {
  const t = await setup();
  historyCall(t).resolve(viewBody);
  await flush();
  reject(t.calls.at(-1), 'cost history unavailable', 502, 'history_unavailable');
  await flush();
  assert.equal(t.container.querySelector('#history-alert').hidden, false);
  formOf(t.container.querySelector('#history-diff-from')).dispatch('submit');
  const again = t.calls.at(-1);
  assert.match(again.path, /\/api\/history\/diff\?from=v35&to=v36/);
  again.resolve({
    status: 'ok',
    unchanged_count: 0,
    added: [],
    removed: [],
    changed: [],
    display: {
      delta: '+FOUR',
      from: 'FROM-TEXT',
      to: 'TO-TEXT',
      unchangedTotal: 'UNCHANGED-TEXT',
      added: [],
      removed: [],
      changed: [],
    },
  });
  await flush();
  assert.equal(t.container.querySelector('#history-alert').hidden, true);
  const views = t.calls.filter((call) => call.path.startsWith('/api/history/view')).length;
  t.emit('ready', {});
  assert.equal(t.calls.filter((call) => call.path.startsWith('/api/history/view')).length, views);
  t.mounted.unmount();
});

test('a view that is not ready hides the diff and still retries when the cost view is ready', async () => {
  const t = await setup();
  historyCall(t).resolve(viewBody);
  await flush();
  assert.equal(t.container.querySelector('.history-diff').hidden, false);
  const from = t.container.querySelector('#history-from');
  from.value = '2026-11-01';
  formOf(from).dispatch('submit');
  reject(t.calls.at(-1), 'cost history is loading', 503, 'not_ready');
  await flush();
  assert.equal(t.container.querySelector('.history-diff').hidden, true);
  assert.equal(t.container.querySelector('#history-status').textContent, 'Cost history could not be loaded.');
  const before = t.calls.filter((call) => call.path.startsWith('/api/history/view')).length;
  formOf(t.container.querySelector('#history-diff-from')).dispatch('submit');
  assert.equal(t.calls.filter((call) => call.path.startsWith('/api/history/view')).length, before);
  assert.equal(t.calls.at(-1).path.startsWith('/api/history/diff'), false);
  t.emit('ready', {});
  assert.equal(t.calls.filter((call) => call.path.startsWith('/api/history/view')).length, before + 1);
  t.mounted.unmount();
});

test('a late history view cannot replace a newer range', async () => {
  const t = await setup();
  const first = historyCall(t);
  const from = t.container.querySelector('#history-from');
  from.value = '2026-10-01';
  let form = from.parent;
  while (form && form.tagName !== 'form') form = form.parent;
  form.dispatch('submit');
  const second = t.calls.at(-1);
  assert.match(second.path, /from=2026-10-01/);
  second.resolve(viewBody);
  await flush();
  first.resolve({
    ...viewBody,
    display: { ...viewBody.display, snapshots: [{ version: 1, date: '2020-01-01', total: 'STALE-TOTAL' }], budget: 'STALE-BUDGET' },
    snapshots: [{ version: 1, timestamp: '2020-01-01T00:00:00Z', total_monthly: 1 }],
  });
  await flush();
  assert.match(t.container.textContent, /TWELVE/);
  assert.doesNotMatch(t.container.textContent, /STALE-TOTAL|STALE-BUDGET/);
  t.mounted.unmount();
});

test('cost detail renders resource history display strings and an empty series', async () => {
  const t = await setup();
  t.calls[0].resolve(costPage('urn:res'));
  await flush();
  t.container.querySelector('.detail-button').dispatch('click');
  const detail = t.calls.at(-1);
  assert.match(detail.path, /\/api\/cost\/actual\/resource\?id=urn%3Ares/);
  detail.resolve({ result: { resourceId: 'urn:res' }, notesDisplay: 'Provider note' });
  await flush();
  const history = t.calls.at(-1);
  assert.equal(history.path, '/api/history/resource?id=urn%3Ares');
  history.resolve({ status: 'ok', points: [{ amount: 8 }], display: { points: [] } });
  await flush();
  const dialog = t.container.querySelector('dialog');
  assert.match(dialog.textContent, /Provider note/);
  assert.match(dialog.textContent, /No cost history points are stored for this resource/);
  assert.equal(dialog.querySelector('.history-points'), undefined);
  assert.equal(dialog.querySelectorAll('h4').at(-1).textContent, 'Cost history');
  t.mounted.unmount();
});

test('resource history uses the section alert and never a raw error object', async () => {
  const t = await setup();
  t.calls[0].resolve(costPage('urn:res'));
  await flush();
  t.container.querySelector('.detail-button').dispatch('click');
  t.calls.at(-1).resolve({ result: { resourceId: 'urn:res' } });
  await flush();
  const history = t.calls.at(-1);
  history.resolve({ status: 'no_history' });
  await flush();
  let dialog = t.container.querySelector('dialog');
  assert.match(dialog.textContent, /finfocus cost history collect/);
  dialog.close();
  t.container.querySelector('.detail-button').dispatch('click');
  t.calls.at(-1).resolve({ result: { resourceId: 'urn:res' } });
  await flush();
  reject(t.calls.at(-1), '[object Object]', 502, 'history_unavailable', { data: { error: { path: '/secret' } } });
  await flush();
  dialog = t.container.querySelector('dialog');
  assert.equal(dialog.querySelector('.error-banner').textContent, 'Cost history is unavailable.');
  assert.doesNotMatch(dialog.textContent, /secret/);
  t.mounted.unmount();
});
