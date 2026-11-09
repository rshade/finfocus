import { el } from './app.js';

const NO_HISTORY = 'No cost history is stored yet. Run finfocus cost history collect.';
const NO_POINTS = 'No cost history points are stored for this resource.';
const NO_SNAPSHOTS = 'No snapshots in this range.';
const NO_CHANGES = 'Nothing was added, removed, or changed.';
const UNAVAILABLE = 'Cost history is unavailable.';
const LOAD_FAILED = 'Cost history could not be loaded.';

// deltaPhrase reads the direction the server already encoded in a display
// string. Color repeats that word; it is not the only signal.
function deltaPhrase(text) {
  const value = typeof text === 'string' ? text : '';
  if (value.startsWith('+')) return { word: 'increase', className: 'delta-up' };
  if (value.startsWith('-') || value.startsWith('\u2212')) return { word: 'decrease', className: 'delta-down' };
  if (value === '') return { word: '', className: '' };
  return { word: 'unchanged', className: 'delta-flat' };
}

function historyMessage(err) {
  const message = err && typeof err.message === 'string' ? err.message : '';
  if (!message || message === '[object Object]') return UNAVAILABLE;
  return message;
}

function isQueryError(err) {
  return err?.status === 400 || err?.code === 'invalid_history_query';
}

function isNotReady(err) {
  return err?.status === 503 || err?.code === 'not_ready';
}

function stillCurrent(current) {
  return typeof current !== 'function' || current();
}

function queryString(params) {
  const parts = [];
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || String(value).trim() === '') continue;
    parts.push(`${encodeURIComponent(key)}=${encodeURIComponent(value)}`);
  }
  return parts.length ? `?${parts.join('&')}` : '';
}

function setDescribed(inputs, id, on) {
  for (const input of inputs) {
    if (on) input.setAttribute('aria-describedby', id);
    else if (typeof input.removeAttribute === 'function') input.removeAttribute('aria-describedby');
  }
}

function pointsTable(points) {
  return el('div', { class: 'table-wrap', tabindex: '0' },
    el('table', { class: 'detail-table history-points' },
      el('caption', { text: 'Cost history' }),
      el('thead', {}, el('tr', {},
        el('th', { scope: 'col', text: 'Date' }),
        el('th', { scope: 'col', text: 'Monthly cost' }))),
      el('tbody', {}, ...points.map((point) => el('tr', {},
        el('td', { text: point.date ?? '' }),
        el('td', { class: 'col-num', text: point.amount ?? '' }))))));
}

// appendResourceHistory adds the detail-panel section and loads it. The
// caller decides when a late response is stale.
export function appendResourceHistory(parent, { get, id, signal, current }) {
  const alert = el('p', { class: 'error-banner', role: 'alert' });
  alert.hidden = true;
  const status = el('p', { role: 'status', text: 'Loading cost history…' });
  const section = el('section', { 'aria-labelledby': 'cost-history-resource-title' },
    el('h4', { id: 'cost-history-resource-title', text: 'Cost history' }), alert, status);
  parent.append(section);
  if (typeof id !== 'string' || id.trim() === '') {
    status.textContent = NO_POINTS;
    return section;
  }
  get(`/api/history/resource?id=${encodeURIComponent(id)}`, { signal }).then((body) => {
    if (!stillCurrent(current)) return;
    alert.hidden = true;
    if (body?.status === 'no_history') {
      status.hidden = false;
      status.textContent = NO_HISTORY;
      return;
    }
    const points = body?.display?.points;
    if (!Array.isArray(points) || points.length === 0) {
      status.hidden = false;
      status.textContent = NO_POINTS;
      return;
    }
    status.hidden = true;
    section.append(pointsTable(points));
  }).catch((err) => {
    if (err?.name === 'AbortError' || !stillCurrent(current)) return;
    alert.hidden = false;
    alert.textContent = historyMessage(err);
    status.hidden = false;
    status.textContent = LOAD_FAILED;
  });
  return section;
}

function svgEl(tag, attrs) {
  const node = document.createElementNS('http://www.w3.org/2000/svg', tag);
  for (const [key, value] of Object.entries(attrs)) node.setAttribute(key, String(value));
  return node;
}

// Chart coordinates come from stored totals and the numeric budget. Every
// visible amount is a display string. The chart is hidden from assistive
// technology; the series table is the text alternative.
function historyChart(body) {
  if (typeof document.createElementNS !== 'function') return null;
  const snapshots = Array.isArray(body?.snapshots) ? body.snapshots : [];
  const points = [];
  for (const snapshot of snapshots) {
    const x = Date.parse(snapshot?.timestamp);
    const y = snapshot?.total_monthly;
    if (Number.isFinite(x) && Number.isFinite(y)) points.push({ x, y, version: snapshot.version });
  }
  if (points.length === 0) return null;
  const budget = typeof body.budget === 'number' && Number.isFinite(body.budget) ? body.budget : null;
  const width = 640;
  const height = 160;
  const pad = 28;
  let minY = Math.min(...points.map((point) => point.y));
  let maxY = Math.max(...points.map((point) => point.y));
  if (budget !== null) {
    minY = Math.min(minY, budget);
    maxY = Math.max(maxY, budget);
  }
  if (minY === maxY) {
    minY -= 1;
    maxY += 1;
  }
  const minX = Math.min(...points.map((point) => point.x));
  const maxX = Math.max(...points.map((point) => point.x));
  const sx = (x) => pad + (maxX === minX ? (width - 2 * pad) / 2 : ((x - minX) / (maxX - minX)) * (width - 2 * pad));
  const sy = (y) => height - pad - ((y - minY) / (maxY - minY)) * (height - 2 * pad);
  const svg = svgEl('svg', {
    class: 'history-chart',
    viewBox: `0 0 ${width} ${height}`,
    role: 'presentation',
    'aria-hidden': 'true',
    focusable: 'false',
  });
  svg.append(svgEl('polyline', {
    points: points.map((point) => `${sx(point.x)},${sy(point.y)}`).join(' '),
    fill: 'none',
    stroke: 'currentColor',
    'stroke-width': '2',
  }));
  if (budget !== null && body.display?.budget) {
    const y = sy(budget);
    svg.append(svgEl('line', {
      class: 'history-budget-line',
      x1: pad,
      x2: width - pad,
      y1: y,
      y2: y,
    }));
  }
  const annotations = Array.isArray(body.annotations) ? body.annotations : [];
  annotations.forEach((annotation, index) => {
    const point = points.find((item) => item.version === annotation.version);
    if (!point) return;
    const mark = svgEl('g', { class: 'history-marker' });
    mark.append(svgEl('circle', { cx: sx(point.x), cy: sy(point.y), r: '7' }));
    const label = svgEl('text', { x: sx(point.x), y: sy(point.y) + 3, 'text-anchor': 'middle' });
    label.textContent = String(index + 1);
    mark.append(label);
    svg.append(mark);
  });
  return svg;
}

function seriesTable(rows) {
  return el('div', { class: 'table-wrap', tabindex: '0' },
    el('table', { class: 'detail-table history-series' },
      el('caption', { text: 'Cost history series' }),
      el('thead', {}, el('tr', {},
        el('th', { scope: 'col', text: 'Version' }),
        el('th', { scope: 'col', text: 'Date' }),
        el('th', { scope: 'col', text: 'Total' }))),
      el('tbody', {}, ...rows.map((row) => el('tr', {},
        el('td', { text: row.version === undefined || row.version === null ? '' : String(row.version) }),
        el('td', { text: row.date ?? '' }),
        el('td', { class: 'col-num', text: row.total ?? '' }))))));
}

function annotationList(annotations) {
  if (!Array.isArray(annotations) || annotations.length === 0) return null;
  return el('ol', { class: 'history-annotations' }, ...annotations.map((item, index) => el('li', {
    text: `${index + 1}. v${item.version} ${item.kind}: ${item.message}`,
  })));
}

function diffRows(body) {
  const rows = [];
  for (const name of ['added', 'removed', 'changed']) {
    const items = Array.isArray(body?.[name]) ? body[name] : [];
    const displays = Array.isArray(body?.display?.[name]) ? body.display[name] : [];
    const count = Math.min(items.length, displays.length);
    for (let i = 0; i < count; i += 1) {
      rows.push({
        group: name,
        urn: items[i]?.urn ?? '',
        type: items[i]?.type ?? '',
        monthly: displays[i]?.monthlyCost ?? '',
        delta: displays[i]?.delta ?? '',
      });
    }
  }
  return rows;
}

function diffTable(rows) {
  return el('div', { class: 'table-wrap', tabindex: '0' },
    el('table', { class: 'detail-table history-diff-table' },
      el('caption', { text: 'Cost history diff' }),
      el('thead', {}, el('tr', {},
        ...['Change', 'Resource', 'Type', 'Monthly cost', 'Delta'].map((text) => el('th', { scope: 'col', text })))),
      el('tbody', {}, ...rows.map((row) => {
        const phrase = deltaPhrase(row.delta);
        const delta = phrase.word ? `${row.delta} ${phrase.word}` : row.delta;
        return el('tr', {},
          el('td', { text: row.group }),
          el('td', { text: row.urn }),
          el('td', { text: row.type }),
          el('td', { class: 'col-num', text: row.monthly }),
          el('td', { class: `col-num ${phrase.className}`, text: delta }));
      }))));
}

// mountCostHistoryPanel builds the Cost view slot. The caller loads it after
// the cost query so that request stays first, and retries only after a 503.
export function mountCostHistoryPanel({ api }) {
  let generation = 0;
  let viewAbort;
  let diffAbort;
  let waitingForReady = false;
  let alertOwner = '';

  const from = el('input', { id: 'history-from', class: 'filter-input', type: 'text' });
  const to = el('input', { id: 'history-to', class: 'filter-input', type: 'text' });
  const currency = el('input', { id: 'history-currency', class: 'filter-input', type: 'text' });
  const queryError = el('p', { id: 'history-query-error', class: 'field-error', role: 'alert' });
  queryError.hidden = true;
  const rangeForm = el('form', { class: 'toolbar history-controls' },
    el('label', { class: 'filter-label', for: 'history-from' }, 'From', from),
    el('label', { class: 'filter-label', for: 'history-to' }, 'To', to),
    el('label', { class: 'filter-label', for: 'history-currency' }, 'Currency', currency),
    el('button', { type: 'submit', text: 'Apply range' }));
  const warning = el('p', { id: 'history-warning', class: 'warn-banner', role: 'status' });
  warning.hidden = true;
  const alert = el('p', { id: 'history-alert', class: 'error-banner', role: 'alert' });
  alert.hidden = true;
  const status = el('p', { id: 'history-status', role: 'status', text: 'Loading cost history…' });
  const series = el('div', { class: 'history-series-slot' });
  const diffFrom = el('input', { id: 'history-diff-from', class: 'filter-input', type: 'text', list: 'history-diff-options' });
  const diffTo = el('input', { id: 'history-diff-to', class: 'filter-input', type: 'text', list: 'history-diff-options' });
  const diffList = el('datalist', { id: 'history-diff-options' });
  const diffError = el('p', { id: 'history-diff-error', class: 'field-error', role: 'alert' });
  diffError.hidden = true;
  const diffForm = el('form', { class: 'toolbar history-controls' },
    el('label', { class: 'filter-label', for: 'history-diff-from' }, 'Compare from', diffFrom),
    el('label', { class: 'filter-label', for: 'history-diff-to' }, 'Compare to', diffTo),
    diffList,
    el('button', { type: 'submit', text: 'Compare snapshots' }));
  const diffStatus = el('p', { id: 'history-diff-status', role: 'status' });
  const diffBody = el('div', { id: 'history-diff-body' });
  const diffSection = el('div', { class: 'history-diff' }, diffForm, diffError, diffStatus, diffBody);
  diffSection.hidden = true;
  const section = el('section', { class: 'history-panel', 'aria-labelledby': 'cost-history-panel-title' },
    el('h3', { id: 'cost-history-panel-title', text: 'Cost history' }),
    rangeForm, queryError, warning, alert, status, series, diffSection);
  const rangeInputs = [from, to, currency];

  function clearOwnedAlert(owner) {
    if (alertOwner !== owner) return;
    alert.hidden = true;
    alert.textContent = '';
    alertOwner = '';
    waitingForReady = false;
  }

  function showAlert(owner, message, notReady) {
    alert.hidden = false;
    alert.textContent = message;
    alertOwner = owner;
    waitingForReady = notReady;
  }

  function showWarning(text) {
    if (typeof text === 'string' && text !== '') {
      warning.textContent = text;
      warning.hidden = false;
      return;
    }
    warning.textContent = '';
    warning.hidden = true;
  }

  function fillSelectors(rows) {
    diffList.replaceChildren();
    const seen = new Set();
    for (const row of rows) {
      for (const value of [`v${row.version}`, row.date]) {
        if (!value || seen.has(value)) continue;
        seen.add(value);
        diffList.append(el('option', { value }));
      }
    }
    diffFrom.value = `v${rows[0].version}`;
    diffTo.value = `v${rows[rows.length - 1].version}`;
  }

  function paintDiff(body) {
    clearOwnedAlert('diff');
    diffError.hidden = true;
    setDescribed([diffFrom, diffTo], 'history-diff-error', false);
    if (body?.status === 'no_history') {
      diffStatus.hidden = false;
      diffStatus.textContent = NO_HISTORY;
      diffBody.replaceChildren();
      return;
    }
    const phrase = deltaPhrase(body?.display?.delta);
    const change = `Change: ${body?.display?.delta ?? ''}${phrase.word ? ` ${phrase.word}` : ''}`;
    const totals = el('div', { id: 'history-diff-totals', class: 'totals-bar', 'aria-label': 'Cost history diff totals' },
      el('p', { class: `history-diff-delta ${phrase.className}`, text: change }),
      el('p', { text: `${body?.display?.from ?? ''} → ${body?.display?.to ?? ''}` }),
      el('p', { text: `Unchanged: ${body?.unchanged_count} (${body?.display?.unchangedTotal ?? ''})` }));
    const rows = diffRows(body);
    diffStatus.hidden = true;
    diffStatus.textContent = '';
    diffBody.replaceChildren(totals);
    if (rows.length === 0) diffBody.append(el('p', { text: NO_CHANGES }));
    else diffBody.append(diffTable(rows));
  }

  function paintDiffError(err) {
    diffBody.replaceChildren();
    const message = historyMessage(err);
    if (isQueryError(err)) {
      diffStatus.hidden = true;
      diffStatus.textContent = '';
      diffError.hidden = false;
      diffError.textContent = message;
      setDescribed([diffFrom, diffTo], 'history-diff-error', true);
      clearOwnedAlert('diff');
      return;
    }
    diffStatus.hidden = false;
    diffStatus.textContent = LOAD_FAILED;
    diffError.hidden = true;
    setDescribed([diffFrom, diffTo], 'history-diff-error', false);
    showAlert('diff', message, isNotReady(err));
  }

  function loadDiff(token) {
    if (diffSection.hidden) return undefined;
    diffAbort?.abort();
    diffAbort = new AbortController();
    clearOwnedAlert('diff');
    diffError.hidden = true;
    setDescribed([diffFrom, diffTo], 'history-diff-error', false);
    diffStatus.hidden = false;
    diffStatus.textContent = 'Loading cost history diff…';
    diffBody.replaceChildren();
    return api.get(`/api/history/diff${queryString({ from: diffFrom.value, to: diffTo.value })}`, { signal: diffAbort.signal })
      .then((body) => {
        if (token !== generation) return;
        paintDiff(body);
      })
      .catch((err) => {
        if (token !== generation || err?.name === 'AbortError') return;
        paintDiffError(err);
      });
  }

  function paintView(body, token) {
    alert.hidden = true;
    alert.textContent = '';
    alertOwner = '';
    queryError.hidden = true;
    setDescribed(rangeInputs, 'history-query-error', false);
    showWarning(body?.currency_warning);
    waitingForReady = false;
    if (body?.status === 'no_history') {
      status.hidden = false;
      status.textContent = NO_HISTORY;
      series.replaceChildren();
      diffSection.hidden = true;
      diffBody.replaceChildren();
      return;
    }
    diffSection.hidden = false;
    const displayed = body?.display?.snapshots;
    if (!Array.isArray(displayed) || displayed.length === 0) {
      status.hidden = false;
      status.textContent = NO_SNAPSHOTS;
      series.replaceChildren();
      diffBody.replaceChildren();
      diffStatus.textContent = '';
      return;
    }
    status.hidden = true;
    status.textContent = '';
    const chart = historyChart(body);
    const budget = body?.display?.budget
      ? el('p', { class: 'history-budget', text: `Budget line: ${body.display.budget}` })
      : null;
    series.replaceChildren(...[chart, seriesTable(displayed), budget, annotationList(body.annotations)].filter(Boolean));
    fillSelectors(displayed);
    loadDiff(token);
  }

  function paintViewError(err) {
    series.replaceChildren();
    diffBody.replaceChildren();
    diffSection.hidden = true;
    const message = historyMessage(err);
    if (isQueryError(err)) {
      status.hidden = true;
      status.textContent = '';
      queryError.hidden = false;
      queryError.textContent = message;
      setDescribed(rangeInputs, 'history-query-error', true);
      clearOwnedAlert('view');
      alert.hidden = true;
      alert.textContent = '';
      alertOwner = '';
      waitingForReady = false;
      return;
    }
    status.hidden = false;
    status.textContent = LOAD_FAILED;
    queryError.hidden = true;
    setDescribed(rangeInputs, 'history-query-error', false);
    showAlert('view', message, isNotReady(err));
  }

  function load() {
    const token = ++generation;
    viewAbort?.abort();
    diffAbort?.abort();
    viewAbort = new AbortController();
    waitingForReady = false;
    alert.hidden = true;
    alert.textContent = '';
    alertOwner = '';
    queryError.hidden = true;
    setDescribed(rangeInputs, 'history-query-error', false);
    status.hidden = false;
    status.textContent = 'Loading cost history…';
    series.replaceChildren();
    diffBody.replaceChildren();
    return api.get(`/api/history/view${queryString({ from: from.value, to: to.value, currency: currency.value })}`, { signal: viewAbort.signal })
      .then((body) => {
        if (token !== generation) return;
        paintView(body, token);
      })
      .catch((err) => {
        if (token !== generation || err?.name === 'AbortError') return;
        paintViewError(err);
      });
  }

  rangeForm.addEventListener('submit', (event) => {
    event.preventDefault();
    load();
  });
  diffForm.addEventListener('submit', (event) => {
    event.preventDefault();
    loadDiff(generation);
  });

  return {
    element: section,
    load,
    retry() {
      if (waitingForReady) load();
    },
    unmount() {
      generation += 1;
      viewAbort?.abort();
      diffAbort?.abort();
    },
  };
}
