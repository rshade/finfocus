import { el } from '../app.js';

// Every value, month label, money string, and the model name come from Go
// (the response's display block). This view collects the query, reads a chosen
// CSV file as text, and lays out the strings it gets back.
const UNALIGNED = new Set(['history', 'spend']);
const LABELS = { forecast: 'Forecast', lower: 'Lower', upper: 'Upper', baseline: 'Baseline' };

function list(text) {
  return text.split(',').map(item => item.trim()).filter(Boolean);
}

function numberOrError(input, label) {
  const raw = input.value.trim();
  if (raw === '' && !input.validity?.badInput) return undefined;
  const value = Number(raw);
  if (raw === '' || !Number.isFinite(value)) throw new Error(`${label} must be a number.`);
  return value;
}

function span(points) {
  const day = point => point.time.slice(0, 10);
  return `${day(points[0])} to ${day(points[points.length - 1])}`;
}

export function mount(container, { api }) {
  let alive = true, version = 0, queryAbort;
  const heading = el('h2', { tabindex: '-1', text: 'Forecast' });
  const intro = el('p', { class: 'forecast-intro', text: 'Monthly cost projected from the current month, the same as finfocus cost forecast. The statistical model fits a spend CSV you choose; the file is read in this browser and sent as text.' });
  const error = el('p', { id: 'forecast-error', class: 'error-banner', role: 'alert' }); error.hidden = true;
  const status = el('p', { role: 'status', text: 'Choose the settings and run the forecast.' });

  const field = attrs => el('input', { class: 'filter-input', autocomplete: 'off', ...attrs });
  const months = field({ id: 'forecast-months', type: 'number', min: '1', max: '36', step: '1', value: '12', required: '' });
  const model = el('select', { id: 'forecast-model' });
  for (const [value, text] of [['growth', 'Plugin growth'], ['arima', 'Statistical (ARIMA)']]) model.append(el('option', { value, text }));
  const growthType = el('select', { id: 'forecast-growth-type' });
  for (const [value, text] of [['', 'As each plugin reports'], ['none', 'None'], ['linear', 'Linear'], ['exponential', 'Exponential']]) growthType.append(el('option', { value, text }));
  const growthRate = field({ id: 'forecast-growth-rate', type: 'text', placeholder: '0.10 is 10% a month' });
  const cap = field({ id: 'forecast-cap', type: 'text', inputmode: 'decimal', placeholder: '1.25 is +25%, optional' });
  const steps = field({ id: 'forecast-steps', type: 'text', placeholder: 'DATE=AMOUNT, comma-separated' });
  const history = el('input', { id: 'forecast-history', type: 'checkbox', checked: '' });
  const spend = el('input', { id: 'forecast-spend', type: 'file', accept: '.csv,text/csv' });
  const confidence = field({ id: 'forecast-confidence', type: 'text', inputmode: 'decimal', placeholder: '0.95' });
  const exclude = field({ id: 'forecast-exclude', type: 'text', placeholder: 'FROM:TO, comma-separated' });
  const arima = el('div', { class: 'forecast-arima' },
    el('label', { class: 'filter-label', for: 'forecast-spend' }, 'Spend CSV (date,value)', spend),
    el('label', { class: 'filter-label', for: 'forecast-confidence' }, 'Confidence', confidence),
    el('label', { class: 'filter-label', for: 'forecast-exclude' }, 'Exclude ranges', exclude));
  arima.hidden = true;
  const submit = el('button', { type: 'submit', text: 'Run forecast' });
  const form = el('form', { class: 'toolbar forecast-toolbar' },
    el('label', { class: 'filter-label', for: 'forecast-months' }, 'Months ahead (1-36)', months),
    el('label', { class: 'filter-label', for: 'forecast-model' }, 'Model', model),
    el('label', { class: 'filter-label', for: 'forecast-growth-type' }, 'Growth type', growthType),
    el('label', { class: 'filter-label', for: 'forecast-growth-rate' }, 'Growth rate', growthRate),
    el('label', { class: 'filter-label', for: 'forecast-cap' }, 'Growth cap', cap),
    el('label', { class: 'filter-label', for: 'forecast-steps' }, 'Steps', steps),
    arima,
    el('label', { class: 'filter-label forecast-check', for: 'forecast-history' }, history, 'Include cost history'),
    submit);

  const totals = el('div', { class: 'totals-bar', 'aria-label': 'Forecast summary' }); totals.hidden = true;
  const head = el('tr');
  const body = el('tbody');
  const table = el('table', { class: 'overview-table forecast-table' },
    el('caption', { text: 'Projected monthly cost' }), el('thead', {}, head), body);
  const wrap = el('div', { class: 'table-wrap', tabindex: '0', 'aria-label': 'Scrollable forecast months' }, table); wrap.hidden = true;
  const inputs = el('ul', { class: 'forecast-inputs', 'aria-label': 'Forecast inputs' }); inputs.hidden = true;
  const warnings = el('ul', { class: 'forecast-warnings' });
  const warningsPanel = el('section', { class: 'forecast-warnings-panel', 'aria-label': 'Warnings' }, el('h3', { text: 'Warnings' }), warnings);
  warningsPanel.hidden = true;
  container.append(heading, intro, form, error, status, totals, wrap, inputs, warningsPanel);

  model.addEventListener('change', () => { arima.hidden = model.value !== 'arima'; });

  function clearResults() {
    totals.hidden = true; wrap.hidden = true; inputs.hidden = true; warningsPanel.hidden = true;
    totals.replaceChildren(); head.replaceChildren(); body.replaceChildren(); inputs.replaceChildren(); warnings.replaceChildren();
  }

  async function buildQuery() {
    const query = {};
    const horizon = numberOrError(months, 'Months ahead');
    if (horizon !== undefined) query.months = horizon;
    if (model.value !== 'growth') query.model = model.value;
    if (growthType.value) query.growthType = growthType.value;
    if (growthRate.value.trim()) query.growthRate = growthRate.value.trim();
    const growthCap = numberOrError(cap, 'Growth cap');
    if (growthCap !== undefined) query.growthCap = growthCap;
    const stepList = list(steps.value);
    if (stepList.length) query.steps = stepList;
    if (!history.checked) query.noHistory = true;
    if (model.value === 'arima') {
      const file = spend.files?.[0];
      if (file) query.spendHistory = await file.text();
      const level = numberOrError(confidence, 'Confidence');
      if (level !== undefined) query.confidence = level;
      const ranges = list(exclude.value);
      if (ranges.length) query.exclude = ranges;
    }
    return query;
  }

  function summary(label, value) {
    return el('span', { class: 'forecast-total' }, el('span', { class: 'totals-label', text: `${label} ` }), el('span', { class: 'totals-value', text: value }));
  }

  function render(doc) {
    const shown = doc.display.series.map((display, i) => ({ display, raw: doc.series[i] }));
    const aligned = shown.filter(s => !UNALIGNED.has(s.display.name));
    const forecast = aligned.find(s => s.display.name === 'forecast').display.points;
    totals.replaceChildren(summary('Model:', doc.display.model));
    if (forecast.length) {
      totals.append(summary(`${forecast[0].month}:`, forecast[0].value));
      if (forecast.length > 1) totals.append(summary(`${forecast[forecast.length - 1].month}:`, forecast[forecast.length - 1].value));
    }
    if (doc.display.budget) totals.append(summary('Budget:', doc.display.budget));
    totals.hidden = false;

    head.replaceChildren(el('th', { scope: 'col', text: 'Month' }),
      ...aligned.map(s => el('th', { scope: 'col', class: 'col-num', text: LABELS[s.display.name] ?? s.display.name })));
    body.replaceChildren(...forecast.map((point, row) => el('tr', {},
      el('th', { scope: 'row', text: point.month }),
      ...aligned.map(s => el('td', { class: 'col-num', text: s.display.points[row]?.value ?? '' })))));
    wrap.hidden = forecast.length === 0;

    inputs.replaceChildren();
    for (const s of shown.filter(item => UNALIGNED.has(item.display.name) && item.raw.points.length)) {
      const what = s.display.name === 'history' ? `History: ${s.raw.points.length} snapshots` : `Spend history: ${s.raw.points.length} points`;
      inputs.append(el('li', { text: `${what}, ${span(s.raw.points)}` }));
    }
    inputs.hidden = inputs.children.length === 0;

    warnings.replaceChildren(...(doc.warnings ?? []).map(text => el('li', { text })));
    warningsPanel.hidden = !doc.warnings?.length;
    status.textContent = `${forecast.length} months projected.`;
  }

  async function run() {
    queryAbort?.abort(); queryAbort = new AbortController();
    const current = ++version;
    const signal = queryAbort.signal;
    error.hidden = true; clearResults();
    let query;
    try {
      query = await buildQuery();
    } catch (err) {
      if (!alive || current !== version) return;
      error.textContent = err.message; error.hidden = false;
      status.textContent = 'Fix the highlighted setting and run again.';
      return;
    }
    if (!alive || current !== version) return;
    status.textContent = 'Running forecast…'; table.setAttribute('aria-busy', 'true');
    try {
      const doc = await api.post('/api/forecast/query', query, { signal });
      if (alive && current === version) render(doc);
    } catch (err) {
      if (!alive || current !== version || err.name === 'AbortError') return;
      error.textContent = err.message; error.hidden = false;
      status.textContent = 'Unable to run the forecast.';
    } finally {
      if (alive && current === version) table.setAttribute('aria-busy', 'false');
    }
  }

  form.addEventListener('submit', event => { event.preventDefault(); return run(); });
  return { unmount() { alive = false; version += 1; queryAbort?.abort(); } };
}
