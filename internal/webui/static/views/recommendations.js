import { el } from '../app.js';

// This read-only view renders server summaries and complete scorer signals.
export function mount(container, { api, stream }) {
  let alive = true, version = 0, detailVersion = 0, page = 1, totalPages = 1;
  let queryAbort, detailAbort, launcher, launcherID, launcherType;
  const heading = el('h2', { tabindex: '-1', text: 'Recommendations' });
  const error = el('p', { class: 'error-banner', role: 'alert' }); error.hidden = true;
  const status = el('p', { role: 'status', text: 'Loading recommendations…' });
  const summary = el('div', { class: 'totals-bar', 'aria-label': 'Recommendation summary' });
  const filter = el('input', { id: 'recommendations-filter', class: 'filter-input', type: 'search', placeholder: 'Resource, action, or description' });
  const sort = el('select', { id: 'recommendations-sort' });
  for (const [value, text] of [['savings', 'Savings'], ['resource', 'Resource'], ['action', 'Action']]) sort.append(el('option', { value, text }));
  sort.value = 'savings';
  const dismissed = el('input', { id: 'recommendations-dismissed', type: 'checkbox' });
  const form = el('form', { class: 'toolbar' },
    el('label', { class: 'filter-label', for: 'recommendations-filter' }, 'Filter', filter),
    el('label', { class: 'filter-label', for: 'recommendations-sort' }, 'Sort by', sort),
    el('label', { class: 'filter-label', for: 'recommendations-dismissed' }, dismissed, 'Include dismissed'),
    el('button', { type: 'submit', text: 'Apply' }));
  const body = el('tbody');
  const table = el('table', { class: 'overview-table recommendations-table' }, el('caption', { text: 'Cost optimization recommendations' }),
    el('thead', {}, el('tr', {}, ...['Resource', 'Action', 'Description', 'Savings', 'Status'].map(text => el('th', { scope: 'col', text })))), body);
  const previous = el('button', { type: 'button', text: 'Previous page' });
  const next = el('button', { type: 'button', text: 'Next page' });
  const pageLabel = el('span', { role: 'status' });
  const pagination = el('nav', { class: 'pager', 'aria-label': 'Recommendation pages' }, previous, pageLabel, next);
  const detailContent = el('div');
  const close = el('button', { type: 'button', text: 'Close details' });
  const dialog = el('dialog', { class: 'detail-panel passphrase-dialog', 'aria-labelledby': 'recommendation-detail-title' },
    el('h3', { id: 'recommendation-detail-title', text: 'Recommendation details' }), close, detailContent);
  container.append(heading, form, error, status, summary, el('div', { class: 'table-wrap', tabindex: '0', 'aria-label': 'Scrollable recommendations' }, table), pagination, dialog);

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
  async function showDetail(item, button, key) {
    launcher = button; launcherID = key || item.id || item.resourceId; launcherType = item.type; detailAbort?.abort(); detailAbort = new AbortController(); const current = ++detailVersion;
    detailContent.replaceChildren(el('p', { role: 'status', text: 'Loading details…' })); dialog.showModal();
    try {
      const detail = await api.get(`/api/recommendations/item?id=${encodeURIComponent(item.id || item.resourceId)}&key=${encodeURIComponent(key || '')}`, { signal: detailAbort.signal });
      if (!alive || current !== detailVersion || !dialog.open) return;
      detailContent.replaceChildren(el('p', { text: detail.item.description }), el('p', { text: `Action: ${detail.actionDisplay}` }), el('p', { text: `Potential savings: ${detail.savingsDisplay}` }));
      if (detail.scoreDisplay?.length) {
        const signals = el('dl');
        for (const field of detail.scoreDisplay) signals.append(el('dt', { text: field.name }), el('dd', { text: field.value }));
        detailContent.append(el('h4', { text: 'Scorer signals' }), signals);
      }
      if (detail.item.reasoning?.length) detailContent.append(el('h4', { text: 'Reasoning' }));
      for (const reason of detail.item.reasoning ?? []) detailContent.append(el('p', { text: reason }));
      const metadata = el('dl');
      for (const [name, value] of [['Resource', detail.item.resourceId], ['ID', detail.item.id], ['Source', detail.item.source], ['Category', detail.item.category], ['Status', detail.item.status]]) {
        if (value) metadata.append(el('dt', { text: name }), el('dd', { text: value }));
      }
      detailContent.append(el('h4', { text: 'Resource and recommendation details' }), metadata);
    } catch (err) {
      if (alive && current === detailVersion && err.name !== 'AbortError') detailContent.replaceChildren(el('p', { role: 'alert', text: err.message }));
    }
  }
  function render(result) {
    page = result.page; totalPages = result.totalPages;
    summary.replaceChildren(el('p', { text: `Recommendations: ${result.summary.total_count}` }), el('p', { text: `Potential savings: ${result.savingsDisplay}` }));
    const actions = el('ul');
    for (const action of result.actions ?? []) actions.append(el('li', { text: `${action.actionDisplay}: ${action.count} (${action.savingsDisplay})` }));
    summary.append(actions); body.replaceChildren();
    if (result.errors?.length) { error.textContent = 'Some plugin data is unavailable. Available results are shown.'; error.hidden = false; }
    for (const [index, item] of (result.items ?? []).entries()) {
      const resource = el('button', { type: 'button', class: 'detail-button', dataset: { id: result.itemKeys?.[index] || item.id || item.resourceId, type: item.type }, text: item.resourceId || item.id });
      resource.addEventListener('click', () => showDetail(item, resource, result.itemKeys?.[index]));
      body.append(el('tr', {}, el('td', {}, resource), el('td', { text: result.itemActions?.[index] }), el('td', { text: item.description }), el('td', { text: result.itemSavings[index] }), el('td', { text: item.status || 'Active' })));
    }
    if (!result.items?.length) body.append(el('tr', {}, el('td', { colspan: '5', text: 'No recommendations match these filters.' })));
    previous.disabled = page <= 1; next.disabled = page >= totalPages; pageLabel.textContent = `Page ${page} of ${totalPages}`; status.textContent = 'Recommendations loaded.';
  }
  async function query() {
    queryAbort?.abort(); queryAbort = new AbortController(); const current = ++version;
    error.hidden = true; status.textContent = 'Loading recommendations…'; table.setAttribute('aria-busy', 'true');
    try {
      const result = await api.post('/api/recommendations/query', { filter: filter.value, sort: sort.value, includeDismissed: Boolean(dismissed.checked), page }, { signal: queryAbort.signal });
      if (alive && current === version) render(result);
    } catch (err) {
      if (!alive || current !== version || err.name === 'AbortError') return;
      if (err.status === 503) status.textContent = 'Waiting for stack data…';
      else { error.textContent = err.message; error.hidden = false; status.textContent = 'Unable to load recommendations.'; }
    } finally { if (alive && current === version) table.setAttribute('aria-busy', 'false'); }
  }
  function reset() { page = 1; if (dialog.open) dialog.close(); query(); }
  form.addEventListener('submit', event => { event.preventDefault(); reset(); });
  for (const input of [sort, dismissed, filter]) input.addEventListener('change', reset);
  previous.addEventListener('click', () => { page -= 1; query(); }); next.addEventListener('click', () => { page += 1; query(); });
  const unsubs = [stream.on('ready', query), stream.on('snapshot', snapshot => { if (snapshot.ready) query(); })];
  query();
  return { unmount() { alive = false; version += 1; detailVersion += 1; queryAbort?.abort(); detailAbort?.abort(); for (const off of unsubs) off(); if (dialog.open) dialog.close(); } };
}
