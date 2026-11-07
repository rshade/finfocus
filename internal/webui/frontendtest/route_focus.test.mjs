import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const flush = () => new Promise((resolve) => setImmediate(resolve));

function deferred() {
  let resolve;
  const promise = new Promise((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
}

async function setup() {
  const body = { name: 'body' };
  const navLink = { name: 'nav-link' };
  const heading = {
    focusCount: 0,
    setAttribute() {},
    focus() {
      this.focusCount += 1;
      document.activeElement = this;
    },
  };
  const element = () => ({
    id: '',
    children: [],
    append() {},
    replaceChildren() {},
    querySelector: () => null,
    querySelectorAll: () => [],
    addEventListener() {},
    setAttribute() {},
    removeAttribute() {},
    focus() {},
  });
  const view = element();
  view.querySelector = (selector) => (selector === 'h2' ? heading : null);
  const main = element();
  const statusNode = { textContent: '', dataset: {} };
  const document = {
    activeElement: body,
    body,
    createElement: element,
    getElementById: (id) => (id === 'view' ? view : id === 'main' ? main : id === 'connection-status' ? statusNode : null),
    querySelector: () => null,
    querySelectorAll: () => [],
    addEventListener() {},
  };
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
  const listeners = new Map();
  const mounts = [];
  const context = vm.createContext({
    document,
    window: { addEventListener: (name, fn) => listeners.set(name, fn) },
    location: { hash: '#/overview' },
    history: { replaceState() {} },
    console,
    EventSource,
    MessageEvent: class {},
    AbortController,
    setTimeout: () => 0,
    clearTimeout() {},
    fetch: () => new Promise(() => {}),
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
  return { document, body, navLink, heading, mounts, hashchange: () => listeners.get('hashchange')() };
}

test('a slow mount does not pull focus back after the user moved it', async () => {
  const t = await setup();
  assert.equal(t.mounts.length, 1);
  t.document.activeElement = t.navLink;
  t.mounts[0].resolve();
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
  t.document.activeElement = t.navLink;
  t.hashchange();
  await flush();
  assert.equal(t.mounts.length, 2);
  t.mounts[1].resolve();
  await flush();
  assert.equal(t.heading.focusCount, 1);
});
