import { el } from '../app.js';

// Cost values, deltas and pricing labels are computed and formatted by Go.
export function mount(container, { api, stream }) {
  let alive = true, generation = 0, requestAbort;
  const heading = el('h2', { tabindex: '-1', text: 'What-If Cost Analysis' });
  const resource = el('select', { id: 'estimate-resource' });
  const mode = el('select', { id: 'estimate-mode' });
  const error = el('p', { class: 'error-banner', role: 'alert' }); error.hidden = true;
  const status = el('p', { role: 'status', text: 'Loading estimate resources…' });
  const comparison = el('div', { class: 'totals-bar', 'aria-label': 'Estimate comparison' });
  const pricing = el('div', { 'aria-label': 'Pricing details' });
  const body = el('tbody');
  const table = el('table', { class: 'overview-table estimate-table', role: 'table' },
    el('caption', { text: 'Edit a current property value and press Enter to recalculate.' }),
    el('thead', {}, el('tr', {}, ...['Property', 'Original', 'Current', 'Cost delta'].map(text => el('th', { scope: 'col', text })))), body);
  container.append(heading, el('div', { class: 'toolbar estimate-toolbar' },
    el('label', { class: 'filter-label', for: 'estimate-resource' }, 'Resource', resource),
    el('label', { class: 'filter-label', for: 'estimate-mode' }, 'Pricing mode', mode)), error, status, comparison, pricing,
    el('div', { class: 'table-wrap' }, table));

  function begin(message) {
    requestAbort?.abort(); requestAbort = new AbortController();
    generation += 1; error.hidden = true; status.textContent = message;
    table.setAttribute('aria-busy', 'true');
    return { current: generation, signal: requestAbort.signal };
  }
  function failed(err, current) {
    if (!alive || current !== generation || err.name === 'AbortError') return;
    if (err.status === 503) status.textContent = 'Waiting for stack data…';
    else { error.textContent = err.message; error.hidden = false; status.textContent = 'Unable to calculate estimate.'; }
  }
  function finish(current) { if (alive && current === generation) table.setAttribute('aria-busy', 'false'); }
  function overrides() {
    const values = Object.create(null);
    for (const input of body.querySelectorAll('.estimate-input')) {
      if (input.value !== input.dataset.original) values[input.dataset.key] = input.value;
    }
    return values;
  }
  function render(result, focusKey) {
    const display = result.display;
    comparison.replaceChildren(el('p', { text: `Baseline: ${display.baseline}` }),
      el('p', { text: `Modified: ${display.modified}` }), el('p', { text: `Change: ${display.change}` }));
    if (result.pricingModes) {
      mode.replaceChildren(el('option', { value: '', text: 'Automatic provider' }));
      for (const item of result.pricingModes) mode.append(el('option', { value: item.id, text: `${item.label} (${item.rate})` }));
      mode.value = result.pricingMode ?? '';
      pricing.replaceChildren();
      for (const item of result.pricingModes) {
        pricing.append(el('p', { text: `${item.label}: ${item.rate}` }));
        if (item.detailsDisplay) pricing.append(el('pre', { text: item.detailsDisplay }));
      }
    }
    body.replaceChildren();
    for (const row of display.properties ?? []) {
      const input = el('input', { class: 'estimate-input filter-input', type: 'text', 'aria-label': `Current ${row.key}`,
        dataset: { key: row.key, original: row.originalValue } });
      input.value = row.currentValue;
      input.addEventListener('keydown', event => { if (event.key === 'Enter') { event.preventDefault(); recalculate(row.key); } });
      body.append(el('tr', { role: 'row' }, el('th', { scope: 'row', role: 'rowheader', text: row.key }),
        el('td', { role: 'cell', 'data-label': 'Original', text: row.originalValue }),
        el('td', { role: 'cell', 'data-label': 'Current' }, input),
        el('td', { role: 'cell', 'data-label': 'Cost delta', text: row.delta })));
      if (focusKey === row.key) input.focus();
    }
    if (!display.properties?.length) body.append(el('tr', {}, el('td', { colspan: '4', text: 'No editable properties.' })));
    status.textContent = 'Estimate calculated.';
  }
  async function recalculate(focusKey) {
    if (!resource.value) return;
    const changes = overrides();
    const { current, signal } = begin('Calculating estimate…');
    try {
      const result = Object.keys(changes).length
        ? await api.post('/api/estimate/recalculate', { urn: resource.value, overrides: changes, pricingMode: mode.value }, { signal })
        : await api.get(`/api/estimate/baseline?urn=${encodeURIComponent(resource.value)}&pricingMode=${encodeURIComponent(mode.value)}`, { signal });
      if (alive && current === generation) render(result, focusKey);
    } catch (err) { failed(err, current); }
    finally { finish(current); }
  }
  function selectResource() {
    mode.replaceChildren(el('option', { value: '', text: 'Automatic provider' })); mode.value = '';
    body.replaceChildren(); comparison.replaceChildren(); pricing.replaceChildren();
    recalculate();
  }
  async function loadResources() {
    const { current, signal } = begin('Loading estimate resources…');
    try {
      const result = await api.get('/api/estimate/resources', { signal });
      if (!alive || current !== generation) return;
      const selected = resource.value;
      resource.replaceChildren();
      for (const item of result.resources ?? []) {
        const name = item.id.split('::').at(-1);
        resource.append(el('option', { value: item.id, title: item.id, text: `${name} (${item.type})` }));
      }
      resource.value = result.resources?.some(item => item.id === selected) ? selected : result.resources?.[0]?.id ?? '';
      if (resource.value) selectResource();
      else { body.replaceChildren(); comparison.replaceChildren(); status.textContent = 'No resources available for estimation.'; }
    } catch (err) { failed(err, current); }
    finally { finish(current); }
  }
  resource.addEventListener('change', selectResource);
  mode.addEventListener('change', () => recalculate());
  const unsubs = [stream.on('ready', loadResources), stream.on('snapshot', snapshot => { if (snapshot.ready) loadResources(); })];
  loadResources();
  return { unmount() { alive = false; generation += 1; requestAbort?.abort(); for (const off of unsubs) off(); } };
}
