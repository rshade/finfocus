import { el } from '../app.js';

// All totals, grouping, filtering, sorting, and sparkline coordinates come from Go.
export function mount(container, { api, stream }) {
  let alive = true, version = 0, detailVersion = 0, page = 1, totalPages = 1;
  let queryAbort, detailAbort, launcher, launcherID, launcherType;
  const heading = el('h2', { tabindex: '-1', text: 'Actual Costs' });
  const error = el('p', { class: 'error-banner', role: 'alert' }); error.hidden = true;
  const status = el('p', { role: 'status', text: 'Loading actual costs…' });
  const summary = el('div', { class: 'totals-bar', 'aria-label': 'Actual cost summary' });
  const filter = el('input', { id: 'cost-filter', class: 'filter-input', type: 'search', placeholder: 'Resource or type' });
  const group = el('select', { id: 'cost-group' });
  for (const [value, text] of [['', 'None'], ['resource', 'Resource'], ['type', 'Type'], ['provider', 'Provider'], ['daily', 'Daily'], ['monthly', 'Monthly']]) group.append(el('option', { value, text }));
  const tag = el('input', { id: 'cost-tag', class: 'filter-input', type: 'text', placeholder: 'key=value' });
  const sort = el('select', { id: 'cost-sort' });
  for (const [value, text] of [['cost', 'Cost'], ['name', 'Name'], ['type', 'Type'], ['delta', 'Delta']]) sort.append(el('option', { value, text }));
  sort.value = 'cost';
  const form = el('form', { class: 'toolbar' },
    el('label', { class: 'filter-label', for: 'cost-filter' }, 'Filter', filter),
    el('label', { class: 'filter-label', for: 'cost-group' }, 'Group by', group),
    el('label', { class: 'filter-label', for: 'cost-tag' }, 'Tag filter', tag),
    el('label', { class: 'filter-label', for: 'cost-sort' }, 'Sort by', sort), el('button', { type: 'submit', text: 'Apply' }));
  const body = el('tbody');
  const table = el('table', { class: 'overview-table' }, el('caption', { text: 'Actual cost results' }),
    el('thead', {}, el('tr', {}, ...['Resource / Period', 'Type / Providers', 'Actual Cost', 'Trend'].map(text => el('th', { scope: 'col', text })))), body);
  const previous = el('button', { type: 'button', text: 'Previous page' });
  const next = el('button', { type: 'button', text: 'Next page' });
  const pageLabel = el('span', { role: 'status' });
  const pagination = el('nav', { class: 'pager', 'aria-label': 'Actual cost pages' }, previous, pageLabel, next);
  const detailContent = el('div');
  const close = el('button', { type: 'button', text: 'Close details' });
  const dialog = el('dialog', { class: 'detail-panel passphrase-dialog', 'aria-labelledby': 'cost-detail-title' },
    el('h3', { id: 'cost-detail-title', text: 'Actual cost details' }), close, detailContent);
  container.append(heading, form, error, status, summary, el('div', { class: 'table-wrap' }, table), pagination, dialog);

  function closeDetails() {
    detailVersion += 1; detailAbort?.abort();
    if (launcher?.isConnected) launcher.focus();
    else {
      const replacement = [...body.querySelectorAll('.detail-button')].find(button => button.dataset.id === launcherID && button.dataset.type === launcherType);
      (replacement || filter).focus();
    }
  }
  dialog.addEventListener('close', closeDetails);
  close.addEventListener('click', () => dialog.close());

  async function showDetail(row, button, context) {
    launcher = button; launcherID = row.id; launcherType = row.type; detailAbort?.abort(); detailAbort = new AbortController();
    const current = ++detailVersion;
    detailContent.replaceChildren(el('p', { role: 'status', text: 'Loading details…' })); dialog.showModal();
    const path = `/api/cost/actual/resource?id=${encodeURIComponent(row.id)}&type=${encodeURIComponent(row.type)}&groupBy=${encodeURIComponent(context.groupBy)}&tag=${encodeURIComponent(context.tag)}`;
    try {
      const detail = await api.get(path, { signal: detailAbort.signal });
      if (!alive || current !== detailVersion || !dialog.open) return;
      detailContent.replaceChildren(el('p', { text: detail.result?.resourceId }));
      const fields = el('dl');
      for (const [name, value] of [['Type', detail.result?.resourceType], ['Provider', detail.provider], ['Total Cost', detail.costDisplay], ['Period', detail.periodDisplay], ['Monthly Cost', detail.monthlyDisplay], ['Hourly Cost', detail.hourlyDisplay], ['Delta', detail.deltaDisplay]]) {
        if (value) fields.append(el('dt', { text: `${name}: ` }), el('dd', { text: value }));
      }
      detailContent.append(fields);
      for (const [title, items, key] of [['Breakdown', detail.breakdown, 'costDisplay'], ['Sustainability', detail.sustainabilityDisplay, 'value']]) {
        if (!items?.length) continue;
        const values = el('dl');
        for (const item of items) values.append(el('dt', { text: item.name }), el('dd', { text: item[key] }));
        detailContent.append(el('h4', { text: title }), values);
      }
      if (detail.recommendations?.length) detailContent.append(el('h4', { text: 'Linked recommendations' }));
      for (const linked of detail.recommendations ?? []) {
        detailContent.append(el('p', { text: `[${linked.recommendation.type}] ${linked.recommendation.description}${linked.savingsDisplay ? ` (${linked.savingsDisplay}/mo savings)` : ''}` }));
        for (const reason of linked.recommendation.reasoning ?? []) detailContent.append(el('p', { text: reason }));
      }
      if (detail.notesDisplay) detailContent.append(el('h4', { text: 'Notes' }), el('p', { text: detail.notesDisplay }));
    } catch (err) {
      if (alive && current === detailVersion && err.name !== 'AbortError') detailContent.replaceChildren(el('p', { role: 'alert', text: err.message }));
    }
  }

  function render(result, context) {
    page = result.page; totalPages = result.totalPages;
    summary.replaceChildren(el('p', { text: `Total actual cost: ${result.summaryDisplay?.totalDisplay ?? result.totalDisplay}` }));
    const display = result.summaryDisplay;
    if (display) {
      summary.append(el('p', { text: `Resources: ${display.resourceCount}` }), el('p', { text: `Recommendations: ${display.recommendationCount}` }));
      for (const provider of display.providers ?? []) summary.append(el('p', { text: `${provider.name}: ${provider.costDisplay} (${provider.shareDisplay})` }));
      if (display.carbonEquivalency) summary.append(el('p', { text: display.carbonEquivalency }));
    }
    if (result.carbonDisplay) summary.append(el('p', { text: `Carbon: ${result.carbonDisplay}` }));
    body.replaceChildren();
    if (result.errors?.length) { error.textContent = 'Some plugin data is unavailable. Available results are shown.'; error.hidden = false; }
    for (const row of result.rows ?? []) {
      let resource = el('span', { text: row.id });
      if (row.canDetail) {
        resource = el('button', { type: 'button', class: 'detail-button', dataset: { id: row.id, type: row.type }, text: row.id });
        resource.addEventListener('click', () => showDetail(row, resource, context));
      }
      const trend = el('td');
      if (row.trendPoints) {
        const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
        svg.setAttribute('viewBox', '0 0 72 28'); svg.setAttribute('width', '84'); svg.setAttribute('height', '28');
        svg.setAttribute('role', 'img'); svg.setAttribute('aria-label', `Cost trend: ${row.trend}`);
        const line = document.createElementNS('http://www.w3.org/2000/svg', 'polyline');
        line.setAttribute('points', row.trendPoints); line.setAttribute('fill', 'none'); line.setAttribute('stroke', 'currentColor'); line.setAttribute('stroke-width', '2'); svg.append(line); trend.append(svg);
      } else trend.append(el('span', { text: '—' }));
      body.append(el('tr', {}, el('td', {}, resource), el('td', { text: row.providersDisplay || row.type }), el('td', { text: row.costDisplay }), trend));
    }
    if (!result.rows?.length) body.append(el('tr', {}, el('td', { colspan: '4', text: 'No actual costs match these filters.' })));
    previous.disabled = page <= 1; next.disabled = page >= totalPages;
    pageLabel.textContent = `Page ${page} of ${totalPages}`;
    status.textContent = 'Actual costs loaded.';
  }

  function updateSortControls() {
    const timeGrouped = group.value === 'daily' || group.value === 'monthly';
    for (const option of sort.querySelectorAll('option')) {
      option.disabled = timeGrouped && (option.value === 'type' || option.value === 'delta');
      if (option.value === 'name') option.textContent = timeGrouped ? 'Period' : 'Name';
    }
    if (timeGrouped && (sort.value === 'type' || sort.value === 'delta')) sort.value = 'cost';
  }

  async function query() {
    updateSortControls();
    const context = Object.freeze({ filter: filter.value, sort: sort.value, groupBy: group.value, tag: tag.value, page });
    queryAbort?.abort(); queryAbort = new AbortController(); const current = ++version;
    error.hidden = true; status.textContent = 'Loading actual costs…'; table.setAttribute('aria-busy', 'true');
    try {
      const result = await api.post('/api/cost/actual/query', context, { signal: queryAbort.signal });
      if (!alive || current !== version) return;
      render(result, context);
    } catch (err) {
      if (!alive || current !== version || err.name === 'AbortError') return;
      if (err.status === 503) status.textContent = 'Waiting for stack data…';
      else { error.textContent = err.message; error.hidden = false; status.textContent = 'Unable to load actual costs.'; }
    } finally { if (alive && current === version) table.setAttribute('aria-busy', 'false'); }
  }
  function reset() { page = 1; if (dialog.open) dialog.close(); query(); }
  form.addEventListener('submit', event => { event.preventDefault(); reset(); });
  for (const input of [group, sort, tag, filter]) input.addEventListener('change', reset);
  previous.addEventListener('click', () => { page -= 1; query(); }); next.addEventListener('click', () => { page += 1; query(); });
  const unsubs = [stream.on('ready', query), stream.on('snapshot', snapshot => { if (snapshot.ready) query(); })];
  query();
  return { unmount() { alive = false; version += 1; detailVersion += 1; queryAbort?.abort(); detailAbort?.abort(); for (const off of unsubs) off(); if (dialog.open) dialog.close(); } };
}
