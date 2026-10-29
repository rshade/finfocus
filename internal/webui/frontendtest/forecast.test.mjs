import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

class Node {
  constructor(tag, doc) { this.tagName = tag; this.doc = doc; this.children = []; this.listeners = new Map(); this.attrs = {}; this.dataset = {}; this.hidden = false; this.value = ''; this.checked = false; }
  append(...children) { for (const child of children) { if (typeof child !== 'string') child.parent = this; this.children.push(child); } }
  replaceChildren(...children) { this.children = []; this.append(...children); }
  set textContent(text) { this.text = String(text); this.children = []; }
  get textContent() { return (this.text ?? '') + this.children.map(c => typeof c === 'string' ? c : c.textContent).join(''); }
  setAttribute(k, v) { this.attrs[k] = String(v); if (k === 'id') this.id = v; if (k === 'class') this.className = v; }
  addEventListener(k, fn) { this.listeners.set(k, fn); }
  dispatch(k, extra = {}) { return this.listeners.get(k)?.({ target: this, preventDefault() {}, ...extra }); }
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
  const mod = new vm.SourceTextModule(await readFile(new URL('../static/views/forecast.js', import.meta.url), 'utf8'), { context });
  await mod.link(() => app); await mod.evaluate();
  const calls = [];
  const api = { post(path, body, options) { return new Promise((resolve, reject) => calls.push({ path, body, options, resolve, reject })); } };
  const container = document.createElement('div'); document.append(container);
  const mounted = mod.namespace.mount(container, { api });
  const field = id => container.querySelector(`#${id}`);
  const submit = async (values = {}) => {
    const defaults = { 'forecast-months': '12', 'forecast-model': 'growth', 'forecast-growth-type': '', 'forecast-growth-rate': '', 'forecast-cap': '', 'forecast-steps': '', 'forecast-confidence': '', 'forecast-exclude': '' };
    for (const [id, value] of Object.entries({ ...defaults, ...values })) field(id).value = value;
    field('forecast-model').dispatch('change');
    container.querySelector('form').dispatch('submit');
    await flush(); await flush();
  };
  field('forecast-history').checked = true;
  return { container, calls, mounted, field, submit };
}

const point = (time, value) => ({ time, value });
const series = (name, values) => ({ name, points: values.map((v, i) => point(`2026-${String(10 + i).padStart(2, '0')}-01T00:00:00Z`, v)) });
const display = (name, values) => ({ name, points: values.map((value, i) => ({ month: ['Oct 2026', 'Nov 2026', 'Dec 2026'][i], value })) });

const growthResponse = {
  currency: 'USD', months: 2, budget: 500,
  series: [
    { name: 'history', points: [point('2026-08-03T00:00:00Z', 90), point('2026-09-14T00:00:00Z', 95)] },
    series('forecast', [100, 110, 121]), series('aws', [100, 110, 121]),
  ],
  warnings: ['bad: no cost data available'],
  display: {
    model: 'Plugin growth', budget: '$500.00 USD',
    series: [
      { name: 'history', points: [{ month: 'Aug 2026', value: '$90.00 USD' }, { month: 'Sep 2026', value: '$95.00 USD' }] },
      display('forecast', ['$100.00 USD', '$110.00 USD', '$121.00 USD']), display('aws', ['$100.00 USD', '$110.00 USD', '$121.00 USD']),
    ],
  },
};

test('growth query sends only the fields that are set and renders the server strings', async () => {
  const t = await setup();
  assert.equal(t.calls.length, 0);
  await t.submit({ 'forecast-months': '2', 'forecast-growth-type': 'linear', 'forecast-growth-rate': ' 0.10 ', 'forecast-steps': '2026-11-01=50, ,2027-01-01=-20', 'forecast-cap': '1.5' });
  assert.equal(t.calls[0].path, '/api/forecast/query');
  assert.equal(JSON.stringify(t.calls[0].body), JSON.stringify({ months: 2, growthType: 'linear', growthRate: '0.10', growthCap: 1.5, steps: ['2026-11-01=50', '2027-01-01=-20'] }));
  assert.match(t.container.textContent, /Running forecast/);
  t.calls[0].resolve(growthResponse);
  await flush();
  const text = t.container.textContent;
  assert.match(text, /Model: Plugin growth/);
  assert.match(text, /Oct 2026: \$100\.00 USD/);
  assert.match(text, /Dec 2026: \$121\.00 USD/);
  assert.match(text, /Budget: \$500\.00 USD/);
  assert.match(text, /History: 2 snapshots, 2026-08-03 to 2026-09-14/);
  assert.match(text, /bad: no cost data available/);
  const headers = t.container.querySelectorAll('thead')[0].querySelectorAll('th').map(th => th.textContent);
  assert.deepEqual(headers, ['Month', 'Forecast', 'aws']);
  const rows = t.container.querySelectorAll('tbody')[0].querySelectorAll('tr');
  assert.equal(rows.length, 3);
  assert.equal(rows[1].textContent, 'Nov 2026$110.00 USD$110.00 USD');
  assert.match(text, /3 months projected/);
  t.mounted.unmount();
});

test('history toggle off sends noHistory', async () => {
  const t = await setup();
  t.field('forecast-history').checked = false;
  await t.submit();
  assert.equal(JSON.stringify(t.calls[0].body), JSON.stringify({ months: 12, noHistory: true }));
  t.mounted.unmount();
});

test('arima query reads the chosen CSV in the browser and shows bounds, baseline, and spend', async () => {
  const t = await setup();
  assert.equal(t.container.querySelector('.forecast-arima').hidden, true, 'statistical fields start hidden');
  t.field('forecast-spend').files = [{ text: async () => 'date,value\n2026-04-01,1\n' }];
  await t.submit({ 'forecast-model': 'arima', 'forecast-confidence': '0.8', 'forecast-exclude': '2025-12-01:2025-12-31' });
  assert.equal(t.container.querySelector('.forecast-arima').hidden, false);
  assert.equal(JSON.stringify(t.calls[0].body), JSON.stringify({ months: 12, model: 'arima', spendHistory: 'date,value\n2026-04-01,1\n', confidence: 0.8, exclude: ['2025-12-01:2025-12-31'] }));
  t.calls[0].resolve({
    currency: 'USD', months: 2, model: 'arima', modelOrder: 'SARIMA(1,1,1)(0,1,1)[7]', confidence: 0.8,
    series: [series('forecast', [10, 11, 12]), series('lower', [8, 7, 6]), series('upper', [12, 15, 18]), series('baseline', [9, 9, 9]),
      { name: 'spend', points: [point('2026-04-01T00:00:00Z', 1), point('2026-09-30T00:00:00Z', 2)] }],
    display: { model: 'SARIMA(1,1,1)(0,1,1)[7], bounds at 80% confidence', series: [
      display('forecast', ['$10.00 USD', '$11.00 USD', '$12.00 USD']), display('lower', ['$8.00 USD', '$7.00 USD', '$6.00 USD']),
      display('upper', ['$12.00 USD', '$15.00 USD', '$18.00 USD']), display('baseline', ['$9.00 USD', '$9.00 USD', '$9.00 USD']),
      { name: 'spend', points: [{ month: 'Apr 2026', value: '$1.00 USD' }, { month: 'Sep 2026', value: '$2.00 USD' }] }] },
  });
  await flush();
  const text = t.container.textContent;
  assert.match(text, /SARIMA\(1,1,1\)\(0,1,1\)\[7\], bounds at 80% confidence/);
  assert.match(text, /Spend history: 2 points, 2026-04-01 to 2026-09-30/);
  assert.deepEqual(t.container.querySelectorAll('thead')[0].querySelectorAll('th').map(th => th.textContent), ['Month', 'Forecast', 'Lower', 'Upper', 'Baseline']);
  assert.equal(t.container.querySelectorAll('tbody')[0].querySelectorAll('tr')[2].textContent, 'Dec 2026$12.00 USD$6.00 USD$18.00 USD$9.00 USD');
  assert.doesNotMatch(text, /Budget:/);
  t.mounted.unmount();
});

test('arima fields are not sent with the growth model', async () => {
  const t = await setup();
  t.field('forecast-spend').files = [{ text: async () => 'ignored' }];
  await t.submit({ 'forecast-confidence': '0.8', 'forecast-exclude': '2025-12-01:2025-12-31' });
  assert.equal(JSON.stringify(t.calls[0].body), JSON.stringify({ months: 12 }));
  t.mounted.unmount();
});

test('a bad number is caught before the request', async () => {
  const t = await setup();
  await t.submit({ 'forecast-cap': 'lots' });
  assert.equal(t.calls.length, 0);
  assert.equal(t.field('forecast-error').hidden, false);
  assert.match(t.field('forecast-error').textContent, /Growth cap must be a number/);
  t.mounted.unmount();
});

test('a server error clears earlier results and shows its text', async () => {
  const t = await setup();
  await t.submit();
  t.calls[0].resolve(growthResponse);
  await flush();
  await t.submit({ 'forecast-model': 'arima' });
  t.calls[1].reject(Object.assign(new Error('--model arima requires --spend-history'), { name: 'ApiError' }));
  await flush();
  const text = t.container.textContent;
  assert.equal(t.field('forecast-error').hidden, false);
  assert.match(text, /--model arima requires --spend-history/);
  assert.doesNotMatch(text, /\$110\.00 USD/);
  assert.match(text, /Unable to run the forecast/);
  t.mounted.unmount();
});

test('a newer query aborts the pending one and unmount aborts the last', async () => {
  const t = await setup();
  await t.submit();
  await t.submit({ 'forecast-months': '3' });
  assert.equal(t.calls[0].options.signal.aborted, true);
  t.calls[0].resolve(growthResponse);
  await flush();
  assert.doesNotMatch(t.container.textContent, /\$110\.00 USD/, 'a stale response is not rendered');
  t.mounted.unmount();
  assert.equal(t.calls[1].options.signal.aborted, true);
});
