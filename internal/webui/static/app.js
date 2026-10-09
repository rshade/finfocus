// FinFocus web SPA shell (T023): hash-based view router, SSE client with
// reconnect, and a JSON fetch wrapper. The client is a renderer and input
// collector only (FR-011b): no cost math, formatting, filtering, or sorting
// happens here — every number rendered comes from server-computed fields.
//
// View module contract: each view lives at ./views/<name>.js and exports
//   export function mount(container, ctx) -> { unmount? } | void
// where ctx = { api, stream, el, navigate }. Views subscribe to stream events
// in mount and must unsubscribe in unmount.

const ROUTES = {
  overview: () => import('./views/overview.js'),
  cost: () => import('./views/cost.js'),
  recommendations: () => import('./views/recommendations.js'),
  estimate: () => import('./views/estimate.js'),
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
  constructor(message, { status, code } = {}) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

async function request(path, { method = 'GET', body, signal } = {}) {
  const init = { method, credentials: 'same-origin', signal };
  if (body !== undefined) {
    init.headers = { 'Content-Type': 'application/json' };
    init.body = JSON.stringify(body);
  }
  let resp;
  try {
    resp = await fetch(path, init);
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
    throw new ApiError(message, { status: resp.status, code });
  }
  return data;
}

export const api = {
  get: (path, opts) => request(path, { ...opts, method: 'GET' }),
  post: (path, body, opts) => request(path, { ...opts, method: 'POST', body: body ?? {} }),
};

// OverviewStream multiplexes named SSE events from /api/overview/stream.
// EventSource reconnects automatically while the stream stays open; when the
// browser reports the connection closed we recreate it with backoff so a
// server restart (or transient network failure) recovers without a reload.
// The server sends a `snapshot` event first on every (re)connect, so views
// recover the current load state from the stream itself.
export class OverviewStream {
  constructor(url = '/api/overview/stream', statusNode = null) {
    this.url = url;
    this.statusNode = statusNode;
    this.handlers = new Map();
    this.retryDelayMs = 1000;
    this.maxRetryDelayMs = 15000;
    this.source = null;
    this.retryTimer = null;
    this.closed = false;
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
    if (this.closed) return;
    clearTimeout(this.retryTimer);
    if (this.source) this.source.close();
    const source = new EventSource(this.url);
    this.source = source;

    source.addEventListener('open', () => {
      this.retryDelayMs = 1000;
      this.setStatus('Live', 'live');
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
      if (this.closed) return;
      if (source.readyState === EventSource.CONNECTING) {
        // EventSource is retrying on its own.
        this.setStatus('Reconnecting…', 'reconnecting');
        return;
      }
      // CLOSED: schedule a manual reconnect with backoff.
      this.setStatus('Disconnected — retrying…', 'down');
      clearTimeout(this.retryTimer);
      this.retryTimer = setTimeout(() => {
        this.retryDelayMs = Math.min(this.retryDelayMs * 2, this.maxRetryDelayMs);
        this.start();
      }, this.retryDelayMs);
    });
  }

  close() {
    this.closed = true;
    clearTimeout(this.retryTimer);
    if (this.source) this.source.close();
    this.setStatus('', 'down');
  }
}

function routeFromHash() {
  const name = location.hash.replace(/^#\/?/, '').split('/')[0];
  return Object.hasOwn(ROUTES, name) ? name : DEFAULT_ROUTE;
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
const stream = new OverviewStream('/api/overview/stream', statusNode);

const ctx = {
  api,
  stream,
  el,
  navigate(name) {
    location.hash = `#/${name}`;
  },
};

let activeView = null;
let routeVersion = 0;

async function renderRoute() {
  const version = ++routeVersion;
  const name = routeFromHash();
  if (!location.hash) {
    history.replaceState(null, '', `#/${DEFAULT_ROUTE}`);
  }
  updateNav(name);

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

  let mod;
  try {
    mod = await ROUTES[name]();
  } catch (err) {
    if (version !== routeVersion) return;
    console.error(`failed to load view "${name}"`, err);
    viewContainer.replaceChildren(
      el('section', { class: 'view-unavailable' },
        el('h2', { tabindex: '-1', text: 'View unavailable' }),
        el('p', { text: `The "${name}" view could not be loaded. It may not be built yet.` }),
      ),
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
    viewContainer.replaceChildren(
      el('section', { class: 'view-unavailable' },
        el('h2', { tabindex: '-1', text: 'View failed to load' }),
        el('p', { text: err instanceof Error ? err.message : String(err) }),
      ),
    );
    return;
  }

  const heading = viewContainer.querySelector('h2');
  if (heading && !viewContainer.querySelector('dialog[open]')) {
    heading.setAttribute('tabindex', '-1');
    heading.focus();
  }
}

window.addEventListener('hashchange', renderRoute);
document.querySelector('.skip-link')?.addEventListener('click', (event) => {
  event.preventDefault();
  document.getElementById('main')?.focus();
});
stream.start();
renderRoute();
