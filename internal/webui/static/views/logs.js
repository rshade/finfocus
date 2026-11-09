import { el } from '../app.js';

const VIEWER_MODULE = '/static/zerolog-viewer/index.js';
const STREAM_URL = '/api/logs/stream';
const STATUS_URL = '/api/logs/status';
const RETRY_START_MS = 1000;
const RETRY_MAX_MS = 15000;
const EVENT_SOURCE_CLOSED = 2;

// probeLogs shows the nav entry only when status reports logs are enabled.
// The status route is always registered: a disabled server answers 200 with
// enabled false, and the entry stays hidden. A failed request does too.
export async function probeLogs(api, link) {
  let status;
  try {
    status = await api.get(STATUS_URL);
  } catch {
    return false;
  }
  if (status?.enabled !== true) return false;
  if (link) link.hidden = false;
  return true;
}

async function defaultCreateViewer(container, options) {
  const lib = await import(VIEWER_MODULE);
  return lib.createLogViewer(container, options);
}

function sourceNames(sources) {
  if (!Array.isArray(sources)) return [];
  return sources.map((s) => s?.name).filter((n) => typeof n === 'string' && n !== '');
}

export async function mount(container, ctx, deps = {}) {
  const Source = deps.EventSource ?? globalThis.EventSource;
  const createViewer = deps.createViewer ?? defaultCreateViewer;
  const setTimer = deps.setTimeout ?? globalThis.setTimeout.bind(globalThis);
  const clearTimer = deps.clearTimeout ?? globalThis.clearTimeout.bind(globalThis);

  const heading = el('h2', { tabindex: '-1', text: 'Logs' });
  const status = el('p', { class: 'logs-status', role: 'status' });
  const host = el('div', { class: 'logs-viewer' });
  container.append(heading, status, host);

  let initial;
  try {
    initial = await ctx.api.get(STATUS_URL);
  } catch (err) {
    status.textContent = err?.status === 404
      ? 'Logs are not enabled for this session.'
      : 'Logs are unavailable.';
    return {};
  }
  if (initial?.enabled !== true) {
    status.textContent = 'Logs are not enabled for this session.';
    return {};
  }

  const viewer = await createViewer(host, { follow: true, sources: sourceNames(initial?.sources) });
  let alive = true;
  let source = null;
  let timer = null;
  let delay = RETRY_START_MS;
  let lastSeq = 0;

  // A recreated EventSource cannot send Last-Event-ID, so the server replays
  // its snapshot; seq makes the replay idempotent.
  const accept = (entry) => {
    if (!entry || typeof entry !== 'object') return;
    if (typeof entry.seq === 'number') {
      if (entry.seq <= lastSeq) return;
      lastSeq = entry.seq;
    }
    viewer.push(entry);
  };
  const parse = (event) => {
    try {
      return JSON.parse(event.data);
    } catch {
      return null;
    }
  };
  const warnMissed = (missed) => {
    viewer.push({
      time: new Date().toISOString(),
      level: 'warn',
      source: 'finfocus',
      message: `${missed} log lines were missed while disconnected`,
      fields: {},
    });
  };

  let recreated = false;
  function connect() {
    if (!alive) return;
    status.textContent = 'Connecting…';
    const resumed = recreated;
    recreated = true;
    source = new Source(STREAM_URL);
    source.addEventListener('open', () => {
      delay = RETRY_START_MS;
      status.textContent = 'Live';
    });
    source.addEventListener('snapshot', (event) => {
      const data = parse(event);
      if (Array.isArray(data?.sources)) viewer.setSources?.(sourceNames(data.sources));
      const entries = data?.entries ?? [];
      const first = entries[0]?.seq;
      if (resumed && lastSeq > 0 && typeof first === 'number' && first > lastSeq + 1) {
        warnMissed(first - lastSeq - 1);
      }
      for (const entry of entries) accept(entry);
    });
    source.addEventListener('log', (event) => accept(parse(event)));
    source.addEventListener('gap', (event) => warnMissed(Number(parse(event)?.missed) || 0));
    source.addEventListener('error', () => {
      if (!alive || source?.readyState !== EVENT_SOURCE_CLOSED) return;
      status.textContent = 'Disconnected — retrying…';
      const wait = delay;
      delay = Math.min(delay * 2, RETRY_MAX_MS);
      timer = setTimer(connect, wait);
    });
  }
  connect();

  return {
    unmount() {
      alive = false;
      clearTimer(timer);
      source?.close();
      viewer.destroy?.();
    },
  };
}
