// FinFocus web SPA shell (T023): hash-based view router, SSE client with
// reconnect, and a JSON fetch wrapper. The client is a renderer and input
// collector only (FR-011b): no cost math, formatting, filtering, or sorting
// happens here — every number rendered comes from server-computed fields.
//
// View module contract: each view lives at ./views/<name>.js and exports
//   export function mount(container, ctx) -> { unmount? } | void
// where ctx = { api, stream, el, navigate }. Views subscribe to stream events
// in mount and must unsubscribe in unmount.
//
// Several stacks (FR-007): when the server lists two or more stacks the shell
// shows a tab bar, addresses become #/s/<stack>/<view>, and every request goes
// under the active stack's /s/<stack> prefix. One overview stream is open at a
// time, for the active stack. With fewer than two stacks nothing changes.

const ROUTES = {
  overview: () => import('./views/overview.js'),
  cost: () => import('./views/cost.js'),
  anomalies: () => import('./views/anomalies.js'),
  recommendations: () => import('./views/recommendations.js'),
  estimate: () => import('./views/estimate.js'),
  forecast: () => import('./views/forecast.js'),
  carbon: () => import('./views/carbon.js'),
  plugins: () => import('./views/plugins.js'),
  config: () => import('./views/config.js'),
  logs: () => import('./views/logs.js'),
};
const DEFAULT_ROUTE = 'overview';

const OVERVIEW_STREAM_EVENTS = [
  'snapshot',
  'phase',
  'row',
  'progress',
  'budget',
  'error',
  'expansion',
  'ready',
  'passphrase_required',
  'preview',
  'run_status',
];

// el builds a DOM node without innerHTML so payload text is never parsed as
// markup. attrs keys: class, text, dataset, aria*/role/etc. via setAttribute.
export function el(tag, attrs = {}, ...children) {
  const node = document.createElement(tag);
  for (const [key, value] of Object.entries(attrs)) {
    if (value === undefined || value === null) continue;
    if (key === 'text') {
      node.textContent = value;
    } else if (key === 'class') {
      node.className = value;
    } else if (key === 'dataset') {
      Object.assign(node.dataset, value);
    } else if (key.startsWith('on') && typeof value === 'function') {
      node.addEventListener(key.slice(2).toLowerCase(), value);
    } else {
      node.setAttribute(key, value);
    }
  }
  for (const child of children.flat()) {
    if (child === undefined || child === null) continue;
    node.append(child);
  }
  return node;
}

export class ApiError extends Error {
  constructor(message, { status, code, fields, data } = {}) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.fields = Array.isArray(fields) ? fields : [];
    this.data = data && typeof data === 'object' ? data : null;
  }
}

const BASE_PATTERN = /^\/s\/[A-Za-z0-9_.-]+$/;
const STACK_NAME_PATTERN = /^[A-Za-z0-9_.-]+$/;

// stackState is the multi-stack shell state. multi is true only for a page
// served at the root when the server lists two or more stacks. streaming is the
// stack the open overview stream belongs to, or null while none is running.
const stackState = { multi: false, active: '', entries: [], streaming: null };

// base is the path prefix requests go under: empty at the root, /s/<stack>
// under a stack. A page served under a stack declares it in a meta tag because
// the CSP forbids inline script. On the root page of a multi-stack dashboard it
// is the active stack. Anything that is not a stack path is ignored, so a
// prefix can never turn a request into a cross-origin one.
export function base() {
  const meta = globalThis.document?.querySelector?.('meta[name="finfocus-base"]');
  const value = meta?.content;
  if (typeof value === 'string' && BASE_PATTERN.test(value)) return value;
  return stackState.multi && STACK_NAME_PATTERN.test(stackState.active) ? `/s/${stackState.active}` : '';
}

// withBase is the only place a root-relative /api or /static path becomes a
// network URL. Every fetch, EventSource, and dynamic import goes through it.
export function withBase(path) {
  return base() + path;
}

// root: true sends the request to the root server, outside any stack prefix.
// Only the stack list and activation routes live there.
async function request(path, { method = 'GET', body, signal, root = false } = {}) {
  const init = { method, credentials: 'same-origin', signal };
  if (body !== undefined) {
    init.headers = { 'Content-Type': 'application/json' };
    init.body = JSON.stringify(body);
  }
  let resp;
  try {
    resp = await fetch(root ? path : withBase(path), init);
  } catch (err) {
    if (err.name === 'AbortError') throw err;
    throw new ApiError('Network error: the finfocus server is unreachable.', {
      status: 0,
      code: 'network',
    });
  }
  const text = await resp.text();
  let data = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = null;
    }
  }
  if (!resp.ok) {
    const message =
      data && typeof data.error === 'string'
        ? data.error
        : `Request failed with status ${resp.status}.`;
    const code = data && typeof data.code === 'string' ? data.code : 'http_error';
    const fields = data && Array.isArray(data.fields) ? data.fields : [];
    throw new ApiError(message, { status: resp.status, code, fields, data });
  }
  return data;
}

export const api = {
  get: (path, opts) => request(path, { ...opts, method: 'GET' }),
  post: (path, body, opts) => request(path, { ...opts, method: 'POST', body: body ?? {} }),
  del: (path, body, opts) => request(path, { ...opts, method: 'DELETE', body: body ?? {} }),
};

// mergeRunStatus applies a snapshot or run_status payload to the three
// state-only flags. Fields that are not booleans are ignored; a loaded
// preview clears detectFailed.
export function mergeRunStatus(current, data) {
  const next = {
    stateOnly: current.stateOnly === true,
    previewLoaded: current.previewLoaded === true,
    detectFailed: current.detectFailed === true,
  };
  for (const key of Object.keys(next)) {
    if (typeof data?.[key] === 'boolean') next[key] = data[key];
  }
  if (data?.previewLoaded) next.detectFailed = false;
  return next;
}

const SESSION_CHECK_TIMEOUT_MS = 5000;

const STREAM_TRANSITIONS = {
  idle: { start: 'connecting', close: 'closed' },
  connecting: { start: 'connecting', open: 'open', error: 'authenticating', close: 'closed' },
  open: { start: 'connecting', error: 'authenticating', close: 'closed' },
  authenticating: {
    start: 'connecting',
    open: 'open',
    recovered: 'reconnecting',
    exhausted: 'backoff',
    unauthorized: 'failed',
    close: 'closed',
  },
  reconnecting: { start: 'connecting', open: 'open', error: 'authenticating', close: 'closed' },
  backoff: { start: 'connecting', retry: 'connecting', close: 'closed' },
  failed: { close: 'closed' },
  closed: { reset: 'idle' },
};

// OverviewStream multiplexes named SSE events from /api/overview/stream.
// EventSource reconnects automatically while the stream stays open; when the
// browser reports the connection closed we recreate it with backoff so a
// server restart (or transient network failure) recovers without a reload.
// A closed connection first checks the session cookie with a same-origin
// fetch. A 401 stops the retry loop. The server sends a `snapshot` event
// first on every (re)connect, so views recover the current load state from
// the stream itself.
export class OverviewStream {
  constructor(url = '/api/overview/stream', statusNode = null) {
    this.url = url;
    this.statusNode = statusNode;
    this.handlers = new Map();
    this.retryDelayMs = 1000;
    this.maxRetryDelayMs = 15000;
    this.source = null;
    this.retryTimer = null;
    this.authAbort = null;
    this.authTimer = null;
    this.state = 'idle';
    this.snapshot = null;
    this.passphraseGeneration = 0;
  }

  on(event, handler) {
    if (!this.handlers.has(event)) this.handlers.set(event, new Set());
    this.handlers.get(event).add(handler);
    return () => this.handlers.get(event)?.delete(handler);
  }

  emit(event, data) {
    this.remember(event, data);
    for (const handler of this.handlers.get(event) ?? []) {
      try {
        handler(data);
      } catch (err) {
        console.error(`overview stream handler for "${event}" failed`, err);
      }
    }
  }

  // Cache transport state so mounting a view after the stream has advanced
  // recovers the same snapshot as a newly connected browser tab.
  remember(event, data) {
    if (!data || typeof data !== 'object') return;
    if (event === 'snapshot') {
      if (typeof data.passphraseRequired === 'boolean') this.passphraseGeneration += 1;
      this.snapshot = { ...data, rows: [...(data.rows ?? [])], phases: [...(data.phases ?? [])] };
      return;
    }
    const snapshot = this.snapshot ??= { rows: [], phases: [], ready: false };
    if (event === 'row') {
      const index = snapshot.rows.findIndex((row) => row.urn === data.urn);
      if (index >= 0) snapshot.rows[index] = data;
      else snapshot.rows.push(data);
    } else if (event === 'phase') {
      const index = snapshot.phases.findIndex((phase) => phase.phase === data.phase);
      if (index >= 0) snapshot.phases[index] = data;
      else snapshot.phases.push(data);
      if (data.status === 'active' || data.status === 'done') {
        snapshot.errors = (snapshot.errors ?? []).filter((error) => error.phase !== data.phase);
      }
    } else if (event === 'expansion') {
      snapshot.rows = [...(data.rows ?? [])];
    } else if (event === 'ready') {
      this.passphraseGeneration += 1;
      snapshot.ready = true;
      snapshot.totals = data.totals;
      snapshot.passphraseRequired = false;
      snapshot.errors = [];
    } else if (event === 'passphrase_required') {
      this.passphraseGeneration += 1;
      snapshot.passphraseRequired = true;
      snapshot.stack = data.stack;
    } else if (event === 'preview' || event === 'progress' || event === 'budget') {
      snapshot[event] = data;
    } else if (event === 'run_status') {
      // A later preview clears state-only. Remounts read this snapshot, not the live view.
      Object.assign(snapshot, mergeRunStatus(snapshot, data));
      if (typeof data.footnote === 'string') snapshot.footnote = data.footnote;
      if (typeof data.sourceWarning === 'string') snapshot.sourceWarning = data.sourceWarning;
      if (data.previewLoaded) snapshot.footnote = '';
    } else if (event === 'error') {
      snapshot.errors = [...(snapshot.errors ?? []).filter((error) => error.phase !== data.phase), data];
    }
  }

  acceptPassphrase(generation) {
    // Modal close and view unmount do not change the server requirement.
    // Only accept the requirement this request answered, never a newer retry.
    if (generation !== this.passphraseGeneration || !this.snapshot) return;
    this.snapshot.passphraseRequired = false;
    this.passphraseGeneration += 1;
  }

  setStatus(text, state) {
    if (!this.statusNode) return;
    this.statusNode.textContent = text;
    this.statusNode.dataset.state = state;
  }

  start() {
    this.dispatch('start');
  }

  close() {
    this.dispatch('close');
  }

  // reset ends the connection and forgets everything the stream learned, so
  // the same object can serve another stack. Handlers stay: a view removes its
  // own on unmount.
  reset() {
    this.close();
    this.dispatch('reset');
    this.snapshot = null;
    this.passphraseGeneration += 1;
    this.retryDelayMs = 1000;
  }

  // dispatch is the only place the connection state changes. A source that
  // errors is never restarted until the session check says it may be.
  dispatch(event) {
    const next = STREAM_TRANSITIONS[this.state]?.[event];
    if (!next) return;
    this.state = next;
    this[`enter_${next}`]();
  }

  enter_idle() {}

  enter_connecting() {
    clearTimeout(this.retryTimer);
    this.retryTimer = null;
    this.abortSessionCheck();
    if (this.source) this.source.close();
    const source = new EventSource(withBase(this.url));
    this.source = source;

    source.addEventListener('open', () => {
      if (source === this.source) this.dispatch('open');
    });
    for (const event of OVERVIEW_STREAM_EVENTS) {
      if (event === 'error') continue; // handled below, disambiguated
      source.addEventListener(event, (msg) => {
        let data = null;
        try {
          data = JSON.parse(msg.data);
        } catch (err) {
          console.error(`malformed SSE "${event}" payload`, err);
          return;
        }
        this.emit(event, data);
      });
    }
    // The contract's named `error` SSE event collides with EventSource's own
    // connection-error event type. Server-sent events arrive as MessageEvent
    // with string data; connection failures arrive as plain Event.
    source.addEventListener('error', (msg) => {
      if (msg instanceof MessageEvent && typeof msg.data === 'string') {
        let data = null;
        try {
          data = JSON.parse(msg.data);
        } catch (err) {
          console.error('malformed SSE "error" payload', err);
          return;
        }
        this.emit('error', data);
        return;
      }
      if (source === this.source) this.dispatch('error');
    });
  }

  enter_open() {
    this.retryDelayMs = 1000;
    this.abortSessionCheck();
    this.setStatus('Live', 'live');
  }

  // The check uses the existing session cookie and no bearer token. A 401
  // means the printed link is stale; anything else, including a timeout, may
  // retry. The timeout settles the check itself because a hung fetch may
  // never reject.
  enter_authenticating() {
    this.setStatus('Reconnecting…', 'reconnecting');
    const controller = new AbortController();
    this.authAbort = controller;
    const settle = (outcome) => {
      if (this.authAbort !== controller) return;
      this.abortSessionCheck();
      if (outcome === 'unauthorized') {
        this.dispatch('unauthorized');
      } else {
        this.dispatch(this.source?.readyState === EventSource.CLOSED ? 'exhausted' : 'recovered');
      }
    };
    this.authTimer = setTimeout(() => settle('failed'), SESSION_CHECK_TIMEOUT_MS);
    fetch(withBase('/api/overview/budget'), { credentials: 'same-origin', signal: controller.signal })
      .then((resp) => settle(resp.status === 401 ? 'unauthorized' : 'ok'))
      .catch(() => settle('failed'));
  }

  enter_reconnecting() {}

  enter_backoff() {
    this.setStatus('Disconnected — retrying…', 'down');
    this.retryTimer = setTimeout(() => {
      this.retryDelayMs = Math.min(this.retryDelayMs * 2, this.maxRetryDelayMs);
      this.dispatch('retry');
    }, this.retryDelayMs);
  }

  enter_failed() {
    this.shutdown();
    this.source = null;
    this.setStatus('Sign-in expired. Open the fresh link printed in the terminal.', 'down');
  }

  enter_closed() {
    this.shutdown();
    this.setStatus('', 'down');
  }

  shutdown() {
    clearTimeout(this.retryTimer);
    this.retryTimer = null;
    this.abortSessionCheck();
    if (this.source) this.source.close();
  }

  abortSessionCheck() {
    clearTimeout(this.authTimer);
    this.authTimer = null;
    const abort = this.authAbort;
    this.authAbort = null;
    abort?.abort();
  }
}

// resolveRoute reads the address. Without tabs it is #/<view>. With tabs it is
// #/s/<stack>/<view>; an address with no stack or an unknown one goes to the
// first stack, and an unknown view goes to the default view.
function resolveRoute() {
  const parts = location.hash.replace(/^#\/?/, '').split('/');
  if (!stackState.multi) {
    return { stack: '', view: Object.hasOwn(ROUTES, parts[0]) ? parts[0] : DEFAULT_ROUTE };
  }
  let stackName = '';
  let viewName = parts[0];
  if (parts[0] === 's') {
    stackName = parts[1] ?? '';
    viewName = parts[2] ?? '';
  }
  const known = stackState.entries.some((entry) => entry.name === stackName);
  return {
    stack: known ? stackName : stackState.entries[0].name,
    view: Object.hasOwn(ROUTES, viewName) ? viewName : DEFAULT_ROUTE,
  };
}

function updateNav(active) {
  for (const link of document.querySelectorAll('.app-nav a[data-route]')) {
    if (link.dataset.route === active) {
      link.setAttribute('aria-current', 'page');
    } else {
      link.removeAttribute('aria-current');
    }
  }
}

const viewContainer = document.getElementById('view');
const statusNode = document.getElementById('connection-status');
const announcer = document.getElementById('view-announcer');
const stream = new OverviewStream('/api/overview/stream', statusNode);

const ctx = {
  api,
  stream,
  el,
  navigate(name) {
    location.hash = stackState.multi ? `#/s/${stackState.active}/${name}` : `#/${name}`;
  },
};

const stackTabs = document.getElementById('stack-tabs');
const stackLabel = document.getElementById('stack-label');
const STACK_STATE_MARKERS = { failed: 'failed', locked: 'locked' };
const TAB_KEYS = new Set(['ArrowLeft', 'ArrowRight', 'Home', 'End']);
const tabNodes = new Map();
let currentView = DEFAULT_ROUTE;
let stackListVersion = 0;

// readStackList takes the stack names from the shell page. The server writes
// them into a meta tag, like the base prefix, only on the root page of a
// dashboard with two or more stacks, so the shell decides about tabs at boot
// without waiting on a request. Anything short of two valid names leaves the
// single-stack page, and a page served under /s/<name>/ never shows tabs.
// Every stack starts idle; the stack list endpoint supplies the live states.
function readStackList() {
  if (base()) return;
  const meta = document.querySelector('meta[name="finfocus-stacks"]');
  const names = typeof meta?.content === 'string' ? meta.content.split(',') : [];
  const list = [...new Set(names.filter((name) => STACK_NAME_PATTERN.test(name)))];
  if (list.length < 2) return;
  stackState.entries = list.map((name) => ({ name, state: 'idle' }));
  stackState.multi = true;
  stackState.active = list[0];
}

const tabId = (name) => `stack-tab-${name}`;
const stackHref = (name) => `#/s/${name}/${currentView}`;

// buildStackChrome creates the tab bar once. The nodes persist so a refresh
// updates them in place and never drops keyboard focus.
function buildStackChrome() {
  stackTabs.setAttribute('role', 'tablist');
  stackTabs.setAttribute('aria-label', 'Stacks');
  stackTabs.hidden = false;
  stackLabel.hidden = false;
  viewContainer.setAttribute('role', 'tabpanel');
  const links = stackState.entries.map((entry) => {
    const marker = el('span', { class: 'stack-tab-state' });
    const link = el('a', {
      id: tabId(entry.name),
      class: 'stack-tab',
      role: 'tab',
      'aria-controls': 'view',
      href: stackHref(entry.name),
      dataset: { stack: entry.name },
    }, el('span', { class: 'stack-tab-name', text: entry.name }), marker);
    tabNodes.set(entry.name, { link, marker });
    return link;
  });
  stackTabs.replaceChildren(...links);
  stackTabs.addEventListener('keydown', onTabKey);
  for (const event of ['error', 'passphrase_required', 'snapshot', 'ready']) {
    stream.on(event, refreshStackList);
  }
}

// updateStackChrome reflects the active stack and each stack's state. State
// comes from the server; a failed or locked tab says so in text and in its
// accessible name, never by color alone.
function updateStackChrome() {
  if (!stackState.multi) return;
  stackLabel.textContent = stackState.active;
  viewContainer.setAttribute('aria-labelledby', tabId(stackState.active));
  for (const entry of stackState.entries) {
    const { link, marker } = tabNodes.get(entry.name);
    const selected = entry.name === stackState.active;
    link.setAttribute('aria-selected', String(selected));
    link.setAttribute('tabindex', selected ? '0' : '-1');
    link.setAttribute('href', stackHref(entry.name));
    const text = STACK_STATE_MARKERS[entry.state] ?? '';
    marker.textContent = text;
    if (text) link.setAttribute('aria-label', `${entry.name}, ${text}`);
    else link.removeAttribute('aria-label');
  }
  for (const link of document.querySelectorAll('.app-nav a[data-route]')) {
    link.setAttribute('href', `#/s/${stackState.active}/${link.dataset.route}`);
  }
}

// refreshStackList reads the stack states again. The error event arrives before
// passphrase_required, so refreshes can overlap; only the newest answer counts.
async function refreshStackList() {
  const version = ++stackListVersion;
  try {
    const data = await api.get('/api/stacks', { root: true });
    if (version !== stackListVersion) return;
    const fresh = Array.isArray(data?.stacks) ? data.stacks : [];
    for (const entry of stackState.entries) {
      const next = fresh.find((item) => item?.name === entry.name);
      if (next && typeof next.state === 'string') entry.state = next.state;
    }
    updateStackChrome();
  } catch (err) {
    console.error('stack list refresh failed', err);
  }
}

function onTabKey(event) {
  if (!TAB_KEYS.has(event.key)) return;
  const names = stackState.entries.map((entry) => entry.name);
  const tab = event.target?.closest?.('[role="tab"]') ?? event.target;
  const from = Math.max(names.indexOf(tab?.dataset?.stack), 0);
  const last = names.length - 1;
  let next = from;
  if (event.key === 'Home') next = 0;
  else if (event.key === 'End') next = last;
  else if (event.key === 'ArrowRight') next = from === last ? 0 : from + 1;
  else next = from === 0 ? last : from - 1;
  event.preventDefault();
  tabNodes.get(names[next]).link.focus();
  location.hash = `#/s/${names[next]}/${currentView}`;
}

// switchStack moves the shell to another stack: stop the old stream and drop
// what it learned, ask the server to start the stack, then start the new
// stream. It reports false when a newer navigation replaced it, in which case
// no stream is left running for this stack.
async function switchStack(name, version) {
  stream.reset();
  stackState.streaming = null;
  stackState.active = name;
  updateStackChrome();
  try {
    await api.post(`/api/stacks/${encodeURIComponent(name)}/activate`, {}, { root: true });
  } catch (err) {
    console.error(`activating stack "${name}" failed`, err);
  }
  if (version !== routeVersion) return false;
  stackState.streaming = name;
  stream.start();
  refreshStackList();
  return true;
}

let activeView = null;
let routeVersion = 0;
let focusGains = 0;
document.addEventListener('focusin', () => {
  focusGains += 1;
});

// A tab focuses itself before it changes the address, so that focus gain
// predates the route; checking the tablist keeps focus on the tab, as the
// WAI-ARIA tabs pattern expects, instead of pulling it into the view.
function stackTabHasFocus() {
  const active = document.activeElement;
  return Boolean(active) && [...tabNodes.values()].some(({ link }) => link === active);
}

function focusHeading(focusGainsAtStart) {
  if (focusGains !== focusGainsAtStart || stackTabHasFocus()) return;
  const heading = viewContainer.querySelector('h2');
  if (heading && !viewContainer.querySelector('dialog[open]')) {
    heading.setAttribute('tabindex', '-1');
    heading.focus();
  }
}

// showViewError announces through the persistent #view-announcer region, which
// outlives the view swap, so the section itself carries no live-region role.
function showViewError(title, message, focusGainsAtStart) {
  announcer.textContent = `${title}. ${message}`;
  viewContainer.replaceChildren(
    el('section', { class: 'view-unavailable' },
      el('h2', { tabindex: '-1', text: title }),
      el('p', { text: message }),
    ),
  );
  focusHeading(focusGainsAtStart);
}

async function renderRoute() {
  const version = ++routeVersion;
  const focusGainsAtStart = focusGains;
  const route = resolveRoute();
  const name = route.view;
  if (stackState.multi) {
    const canonical = `#/s/${route.stack}/${name}`;
    if (location.hash !== canonical) history.replaceState(null, '', canonical);
  } else if (!location.hash) {
    history.replaceState(null, '', `#/${DEFAULT_ROUTE}`);
  }
  currentView = name;
  updateNav(name);
  announcer.textContent = '';

  if (activeView && typeof activeView.unmount === 'function') {
    try {
      activeView.unmount();
    } catch (err) {
      console.error('view unmount failed', err);
    }
  }
  activeView = null;
  viewContainer.replaceChildren(
    el('p', { class: 'view-loading', role: 'status' }, 'Loading view…'),
  );

  if (stackState.multi) {
    if (route.stack === stackState.streaming) updateStackChrome();
    else if (!(await switchStack(route.stack, version))) return;
  }

  let mod;
  try {
    mod = await ROUTES[name]();
  } catch (err) {
    if (version !== routeVersion) return;
    console.error(`failed to load view "${name}"`, err);
    showViewError(
      'View unavailable',
      `The "${name}" view could not be loaded. It may not be built yet.`,
      focusGainsAtStart,
    );
    return;
  }

  if (version !== routeVersion) return;

  // Each mount owns its container, including while an asynchronous mount is
  // pending. A late mount can only mutate its now-detached previous route.
  const routeContainer = el('div', { class: 'route-view' });
  viewContainer.replaceChildren(routeContainer);
  try {
    const mounted = (await mod.mount(routeContainer, ctx)) ?? null;
    if (version !== routeVersion) {
      mounted?.unmount?.();
      return;
    }
    activeView = mounted;
  } catch (err) {
    if (version !== routeVersion) return;
    console.error(`view "${name}" failed to mount`, err);
    showViewError('View failed to load', err instanceof Error ? err.message : String(err), focusGainsAtStart);
    return;
  }

  focusHeading(focusGainsAtStart);
}

readStackList();
if (stackState.multi) buildStackChrome();
window.addEventListener('hashchange', renderRoute);
document.querySelector('.skip-link')?.addEventListener('click', (event) => {
  event.preventDefault();
  document.getElementById('main')?.focus();
});
if (!stackState.multi) stream.start();
renderRoute();
ROUTES.logs()
  .then((mod) => mod.probeLogs(api, document.querySelector('.app-nav a[data-route="logs"]')))
  .catch((err) => console.error('logs probe failed', err));
