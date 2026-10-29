import { el } from '../app.js';

// The totals, enum values, and timestamps in the report come from Go. This view only collects the query and renders the returned records.
const METHODS = { CARBON_ACCOUNTING_METHOD_MARKET_BASED: 'Market-based', CARBON_ACCOUNTING_METHOD_LOCATION_BASED: 'Location-based' };
const SCOPES = {
  CARBON_EMISSIONS_SCOPE_TOTAL: 'Total', CARBON_EMISSIONS_SCOPE_SCOPE_1: 'Scope 1',
  CARBON_EMISSIONS_SCOPE_SCOPE_2: 'Scope 2', CARBON_EMISSIONS_SCOPE_SCOPE_3: 'Scope 3',
};
const COLUMNS = ['ID', 'Provider', 'Source', 'Account', 'Region', 'Service', 'Period (end exclusive)', 'Method', 'Scope', 'Value', 'Model', 'Retrieved at'];

function list(text) {
  return text.split(',').map(item => item.trim()).filter(Boolean);
}

function dimension(value) {
  return value ? el('td', { text: value }) : el('td', { class: 'carbon-absent', title: 'Not reported', text: '—' });
}

export function mount(container, { api }) {
  let alive = true, version = 0, queryAbort;
  const heading = el('h2', { tabindex: '-1', text: 'Historical Carbon' });
  const intro = el('p', { class: 'carbon-intro', text: 'Provider-published monthly emissions for complete UTC months. The end date is exclusive.' });
  const error = el('p', { id: 'carbon-error', class: 'error-banner', role: 'alert' }); error.hidden = true;
  const status = el('p', { role: 'status', text: 'Choose a period and run the report.' });
  const dateAttrs = { type: 'text', class: 'filter-input', required: '', pattern: '\\d{4}-\\d{2}-01', placeholder: 'YYYY-MM-01', autocomplete: 'off' };
  const start = el('input', { ...dateAttrs, id: 'carbon-start' });
  const end = el('input', { ...dateAttrs, id: 'carbon-end' });
  const method = el('select', { id: 'carbon-method' });
  for (const [value, text] of [['market-based', 'Market-based'], ['location-based', 'Location-based']]) method.append(el('option', { value, text }));
  const scope = el('select', { id: 'carbon-scope' });
  for (const [value, text] of [['total', 'Total'], ['1', 'Scope 1'], ['2', 'Scope 2'], ['3', 'Scope 3']]) scope.append(el('option', { value, text }));
  const accounts = el('input', { id: 'carbon-accounts', class: 'filter-input', type: 'text', placeholder: 'Comma-separated, optional', autocomplete: 'off' });
  const regions = el('input', { id: 'carbon-regions', class: 'filter-input', type: 'text', placeholder: 'Comma-separated, optional', autocomplete: 'off' });
  const services = el('input', { id: 'carbon-services', class: 'filter-input', type: 'text', placeholder: 'Comma-separated, optional', autocomplete: 'off' });
  const submit = el('button', { type: 'submit', text: 'Run report' });
  const form = el('form', { class: 'toolbar carbon-toolbar' },
    el('label', { class: 'filter-label', for: 'carbon-start' }, 'Start (first of month)', start),
    el('label', { class: 'filter-label', for: 'carbon-end' }, 'End (first of month, exclusive)', end),
    el('label', { class: 'filter-label', for: 'carbon-method' }, 'Accounting method', method),
    el('label', { class: 'filter-label', for: 'carbon-scope' }, 'Scope', scope),
    el('label', { class: 'filter-label', for: 'carbon-accounts' }, 'Accounts', accounts),
    el('label', { class: 'filter-label', for: 'carbon-regions' }, 'Regions', regions),
    el('label', { class: 'filter-label', for: 'carbon-services' }, 'Services', services),
    submit);
  const context = el('dl', { class: 'detail-list carbon-context', 'aria-label': 'Query context' }); context.hidden = true;
  const total = el('div', { class: 'totals-bar', 'aria-label': 'Carbon total' }); total.hidden = true;
  const body = el('tbody');
  const table = el('table', { class: 'overview-table carbon-table' },
    el('caption', { text: 'Emissions records' }),
    el('thead', {}, el('tr', {}, ...COLUMNS.map(text => el('th', { scope: 'col', text })))), body);
  const wrap = el('div', { class: 'table-wrap', tabindex: '0', 'aria-label': 'Scrollable emissions records' }, table); wrap.hidden = true;
  container.append(heading, intro, form, error, status, context, total, wrap);

  function clearResults() {
    context.hidden = true; total.hidden = true; wrap.hidden = true;
    context.replaceChildren(); total.replaceChildren(); body.replaceChildren();
  }

  function renderContext(request) {
    const rows = [
      ['Period', `${request.start} to ${request.end} (exclusive)`],
      ['Method', METHODS[request.method] ?? request.method],
      ['Scope', SCOPES[request.scope] ?? request.scope],
      ['Accounts', request.accountIds?.join(', ') || 'All'],
      ['Regions', request.regions?.join(', ') || 'All'],
      ['Services', request.services?.join(', ') || 'All'],
    ];
    context.replaceChildren();
    for (const [name, value] of rows) context.append(el('dt', { text: name }), el('dd', { text: value }));
    context.hidden = false;
  }

  function renderValue(record) {
    // The report omits value for missing data, so an explicit zero stays a number.
    if (record.value === undefined || record.value === null) {
      return el('td', { class: 'carbon-missing' }, el('em', { text: 'Missing' }));
    }
    return el('td', { class: 'col-num', text: `${record.value} ${record.unit ?? ''}`.trim() });
  }

  function render(report) {
    renderContext(report.request);
    const records = report.records ?? [];
    total.replaceChildren();
    if (report.totalGrams === null || report.totalGrams === undefined) {
      total.append(el('p', { text: 'No available emissions data for this query. The total is unavailable, not zero.' }));
    } else {
      total.append(el('span', { class: 'totals-label', text: 'Sum of returned available records:' }), el('span', { class: 'totals-value', text: `${report.totalGrams} gCO2e` }));
    }
    total.hidden = false;
    body.replaceChildren();
    for (const record of records) {
      body.append(el('tr', {},
        el('td', { text: record.id }), el('td', { text: record.provider }), el('td', { text: record.source }),
        dimension(record.accountId), dimension(record.region), dimension(record.service),
        el('td', { text: `${record.start} to ${record.end}` }),
        el('td', { text: METHODS[record.method] ?? record.method }), el('td', { text: SCOPES[record.scope] ?? record.scope }),
        renderValue(record), dimension(record.modelVersion), el('td', { text: record.retrievedAt })));
    }
    wrap.hidden = records.length === 0;
    const missing = records.filter(record => record.value === undefined || record.value === null).length;
    status.textContent = records.length
      ? `${records.length} ${records.length === 1 ? 'record' : 'records'} returned, ${missing} with missing values.`
      : 'No emissions records match this query.';
  }

  async function run() {
    queryAbort?.abort(); queryAbort = new AbortController();
    const current = ++version;
    error.hidden = true; clearResults();
    status.textContent = 'Loading carbon report…'; table.setAttribute('aria-busy', 'true');
    const query = { start: start.value.trim(), end: end.value.trim(), method: method.value, scope: scope.value };
    for (const [key, input] of [['accounts', accounts], ['regions', regions], ['services', services]]) {
      const values = list(input.value);
      if (values.length) query[key] = values;
    }
    try {
      const report = await api.post('/api/carbon/actual/query', query, { signal: queryAbort.signal });
      if (alive && current === version) render(report);
    } catch (err) {
      if (!alive || current !== version || err.name === 'AbortError') return;
      error.textContent = err.message; error.hidden = false;
      status.textContent = 'Unable to load the carbon report.';
    } finally {
      if (alive && current === version) table.setAttribute('aria-busy', 'false');
    }
  }

  form.addEventListener('submit', event => { event.preventDefault(); run(); });
  return { unmount() { alive = false; version += 1; queryAbort?.abort(); } };
}
