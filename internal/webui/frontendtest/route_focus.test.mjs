import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const flush = () => new Promise((resolve) => setImmediate(resolve));

function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

async function setup({ stacks } = {}) {
  const body = { name: 'body' };
  const navLink = { name: 'nav-link' };
  const state = { openDialog: false };
  const heading = {
    focusCount: 0,
    setAttribute() {},
    focus() {
      this.focusCount += 1;
      focusTo(this);
    },
  };
  const element = () => ({
    id: '',
    attrs: {},
    dataset: {},
    children: [],
    append() {},
    replaceChildren() {},
    querySelector: () => null,
    querySelectorAll: () => [],
    addEventListener() {},
    setAttribute(key, value) {
      this.attrs[key] = value;
    },
    removeAttribute() {},
    focus() {},
  });
  const view = element();
  view.querySelector = (selector) => {
    if (selector === 'h2') return heading;
    if (selector === 'dialog[open]') return state.openDialog ? {} : null;
    return null;
  };
  const replaced = [];
  view.replaceChildren = (...nodes) => replaced.push(...nodes);
  const main = element();
  const statusNode = { textContent: '', dataset: {} };
  const announcer = element();
  const stackTabs = element();
  const stackLabel = element();
  const documentListeners = new Map();
  const document = {
    activeElement: body,
    body,
    createElement: element,
    getElementById: (id) => (id === 'view' ? view : id === 'main' ? main : id === 'connection-status' ? statusNode : id === 'view-announcer' ? announcer : id === 'stack-tabs' ? stackTabs : id === 'stack-label' ? stackLabel : null),
    querySelector: (selector) => (stacks && selector === 'meta[name="finfocus-stacks"]' ? { content: stacks } : null),
    querySelectorAll: () => [],
    addEventListener: (name, fn) => documentListeners.set(name, fn),
  };
  function focusTo(target) {
    document.activeElement = target;
    documentListeners.get('focusin')?.();
  }
  class EventSource {
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSED = 2;
    constructor() {
      this.readyState = EventSource.CONNECTING;
    }
    addEventListener() {}
    close() {}
  }
  const windowListeners = new Map();
  const location = { hash: '#/overview' };
  const mounts = [];
  const context = vm.createContext({
    document,
    window: { addEventListener: (name, fn) => windowListeners.set(name, fn) },
    location,
    history: { replaceState() {} },
    console: { ...console, error() {} },
    EventSource,
    MessageEvent: class {},
    AbortController,
    setTimeout: () => 0,
    clearTimeout() {},
    fetch: (url) =>
      /\/activate$/.test(url)
        ? Promise.resolve({ ok: true, status: 202, text: async () => '{}' })
        : new Promise(() => {}),
  });
  const app = new vm.SourceTextModule(await readFile(new URL('../static/app.js', import.meta.url), 'utf8'), {
    context,
    importModuleDynamically: async () => {
      const mod = new vm.SyntheticModule(
        ['mount', 'probeLogs'],
        function () {
          this.setExport('probeLogs', () => {});
          this.setExport('mount', () => {
            const pending = deferred();
            mounts.push(pending);
            return pending.promise;
          });
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
  await flush();
  return {
    location,
    document,
    announcer,
    body,
    navLink,
    heading,
    mounts,
    state,
    replaced,
    focusTo,
    loseFocusToBody: () => {
      document.activeElement = body;
    },
    hashchange: () => windowListeners.get('hashchange')(),
  };
}

test('a slow mount does not pull focus back after the user moved it', async () => {
  const t = await setup();
  assert.equal(t.mounts.length, 1);
  t.focusTo(t.navLink);
  t.mounts[0].resolve();
  await flush();
  assert.equal(t.heading.focusCount, 0);
  assert.equal(t.document.activeElement, t.navLink);
});

test('focus that leaves and returns to the starting link during a mount is not stolen', async () => {
  const t = await setup();
  t.mounts[0].resolve();
  await flush();
  t.focusTo(t.navLink);
  t.hashchange();
  await flush();
  t.heading.focusCount = 0;
  t.focusTo({ name: 'elsewhere' });
  t.focusTo(t.navLink);
  t.mounts[1].resolve();
  await flush();
  assert.equal(t.heading.focusCount, 0);
  assert.equal(t.document.activeElement, t.navLink);
});

test('an initial mount that finishes with focus untouched focuses the heading', async () => {
  const t = await setup();
  t.mounts[0].resolve();
  await flush();
  assert.equal(t.heading.focusCount, 1);
});

test('navigating from a focused nav link still moves focus to the new heading', async () => {
  const t = await setup();
  t.mounts[0].resolve();
  await flush();
  t.heading.focusCount = 0;
  t.focusTo(t.navLink);
  t.hashchange();
  await flush();
  assert.equal(t.mounts.length, 2);
  t.mounts[1].resolve();
  await flush();
  assert.equal(t.heading.focusCount, 1);
});

test('focus lost to body without a focus event, as when the old view is removed, still gets the heading', async () => {
  const t = await setup();
  t.mounts[0].resolve();
  await flush();
  t.heading.focusCount = 0;
  t.focusTo(t.navLink);
  t.hashchange();
  await flush();
  t.loseFocusToBody();
  t.mounts[1].resolve();
  await flush();
  assert.equal(t.heading.focusCount, 1);
});

test('an open dialog in the view keeps focus off the heading', async () => {
  const t = await setup();
  t.state.openDialog = true;
  t.mounts[0].resolve();
  await flush();
  assert.equal(t.heading.focusCount, 0);
});

test('a mount failure announces through the persistent region and focuses its heading', async () => {
  const t = await setup();
  t.mounts[0].reject(new Error('boom'));
  await flush();
  const section = t.replaced.at(-1);
  assert.equal(section.attrs.role, undefined);
  assert.match(t.announcer.textContent, /^View failed to load\. .*boom$/);
  assert.equal(t.heading.focusCount, 1);
});

test('a mount failure does not steal focus the user already moved', async () => {
  const t = await setup();
  t.focusTo(t.navLink);
  t.mounts[0].reject(new Error('boom'));
  await flush();
  assert.equal(t.heading.focusCount, 0);
  assert.equal(t.document.activeElement, t.navLink);
});

// ---- switching stacks (several stacks) --------------------------------------

test('a stack switch does not pull focus back after the user moved it', async () => {
  const t = await setup({ stacks: 'a,b' });
  await flush();
  t.mounts[0].resolve();
  await flush();
  t.heading.focusCount = 0;
  t.location.hash = '#/s/b/overview';
  t.hashchange();
  await flush();
  assert.equal(t.mounts.length, 2, 'the new stack mounts its view once it is activated');
  const elsewhere = { name: 'elsewhere' };
  t.focusTo(elsewhere);
  t.mounts[1].resolve();
  await flush();
  assert.equal(t.heading.focusCount, 0);
  assert.equal(t.document.activeElement, elsewhere);
});

test('a stack switch from a focused tab still moves focus to the new heading', async () => {
  const t = await setup({ stacks: 'a,b' });
  await flush();
  t.mounts[0].resolve();
  await flush();
  t.heading.focusCount = 0;
  t.focusTo(t.navLink);
  t.location.hash = '#/s/b/overview';
  t.hashchange();
  await flush();
  t.mounts[1].resolve();
  await flush();
  assert.equal(t.heading.focusCount, 1);
});

test('a view that fails after a stack switch announces through the persistent region', async () => {
  const t = await setup({ stacks: 'a,b' });
  await flush();
  t.mounts[0].resolve();
  await flush();
  t.heading.focusCount = 0;
  t.location.hash = '#/s/b/overview';
  t.hashchange();
  await flush();
  t.mounts[1].reject(new Error('stack b is not available'));
  await flush();
  assert.match(t.announcer.textContent, /^View failed to load\. .*stack b is not available$/);
  const section = t.replaced.at(-1);
  assert.equal(section.attrs.role, undefined, 'the section carries no live-region role, or the error is read twice');
  assert.equal(t.heading.focusCount, 1);
});
