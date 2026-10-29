import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

// A small DOM boundary double keeps these tests dependency-free. Browser
// integration tests cover native dialog focus trapping and CSS rendering.
class Node {
  constructor(tag, document) {
    this.tagName = tag; this.document = document; this.children = [];
    this.attrs = {}; this.dataset = {}; this.listeners = new Map();
    this.hidden = false; this.disabled = false; this.value = ''; this.open = false;
    this.classList = { add: (name) => { this.className = `${this.className ?? ''} ${name}`; }, contains: (name) => this.className?.split(' ').includes(name) };
  }
  set textContent(value) { this.text = String(value); this.children = []; }
  get textContent() { return (this.text ?? '') + this.children.map((child) => typeof child === 'string' ? child : child.textContent).join(''); }
  setAttribute(name, value) {
    this.attrs[name] = String(value);
    if (name === 'class') this.className = String(value);
    if (name === 'id') this.id = String(value);
  }
  removeAttribute(name) { delete this.attrs[name]; }
  append(...children) { for (const child of children) { if (typeof child !== 'string') child.parent = this; this.children.push(child); } }
  replaceChildren(...children) { for (const child of this.children) if (typeof child !== 'string') child.parent = null; this.text = ''; this.children = []; this.append(...children); }
  replaceWith(node) { const index = this.parent.children.indexOf(this); this.parent.children[index] = node; node.parent = this.parent; this.parent = null; }
  addEventListener(name, fn) { if (!this.listeners.has(name)) this.listeners.set(name, new Set()); this.listeners.get(name).add(fn); }
  removeEventListener(name, fn) { this.listeners.get(name)?.delete(fn); }
  dispatch(name, options = {}) { for (const fn of this.listeners.get(name) ?? []) fn({ target: this, preventDefault() {}, ...options }); }
  get isConnected() { return this === this.document || Boolean(this.parent?.isConnected); }
  focus() { if (this.isConnected) this.document.activeElement = this; }
  closest(tag) { return this.tagName === tag ? this : this.parent?.closest(tag); }
  showModal() { this.open = true; }
  close() { this.open = false; this.dispatch('close'); }
  querySelector(selector) { return this.querySelectorAll(selector)[0] ?? null; }
  querySelectorAll(selector) {
    const matches = (node) => selector.startsWith('.') ? node.className?.split(' ').includes(selector.slice(1))
      : selector.startsWith('#') ? node.id === selector.slice(1)
      : selector === 'dialog[open]' ? node.tagName === 'dialog' && node.open
      : selector.startsWith('tr[data-urn=') ? node.tagName === 'tr' && node.dataset.urn === selector.match(/"(.*)"/)[1]
      : node.tagName === selector;
    return this.children.filter((child) => typeof child !== 'string').flatMap((child) => [...(matches(child) ? [child] : []), ...child.querySelectorAll(selector)]);
  }
}
function deferred() { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; }
const flush = () => new Promise((resolve) => setImmediate(resolve));
async function setup({ routeModules = {} } = {}) {
  const document = new Node('document'); document.document = document;
  document.createElement = (tag) => new Node(tag, document);
  const view = document.createElement('div'); view.id = 'view';
  const status = document.createElement('span'); status.id = 'connection-status';
  const main = document.createElement('main'); main.id = 'main'; main.append(view); document.append(main, status);
  document.getElementById = (id) => document.querySelector(`#${id}`);
  const window = new Node('window', document);
  const location = { hash: '#/overview' };
  const history = { replaceState(_state, _title, hash) { location.hash = hash; } };
  class EventSource {
    static CONNECTING = 0;
    static all = [];
    constructor() { this.listeners = new Map(); EventSource.all.push(this); }
    addEventListener(name, fn) { this.listeners.set(name, fn); }
    close() { this.closed = true; }
  }
  let timerId = 0; const timers = new Map();
  const context = vm.createContext({ document, window, location, history, console, EventSource,
    MessageEvent: class {}, AbortController, Date, CSS: { escape: (value) => value },
    setTimeout: (fn) => { const id = ++timerId; timers.set(id, fn); return id; }, clearTimeout: (id) => timers.delete(id),
    setInterval: (fn) => { const id = ++timerId; timers.set(id, fn); return id; }, clearInterval: (id) => timers.delete(id),
    fetch: async () => ({ ok: true, text: async () => '{"rows":[],"page":1,"totalPages":1}' }),
  });
  const app = new vm.SourceTextModule(await readFile(new URL('../static/app.js', import.meta.url), 'utf8'), {
    context,
    importModuleDynamically: async (name) => {
      const result = routeModules[name];
      if (result) return typeof result === 'function' ? await result(context) : await result;
      const mod = new vm.SyntheticModule(['mount'], function () { this.setExport('mount', () => {}); }, { context });
      await mod.link(() => {}); await mod.evaluate(); return mod;
    },
  });
  await app.link(() => {}); await app.evaluate(); await flush();
  const overview = new vm.SourceTextModule(await readFile(new URL('../static/views/overview.js', import.meta.url), 'utf8'), { context });
  await overview.link(() => app); await overview.evaluate();
  const stream = new app.namespace.OverviewStream();
  const calls = [];
  const api = {
    post(path, body, options) { const pending = deferred(); calls.push({ path, body, options, ...pending }); return pending.promise; },
    get(path, options) { const pending = deferred(); calls.push({ path, options, ...pending }); return pending.promise; },
  };
  const container = document.createElement('div'); document.append(container);
  const mounted = overview.namespace.mount(container, { stream, api });
  return { document, container, stream, calls, mounted, timers, app, context, location, window, view };
}
const row = (urn, extra = {}) => ({ urn, type: 'aws:ec2:Instance', resourceDisplay: urn,
  statusDisplay: 'active', actualDisplay: '$12.34', projectedDisplay: '$25.00',
  deltaDisplay: '+$12.66', driftDisplay: '2.0%', recsDisplay: '1', ...extra });
const page = (rows, extra = {}) => ({ rows, page: 1, totalPages: 1, ...extra });

test('a remounted overview recovers ready state from cached stream events', async () => {
  const t = await setup();
  t.stream.emit('snapshot', { phases: [], rows: [row('alpha')], ready: false });
  t.stream.emit('ready', { totals: {} });
  t.mounted.unmount();
  const restored = t.document.createElement('div'); t.document.append(restored);
  const instance = (await importOverview(t)).mount(restored, { stream: t.stream, api: { post: async () => page([row('alpha')]), get: async () => null } });
  await flush();
  assert.equal(restored.querySelector('.phase-checklist').hidden, true);
  instance.unmount();
});
async function importOverview(t) {
  const mod = new vm.SourceTextModule(await readFile(new URL('../static/views/overview.js', import.meta.url), 'utf8'), { context: t.context });
  await mod.link(() => t.app); await mod.evaluate(); return mod.namespace;
}

test('an older query cannot overwrite the page after sorting', async () => {
  const t = await setup();
  const first = t.calls.find((call) => call.path.endsWith('/query'));
  t.container.querySelector('.sort-button').dispatch('click');
  const second = t.calls.filter((call) => call.path.endsWith('/query')).at(-1);
  second.resolve(page([row('newest')])); await flush();
  first.resolve(page([row('stale')])); await flush();
  assert.equal(t.container.querySelector('.resource-link').textContent, 'newest');
  t.mounted.unmount();
});

test('late detail replies keep the most recently selected resource focused', async () => {
  const t = await setup();
  t.calls[0].resolve(page([row('alpha'), row('beta')])); await flush();
  const buttons = t.container.querySelectorAll('.resource-link');
  buttons[0].focus(); buttons[0].dispatch('click');
  const alpha = t.calls.at(-1);
  buttons[1].focus(); buttons[1].dispatch('click');
  const beta = t.calls.at(-1);
  beta.resolve(row('beta', { displayName: 'Beta' })); await flush();
  alpha.resolve(row('alpha', { displayName: 'Alpha' })); await flush();
  assert.equal(t.container.querySelector('.detail-panel').querySelector('h3').textContent, 'Beta');
  t.container.querySelector('.detail-close').dispatch('click');
  assert.equal(t.document.activeElement, buttons[1]);
  t.mounted.unmount();
});

test('cluster expansion carries the current filter sort and page', async () => {
  const t = await setup();
  t.calls[0].resolve(page([row('cluster', { childUrns: ['child'] })], { page: 3, totalPages: 4 })); await flush();
  t.container.querySelector('.cluster-toggle').dispatch('click');
  const call = t.calls.at(-1);
  assert.equal(call.path, '/api/overview/cluster/toggle');
  assert.equal(call.body.page, 3);
  assert.equal(call.body.sort, 'cost');
  assert.equal(call.body.filter, '');
  t.mounted.unmount();
});

test('canceling a passphrase prompt erases its value', async () => {
  const t = await setup(); t.stream.emit('passphrase_required', { stack: 'secret-stack' });
  const input = t.container.querySelector('#passphrase-input'); input.value = 'not-for-storage';
  t.container.querySelector('.dialog-cancel').dispatch('click');
  assert.equal(input.value, '');
  t.mounted.unmount();
});

test('a late close event does not clear the retry prompt after it reopened', async () => {
  const t = await setup();
  t.stream.emit('passphrase_required', { stack: 'encrypted' });
  const dialog = t.container.querySelector('.passphrase-dialog');
  const input = t.container.querySelector('#passphrase-input');
  input.value = 'wrong';
  t.container.querySelector('.passphrase-form').dispatch('submit');
  const unlock = t.calls.at(-1);
  unlock.resolve(null); await flush();
  t.stream.emit('passphrase_required', { stack: 'encrypted' });
  input.value = 'correct';
  assert.equal(dialog.open, true);
  dialog.dispatch('close');
  assert.equal(input.value, 'correct');
  t.mounted.unmount();
});

test('server-formatted totals render verbatim without browser currency formatting', async () => {
  const t = await setup();
  t.calls[0].resolve(page([row('alpha')], { totals: { totalActual: 12.345, totalActualDisplay: '$12.35', totalProjectedDisplay: '$25.00', totalDeltaDisplay: '+$12.65', totalSavingsDisplay: '$2.00', currency: 'USD' } }));
  await flush();
  assert.deepEqual(t.container.querySelectorAll('.totals-value').map((node) => node.textContent), ['$12.35', '$25.00', '+$12.65', '$2.00']);
  t.mounted.unmount();
});

test('a slower previous hash route cannot replace the current view', async () => {
  const pending = deferred();
  const route = (label, delay) => async (context) => {
    if (delay) await delay;
    const mod = new vm.SyntheticModule(['mount'], function () {
      this.setExport('mount', (container) => { container.textContent = label; });
    }, { context });
    await mod.link(() => {}); await mod.evaluate(); return mod;
  };
  const t = await setup({ routeModules: {
    './views/overview.js': route('Overview', pending.promise),
    './views/cost.js': route('Actual cost'),
  } });
  t.location.hash = '#/cost'; t.window.dispatch('hashchange'); await flush();
  assert.equal(t.view.textContent, 'Actual cost');
  pending.resolve(); await flush();
  assert.equal(t.view.textContent, 'Actual cost');
  t.mounted.unmount();
});

test('a fast rejected passphrase does not close the newly requested retry prompt', async () => {
  const t = await setup();
  t.stream.emit('passphrase_required', { stack: 'encrypted' });
  t.container.querySelector('#passphrase-input').value = 'wrong';
  t.container.querySelector('.passphrase-form').dispatch('submit');
  const unlock = t.calls.at(-1);
  t.stream.emit('error', { message: 'Wrong passphrase', phase: 1 });
  t.stream.emit('passphrase_required', { stack: 'encrypted' });
  unlock.resolve(null); await flush();
  assert.equal(t.container.querySelector('.passphrase-dialog').open, true);
  assert.equal(t.container.querySelector('.dialog-error').hidden, false);
  t.mounted.unmount();
});

test('new SSE rows request a server page before ready', async () => {
  const t = await setup(); t.calls[0].resolve(page([])); await flush();
  t.stream.emit('row', row('progressive'));
  for (const callback of [...t.timers.values()]) callback();
  const query = t.calls.at(-1); query.resolve(page([row('progressive')])); await flush();
  assert.equal(t.container.querySelector('.resource-link').textContent, 'progressive');
  t.mounted.unmount();
});

test('all nine cells use server values and payload markup stays text', async () => {
  const t = await setup();
  t.calls[0].resolve(page([row('unsafe', { resourceDisplay: '<img src=x onerror=alert(1)>', warnings: ['<script>bad</script>'] })])); await flush();
  const cells = t.container.querySelector('tbody').querySelectorAll('td');
  assert.deepEqual(cells.map((node) => node.textContent), ['<img src=x onerror=alert(1)>', 'aws:ec2:Instance', 'active', '$12.34', '$25.00', '+$12.66', '2.0%', '1', '<script>bad</script>']);
  assert.equal(t.container.querySelectorAll('img').length, 0);
  t.mounted.unmount();
});

test('unmount clears preview and filter timers and stream subscriptions', async () => {
  const t = await setup(); t.stream.emit('preview', { status: 'running', elapsedMs: 2400 });
  t.container.querySelector('.filter-input').dispatch('input');
  t.mounted.unmount();
  assert.equal(t.timers.size, 0);
  const before = t.container.textContent;
  t.stream.emit('phase', { phase: 2, name: 'Should not render', status: 'active' });
  assert.equal(t.container.textContent, before);
});

test('light and dark palettes provide AA text and control contrast', async () => {
  const css = await readFile(new URL('../static/styles.css', import.meta.url), 'utf8');
  const palettes = [...css.matchAll(/:root\s*\{([^}]+)\}/g)].map((match) => Object.fromEntries([...match[1].matchAll(/--([\w-]+):\s*(#[\da-f]{6})/gi)].map((value) => [value[1], value[2]])));
  const luminance = (hex) => {
    const values = hex.slice(1).match(/../g).map((pair) => parseInt(pair, 16) / 255).map((v) => v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4);
    return values[0] * 0.2126 + values[1] * 0.7152 + values[2] * 0.0722;
  };
  const contrast = (a, b) => (Math.max(luminance(a), luminance(b)) + 0.05) / (Math.min(luminance(a), luminance(b)) + 0.05);
  for (const [index, colors] of palettes.entries()) {
    for (const foreground of ['fg', 'fg-muted', 'link', 'error', 'warn', 'ok', 'delta-up', 'delta-down']) {
      for (const background of ['bg', 'bg-subtle', 'row-alt', 'row-hover']) {
        assert.ok(contrast(colors[foreground], colors[background]) >= 4.5, `theme ${index}: ${foreground}/${background}`);
      }
    }
    for (const background of ['bg', 'bg-subtle']) assert.ok(contrast(colors.border, colors[background]) >= 3, `theme ${index}: control border/${background}`);
    assert.ok(contrast(colors['accent-fg'], colors.accent) >= 4.5, `theme ${index}: accent text`);
  }
});

test('a previous asynchronous mount cannot write into a newer route', async () => {
  const pending = deferred();
  const route = (label, delay) => async (context) => {
    const mod = new vm.SyntheticModule(['mount'], function () {
      this.setExport('mount', async (container) => { if (delay) await delay; container.textContent = label; });
    }, { context });
    await mod.link(() => {}); await mod.evaluate(); return mod;
  };
  const t = await setup({ routeModules: {
    './views/overview.js': route('Slow Overview', pending.promise),
    './views/cost.js': route('Actual cost'),
  } });
  t.location.hash = '#/cost'; t.window.dispatch('hashchange'); await flush();
  pending.resolve(); await flush();
  assert.equal(t.view.textContent, 'Actual cost');
  t.mounted.unmount();
});

test('preview-only source warning survives ready, remount, and reconnect', async () => {
  const t = await setup();
  const warning = 'Stack state unavailable (backend_unreachable); 4 unchanged resources omitted.';
  t.stream.emit('run_status', { stateOnly: false, sourceWarning: warning });
  t.stream.emit('ready', { totals: {} });
  await flush();
  assert.equal(t.container.querySelector('.source-warning').textContent, warning);
  assert.equal(t.container.querySelector('.source-warning').hidden, false);
  assert.equal(t.container.querySelector('.preview-button').disabled, true);
  t.mounted.unmount();
  const restored = t.document.createElement('div'); t.document.append(restored);
  const remounted = (await importOverview(t)).mount(restored, {
    stream: t.stream, api: { post: async () => page([]), get: async () => null },
  });
  await flush();
  assert.equal(restored.querySelector('.source-warning').textContent, warning);
  t.stream.emit('snapshot', { phases: [], rows: [], ready: true, stateOnly: false, sourceWarning: warning });
  await flush();
  assert.equal(restored.querySelector('.source-warning').hidden, false);
  remounted.unmount();
});

test('snapshot displays a pending passphrase prompt and progress on initial mount', async () => {
  const t = await setup(); t.mounted.unmount();
  t.stream.emit('snapshot', { phases: [{ phase: 1, name: 'Load stack', status: 'active' }], rows: [], ready: false,
    passphraseRequired: true, stack: 'encrypted', progress: { loaded: 2, total: 4 }, preview: { status: 'running', elapsedMs: 4000 } });
  const container = t.document.createElement('div'); t.document.append(container);
  const instance = (await importOverview(t)).mount(container, { stream: t.stream, api: { post: async () => page([]), get: async () => null } });
  await flush();
  assert.equal(container.querySelector('.passphrase-dialog').open, true);
  assert.equal(t.document.activeElement.id, 'passphrase-input');
  assert.equal(container.querySelector('.progress-line').textContent, 'Enriched 2 of 4 resources');
  assert.equal(container.querySelector('.preview-button').disabled, true);
  instance.unmount();
});

test('preview is offered only for a ready state-only run and clears after a preview', async () => {
  const t = await setup();
  const button = t.container.querySelector('.preview-button');
  const projected = t.container.querySelectorAll('th')[4];
  assert.equal(button.disabled, true);
  assert.equal(projected.textContent, 'Projected');
  t.stream.emit('snapshot', {
    phases: [], rows: [], ready: false, stateOnly: true, previewLoaded: false,
    detectFailed: true, footnote: '* projected at current state (730h/mo)',
  });
  await flush();
  assert.equal(button.disabled, true);
  t.stream.emit('ready', { totals: {} });
  await flush();
  assert.equal(button.disabled, false);
  assert.equal(projected.textContent, 'Projected*');
  assert.match(t.container.querySelector('.state-only-footnote').textContent, /730h\/mo/);
  assert.match(t.container.querySelector('.state-only-hint').textContent, /change detection failed/);
  t.stream.emit('run_status', { stateOnly: false, previewLoaded: true, detectFailed: false, footnote: '' });
  await flush();
  assert.equal(button.disabled, true);
  assert.equal(projected.textContent, 'Projected');
  assert.equal(t.container.querySelector('.state-only-footnote').hidden, true);
  assert.equal(t.container.querySelector('.state-only-hint').hidden, true);
  t.mounted.unmount();
});

test('a disabled preview button says why for a session that already planned', async () => {
  const t = await setup();
  const button = t.container.querySelector('.preview-button');
  const reason = t.container.querySelector('.preview-reason');
  t.stream.emit('snapshot', { phases: [], rows: [row('aws_instance.web')], ready: true, stateOnly: false, previewLoaded: false, detectFailed: false, footnote: '' });
  await flush();
  assert.equal(button.disabled, true);
  assert.doesNotMatch(button.attrs['aria-label'], /pulumi/i);
  assert.equal(button.attrs['aria-describedby'], reason.id);
  assert.equal(reason.hidden, false);
  assert.match(reason.textContent, /already included/);
  assert.equal(button.attrs.title, reason.textContent);
  t.stream.emit('run_status', { stateOnly: true, previewLoaded: false, detectFailed: false, footnote: '' });
  await flush();
  assert.equal(button.disabled, false);
  assert.equal(reason.hidden, true);
  assert.equal(button.attrs.title, undefined);
  t.mounted.unmount();
});

test('a row with no projected price reads as unavailable, not zero or a dash', async () => {
  const t = await setup();
  const priced = row('aws_instance.web', { status: 'creating', projected: 7.59 });
  const free = row('aws_s3_bucket.free', { status: 'creating', projected: 0, projectedDisplay: '$0.00' });
  const unpriced = row('aws_s3_bucket.assets', { status: 'creating', projectedDisplay: '-' });
  const deleting = row('aws_old', { status: 'deleting', projectedDisplay: '-' });
  const failed = row('aws_bad', { status: 'creating', hasError: true, projectedDisplay: 'ERR' });
  t.stream.emit('snapshot', { phases: [], rows: [], ready: true });
  await flush();
  t.calls.at(-1)?.resolve?.(page([priced, free, unpriced, deleting, failed]));
  await flush();
  const cells = (urn) => t.container.querySelector(`tr[data-urn="${urn}"]`).querySelectorAll('.col-num');
  const projectedOf = (urn) => cells(urn)[1];
  assert.equal(projectedOf('aws_instance.web').textContent, '$25.00');
  assert.equal(projectedOf('aws_s3_bucket.free').textContent, '$0.00');
  assert.equal(projectedOf('aws_s3_bucket.assets').classList.contains('price-unavailable'), true);
  assert.match(projectedOf('aws_s3_bucket.assets').textContent, /Price unavailable/);
  assert.equal(projectedOf('aws_old').classList.contains('price-unavailable'), false);
  assert.equal(projectedOf('aws_bad').classList.contains('price-unavailable'), false);
  t.mounted.unmount();
});

test('a long address keeps its full value for selectors and offers break points', async () => {
  const t = await setup();
  const address = 'module.network.aws_instance.web["a-b"]';
  t.stream.emit('snapshot', { phases: [], rows: [], ready: true });
  await flush();
  t.calls.at(-1)?.resolve?.(page([row(address)]));
  await flush();
  const tr = t.container.querySelector(`tr[data-urn="${address}"]`);
  const link = tr.querySelector('.resource-link');
  assert.equal(link.textContent, address);
  assert.equal(link.attrs.title, address);
  assert.ok(link.children.some((child) => child.tagName === 'wbr'));
  t.mounted.unmount();
});

test('a remount after preview keeps the preview gate closed', async () => {
  const t = await setup();
  t.stream.emit('snapshot', {
    phases: [], rows: [], ready: false, stateOnly: true, previewLoaded: false,
    detectFailed: true, footnote: '* projected at current state (730h/mo)',
  });
  t.stream.emit('ready', { totals: {} });
  t.stream.emit('run_status', { stateOnly: false, previewLoaded: true, detectFailed: false, footnote: '' });
  await flush();
  t.mounted.unmount();
  const restored = t.document.createElement('div');
  t.document.append(restored);
  const instance = (await importOverview(t)).mount(restored, {
    stream: t.stream,
    api: { post: async () => page([]), get: async () => null },
  });
  await flush();
  assert.equal(t.stream.snapshot.previewLoaded, true);
  assert.equal(t.stream.snapshot.stateOnly, false);
  assert.equal(t.stream.snapshot.detectFailed, false);
  assert.equal(t.stream.snapshot.footnote, '');
  assert.equal(restored.querySelector('.preview-button').disabled, true);
  assert.equal(restored.querySelectorAll('th')[4].textContent, 'Projected');
  assert.equal(restored.querySelector('.state-only-footnote').hidden, true);
  assert.equal(restored.querySelector('.state-only-hint').hidden, true);
  instance.unmount();
});

test('numeric totals never bypass server display formatting', async () => {
  const t = await setup();
  t.calls[0].resolve(page([], { totals: { totalActual: 12.345, totalProjected: 25, totalDelta: 12.655, totalSavings: 2, currency: 'USD' } })); await flush();
  assert.deepEqual(t.container.querySelectorAll('.totals-value').map((node) => node.textContent), []);
  t.mounted.unmount();
});

test('resource detail displays actual and projected breakdowns from the server', async () => {
  const t = await setup(); t.calls[0].resolve(page([row('alpha')])); await flush();
  t.container.querySelector('.resource-link').dispatch('click');
  t.calls.at(-1).resolve({ row: row('alpha', { actualCost: { breakdown: { Compute: 10, Storage: 2.34 } }, projectedCost: { breakdown: { Compute: 20, Storage: 5 } } }), display: { actualBreakdown: [{name:'Compute',costDisplay:'$10.00'},{name:'Storage',costDisplay:'$2.34'}], projectedBreakdown:[{name:'Compute',costDisplay:'$20.00'},{name:'Storage',costDisplay:'$5.00'}] }, budgets: [] }); await flush();
  assert.deepEqual(t.container.querySelectorAll('.breakdown-list').map((node) => node.textContent), ['Compute$10.00Storage$2.34', 'Compute$20.00Storage$5.00']);
  t.mounted.unmount();
});

test('budget health shows each server spend limit and utilization', async () => {
  const t = await setup(); t.stream.emit('budget', { display: {footerDisplay:'Budget: WARNING $75.68 / $100.12 (76%)',details:[{name:'Production',healthDisplay:'WARNING',currentSpendDisplay:'$75.68',limitDisplay:'$100.12',utilizationDisplay:'75.6%',forecastedDisplay:'$90.12',thresholds:[{text:'75% threshold triggered (ACTUAL)'}]}]}, budgets: [{ budgetID: 'b1', budgetName: 'Production', health: 'WARNING', utilization: 75, forecasted: 90, currency: 'USD', limit: 100, currentSpend: 75 }] });
  const footer = t.container.querySelector('.budget-footer').textContent;
  assert.match(footer, /\$75.68 \/ \$100.12/);
  assert.match(footer, /75.6%/);
  t.mounted.unmount();
});

test('progressive enrichment preserves keyboard focus on a resource button', async () => {
  const t = await setup(); t.calls[0].resolve(page([row('alpha')])); await flush();
  const button = t.container.querySelector('.resource-link'); button.focus();
  t.stream.emit('row', row('alpha', { actualDisplay: '$14.00' }));
  assert.ok(t.document.activeElement === t.container.querySelector('.resource-link'), 'the replacement resource button retains focus');
  t.mounted.unmount();
});

test('cluster toggle cannot be undone by an older overview query', async () => {
  const t = await setup();
  t.calls[0].resolve(page([row('cluster', { childUrns: ['child'] })])); await flush();
  t.container.querySelector('.sort-button').dispatch('click');
  const oldQuery = t.calls.at(-1);
  t.container.querySelector('.cluster-toggle').dispatch('click');
  const toggle = t.calls.at(-1);
  toggle.resolve(page([row('cluster', { childUrns: ['child'] }), row('child')], { expanded: ['cluster'] })); await flush();
  oldQuery.resolve(page([row('cluster', { childUrns: ['child'] })])); await flush();
  assert.equal(t.container.querySelectorAll('.resource-link').length, 2);
  assert.equal(t.container.querySelector('.cluster-toggle').attrs['aria-expanded'], 'true');
  t.mounted.unmount();
});

test('an accepted passphrase request followed by a rejection shows the retry error inline', async () => {
  const t = await setup(); t.stream.emit('passphrase_required', { stack: 'encrypted' });
  t.container.querySelector('#passphrase-input').value = 'wrong';
  t.container.querySelector('.passphrase-form').dispatch('submit');
  t.calls.at(-1).resolve(null); await flush();
  t.stream.emit('error', { message: 'Wrong passphrase', phase: 1 });
  t.stream.emit('passphrase_required', { stack: 'encrypted' });
  assert.equal(t.container.querySelector('.dialog-error').hidden, false);
  assert.match(t.container.querySelector('.dialog-error').textContent, /Wrong passphrase/);
  t.mounted.unmount();
});

test('accepted unlock does not reopen a stale passphrase prompt on remount during enrichment', async () => {
  const t = await setup(); t.stream.emit('passphrase_required', { stack: 'encrypted' });
  t.container.querySelector('#passphrase-input').value = 'correct';
  t.container.querySelector('.passphrase-form').dispatch('submit');
  t.calls.at(-1).resolve(null); await flush();
  t.mounted.unmount();
  const container = t.document.createElement('div'); t.document.append(container);
  const mounted = (await importOverview(t)).mount(container, { stream: t.stream, api: { post: async () => page([]), get: async () => null } });
  await flush();
  assert.equal(container.querySelector('.passphrase-dialog').open, false);
  assert.equal(container.querySelector('.phase-checklist').hidden, false);
  mounted.unmount();
});

test('closing detail after an SSE row replacement focuses the current launcher', async () => {
  const t = await setup(); t.calls[0].resolve(page([row('alpha')])); await flush();
  const oldButton = t.container.querySelector('.resource-link'); oldButton.focus(); oldButton.dispatch('click');
  t.calls.at(-1).resolve(row('alpha')); await flush();
  t.stream.emit('row', row('alpha', { actualDisplay: '$14.00' }));
  t.container.querySelector('.detail-close').dispatch('click');
  assert.ok(t.document.activeElement === t.container.querySelector('.resource-link'), 'focus resolves the live launcher after enrichment');
  t.mounted.unmount();
});

test('closing detail after its row disappears focuses the filter', async () => {
  const t = await setup(); t.calls[0].resolve(page([row('alpha')])); await flush();
  const button = t.container.querySelector('.resource-link'); button.focus(); button.dispatch('click');
  t.calls.at(-1).resolve(row('alpha')); await flush();
  t.container.querySelector('.sort-button').dispatch('click');
  t.calls.at(-1).resolve(page([])); await flush();
  t.container.querySelector('.detail-close').dispatch('click');
  assert.ok(t.document.activeElement === t.container.querySelector('.filter-input'), 'focus falls back to a connected control');
  t.mounted.unmount();
});

for (const event of ['budget', 'snapshot']) {
  test(`a stale initial budget GET cannot overwrite newer ${event} budget state`, async () => {
    const t = await setup();
    const initial = t.calls.find((call) => call.path === '/api/overview/budget');
    const budget = { display:{details:[{name:'Current budget',healthDisplay:'OK',currentSpendDisplay:'$70.00',limitDisplay:'$100.00'}]}, budgets: [{ budgetName: 'Current budget', health: 'OK', currentSpend: 70, limit: 100, currency: 'USD', utilization: 70 }] };
    t.stream.emit(event, event === 'snapshot' ? { rows: [], phases: [], budget, ready: false } : budget);
    initial.resolve({ budgets: [] }); await flush();
    assert.match(t.container.querySelector('.budget-footer').textContent, /Current budget/);
    assert.match(t.container.querySelector('.budget-footer').textContent, /\$70.00 \/ \$100.00/);
    t.mounted.unmount();
  });
}

test('resource detail renders only server-prepared active recommendations with savings', async () => {
  const t = await setup(); t.calls[0].resolve(page([row('alpha')])); await flush();
  t.container.querySelector('.resource-link').dispatch('click');
  t.calls.at(-1).resolve({
    row: row('alpha', { recommendations: [{ type: 'RIGHTSIZE', description: 'Active opportunity' }, { type: 'TERMINATE', description: 'Dismissed opportunity' }, { type: 'SCHEDULE', description: 'Snoozed opportunity' }] }),
    activeRecommendations: [{ recommendation: { type: 'RIGHTSIZE', description: 'Active opportunity' }, savingsDisplay: '$8.25/mo' }], budgets: [],
  }); await flush();
  const recommendations = t.container.querySelector('.detail-recs');
  assert.equal(recommendations.querySelectorAll('li').length, 1);
  assert.match(recommendations.textContent, /Active opportunity/);
  assert.match(recommendations.textContent, /\$8\.25\/mo/);
  assert.doesNotMatch(recommendations.textContent, /Dismissed|Snoozed/);
  t.mounted.unmount();
});

test('canceling a pending unlock does not retain an accepted requirement on remount', async () => {
  const t = await setup(); t.stream.emit('passphrase_required', { stack: 'encrypted' });
  t.container.querySelector('#passphrase-input').value = 'correct';
  t.container.querySelector('.passphrase-form').dispatch('submit');
  const unlock = t.calls.at(-1);
  t.container.querySelector('.dialog-cancel').dispatch('click');
  unlock.resolve(null); await flush();
  t.mounted.unmount();
  const container = t.document.createElement('div'); t.document.append(container);
  const mounted = (await importOverview(t)).mount(container, { stream: t.stream, api: { post: async () => page([]), get: async () => null } });
  await flush();
  assert.equal(container.querySelector('.passphrase-dialog').open, false);
  mounted.unmount();
});

for (const close of ['Cancel', 'Escape']) {
  test(`authoritative acceptance after ${close} closes a remounted pending-unlock prompt`, async () => {
    const t = await setup(); t.stream.emit('passphrase_required', { stack: 'encrypted' });
    t.container.querySelector('#passphrase-input').value = 'correct';
    t.container.querySelector('.passphrase-form').dispatch('submit');
    if (close === 'Cancel') t.container.querySelector('.dialog-cancel').dispatch('click');
    else t.container.querySelector('.passphrase-dialog').close(); // native Escape closes a dialog
    t.mounted.unmount();
    const container = t.document.createElement('div'); t.document.append(container);
    const mounted = (await importOverview(t)).mount(container, { stream: t.stream, api: { post: async () => page([]), get: async () => null } });
    assert.equal(container.querySelector('.passphrase-dialog').open, true);
    t.stream.emit('snapshot', { rows: [], phases: [], ready: false, passphraseRequired: false });
    assert.equal(container.querySelector('.passphrase-dialog').open, false);
    mounted.unmount();
  });
}

test('an authoritative accepted snapshot closes the retry prompt after a pending view unmount', async () => {
  const t = await setup(); t.stream.emit('passphrase_required', { stack: 'encrypted' });
  t.container.querySelector('#passphrase-input').value = 'correct';
  t.container.querySelector('.passphrase-form').dispatch('submit');
  const unlock = t.calls.at(-1);
  t.mounted.unmount();
  unlock.reject(Object.assign(new Error('navigation aborted request'), { name: 'AbortError' })); await flush();
  const container = t.document.createElement('div'); t.document.append(container);
  const mounted = (await importOverview(t)).mount(container, { stream: t.stream, api: { post: async () => page([]), get: async () => null } });
  assert.equal(container.querySelector('.passphrase-dialog').open, true);
  t.stream.emit('snapshot', { rows: [], phases: [], ready: false, passphraseRequired: false });
  assert.equal(container.querySelector('.passphrase-dialog').open, false);
  mounted.unmount();
});

test('a late accepted request cannot erase a newer authoritative rejection requirement', async () => {
  const t = await setup(); t.stream.emit('passphrase_required', { stack: 'encrypted' });
  t.container.querySelector('#passphrase-input').value = 'wrong';
  t.container.querySelector('.passphrase-form').dispatch('submit'); const unlock = t.calls.at(-1);
  t.container.querySelector('.dialog-cancel').dispatch('click');
  t.stream.emit('snapshot', { rows: [], phases: [], ready: false, passphraseRequired: false });
  t.stream.emit('error', { message: 'Wrong passphrase', phase: 1 });
  t.stream.emit('passphrase_required', { stack: 'encrypted' });
  unlock.resolve(null); await flush();
  assert.equal(t.container.querySelector('.passphrase-dialog').open, true);
  assert.equal(t.stream.snapshot.passphraseRequired, true);
  t.mounted.unmount();
});

for (const recovery of ['phase', 'ready']) {
  test(`${recovery} recovery removes cached and visible loading errors`, async () => {
    const t = await setup();
    t.stream.emit('phase', { phase: 1, name: 'Loading stack', status: 'error' });
    t.stream.emit('error', { phase: 1, message: 'Stack loading failed' });
    assert.equal(t.container.querySelector('.error-banner').hidden, false);
    t.stream.emit(recovery, recovery === 'phase' ? { phase: 1, name: 'Retry loading', status: 'active' } : { totals: {} });
    assert.equal(t.stream.snapshot.errors.length, 0);
    assert.equal(t.container.querySelector('.error-banner').hidden, true);
    t.mounted.unmount();
  });
}

test('authoritative snapshots recover inline failures and clear resolved failures', async () => {
  const t = await setup();
  t.stream.emit('snapshot', { phases: [{ phase: 1, name: 'Loading stack', status: 'error' }], rows: [], ready: false,
    errors: [{ phase: 1, message: 'Stack loading failed' }] });
  assert.equal(t.container.querySelector('.error-banner').hidden, false);
  assert.match(t.container.querySelector('.error-banner').textContent, /Stack loading failed/);
  t.stream.emit('snapshot', { phases: [{ phase: 1, name: 'Loading stack', status: 'active' }], rows: [], ready: false, errors: [] });
  assert.equal(t.container.querySelector('.error-banner').hidden, true);
  t.mounted.unmount();
});

for(const event of ['snapshot','ready']) test(`mixed currency ${event} keeps rows and suppresses money`,async()=>{
 const t=await setup();t.calls[0].resolve(page([row('alpha')]));await flush();t.stream.emit(event,{rows:[row('alpha')],phases:[],ready:false,totals:{mixedCurrencies:true,totalActualDisplay:'$999.00',unavailableReason:'Totals unavailable: resources use different currencies.'}});await flush();assert.match(t.container.textContent,/Totals unavailable/);assert.doesNotMatch(t.container.querySelector('.totals-bar').textContent,/999/);t.mounted.unmount();
});

const dependent = (name, depth, via, extra = {}) => ({
  urn: `urn:${name}`, type: 'aws:ec2/instance:Instance', displayName: name, status: 'active',
  depth, via: via === 'root' ? 'urn:root' : `urn:${via}`, monthlyCost: 10, priced: true, component: false,
  monthlyDisplay: '$10.00', statusLabel: '', ...extra,
});
const radius = (dependents, extra = {}) => ({
  urn: 'urn:root', available: true, count: dependents.length, monthlyTotal: 10 * dependents.length,
  currency: 'USD', mixedCurrency: false, leftOut: { deleting: 0, unpriced: 0, unresolved: 0 }, dependents,
  summaryDisplay: `${dependents.length} dependents, $${10 * dependents.length}.00/month`, noteDisplay: '', ...extra,
});
const walk = (node, found = []) => {
  for (const child of node.children) {
    if (typeof child === 'string') continue;
    found.push(child); walk(child, found);
  }
  return found;
};
async function openBlast(blastRadius, { name = 'Root' } = {}) {
  const t = await setup(); t.calls[0].resolve(page([row('urn:root')])); await flush();
  t.container.querySelector('.resource-link').dispatch('click');
  t.calls.at(-1).resolve({ row: row('urn:root', { displayName: name }), blastRadius }); await flush();
  return t;
}

test('blast radius tree draws connectors from sibling position', async () => {
  const t = await openBlast(radius([
    dependent('a', 1, 'root'), dependent('c', 2, 'a'), dependent('b', 1, 'root'),
  ]));
  const names = t.container.querySelectorAll('.blast-name');
  assert.deepEqual(names.map((n) => n.querySelector('.blast-guides').textContent), ['├ ', '│ └ ', '└ ']);
  assert.deepEqual(names.map((n) => n.querySelector('.blast-link').textContent), ['a', 'c', 'b']);
  t.mounted.unmount();
});

test('blast radius names are row headers with hidden level text', async () => {
  const t = await openBlast(radius([dependent('a', 1, 'root'), dependent('c', 2, 'a')]));
  const headers = walk(t.container).filter((n) => n.tagName === 'th' && n.attrs.scope === 'row');
  assert.equal(headers.length, 2);
  assert.deepEqual(headers.map((h) => h.querySelector('.visually-hidden').textContent), ['Level 1', 'Level 2']);
  assert.ok(walk(t.container).some((n) => n.tagName === 'th' && n.attrs.scope === 'col'));
  assert.ok(t.container.querySelector('caption'));
  t.mounted.unmount();
});

test('blast radius renders the server status label and cost text in their own cells', async () => {
  const t = await openBlast(radius([
    dependent('n', 1, 'root', { status: 'creating', statusLabel: 'new' }),
    dependent('d', 1, 'root', { status: 'deleting', monthlyCost: null, statusLabel: 'deleting', monthlyDisplay: '-' }),
    dependent('k', 1, 'root', { component: true, monthlyCost: null, statusLabel: 'component', monthlyDisplay: '-' }),
    dependent('p', 1, 'root', { priced: false, monthlyCost: 1234.5, monthlyDisplay: 'no price' }),
  ]));
  assert.deepEqual(t.container.querySelectorAll('.blast-status').map((n) => n.textContent),
    ['new', 'deleting', 'component', '']);
  assert.deepEqual(t.container.querySelectorAll('.blast-status').map((n) => n.className),
    ['blast-status blast-status-new', 'blast-status blast-status-deleting', 'blast-status blast-status-component', 'blast-status blast-status-active']);
  assert.deepEqual(t.container.querySelector('.blast-scroll').querySelectorAll('.col-num').map((n) => n.textContent).filter((x) => x !== 'Monthly'),
    ['$10.00', '-', '-', 'no price']);
  t.mounted.unmount();
});

test('a component dependent is plain text, not a link to a detail it does not have', async () => {
  const t = await openBlast(radius([
    dependent('k', 1, 'root', { component: true, statusLabel: 'component', monthlyDisplay: '-' }), dependent('a', 2, 'k'),
  ]));
  assert.equal(t.container.querySelector('.blast-component').textContent, 'k');
  assert.deepEqual(t.container.querySelectorAll('.blast-link').map((n) => n.textContent), ['a']);
  t.mounted.unmount();
});

test('blast radius copy is the server text and the contract messages', async () => {
  const note = '1 deleted and 2 unpriced dependents are not in the total; 3 unresolved references';
  const t = await openBlast(radius([dependent('a', 1, 'root')], {
    leftOut: { deleting: 1, unpriced: 2, unresolved: 3 }, monthlyTotal: 9999, summaryDisplay: '1 dependent', noteDisplay: note,
  }));
  const caption = t.container.querySelector('caption');
  assert.equal(caption.querySelector('.blast-summary').textContent, '1 dependent');
  assert.ok(caption.querySelector('.blast-note') === null, 'the note wraps outside the fixed-height caption');
  assert.equal(t.container.querySelector('.blast-note').textContent, note);
  assert.ok(t.container.querySelector('.blast-scroll').querySelector('.blast-note') === null, 'the note is outside the scroll region');
  assert.doesNotMatch(caption.textContent, /9999|9,999/);
  t.mounted.unmount();
  const empty = await openBlast(radius([]));
  assert.equal(empty.container.querySelector('.detail-note').textContent, 'Nothing depends on this resource');
  assert.ok(empty.container.querySelector('.blast-note') === null, 'element is absent');
  empty.mounted.unmount();
  const leaf = await openBlast(radius([], { leftOut: { deleting: 0, unpriced: 0, unresolved: 1 }, noteDisplay: '1 unresolved reference' }));
  assert.deepEqual(leaf.container.querySelectorAll('.detail-note').map((n) => n.textContent),
    ['Nothing depends on this resource', '1 unresolved reference']);
  leaf.mounted.unmount();
  const unavailable = await openBlast({ urn: 'urn:root', available: false });
  assert.equal(unavailable.container.querySelector('.detail-note').textContent, 'Dependency information is not available');
  unavailable.mounted.unmount();
});

test('blast radius lists sit in a bounded scroll container', async () => {
  const t = await openBlast(radius([dependent('a', 1, 'root')]));
  const scroll = t.container.querySelector('.blast-scroll');
  assert.ok(scroll.querySelector('table'));
  assert.equal(scroll.attrs.tabindex, '0');
  assert.equal(scroll.attrs.role, 'region');
  assert.ok(scroll.attrs.style === undefined, 'inline style is blocked by the dashboard CSP');
  assert.equal(scroll.attrs['aria-label'], 'Blast radius dependents');
  t.mounted.unmount();
});

test('selecting a dependent opens its detail and Back returns to the earlier resource', async () => {
  const t = await openBlast(radius([dependent('a', 1, 'root')]), { name: 'Root' });
  assert.ok(t.container.querySelector('.detail-back') === null, 'element is absent');
  t.container.querySelector('.blast-link').dispatch('click');
  const request = t.calls.at(-1);
  assert.equal(request.path, `/api/overview/resource?urn=${encodeURIComponent('urn:a')}`);
  request.resolve({ row: row('urn:a', { displayName: 'A' }), blastRadius: radius([]) }); await flush();
  assert.equal(t.container.querySelector('.detail-panel').querySelector('h3').textContent, 'A');
  const back = t.container.querySelector('.detail-back');
  assert.equal(back.textContent, 'Back to Root');
  back.dispatch('click');
  assert.equal(t.calls.at(-1).path, `/api/overview/resource?urn=${encodeURIComponent('urn:root')}`);
  t.calls.at(-1).resolve({ row: row('urn:root', { displayName: 'Root' }), blastRadius: radius([dependent('a', 1, 'root')]) }); await flush();
  assert.ok(t.container.querySelector('.detail-back') === null, 'element is absent');
  assert.equal(t.container.querySelector('.detail-panel').querySelector('h3').textContent, 'Root');
  t.mounted.unmount();
});

test('closing detail after following a dependent returns focus to the originating row', async () => {
  const t = await setup(); t.calls[0].resolve(page([row('urn:root')])); await flush();
  const launcher = t.container.querySelector('.resource-link');
  launcher.focus(); launcher.dispatch('click');
  t.calls.at(-1).resolve({ row: row('urn:root', { displayName: 'Root' }), blastRadius: radius([dependent('a', 1, 'root')]) }); await flush();
  t.container.querySelector('.blast-link').focus();
  t.container.querySelector('.blast-link').dispatch('click');
  t.calls.at(-1).resolve({ row: row('urn:a', { displayName: 'A' }), blastRadius: radius([]) }); await flush();
  t.container.querySelector('.detail-close').dispatch('click');
  assert.equal(t.container.querySelector('.detail-panel').hidden, true);
  assert.ok(t.container.querySelector('.detail-back') === null, 'element is absent');
  assert.ok(t.document.activeElement === launcher, 'focus returns to the originating row link');
  t.mounted.unmount();
});

test('a dependent that no longer exists keeps Back available', async () => {
  const t = await openBlast(radius([dependent('gone', 1, 'root')]), { name: 'Root' });
  t.container.querySelector('.blast-link').dispatch('click');
  t.calls.at(-1).reject(new Error('not found')); await flush();
  assert.equal(t.container.querySelector('.detail-back').textContent, 'Back to Root');
  t.mounted.unmount();
});

test('a left-out note follows the scroll container so it can wrap', async () => {
  const t = await openBlast(radius([dependent('a', 1, 'root')], { noteDisplay: '1 deleted dependent is not in the total' }));
  const scroll = t.container.querySelector('.blast-scroll');
  const siblings = scroll.parent.children.filter((n) => typeof n !== 'string');
  assert.equal(siblings[siblings.indexOf(scroll) + 1].className, 'detail-note blast-note');
  assert.equal(scroll.className, 'table-wrap blast-scroll');
  t.mounted.unmount();
});
