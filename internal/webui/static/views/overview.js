// Overview view (T024): the 9-column resource dashboard fed by the SSE
// overview stream and the /api/overview/* endpoints.
//
// FR-011b hard rule: this module performs NO arithmetic on cost/number
// fields. Table cells render only the server-computed display strings
// (resourceDisplay, statusDisplay, actualDisplay, projectedDisplay,
// deltaDisplay, driftDisplay, recsDisplay); totals render the raw
// server-serialized values with the server-provided currency label and no
// rounding or reformatting. Filtering, sorting, cluster flattening, and
// pagination are all computed server-side via /api/overview/query; the
// client only sends the user's interaction state (filter text, sort field,
// page, expanded set).

import { el } from '../app.js';

// Sort cycle matches viewmodel.SortField order (cost, name, type, delta) and
// the wire names used by the contract examples ({"sort":"cost"}).
const SORT_CYCLE = ['cost', 'name', 'type', 'delta'];
const SORT_LABELS = { cost: 'Cost', name: 'Name', type: 'Type', delta: 'Delta' };
const PHASE_COUNT = 6;
const FILTER_DEBOUNCE_MS = 300;

const COLUMNS = [
  'Resource',
  'Type',
  'Status',
  'Actual MTD',
  'Projected',
  'Delta',
  'Drift%',
  'Recs',
  'Warn',
];

// pick reads the first present key, tolerating Go's default capitalized
// serialization for types that carry no JSON tags (e.g. BudgetResult).
function pick(obj, ...keys) {
  if (!obj || typeof obj !== 'object') return undefined;
  for (const key of keys) {
    if (obj[key] !== undefined && obj[key] !== null) return obj[key];
  }
  return undefined;
}

export function mount(container, ctx) {
  const { api, stream } = ctx;
  const unsubs = [];
  const aborter = new AbortController();
  let filterTimer = null;
  let previewTimer = null;
  let previewBase = { at: 0 };
  let lastFocus = null;
  let detailLauncher = null;
  let queryVersion = 0;
  let detailVersion = 0;
  let queryTimer = null;
  let budgetVersion = 0;

  const state = {
    phases: new Map(), // phase number -> { name, status }
    rowsByUrn: new Map(), // enrichment cache fed by row/snapshot/expansion events
    pageRows: [], // current flattened page from the server
    totals: null,
    budget: null,
    filter: '',
    sort: SORT_CYCLE[0],
    page: 1,
    totalPages: 1,
    expanded: [],
    loading: true,
    ready: false,
    errorPhase: null,
    progress: null,
    previewStatus: null,
    previewElapsedMs: null,
  };

  // --- Static DOM skeleton -------------------------------------------------

  const heading = el('h2', { tabindex: '-1', text: 'Cost Overview' });

  const errorBanner = el('div', { class: 'error-banner', role: 'alert' });
  errorBanner.hidden = true;

  const checklist = el('ol', { class: 'phase-checklist', 'aria-label': 'Loading checklist' });
  const phaseItems = new Map();
  for (let phase = 1; phase <= PHASE_COUNT; phase += 1) {
    const icon = el('span', { class: 'phase-icon', 'aria-hidden': 'true', text: '○' });
    const name = el('span', { class: 'phase-name', text: `Phase ${phase}` });
    const item = el('li', { class: 'phase', dataset: { status: 'pending' } }, icon, name);
    phaseItems.set(phase, { item, icon, name });
    checklist.append(item);
  }

  const progressLine = el('p', { class: 'progress-line', role: 'status' });
  progressLine.hidden = true;

  const filterInput = el('input', {
    id: 'overview-filter',
    class: 'filter-input',
    type: 'search',
    autocomplete: 'off',
    placeholder: 'Filter by resource or type',
  });
  const filterLabel = el('label', { class: 'filter-label', for: 'overview-filter' }, 'Filter');
  filterLabel.append(filterInput);

  const sortButton = el('button', {
    type: 'button',
    class: 'sort-button',
    'aria-label': 'Cycle sort field. Currently sorting by Cost',
    text: `Sort: ${SORT_LABELS[state.sort]}`,
  });

  const previewButton = el('button', {
    type: 'button',
    class: 'preview-button',
    text: 'Run preview',
    'aria-label': 'Run an on-demand pulumi preview',
  });

  const toolbar = el(
    'div',
    { class: 'toolbar', role: 'group', 'aria-label': 'Overview controls' },
    filterLabel,
    sortButton,
    previewButton,
  );

  const tbody = el('tbody');
  const headerRow = el('tr');
  for (const col of COLUMNS) headerRow.append(el('th', { scope: 'col', text: col }));
  const table = el(
    'table',
    { class: 'overview-table', 'aria-label': 'Resource cost overview' },
    el('thead', {}, headerRow),
    tbody,
  );
  const tableWrap = el('div', { class: 'table-wrap', tabindex: '0', role: 'region', 'aria-label': 'Scrollable resource cost table' }, table);

  const totalsBar = el('p', { class: 'totals-bar' });
  totalsBar.hidden = true;

  const prevButton = el('button', { type: 'button', class: 'pager-button', text: 'Previous' });
  const nextButton = el('button', { type: 'button', class: 'pager-button', text: 'Next' });
  const pageLabel = el('span', { class: 'page-label', text: 'Page 1 of 1' });
  const pager = el(
    'div',
    { class: 'pager', role: 'group', 'aria-label': 'Pagination' },
    prevButton,
    pageLabel,
    nextButton,
  );

  const budgetFooter = el('section', { class: 'budget-footer', 'aria-label': 'Budget health' });
  budgetFooter.hidden = true;

  const detailPanel = el('section', { class: 'detail-panel', 'aria-label': 'Resource detail' });
  detailPanel.hidden = true;

  const passphraseDialog = buildPassphraseDialog();

  container.append(
    el('section', { class: 'overview-view', 'aria-labelledby': 'overview-heading' },
      heading,
      errorBanner,
      checklist,
      progressLine,
      toolbar,
      totalsBar,
      tableWrap,
      pager,
      budgetFooter,
      detailPanel,
      passphraseDialog.dialog,
    ),
  );
  heading.id = 'overview-heading';

  // --- Renderers -----------------------------------------------------------

  const PHASE_ICONS = { pending: '○', active: '◌', done: '●', error: '✕' };

  function renderPhases() {
    for (const [phase, refs] of phaseItems) {
      const info = state.phases.get(phase);
      const status = info?.status ?? 'pending';
      refs.item.dataset.status = status;
      refs.icon.textContent = PHASE_ICONS[status] ?? PHASE_ICONS.pending;
      refs.name.textContent = info?.name ?? `Phase ${phase}`;
      refs.item.setAttribute('aria-label', `${refs.name.textContent} — ${status}`);
    }
    checklist.hidden = state.ready;
  }

  function renderProgress() {
    if (state.progress && !state.ready) {
      progressLine.hidden = false;
      progressLine.textContent = `Enriched ${state.progress.loaded} of ${state.progress.total} resources`;
    } else {
      progressLine.hidden = true;
    }
  }

  function renderTotals() {
    if (!state.totals) {
      totalsBar.hidden = true;
      return;
    }
    const t = state.totals;
    if (t.mixedCurrencies) {
      totalsBar.replaceChildren(el('p', { role: 'status', text: t.unavailableReason }));
      totalsBar.hidden = false;
      return;
    }
    const display = (key) => pick(t, `${key}Display`);
    // Values are server-computed and serialized; rendered verbatim, no
    // rounding or reformatting (FR-011b).
    const parts = [
      ['Total actual MTD', display('totalActual')],
      ['Projected', display('totalProjected')],
      ['Delta', display('totalDelta')],
      ['Potential savings', display('totalSavings')],
    ];
    totalsBar.replaceChildren(
      ...parts
        .filter(([, value]) => value !== undefined)
        .map(([label, value]) =>
          el('span', { class: 'totals-item' },
            el('span', { class: 'totals-label', text: `${label}: ` }),
            el('span', { class: 'totals-value', text: String(value) })),
        ),
    );
    totalsBar.hidden = false;
  }

  function warnText(row) {
    const warnings = pick(row, 'warnings');
    if (Array.isArray(warnings) && warnings.length > 0) return warnings.join(', ');
    if (pick(row, 'hasError')) return 'error';
    return '—';
  }

  function deltaClass(row) {
    const display = pick(row, 'deltaDisplay') ?? '';
    if (display.startsWith('-')) return 'delta-down';
    if (display.startsWith('+')) return 'delta-up';
    return '';
  }

  function renderRow(row) {
    const urn = pick(row, 'urn') ?? '';
    const childUrns = pick(row, 'childUrns');
    const isCluster = Array.isArray(childUrns) && childUrns.length > 0;
    const isExpanded = state.expanded.includes(urn);

    const resourceCell = el('td', { class: 'col-resource' });
    if (isCluster) {
      resourceCell.append(
        el('button', {
          type: 'button',
          class: 'cluster-toggle',
          'aria-expanded': String(isExpanded),
          'aria-label': `${isExpanded ? 'Collapse' : 'Expand'} cluster ${pick(row, 'displayName') ?? urn}`,
          text: isExpanded ? '▾' : '▸',
          onclick: () => toggleCluster(urn),
        }),
      );
    }
    resourceCell.append(
      el('button', {
        type: 'button',
        class: 'resource-link',
        text: pick(row, 'resourceDisplay') ?? urn,
        onclick: () => openDetail(urn),
      }),
    );

    const statusDisplay = pick(row, 'statusDisplay') ?? '';
    const tr = el(
      'tr',
      { dataset: { urn } },
      resourceCell,
      el('td', { class: 'col-type', text: pick(row, 'type') ?? '' }),
      el('td', {
        class: `col-status${pick(row, 'hasError') ? ' status-error' : ''}`,
        text: statusDisplay,
      }),
      el('td', { class: 'col-num', text: pick(row, 'actualDisplay') ?? '' }),
      el('td', { class: 'col-num', text: pick(row, 'projectedDisplay') ?? '' }),
      el('td', { class: `col-num ${deltaClass(row)}`.trim(), text: pick(row, 'deltaDisplay') ?? '' }),
      el('td', {
        class: `col-num${pick(row, 'driftWarning') ? ' drift-warning' : ''}`.trim(),
        text: pick(row, 'driftDisplay') ?? '',
      }),
      el('td', { class: 'col-num', text: pick(row, 'recsDisplay') ?? '' }),
      el('td', { class: 'col-warn', text: warnText(row) }),
    );
    if (pick(row, 'hasError')) tr.classList.add('row-error');
    return tr;
  }

  function renderTable() {
    const focus = captureRowFocus();
    table.setAttribute('aria-busy', String(state.loading && state.pageRows.length === 0));
    if (state.pageRows.length === 0) {
      const message = state.loading
        ? 'Loading resources…'
        : state.filter
          ? 'No resources match the filter.'
          : 'No resources.';
      tbody.replaceChildren(
        el('tr', { class: 'empty-row' },
          el('td', { colspan: String(COLUMNS.length), text: message })),
      );
      return;
    }
    tbody.replaceChildren(...state.pageRows.map(renderRow));
    restoreRowFocus(focus);
  }

  function captureRowFocus() {
    const active = document.activeElement;
    const row = active?.closest?.('tr');
    if (!row?.dataset.urn) return null;
    return { urn: row.dataset.urn, selector: active.classList.contains('cluster-toggle') ? '.cluster-toggle' : '.resource-link' };
  }

  function restoreRowFocus(focus) {
    if (!focus) return;
    const row = tbody.querySelector(`tr[data-urn="${CSS.escape(focus.urn)}"]`);
    (row?.querySelector(focus.selector) ?? filterInput).focus();
  }

  function renderPager() {
    pageLabel.textContent = `Page ${state.page} of ${state.totalPages}`;
    prevButton.disabled = state.page <= 1;
    nextButton.disabled = state.page >= state.totalPages;
    pager.hidden = state.totalPages <= 1 && state.pageRows.length === 0;
  }

  function renderBudget() {
    const result = state.budget;
    if (!result) {
      budgetFooter.hidden = true;
      return;
    }
    const display = result?.display;
    const children = [el('h3', { class: 'budget-heading', text: 'Budget health' })];
    if (display?.footerDisplay) children.push(el('p', { class: 'budget-overall', text: display.footerDisplay }));
    if (display?.details?.length) children.push(budgetList(display.details));
    if (children.length === 1) children.push(el('p', { class: 'budget-empty', text: 'No budget data.' }));
    budgetFooter.replaceChildren(...children);
    budgetFooter.hidden = false;
  }

  function budgetList(budgets) {
    const list = el('ul', { class: 'budget-list' });
    for (const budget of budgets) {
      const item = el('li', {}, el('span', { class: 'budget-name', text: budget.name }), el('span', { class: 'budget-health-badge', text: budget.healthDisplay }),
        el('span', { class: 'budget-values', text: `${budget.currentSpendDisplay} / ${budget.limitDisplay}${budget.utilizationDisplay ? ` (${budget.utilizationDisplay})` : ''}` }));
      if (budget.forecastedDisplay) item.append(el('p', { text: `Forecasted: ${budget.forecastedDisplay}` }));
      for (const threshold of budget.thresholds ?? []) item.append(el('p', { class: 'budget-threshold', text: `ALERT: ${threshold.text}` }));
      list.append(item);
    }
    return list;
  }

  function renderPreviewButton() {
    if (state.previewStatus === 'running') {
      previewButton.disabled = true;
      previewButton.textContent = `Preview running… ${previewElapsedText()}`;
      previewButton.setAttribute('aria-label', 'Preview in progress');
    } else {
      previewButton.disabled = false;
      previewButton.textContent = 'Run preview';
      previewButton.setAttribute('aria-label', 'Run an on-demand pulumi preview');
    }
  }

  function previewElapsedText() {
    const extra = previewBase.at > 0 ? Date.now() - previewBase.at : 0;
    const ms = (state.previewElapsedMs ?? 0) + extra;
    return `${Math.round(ms / 1000)}s elapsed`;
  }

  function showError(message) {
    errorBanner.textContent = message;
    errorBanner.hidden = false;
  }

  // --- Server interactions -------------------------------------------------

  function applyPage(resp) {
    if (!resp || typeof resp !== 'object') return;
    const rows = pick(resp, 'rows');
    if (Array.isArray(rows)) {
      state.pageRows = rows;
      for (const row of rows) {
        const urn = pick(row, 'urn');
        if (urn) state.rowsByUrn.set(urn, row);
      }
    }
    const totals = pick(resp, 'totals');
    if (totals) state.totals = totals;
    const page = pick(resp, 'page');
    if (typeof page === 'number') state.page = page;
    const totalPages = pick(resp, 'totalPages');
    if (typeof totalPages === 'number') state.totalPages = Math.max(totalPages, 1);
    const expanded = pick(resp, 'expanded');
    if (Array.isArray(expanded)) state.expanded = expanded;
    renderTable();
    renderTotals();
    renderPager();
  }

  async function runQuery() {
    const version = ++queryVersion;
    try {
      const resp = await api.post(
        '/api/overview/query',
        {
          filter: state.filter,
          sort: state.sort,
          page: state.page,
          expanded: state.expanded,
        },
        { signal: aborter.signal },
      );
      if (aborter.signal.aborted || version !== queryVersion) return;
      state.loading = false;
      applyPage(resp);
    } catch (err) {
      if (err.name === 'AbortError' || aborter.signal.aborted || version !== queryVersion) return;
      state.loading = false;
      showError(err.message);
      renderTable();
    }
  }

  async function toggleCluster(urn) {
    const version = ++queryVersion;
    const previous = state.expanded;
    const tentative = state.expanded.includes(urn)
      ? state.expanded.filter((u) => u !== urn)
      : [...state.expanded, urn];
    state.expanded = tentative;
    try {
      const resp = await api.post(
        '/api/overview/cluster/toggle',
        { urn, expanded: previous, filter: state.filter, sort: state.sort, page: state.page },
        { signal: aborter.signal },
      );
      if (aborter.signal.aborted || version !== queryVersion) return;
      state.expanded = Array.isArray(pick(resp, 'expanded')) ? resp.expanded : tentative;
      applyPage(resp);
    } catch (err) {
      if (err.name !== 'AbortError' && !aborter.signal.aborted && version === queryVersion) {
        state.expanded = previous;
        showError(err.message);
        runQuery();
      }
    }
  }

  function openDetail(urn) {
    const version = ++detailVersion;
    lastFocus = document.activeElement;
    detailLauncher = captureRowFocus();
    detailPanel.replaceChildren(el('p', { class: 'view-loading', role: 'status', text: 'Loading detail…' }));
    detailPanel.hidden = false;
    api
      .get(`/api/overview/resource?urn=${encodeURIComponent(urn)}`, { signal: aborter.signal })
      .then((resp) => {
        if (!aborter.signal.aborted && version === detailVersion) renderDetail(resp, urn);
      })
      .catch((err) => {
        if (err.name === 'AbortError' || aborter.signal.aborted || version !== detailVersion) return;
        detailPanel.replaceChildren(
          el('p', { class: 'error-banner', role: 'alert', text: err.message }),
          closeDetailButton(),
        );
      });
  }

  function closeDetailButton() {
    return el('button', {
      type: 'button',
      class: 'detail-close',
      text: 'Close detail',
      onclick: closeDetail,
    });
  }

  function closeDetail() {
    detailVersion += 1;
    detailPanel.hidden = true;
    detailPanel.replaceChildren();
    if (detailLauncher) restoreRowFocus(detailLauncher);
    else if (lastFocus?.isConnected && typeof lastFocus.focus === 'function') lastFocus.focus();
    else filterInput.focus();
  }

  function renderDetail(resp, urn) {
    const row =
      resp && typeof resp === 'object' && resp.urn !== undefined
        ? resp
        : pick(resp, 'row', 'resource') ?? {};
    const children = [
      el('div', { class: 'detail-header' },
        el('h3', { tabindex: '-1', text: pick(row, 'displayName') ?? urn }),
        closeDetailButton()),
      el('dl', { class: 'detail-list' },
        ...[
          ['URN', urn],
          ['Type', pick(row, 'type')],
          ['Status', pick(row, 'statusDisplay')],
          ['Actual MTD', pick(row, 'actualDisplay')],
          ['Projected', pick(row, 'projectedDisplay')],
          ['Delta', pick(row, 'deltaDisplay')],
          ['Drift', pick(row, 'driftDisplay')],
        ]
          .filter(([, value]) => value !== undefined && value !== '')
          .flatMap(([label, value]) => [
            el('dt', { text: label }),
            el('dd', { text: String(value) }),
          ])),
    ];

    const diffs = pick(row, 'propertyDiffs');
    for (const [key, title] of [['actualBreakdown', 'Actual cost breakdown'], ['projectedBreakdown', 'Projected cost breakdown']]) {
      const values = resp.display?.[key];
      if (values?.length) children.push(el('h4', { text: title }), el('dl', { class: 'detail-list breakdown-list' },
        values.flatMap(item => [el('dt', { text: item.name }), el('dd', { text: item.costDisplay })])));
    }
    for (const [key, title] of [['impact', 'Cost impact'], ['drift', 'Cost drift']]) {
      const fields = resp.display?.[key];
      if (fields?.length) children.push(el('h4', { text: title }), el('dl', { class: 'detail-list' },
        fields.flatMap(field => [el('dt', { text: field.name }), el('dd', { text: field.value })])));
    }
    if (Array.isArray(diffs) && diffs.length > 0) {
      children.push(
        el('h4', { text: 'Property changes' }),
        el('div', { class: 'table-wrap' },
          el('table', { class: 'detail-table' },
            el('thead', {},
              el('tr', {},
                el('th', { scope: 'col', text: 'Property' }),
                el('th', { scope: 'col', text: 'Before' }),
                el('th', { scope: 'col', text: 'After' }))),
            el('tbody', {},
              diffs.map((d) =>
                el('tr', {},
                  el('td', { text: pick(d, 'key') ?? '' }),
                  el('td', { text: pick(d, 'oldValue') ?? '' }),
                  el('td', { text: pick(d, 'newValue') ?? '' })))))),
      );
    }

    const warnings = pick(row, 'warnings');
    if (Array.isArray(warnings) && warnings.length > 0) {
      children.push(
        el('h4', { text: 'Warnings' }),
        el('ul', { class: 'detail-warnings' }, warnings.map((w) => el('li', { text: String(w) }))),
      );
    }

    const error = pick(row, 'error');
    if (error && typeof error === 'object') {
      children.push(
        el('h4', { text: 'Error' }),
        el('p', { class: 'error-banner', text: pick(error, 'message') ?? String(error) }),
      );
    }

    const recs = pick(resp, 'activeRecommendations');
    if (Array.isArray(recs) && recs.length > 0) {
      const list = el('ul', { class: 'detail-recs' });
      for (const entry of recs) {
        const rec = entry.recommendation;
        const type = pick(rec, 'type');
        const description = pick(rec, 'description');
        list.append(
          el('li', {},
            el('span', { class: 'rec-type', text: type ? `[${type}] ` : '' }),
            el('span', { text: description ?? '' }),
            el('span', { class: 'rec-savings', text: entry.savingsDisplay ? ` — ${entry.savingsDisplay}` : '' })),
        );
      }
      children.push(el('h4', { text: 'Recommendations' }), list);
    }

    if (resp.budgetDisplay?.details?.length) children.push(el('h4', { text: 'Budget status' }), budgetList(resp.budgetDisplay.details));

    detailPanel.replaceChildren(...children);
    detailPanel.querySelector('h3')?.focus();
  }

  async function triggerPreview() {
    state.previewStatus = 'running';
    state.previewElapsedMs = 0;
    previewBase = { at: Date.now() };
    renderPreviewButton();
    startPreviewTimer();
    try {
      // 202 means attached to an already-running preview; both are fine.
      await api.post('/api/overview/preview', {}, { signal: aborter.signal });
    } catch (err) {
      if (err.name === 'AbortError') return;
      state.previewStatus = null;
      stopPreviewTimer();
      renderPreviewButton();
      showError(err.message);
    }
  }

  function startPreviewTimer() {
    stopPreviewTimer();
    previewTimer = setInterval(() => {
      if (state.previewStatus === 'running') renderPreviewButton();
    }, 1000);
  }

  function stopPreviewTimer() {
    if (previewTimer !== null) {
      clearInterval(previewTimer);
      previewTimer = null;
    }
  }

  // --- Passphrase modal -----------------------------------------------------

  function buildPassphraseDialog() {
    const input = el('input', {
      id: 'passphrase-input',
      type: 'password',
      autocomplete: 'off',
      required: '',
      'aria-describedby': 'passphrase-error',
    });
    const errorText = el('p', { id: 'passphrase-error', class: 'dialog-error', role: 'alert' });
    errorText.hidden = true;
    const stackLine = el('p', { class: 'dialog-stack' });
    const submit = el('button', { type: 'submit', class: 'dialog-submit', text: 'Unlock' });
    const cancel = el('button', { type: 'button', class: 'dialog-cancel', text: 'Cancel' });

    const form = el(
      'form',
      { class: 'passphrase-form' },
      el('h3', { id: 'passphrase-title', text: 'Encrypted stack' }),
      stackLine,
      el('label', { for: 'passphrase-input', text: 'Passphrase' }),
      input,
      errorText,
      el('div', { class: 'dialog-actions' }, submit, cancel),
    );
    const dialog = el('dialog', { class: 'passphrase-dialog', 'aria-labelledby': 'passphrase-title' }, form);

    let busy = false;
    let promptVersion = 0;
    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      if (busy) return;
      const passphrase = input.value;
      const version = promptVersion;
      const requirement = stream.passphraseGeneration;
      if (!passphrase) {
        errorText.textContent = 'Enter the stack passphrase.';
        errorText.hidden = false;
        return;
      }
      busy = true;
      submit.disabled = true;
      errorText.hidden = true;
      try {
        await api.post('/api/passphrase', { passphrase }, { signal: aborter.signal });
        stream.acceptPassphrase(requirement);
        if (version === promptVersion && !aborter.signal.aborted) {
          dialog.close();
        }
      } catch (err) {
        if (err.name !== 'AbortError' && version === promptVersion && !aborter.signal.aborted) {
          errorText.textContent = err.message;
          errorText.hidden = false;
        }
      } finally {
        if (version === promptVersion) {
          busy = false;
          submit.disabled = false;
          input.value = '';
        }
      }
    });
    cancel.addEventListener('click', () => dialog.close());
    dialog.addEventListener('close', () => {
      promptVersion += 1;
      input.value = '';
      busy = false;
      submit.disabled = false;
    });

    return {
      dialog,
      close() {
        if (dialog.open) dialog.close();
      },
      open(stack) {
        promptVersion += 1;
        busy = false;
        submit.disabled = false;
        stackLine.textContent = stack
          ? `The stack "${stack}" is passphrase-protected.`
          : 'The stack is passphrase-protected.';
        if (!dialog.open) errorText.hidden = true;
        input.value = '';
        if (!dialog.open) dialog.showModal();
        input.focus();
      },
      fail(message) {
        errorText.textContent = message;
        errorText.hidden = false;
      },
      get isOpen() {
        return dialog.open;
      },
    };
  }

  // --- SSE event handlers ----------------------------------------------------

  function onSnapshot(data) {
    state.ready = Boolean(data?.ready);
    state.loading = !state.ready;
    state.phases.clear();
    state.rowsByUrn.clear();
    const phases = pick(data, 'phases');
    if (Array.isArray(phases)) {
      for (const p of phases) {
        const num = pick(p, 'phase');
        if (typeof num === 'number') {
          state.phases.set(num, { name: pick(p, 'name'), status: pick(p, 'status') });
        }
      }
    }
    const rows = pick(data, 'rows');
    if (Array.isArray(rows)) {
      for (const row of rows) {
        const urn = pick(row, 'urn');
        if (urn) state.rowsByUrn.set(urn, row);
      }
    }
    const totals = pick(data, 'totals');
    if (totals) state.totals = totals;
    const budget = pick(data, 'budget');
    if (budget) {
      budgetVersion += 1;
      state.budget = budget;
    }
    renderPhases();
    renderTotals();
    renderBudget();
    if (data?.progress) onProgress(data.progress);
    if (data?.preview) onPreview(data.preview);
    if (Array.isArray(data?.errors)) {
      state.errorPhase = null;
      errorBanner.hidden = true;
      errorBanner.textContent = '';
      for (const error of data.errors) onError(error);
    }
    if (data?.passphraseRequired) onPassphraseRequired({ stack: data.stack });
    else if (data?.passphraseRequired === false) passphraseDialog.close();
    runQuery();
  }

  function onPhase(data) {
    const num = pick(data, 'phase');
    if (typeof num !== 'number') return;
    state.phases.set(num, { name: pick(data, 'name'), status: pick(data, 'status') });
    state.loading = true;
    if ((data.status === 'active' || data.status === 'done') && state.errorPhase === num) {
      state.errorPhase = null;
      errorBanner.hidden = true;
      errorBanner.textContent = '';
    }
    renderPhases();
  }

  function onRow(row) {
    const urn = pick(row, 'urn');
    if (!urn) return;
    state.rowsByUrn.set(urn, row);
    const index = state.pageRows.findIndex((r) => pick(r, 'urn') === urn);
    if (index >= 0) {
      state.pageRows[index] = row;
      const existing = tbody.querySelector(`tr[data-urn="${CSS.escape(urn)}"]`);
      if (existing) {
        const focus = captureRowFocus();
        existing.replaceWith(renderRow(row));
        restoreRowFocus(focus);
      }
    }
    // Ask the shared Go view model for the current page as enrichment can
    // change ordering, filter matches, and previously unseen rows.
    if (queryTimer === null) {
      queryTimer = setTimeout(() => { queryTimer = null; runQuery(); }, 50);
    }
  }

  function onProgress(data) {
    const loaded = pick(data, 'loaded');
    const total = pick(data, 'total');
    if (typeof loaded === 'number' && typeof total === 'number') {
      state.progress = { loaded, total };
      renderProgress();
    }
  }

  function onBudget(data) {
    budgetVersion += 1;
    state.budget = data;
    renderBudget();
  }

  function onError(data) {
    state.errorPhase = typeof data?.phase === 'number' ? data.phase : null;
    const message = pick(data, 'message') ?? 'An unknown error occurred.';
    const phase = pick(data, 'phase');
    const text = typeof phase === 'number' ? `Phase ${phase}: ${message}` : message;
    if (passphraseDialog.isOpen) {
      passphraseDialog.fail(message);
    } else {
      showError(text);
    }
    if (typeof phase === 'number' && state.phases.has(phase)) {
      const info = state.phases.get(phase);
      state.phases.set(phase, { ...info, status: 'error' });
      renderPhases();
    }
  }

  function onExpansion(data) {
    const rows = pick(data, 'rows');
    if (Array.isArray(rows)) {
      state.rowsByUrn.clear();
      for (const row of rows) {
        const urn = pick(row, 'urn');
        if (urn) state.rowsByUrn.set(urn, row);
      }
    }
    const notes = pick(data, 'notes');
    if (Array.isArray(notes) && notes.length > 0) {
      progressLine.hidden = false;
      progressLine.textContent = notes.join(' ');
    }
    runQuery();
  }

  function onReady(data) {
    if (state.errorPhase !== null) {
      state.errorPhase = null;
      errorBanner.hidden = true;
      errorBanner.textContent = '';
    }
    const totals = pick(data, 'totals');
    if (totals) state.totals = totals;
    state.ready = true;
    state.loading = false;
    for (const [phase, info] of state.phases) {
      if (info.status === 'active') state.phases.set(phase, { ...info, status: 'done' });
    }
    renderPhases();
    renderProgress();
    renderTotals();
    runQuery();
  }

  function onPassphraseRequired(data) {
    passphraseDialog.open(pick(data, 'stack'));
    // A rejection can arrive after the POST was accepted and the previous
    // dialog closed. Keep that error next to the retry input as well.
    if (!errorBanner.hidden) {
      passphraseDialog.fail(errorBanner.textContent);
      errorBanner.hidden = true;
    }
  }

  function onPreview(data) {
    const status = pick(data, 'status');
    const elapsedMs = pick(data, 'elapsedMs');
    if (typeof elapsedMs === 'number') {
      state.previewElapsedMs = elapsedMs;
      previewBase.at = Date.now();
    }
    const rows = pick(data, 'rows');
    if (Array.isArray(rows)) {
      for (const row of rows) {
        const urn = pick(row, 'urn');
        if (urn) state.rowsByUrn.set(urn, row);
      }
      state.previewStatus = 'done';
      stopPreviewTimer();
      renderPreviewButton();
      runQuery();
      return;
    }
    if (status === 'running' || status === 'started') {
      state.previewStatus = 'running';
      startPreviewTimer();
    } else if (status) {
      state.previewStatus = status === 'error' || status === 'failed' ? null : status;
      if (state.previewStatus !== 'running') stopPreviewTimer();
      if (status === 'error' || status === 'failed') {
        showError('Preview failed.');
        state.previewStatus = null;
      }
    }
    renderPreviewButton();
  }

  // --- Wiring ----------------------------------------------------------------

  for (const [event, handler] of [
    ['snapshot', onSnapshot],
    ['phase', onPhase],
    ['row', onRow],
    ['progress', onProgress],
    ['budget', onBudget],
    ['error', onError],
    ['expansion', onExpansion],
    ['ready', onReady],
    ['passphrase_required', onPassphraseRequired],
    ['preview', onPreview],
  ]) {
    unsubs.push(stream.on(event, handler));
  }

  filterInput.addEventListener('input', () => {
    clearTimeout(filterTimer);
    filterTimer = setTimeout(() => {
      state.filter = filterInput.value;
      state.page = 1;
      runQuery();
    }, FILTER_DEBOUNCE_MS);
  });
  filterInput.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') {
      filterInput.value = '';
      state.filter = '';
      state.page = 1;
      runQuery();
    }
  });

  sortButton.addEventListener('click', () => {
    const index = SORT_CYCLE.indexOf(state.sort);
    state.sort = SORT_CYCLE[(index + 1) % SORT_CYCLE.length];
    sortButton.textContent = `Sort: ${SORT_LABELS[state.sort]}`;
    sortButton.setAttribute(
      'aria-label',
      `Cycle sort field. Currently sorting by ${SORT_LABELS[state.sort]}`,
    );
    state.page = 1;
    runQuery();
  });

  prevButton.addEventListener('click', () => {
    if (state.page > 1) {
      state.page -= 1;
      runQuery();
    }
  });
  nextButton.addEventListener('click', () => {
    if (state.page < state.totalPages) {
      state.page += 1;
      runQuery();
    }
  });

  previewButton.addEventListener('click', triggerPreview);

  document.addEventListener('keydown', onEscape);
  function onEscape(event) {
    if (event.key === 'Escape' && !detailPanel.hidden) closeDetail();
  }
  unsubs.push(() => document.removeEventListener('keydown', onEscape));

  renderPhases();
  renderTable();
  renderPager();
  renderPreviewButton();
  if (stream.snapshot) onSnapshot(stream.snapshot);
  else runQuery();
  if (!state.budget) {
    const version = budgetVersion;
    api
      .get('/api/overview/budget', { signal: aborter.signal })
      .then((resp) => {
        if (resp && !aborter.signal.aborted && version === budgetVersion) {
          state.budget = resp;
          renderBudget();
        }
      })
      .catch(() => {
        // Budget data is optional; the SSE budget event may still deliver it.
      });
  }

  return {
    unmount() {
      for (const unsub of unsubs) unsub();
      clearTimeout(filterTimer);
      clearTimeout(queryTimer);
      stopPreviewTimer();
      aborter.abort();
      if (passphraseDialog.isOpen) passphraseDialog.dialog.close();
    },
  };
}
