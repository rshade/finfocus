import { el } from '../app.js';

// Every amount, percentage, and period shown here is display text from the
// server (internal/viewmodel). This view only fetches the list and renders it.
const COLUMNS = [
  ['Resource', 'col-resource'], ['Type', ''], ['Period', ''], ['Baseline', 'col-num'],
  ['Observed', 'col-num'], ['Deviation', 'col-num'], ['Drift %', 'col-num'], ['Currency', ''],
];
const ABSENT = '—';

export function mount(container, { api, stream }) {
  let alive = true, version = 0, queryAbort;
  const heading = el('h2', { tabindex: '-1', text: 'Anomalies' });
  const intro = el('p', { class: 'anomalies-intro', text: 'Resources whose month-to-date run-rate has diverged from the projected month.' });
  const error = el('p', { class: 'error-banner', role: 'alert' }); error.hidden = true;
  const status = el('p', { role: 'status', text: 'Loading anomalies…' });
  const body = el('tbody');
  const table = el('table', { class: 'overview-table anomalies-table' },
    el('caption', { text: 'Cost anomalies' }),
    el('thead', {}, el('tr', {}, ...COLUMNS.map(([text, cls]) => el('th', { scope: 'col', class: cls === 'col-num' ? cls : '', text })))), body);
  const wrap = el('div', { class: 'table-wrap', tabindex: '0', 'aria-label': 'Scrollable anomalies' }, table); wrap.hidden = true;
  container.append(heading, intro, error, status, wrap);

  function row(item) {
    const display = item.display ?? {};
    const values = [
      item.resource, item.type, display.period || ABSENT, display.baseline,
      display.observed, display.deviation, display.drift, item.currency || ABSENT,
    ];
    return el('tr', {}, ...values.map((text, index) => el('td', { class: COLUMNS[index][1], text })));
  }

  function render(result) {
    error.hidden = true;
    body.replaceChildren(...result.anomalies.map(row));
    wrap.hidden = result.anomalies.length === 0;
    status.textContent = result.anomalies.length ? result.summary : `No anomalies found (${result.summary}).`;
  }

  function showFailure(err) {
    body.replaceChildren(); wrap.hidden = true;
    if (err.status === 503) { error.hidden = true; status.textContent = 'Waiting for stack data…'; return; }
    error.textContent = err.message; error.hidden = false;
    status.textContent = 'Unable to load anomalies.';
  }

  async function query() {
    queryAbort?.abort(); queryAbort = new AbortController(); const current = ++version;
    table.setAttribute('aria-busy', 'true');
    try {
      const result = await api.get('/api/overview/anomalies', { signal: queryAbort.signal });
      if (alive && current === version) render(result);
    } catch (err) {
      if (alive && current === version && err.name !== 'AbortError') showFailure(err);
    } finally { if (alive && current === version) table.setAttribute('aria-busy', 'false'); }
  }

  const unsubs = [stream.on('ready', query), stream.on('snapshot', snapshot => { if (snapshot.ready) query(); })];
  query();
  return { unmount() { alive = false; version += 1; queryAbort?.abort(); for (const off of unsubs) off(); } };
}
