import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const staticDir = new URL('../static/', import.meta.url);
const flush = () => new Promise((resolve) => setImmediate(resolve));
const settle = async () => {
  for (let i = 0; i < 8; i += 1) await flush();
};

function deferred() {
  let resolve;
  const promise = new Promise((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
}

class FakeNode {
  constructor(tag, doc) {
    this.tag = tag;
    this.doc = doc;
    this.id = '';
    this.className = '';
    this.textContent = '';
    this.hidden = false;
    this.attrs = new Map();
    this.dataset = {};
    this.children = [];
    this.listeners = new Map();
  }

  append(...kids) {
    this.children.push(...kids);
  }

  replaceChildren(...kids) {
    this.children = [];
    this.append(...kids);
  }

  setAttribute(key, value) {
    this.attrs.set(key, String(value));
    if (key === 'id') this.id = String(value);
  }

  getAttribute(key) {
    return this.attrs.get(key) ?? null;
  }

  removeAttribute(key) {
    this.attrs.delete(key);
  }

  addEventListener(name, fn) {
    this.listeners.set(name, fn);
  }

  focus() {
    this.doc.focusTo(this);
  }

  get href() {
    return this.getAttribute('href');
  }

  matches(selector) {
    if (selector === 'h2') return this.tag === 'h2';
    if (selector === '[role="tab"]') return this.attrs.get('role') === 'tab';
    return false;
  }

  querySelectorAll(selector) {
    const found = [];
    for (const child of this.children) {
      if (typeof child !== 'object') continue;
      if (child.matches(selector)) found.push(child);
      found.push(...child.querySelectorAll(selector));
    }
    return found;
  }

  querySelector(selector) {
    return this.querySelectorAll(selector)[0] ?? null;
  }

  get tabs() {
    return this.children.filter((child) => typeof child === 'object' && child.attrs.get('role') === 'tab');
  }
}

async function setup({ stacks = [], hash = '#/overview', prefix, stacksResponse, stackNames } = {}) {
  // The server lists the stacks in the shell page only for two or more, so the
  // page carries a meta tag exactly when it has them.
  if (stackNames === undefined && stacks.length > 0) stackNames = stacks.map((entry) => entry.name).join(',');
  const log = [];
  const docListeners = new Map();
  const doc = {
    activeElement: null,
    focusTo(node) {
      doc.activeElement = node;
      docListeners.get('focusin')?.();
    },
  };
  const node = (tag) => new FakeNode(tag, doc);
  const view = node('div');
  const main = node('main');
  const tablist = node('div');
  tablist.hidden = true;
  const label = node('span');
  label.hidden = true;
  const announcer = node('div');
  const status = node('span');
  const notice = node('p');
  notice.hidden = true;
  const navLinks = ['overview', 'cost'].map((route) => {
    const link = node('a');
    link.dataset.route = route;
    link.setAttribute('href', `#/${route}`);
    return link;
  });
  const byId = { view, main, 'stack-tabs': tablist, 'stack-label': label, 'view-announcer': announcer, 'connection-status': status, 'stack-notice': notice };
  const document = {
    createElement: node,
    getElementById: (id) => byId[id] ?? null,
    querySelector: (selector) => {
      if (selector === 'meta[name="finfocus-base"]' && prefix !== undefined) return { content: prefix };
      if (selector === 'meta[name="finfocus-stacks"]' && stackNames !== undefined) return { content: stackNames };
      return null;
    },
    querySelectorAll: (selector) => (selector === '.app-nav a[data-route]' ? navLinks : []),
    addEventListener: (name, fn) => docListeners.set(name, fn),
    get activeElement() {
      return doc.activeElement;
    },
  };

  class MessageEvent {
    constructor(data) {
      this.data = data;
    }
  }
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
      log.push(`es:${url}`);
    }
    addEventListener(name, fn) {
      this.listeners.set(name, fn);
    }
    close() {
      if (this.readyState !== 2) log.push(`es-close:${this.url}`);
      this.readyState = 2;
    }
  }

  const state = { stacks, stacksResponse };
  const fetches = [];
  const respond = (body, status = 200) =>
    Promise.resolve({ ok: status < 400, status, text: async () => (body === null ? '' : JSON.stringify(body)) });
  const location = { hash };
  const windowListeners = new Map();
  const context = vm.createContext({
    document,
    window: { addEventListener: (name, fn) => windowListeners.set(name, fn) },
    location,
    history: {
      replaceState(_a, _b, url) {
        location.hash = url;
        log.push(`replace:${url}`);
      },
    },
    console,
    EventSource,
    MessageEvent,
    AbortController,
    setTimeout: () => 1,
    clearTimeout() {},
    fetch: (url, init = {}) => {
      const method = init.method ?? 'GET';
      fetches.push({ url, method });
      log.push(`fetch:${method} ${url}`);
      if (url === '/api/stacks' || url.endsWith('/api/stacks')) {
        if (state.stacksResponse) return state.stacksResponse(url);
        return respond({ stacks: state.stacks });
      }
      if (/\/api\/stacks\/[^/]+\/activate$/.test(url)) return respond({ name: 'x', state: 'loading' }, 202);
      return respond({});
    },
  });

  const mounts = [];
  const app = new vm.SourceTextModule(await readFile(new URL('app.js', staticDir), 'utf8'), {
    context,
    importModuleDynamically: async (specifier) => {
      const name = specifier.replace(/^.*\/(\w+)\.js$/, '$1');
      const mod = new vm.SyntheticModule(
        ['mount', 'probeLogs'],
        function () {
          this.setExport('mount', (container, viewCtx) => {
            container.append(new FakeNode('h2', doc));
            const record = { view: name, container, ctx: viewCtx, unmounted: false };
            mounts.push(record);
            log.push(`mount:${name}`);
            return {
              unmount() {
                record.unmounted = true;
                log.push(`unmount:${name}`);
              },
            };
          });
          this.setExport('probeLogs', async () => false);
        },
        { context },
      );
      await mod.link(() => {});
      await mod.evaluate();
      return mod;
    },
  });
  await app.link(() => {});
  await app.evaluate();
  await settle();

  return {
    ns: app.namespace,
    EventSource,
    fetches,
    log,
    mounts,
    nodes: { view, tablist, label, announcer, status, navLinks, notice },
    loc: location,
    doc,
    state,
    openSources: () => EventSource.all.filter((source) => source.readyState !== 2),
    lastSource: () => EventSource.all.at(-1),
    async hashchange(next) {
      if (next !== undefined) location.hash = next;
      windowListeners.get('hashchange')?.();
      await settle();
    },
    emit(source, name, data) {
      const listener = source.listeners.get(name);
      listener(name === 'error' ? new MessageEvent(JSON.stringify(data)) : { data: JSON.stringify(data) });
    },
    names: () => tablist.tabs.map((tab) => tab.dataset.stack),
  };
}

const entries = (...names) => names.map((name) => ({ name, state: 'idle' }));
const markerOf = (tab) => tab.children.find((child) => typeof child === 'object' && child.className === 'stack-tab-state');

// ---- one stack behaves exactly as today (US6, FR-013) ----------------------

test('with no stack entries no tab bar, label, or stack address appears', async () => {
  const t = await setup({ stacks: [] });
  assert.equal(t.nodes.tablist.hidden, true);
  assert.equal(t.nodes.tablist.tabs.length, 0);
  assert.equal(t.nodes.label.hidden, true);
  assert.equal(t.ns.base(), '');
  assert.equal(t.loc.hash, '#/overview');
  assert.equal(t.EventSource.all.length, 1);
  assert.equal(t.EventSource.all[0].url, '/api/overview/stream');
  assert.equal(t.fetches.filter((f) => f.method === 'POST').length, 0);
  assert.equal(t.nodes.view.getAttribute('role'), null);
  assert.equal(t.nodes.view.getAttribute('aria-labelledby'), null);
});

test('one stack entry is the single-stack page', async () => {
  const t = await setup({ stacks: entries('dev'), hash: '#/cost' });
  assert.equal(t.nodes.tablist.hidden, true);
  assert.equal(t.nodes.label.hidden, true);
  assert.equal(t.ns.base(), '');
  assert.equal(t.loc.hash, '#/cost');
  assert.equal(t.EventSource.all[0].url, '/api/overview/stream');
  assert.equal(t.fetches.filter((f) => f.method === 'POST').length, 0);
  assert.deepEqual(
    t.nodes.navLinks.map((link) => link.href),
    ['#/overview', '#/cost'],
  );
});

test('a page served under a stack prefix never shows tabs', async () => {
  const t = await setup({ stacks: entries('a', 'b'), prefix: '/s/dev' });
  assert.equal(t.nodes.tablist.hidden, true);
  assert.equal(t.ns.base(), '/s/dev');
  assert.equal(t.EventSource.all[0].url, '/s/dev/api/overview/stream');
  assert.equal(
    t.fetches.some((f) => f.url.endsWith('/api/stacks')),
    false,
  );
});

test('a malformed stack list in the page leaves the single-stack page', async () => {
  for (const stackNames of ['', 'a', 'a,a', 'a,,b/c', 'bad name,../x', 'a,b c']) {
    const t = await setup({ stackNames });
    assert.equal(t.nodes.tablist.hidden, true, stackNames);
    assert.equal(t.ns.base(), '', stackNames);
    assert.equal(t.EventSource.all[0].url, '/api/overview/stream', stackNames);
  }
});

test('booting reads the page and makes no request before the stream starts', async () => {
  const t = await setup({ stacks: entries('a', 'b') });
  const firstSource = t.log.findIndex((line) => line.startsWith('es:'));
  assert.ok(firstSource > 0);
  const before = t.log.slice(0, firstSource).filter((line) => line.startsWith('fetch:'));
  assert.deepEqual(before, ['fetch:POST /api/stacks/a/activate'], 'only the activation precedes the stream');
});

test('a stack list that never answers does not hold the boot', async () => {
  const t = await setup({ stacks: entries('a', 'b'), stacksResponse: () => new Promise(() => {}) });
  assert.equal(t.nodes.tablist.hidden, false);
  assert.equal(t.lastSource().url, '/s/a/api/overview/stream');
  assert.equal(t.mounts.at(-1).view, 'overview');
});

// ---- several stacks in one dashboard (US1) --------------------------------

test('three entries render a tablist in order with the first selected', async () => {
  const t = await setup({ stacks: entries('a', 'b', 'c') });
  assert.equal(t.nodes.tablist.hidden, false);
  assert.deepEqual(t.names(), ['a', 'b', 'c']);
  assert.equal(t.loc.hash, '#/s/a/overview');
  assert.equal(t.nodes.label.hidden, false);
  assert.equal(t.nodes.label.textContent, 'a');
  assert.equal(t.ns.base(), '/s/a');
});

test('an address with a stack selects that stack and its view', async () => {
  const t = await setup({ stacks: entries('a', 'b', 'c'), hash: '#/s/b/cost' });
  assert.equal(t.ns.base(), '/s/b');
  assert.equal(t.nodes.label.textContent, 'b');
  assert.equal(t.mounts.at(-1).view, 'cost');
  assert.equal(t.lastSource().url, '/s/b/api/overview/stream');
  assert.equal(t.nodes.tablist.tabs[1].getAttribute('aria-selected'), 'true');
});

test('a hash without a stack or with an unknown stack goes to the first stack, same view', async () => {
  for (const hash of ['#/cost', '#/s/zzz/cost', '#/s/', '#/s']) {
    const t = await setup({ stacks: entries('a', 'b'), hash });
    const want = hash.endsWith('cost') ? '#/s/a/cost' : '#/s/a/overview';
    assert.equal(t.loc.hash, want, hash);
    assert.equal(t.ns.base(), '/s/a', hash);
  }
});

test('requests go under the active stack and the stack list calls bypass it', async () => {
  const t = await setup({ stacks: entries('a', 'b'), hash: '#/s/b/overview' });
  await t.ns.api.get('/api/cost');
  assert.equal(t.fetches.at(-1).url, '/s/b/api/cost');
  const root = t.fetches.filter((f) => !f.url.startsWith('/s/'));
  assert.ok(root.some((f) => f.url === '/api/stacks' && f.method === 'GET'));
  assert.ok(root.some((f) => f.url === '/api/stacks/b/activate' && f.method === 'POST'));
  assert.equal(
    t.fetches.some((f) => f.url === '/s/b/api/stacks'),
    false,
  );
});

test('the header label follows the active stack', async () => {
  const t = await setup({ stacks: entries('a', 'b') });
  assert.equal(t.nodes.label.textContent, 'a');
  await t.hashchange('#/s/b/overview');
  assert.equal(t.nodes.label.textContent, 'b');
});

test('every tab link is a fragment, with no absolute URL', async () => {
  const t = await setup({ stacks: entries('a', 'b') });
  for (const tab of t.nodes.tablist.tabs) {
    assert.match(tab.href, /^#\/s\/[\w.-]+\/\w+$/);
  }
  for (const link of t.nodes.navLinks) {
    assert.match(link.href, /^#\/s\/a\/\w+$/);
  }
});

test('navigate keeps the active stack in the address', async () => {
  const t = await setup({ stacks: entries('a', 'b'), hash: '#/s/b/overview' });
  t.mounts.at(-1).ctx.navigate('cost');
  assert.equal(t.loc.hash, '#/s/b/cost');
});

// ---- stacks load only when opened (US2) -----------------------------------

test('switching stacks stops the old stream, activates, starts the new one, then mounts', async () => {
  const t = await setup({ stacks: entries('a', 'b') });
  const before = t.log.length;
  await t.hashchange('#/s/b/overview');
  const relevant = t.log
    .slice(before)
    .filter((line) => /^(unmount|es-close|es:|mount)/.test(line) || line === 'fetch:POST /api/stacks/b/activate');
  assert.deepEqual(relevant, [
    'unmount:overview',
    'es-close:/s/a/api/overview/stream',
    'fetch:POST /api/stacks/b/activate',
    'es:/s/b/api/overview/stream',
    'mount:overview',
  ]);
});

test('exactly one overview stream is open after switching back and forth', async () => {
  const t = await setup({ stacks: entries('a', 'b', 'c') });
  assert.equal(t.openSources().length, 1);
  for (const hash of ['#/s/b/overview', '#/s/c/cost', '#/s/a/overview', '#/s/b/overview']) {
    await t.hashchange(hash);
    assert.equal(t.openSources().length, 1, hash);
  }
  assert.equal(t.openSources()[0].url, '/s/b/api/overview/stream');
});

test('a view change inside one stack keeps the stream and activates nothing', async () => {
  const t = await setup({ stacks: entries('a', 'b') });
  const sources = t.EventSource.all.length;
  const posts = t.fetches.filter((f) => f.method === 'POST').length;
  await t.hashchange('#/s/a/cost');
  assert.equal(t.EventSource.all.length, sources);
  assert.equal(t.fetches.filter((f) => f.method === 'POST').length, posts);
  assert.equal(t.mounts.at(-1).view, 'cost');
});

test('the previous stack snapshot is discarded before the next view mounts', async () => {
  const t = await setup({ stacks: entries('a', 'b') });
  t.emit(t.lastSource(), 'snapshot', { rows: [{ urn: 'urn::a' }], phases: [], ready: true });
  await t.hashchange('#/s/b/overview');
  assert.equal(t.mounts.at(-1).ctx.stream.snapshot, null);
});

test('a switch that is superseded does not leave the stream stopped', async () => {
  const t = await setup({ stacks: entries('a', 'b') });
  await t.hashchange('#/s/b/overview');
  t.loc.hash = '#/s/a/overview';
  const pending = t.hashchange();
  await pending;
  assert.equal(t.openSources().length, 1);
  assert.equal(t.openSources()[0].url, '/s/a/api/overview/stream');
});

// ---- a broken stack keeps its tab (US3) -----------------------------------

test('after a stream error the tab states refresh and the failed tab is marked', async () => {
  const t = await setup({ stacks: entries('a', 'b') });
  const lists = t.fetches.filter((f) => f.url === '/api/stacks').length;
  t.state.stacks = [{ name: 'a', state: 'failed' }, { name: 'b', state: 'idle' }];
  t.emit(t.lastSource(), 'error', { message: 'Overview loading failed', phase: 1 });
  await settle();

  assert.equal(t.fetches.filter((f) => f.url === '/api/stacks').length, lists + 1);
  const [a, b] = t.nodes.tablist.tabs;
  assert.equal(markerOf(a).textContent, 'failed');
  assert.equal(a.getAttribute('aria-label'), 'a, failed');
  assert.equal(markerOf(b).textContent, '');
  assert.equal(b.getAttribute('aria-label'), null);
  assert.deepEqual(t.names(), ['a', 'b'], 'a failed stack keeps its tab');
});

const noticeText = (t) => t.nodes.notice.children.map((child) => (typeof child === 'string' ? child : child.textContent)).join('');
const noticeLink = (t) => t.nodes.notice.children.find((child) => typeof child === 'object')?.getAttribute('href');

test('a locked stack seen from another view points to its Overview', async () => {
  const t = await setup({ stacks: entries('a', 'b'), hash: '#/s/a/cost' });
  assert.equal(t.nodes.notice.hidden, true, 'no notice while the stack is fine');
  t.state.stacks = [{ name: 'a', state: 'locked' }, { name: 'b', state: 'idle' }];
  t.emit(t.lastSource(), 'passphrase_required', { stack: 'a' });
  await settle();

  assert.equal(t.nodes.notice.hidden, false);
  assert.equal(noticeText(t), 'a needs a passphrase. Open Overview to unlock it.');
  assert.equal(noticeLink(t), '#/s/a/overview');

  await t.hashchange('#/s/a/overview');
  assert.equal(t.nodes.notice.hidden, true, 'Overview shows the prompt itself');
});

test('a failed stack seen from another view points to its Overview, and the notice is not rewritten on refresh', async () => {
  const t = await setup({ stacks: entries('a', 'b'), hash: '#/s/a/estimate' });
  t.state.stacks = [{ name: 'a', state: 'failed' }, { name: 'b', state: 'idle' }];
  t.emit(t.lastSource(), 'error', { message: 'Overview loading failed', phase: 1 });
  await settle();
  assert.equal(noticeText(t), 'a failed to load. Open Overview for details.');
  const children = t.nodes.notice.children;

  t.emit(t.lastSource(), 'snapshot', {});
  await settle();
  assert.ok(t.nodes.notice.children === children, 'an unchanged notice is left alone');

  t.state.stacks = [{ name: 'a', state: 'failed' }, { name: 'b', state: 'ready' }];
  await t.hashchange('#/s/b/estimate');
  assert.equal(t.nodes.notice.hidden, true, 'the notice belongs to the active stack');
});

test('a rejected passphrase ends locked, not failed, whichever way the refreshes land', async () => {
  const t = await setup({ stacks: entries('a', 'b') });
  const first = deferred();
  const second = deferred();
  const queue = [first, second];
  t.state.stacksResponse = () => queue.shift().promise;

  t.emit(t.lastSource(), 'error', { message: 'Unable to decrypt stack. Try the passphrase again.', phase: 1 });
  t.emit(t.lastSource(), 'passphrase_required', { stack: 'a' });
  await settle();
  second.resolve({ ok: true, status: 200, text: async () => JSON.stringify({ stacks: [{ name: 'a', state: 'locked' }, { name: 'b', state: 'idle' }] }) });
  await settle();
  first.resolve({ ok: true, status: 200, text: async () => JSON.stringify({ stacks: [{ name: 'a', state: 'failed' }, { name: 'b', state: 'idle' }] }) });
  await settle();

  assert.equal(markerOf(t.nodes.tablist.tabs[0]).textContent, 'locked');
  assert.equal(t.nodes.tablist.tabs[0].getAttribute('aria-label'), 'a, locked');
});

test('the tab states also refresh on snapshot and ready', async () => {
  const t = await setup({ stacks: entries('a', 'b') });
  for (const event of ['snapshot', 'ready']) {
    const before = t.fetches.filter((f) => f.url === '/api/stacks').length;
    t.emit(t.lastSource(), event, event === 'ready' ? { totals: {} } : { rows: [], phases: [] });
    await settle();
    assert.equal(t.fetches.filter((f) => f.url === '/api/stacks').length, before + 1, event);
  }
});

// ---- per-stack passphrase (US4) --------------------------------------------

test('switching back to a locked stack reopens its prompt state and answers go to that stack', async () => {
  const t = await setup({ stacks: entries('a', 'b') });
  await t.hashchange('#/s/b/overview');
  await t.hashchange('#/s/a/overview');
  const stream = t.mounts.at(-1).ctx.stream;
  t.emit(t.lastSource(), 'snapshot', { rows: [], phases: [], passphraseRequired: true, stack: 'a' });
  assert.equal(stream.snapshot.passphraseRequired, true);
  assert.equal(stream.snapshot.stack, 'a');

  await t.mounts.at(-1).ctx.api.post('/api/passphrase', { passphrase: 'x' });
  assert.equal(t.fetches.at(-1).url, '/s/a/api/passphrase');
});

// ---- keyboard and ARIA (US5) ------------------------------------------------

test('the tablist exposes tablist, tab, selected state, roving tabindex, and a panel', async () => {
  const t = await setup({ stacks: entries('a', 'b', 'c'), hash: '#/s/b/overview' });
  assert.equal(t.nodes.tablist.getAttribute('role'), 'tablist');
  assert.ok(t.nodes.tablist.getAttribute('aria-label'));
  const tabs = t.nodes.tablist.tabs;
  assert.deepEqual(tabs.map((tab) => tab.getAttribute('aria-selected')), ['false', 'true', 'false']);
  assert.deepEqual(tabs.map((tab) => tab.getAttribute('tabindex')), ['-1', '0', '-1']);
  for (const tab of tabs) assert.equal(tab.getAttribute('aria-controls'), 'view');
  assert.equal(t.nodes.view.getAttribute('role'), 'tabpanel');
  assert.equal(t.nodes.view.getAttribute('aria-labelledby'), tabs[1].id);
});

const keydown = (t, tab, key, modifiers = {}) => {
  let prevented = false;
  t.nodes.tablist.listeners.get('keydown')({ key, target: tab, ...modifiers, preventDefault() { prevented = true; } });
  return prevented;
};

test('arrow keys wrap, Home and End jump, and each moves focus and selection', async () => {
  const t = await setup({ stacks: entries('a', 'b', 'c') });
  const tabs = t.nodes.tablist.tabs;
  const selected = () => tabs.findIndex((tab) => tab.getAttribute('aria-selected') === 'true');

  assert.equal(keydown(t, tabs[0], 'ArrowLeft'), true);
  await t.hashchange();
  assert.equal(selected(), 2, 'Left from the first tab wraps to the last');
  assert.ok(t.doc.activeElement === tabs[2], 'focus stays on the newly selected tab');

  keydown(t, tabs[2], 'ArrowRight');
  await t.hashchange();
  assert.equal(selected(), 0, 'Right from the last tab wraps to the first');
  assert.ok(t.doc.activeElement === tabs[0], 'focus stays on the newly selected tab');

  keydown(t, tabs[0], 'End');
  await t.hashchange();
  assert.equal(selected(), 2);

  keydown(t, tabs[2], 'Home');
  await t.hashchange();
  assert.equal(selected(), 0);

  keydown(t, tabs[0], 'ArrowRight');
  await t.hashchange();
  assert.equal(selected(), 1);
  assert.equal(t.loc.hash, '#/s/b/overview');
  assert.deepEqual(tabs.map((tab) => tab.getAttribute('tabindex')), ['-1', '0', '-1']);
});

test('clicking a tab keeps focus on it after the stack view mounts', async () => {
  const t = await setup({ stacks: entries('a', 'b') });
  const tabs = t.nodes.tablist.tabs;
  tabs[1].focus();
  await t.hashchange('#/s/b/overview');
  assert.equal(t.mounts.at(-1).view, 'overview');
  assert.ok(t.doc.activeElement === tabs[1], 'focus stays on the clicked tab');
});

test('a stack switch that does not start in the tablist still focuses the view heading', async () => {
  const t = await setup({ stacks: entries('a', 'b') });
  assert.equal(t.doc.activeElement?.tag, 'h2', 'the first view takes focus at boot');
  await t.hashchange('#/s/b/overview');
  assert.equal(t.doc.activeElement?.tag, 'h2');
  assert.ok(t.doc.activeElement !== t.mounts[0].container.children[0], 'focus moved to the new view');
});

test('keys other than the tab keys are left alone and keep the view', async () => {
  const t = await setup({ stacks: entries('a', 'b'), hash: '#/s/a/cost' });
  const tabs = t.nodes.tablist.tabs;
  assert.equal(keydown(t, tabs[0], 'Enter'), false);
  assert.equal(keydown(t, tabs[0], 'a'), false);
  assert.equal(t.loc.hash, '#/s/a/cost');
  keydown(t, tabs[0], 'ArrowRight');
  assert.equal(t.loc.hash, '#/s/b/cost', 'a tab switch keeps the current view');
});

test('a tab key with Alt, Ctrl, or Meta is left to the browser', async () => {
  const t = await setup({ stacks: entries('a', 'b'), hash: '#/s/a/cost' });
  const tabs = t.nodes.tablist.tabs;
  for (const modifiers of [{ altKey: true }, { ctrlKey: true }, { metaKey: true }]) {
    for (const key of ['ArrowLeft', 'ArrowRight', 'Home', 'End']) {
      assert.equal(keydown(t, tabs[0], key, modifiers), false, `${key} with ${Object.keys(modifiers)[0]}`);
    }
  }
  assert.equal(t.loc.hash, '#/s/a/cost');
});
