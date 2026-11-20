import { el } from '../app.js';

// Every amount, share, mode line, and policy line comes from Go (the
// response's display block, whose lists follow the report's order). This view
// collects the query, reads a chosen policy file as text, and lays out the
// strings it gets back.
const GROUP_BY = [
  ['namespace', 'Namespace'], ['controller', 'Controller'], ['pod', 'Pod'],
  ['node', 'Node'], ['pulumi-stack', 'Pulumi stack'], ['label', 'Label'],
];
const UNLABELED_CAPACITY = 'unlabeled';

function list(text) {
  return text.split(',').map(item => item.trim()).filter(Boolean);
}

function summary(label, value) {
  return el('span', { class: 'cluster-total' }, el('span', { class: 'totals-label', text: `${label} ` }), el('span', { class: 'totals-value', text: value }));
}

function num(text) {
  return el('td', { class: 'col-num', text });
}

function notesCell(notes) {
  const cell = el('td', { class: 'cluster-notes' });
  if (notes?.length) cell.append(el('ul', {}, notes.map(text => el('li', { text }))));
  return cell;
}

function bulletList(items) {
  return el('ul', { class: 'cluster-list' }, items.map(text => el('li', { text })));
}

export function mount(container, { api, navigate }) {
  let alive = true, version = 0, policyVersion = 0, queryAbort, policyAbort;
  const heading = el('h2', { tabindex: '-1', text: 'Cluster' });
  const intro = el('p', { class: 'cluster-intro', text: 'Kubernetes cost allocation, the same as finfocus cost cluster. A policy file you choose is read in this browser and sent as text.' });
  const error = el('p', { id: 'cluster-error', class: 'error-banner', role: 'alert' }); error.hidden = true;
  const status = el('p', { id: 'cluster-status', class: 'cluster-status', role: 'status', text: 'Choose the settings and run the allocation.' });

  const field = attrs => el('input', { class: 'filter-input', type: 'text', autocomplete: 'off', ...attrs });
  const labelled = (id, text, input) => el('label', { class: 'filter-label', for: id }, text, input);
  const context = field({ id: 'cluster-context', placeholder: 'Current context' });
  const namespace = field({ id: 'cluster-namespace', placeholder: 'All namespaces' });
  const selector = field({ id: 'cluster-selector', placeholder: 'key=value, key=value' });
  const groupBy = el('select', { id: 'cluster-group-by' });
  for (const [value, text] of GROUP_BY) groupBy.append(el('option', { value, text }));
  const labelKey = field({ id: 'cluster-label-key', placeholder: 'app_kubernetes_io_name' });
  const labelField = labelled('cluster-label-key', 'Label key', labelKey); labelField.hidden = true;
  const from = field({ id: 'cluster-from', placeholder: 'YYYY-MM-DD' });
  const to = field({ id: 'cluster-to', placeholder: 'Now' });
  const usageSource = field({ id: 'cluster-usage-source', placeholder: 'Plugin name' });
  const usageField = labelled('cluster-usage-source', 'Usage source', usageSource); usageField.hidden = true;
  const allocator = field({ id: 'cluster-allocator', placeholder: 'Plugin name' });
  const allocatorField = labelled('cluster-allocator', 'Allocator', allocator); allocatorField.hidden = true;
  const policyFile = el('input', { id: 'cluster-policy-file', type: 'file', accept: '.hujson,.json,application/json' });
  const submit = el('button', { type: 'submit', text: 'Run allocation' });
  const form = el('form', { class: 'toolbar cluster-toolbar' },
    labelled('cluster-context', 'Context', context),
    labelled('cluster-namespace', 'Namespace', namespace),
    labelled('cluster-selector', 'Selector', selector),
    labelled('cluster-group-by', 'Group by', groupBy),
    labelField,
    labelled('cluster-from', 'From', from),
    labelled('cluster-to', 'To', to),
    usageField,
    allocatorField,
    labelled('cluster-policy-file', 'Policy file (optional)', policyFile),
    submit);

  const unavailable = el('section', { class: 'cluster-unavailable', 'aria-label': 'Cluster allocation unavailable' }); unavailable.hidden = true;
  const totals = el('div', { class: 'totals-bar', 'aria-label': 'Cluster summary' }); totals.hidden = true;

  const caption = el('caption');
  const groupBody = el('tbody');
  const groupTable = el('table', { class: 'overview-table cluster-table' }, caption,
    el('thead', {}, el('tr', {},
      el('th', { scope: 'col', text: 'Group' }),
      el('th', { scope: 'col', class: 'col-num', text: 'CPU' }),
      el('th', { scope: 'col', class: 'col-num', text: 'Memory' }),
      el('th', { scope: 'col', class: 'col-num', text: 'Total' }),
      el('th', { scope: 'col', class: 'col-num', text: 'Share' }),
      el('th', { scope: 'col', text: 'Notes' }))),
    groupBody);
  const groupWrap = el('div', { class: 'table-wrap cluster-wrap', tabindex: '0', 'aria-label': 'Scrollable cluster groups' }, groupTable); groupWrap.hidden = true;

  const capacityBody = el('tbody');
  const capacityTable = el('table', { class: 'overview-table cluster-capacity-table' },
    el('caption', { text: 'Cost by node capacity type' }),
    el('thead', {}, el('tr', {},
      el('th', { scope: 'col', text: 'Capacity type' }),
      el('th', { scope: 'col', class: 'col-num', text: 'Nodes' }),
      el('th', { scope: 'col', class: 'col-num', text: 'Total' }),
      el('th', { scope: 'col', class: 'col-num', text: 'Share' }),
      el('th', { scope: 'col', class: 'col-num', text: 'Idle' }),
      el('th', { scope: 'col', text: 'Notes' }))),
    capacityBody);
  const capacityWrap = el('div', { class: 'table-wrap cluster-wrap', tabindex: '0', 'aria-label': 'Scrollable capacity types' }, capacityTable); capacityWrap.hidden = true;

  const warningsPanel = el('section', { class: 'cluster-warnings', 'aria-label': 'Warnings' }); warningsPanel.hidden = true;
  const incompletePanel = el('section', { class: 'cluster-incomplete', 'aria-label': 'Incomplete report' }); incompletePanel.hidden = true;

  const policyStatus = el('p', { class: 'cluster-policy-status', role: 'status' });
  const policyFacts = el('dl', { class: 'cluster-policy-facts' }); policyFacts.hidden = true;
  const policyText = el('pre', { class: 'cluster-policy-effective', tabindex: '0', 'aria-label': 'Effective policy' }); policyText.hidden = true;
  const policyPanel = el('details', { class: 'cluster-policy' },
    el('summary', { text: 'Allocation policy' }), policyStatus, policyFacts, policyText);

  container.append(heading, intro, form, error, status, unavailable, totals, groupWrap, capacityWrap, warningsPanel, incompletePanel, policyPanel);

  groupBy.addEventListener('change', () => { labelField.hidden = groupBy.value !== 'label'; });

  function setStatus(state, text) {
    status.dataset.state = state;
    status.textContent = text;
  }

  // A plugin-choice rejection names the flag that picks the plugin, so each
  // choice field appears only once the server has asked for it, and stays.
  function revealChoices(message) {
    if (message.includes('--usage-source')) usageField.hidden = false;
    if (message.includes('--allocator')) allocatorField.hidden = false;
  }

  async function readPolicy() {
    const file = policyFile.files?.[0];
    return file ? file.text() : undefined;
  }

  async function buildQuery() {
    const query = { groupBy: groupBy.value === 'label' ? `label:${labelKey.value.trim()}` : groupBy.value };
    for (const [key, input] of [['context', context], ['namespace', namespace], ['from', from], ['to', to], ['usageSource', usageSource], ['allocator', allocator]]) {
      const value = input.value.trim();
      if (value) query[key] = value;
    }
    const selectors = list(selector.value);
    if (selectors.length) query.selector = selectors;
    const policy = await readPolicy();
    if (policy !== undefined) query.policy = policy;
    return query;
  }

  function clearResults() {
    unavailable.hidden = true; totals.hidden = true; groupWrap.hidden = true; capacityWrap.hidden = true;
    warningsPanel.hidden = true; incompletePanel.hidden = true;
    unavailable.replaceChildren(); totals.replaceChildren(); groupBody.replaceChildren(); capacityBody.replaceChildren();
    warningsPanel.replaceChildren(); incompletePanel.replaceChildren();
  }

  function unavailableContent(answer) {
    const plugins = el('a', { href: '#/plugins', text: 'Open Plugins' });
    plugins.addEventListener('click', event => { event.preventDefault(); navigate?.('plugins'); });
    return [
      el('p', { class: 'cluster-unavailable-title', text: 'Cluster allocation needs a plugin that is not installed.' }),
      el('p', {}, 'Missing capability: ', el('code', { text: answer.capability })),
      el('p', { text: answer.error }),
      el('p', {}, plugins, ' to install one.'),
    ];
  }

  function showUnavailable(answer) {
    unavailable.replaceChildren(...unavailableContent(answer));
    unavailable.hidden = false;
    setStatus('unavailable', `Cluster allocation is unavailable: no plugin provides ${answer.capability}.`);
  }

  function render(doc) {
    const d = doc.display;
    totals.replaceChildren(summary('Total:', d.total));
    if (d.scope) totals.append(summary('Idle:', 'omitted'), summary('Scoped by:', d.scope));
    else totals.append(summary('Idle:', d.idle), summary('Idle share:', d.idleShare));
    totals.append(summary('Mode:', d.mode), summary('Policy:', d.policy));
    totals.hidden = false;

    caption.textContent = `Cost by ${doc.group_by}`;
    groupBody.replaceChildren(...(d.groups.length
      ? d.groups.map((g, i) => el('tr', {},
        el('th', { scope: 'row', class: 'cluster-row-head', text: g.key }),
        num(g.cpu), num(g.memory), num(g.total), num(g.share), notesCell(doc.groups[i]?.notes)))
      : [el('tr', {}, el('td', { colspan: '6', text: 'No groups.' }))]));
    groupWrap.hidden = false;

    const labeled = d.capacity.some(c => c.capacityType !== UNLABELED_CAPACITY);
    capacityBody.replaceChildren(...(labeled ? d.capacity.map((c, i) => el('tr', {},
      el('th', { scope: 'row', class: 'cluster-row-head', text: c.capacityType }),
      num(String(doc.capacity[i]?.nodes ?? '')), num(c.total), num(c.share), num(c.idle || '-'),
      notesCell(doc.capacity[i]?.notes))) : []));
    capacityWrap.hidden = !labeled;

    if (d.warnings.length) {
      warningsPanel.replaceChildren(el('h3', { text: 'Warnings' }), bulletList(d.warnings));
      warningsPanel.hidden = false;
    }
    if (doc.incomplete) {
      const parts = [el('h3', { text: 'Incomplete report' })];
      if (d.incompleteCauses.length) parts.push(el('p', { text: 'The usage source reported partial data:' }), bulletList(d.incompleteCauses));
      if (d.unpriced.length) parts.push(el('p', { text: 'Could not be priced:' }), bulletList(d.unpriced));
      if (!d.incompleteCauses.length && !d.unpriced.length) {
        parts.push(el('p', { text: d.warnings.length ? 'The warnings above explain what is missing.' : 'The usage source did not say what is missing.' }));
      }
      incompletePanel.replaceChildren(...parts);
      incompletePanel.hidden = false;
    }
    const count = d.groups.length === 1 ? '1 group' : `${d.groups.length} groups`;
    setStatus('done', doc.incomplete ? `${count} allocated. The report is incomplete.` : `${count} allocated.`);
  }

  function invalidQuery(err) {
    if (err.status !== 400 || err.code !== 'invalid_cluster_query') return false;
    revealChoices(err.message);
    return true;
  }

  async function run() {
    queryAbort?.abort(); queryAbort = new AbortController();
    const current = ++version;
    const signal = queryAbort.signal;
    error.hidden = true; clearResults();
    setStatus('loading', 'Running allocation…'); groupTable.setAttribute('aria-busy', 'true');
    try {
      const query = await buildQuery();
      if (!alive || current !== version) return;
      const doc = await api.post('/api/cluster/query', query, { signal });
      if (!alive || current !== version) return;
      if (doc.status === 'unavailable') showUnavailable(doc);
      else render(doc);
    } catch (err) {
      if (!alive || current !== version || err.name === 'AbortError') return;
      if (invalidQuery(err)) {
        setStatus('invalid', err.message);
        return;
      }
      error.textContent = err.message; error.hidden = false;
      setStatus('failed', 'Unable to run the allocation.');
    } finally {
      if (alive && current === version) groupTable.setAttribute('aria-busy', 'false');
    }
  }

  function policyMessage(state, text) {
    policyStatus.dataset.state = state;
    policyStatus.textContent = text;
  }

  async function loadPolicy() {
    policyAbort?.abort(); policyAbort = new AbortController();
    const current = ++policyVersion;
    const signal = policyAbort.signal;
    policyFacts.hidden = true; policyText.hidden = true;
    policyFacts.replaceChildren(); policyText.textContent = '';
    policyMessage('loading', 'Loading policy…');
    try {
      const query = {};
      const chosen = allocator.value.trim();
      if (chosen) query.allocator = chosen;
      const policy = await readPolicy();
      if (policy !== undefined) query.policy = policy;
      if (!alive || current !== policyVersion) return;
      const doc = await api.post('/api/cluster/policy', query, { signal });
      if (!alive || current !== policyVersion) return;
      if (doc.status === 'unavailable') {
        policyMessage('unavailable', `The policy is unavailable: ${doc.error}`);
        return;
      }
      policyFacts.replaceChildren(
        el('dt', { text: 'Source' }), el('dd', { text: doc.display.source }),
        el('dt', { text: 'Digest' }), el('dd', {}, el('code', { text: doc.display.digest })));
      policyText.textContent = JSON.stringify(doc.effective ?? {}, null, 2);
      policyFacts.hidden = false; policyText.hidden = false;
      policyMessage('done', 'Effective policy loaded.');
    } catch (err) {
      if (!alive || current !== policyVersion || err.name === 'AbortError') return;
      if (invalidQuery(err)) policyMessage('invalid', err.message);
      else policyMessage('failed', `Unable to load the policy. ${err.message}`);
    }
  }

  form.addEventListener('submit', event => { event.preventDefault(); return run(); });
  policyPanel.addEventListener('toggle', () => (policyPanel.open ? loadPolicy() : undefined));
  return { unmount() { alive = false; version += 1; policyVersion += 1; queryAbort?.abort(); policyAbort?.abort(); } };
}
