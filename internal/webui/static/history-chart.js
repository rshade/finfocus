import { el } from './app.js';

function svgEl(tag, attrs) {
  const node = document.createElementNS('http://www.w3.org/2000/svg', tag);
  for (const [key, value] of Object.entries(attrs)) node.setAttribute(key, String(value));
  return node;
}

// Old snapshots can carry Pulumi package keys. Fold their coordinates into
// the cloud names used for budget thresholds (see resourcetype.NormalizeProvider).
function providerCoordinates(amounts) {
  const clouds = new Map();
  for (const [key, amount] of Object.entries(amounts ?? {})) {
    if (!Number.isFinite(amount)) continue;
    let provider = key.trim().toLowerCase();
    switch (provider) {
      case 'aws-native': provider = 'aws'; break;
      case 'azure-native':
      case 'azurerm': provider = 'azure'; break;
      case 'google-native':
      case 'google': provider = 'gcp'; break;
    }
    clouds.set(provider, (clouds.get(provider) ?? 0) + amount);
  }
  return clouds;
}

// Only chart geometry uses numeric amounts. Text comes from the history response.
function chartPoints(body) {
  const snapshots = Array.isArray(body?.snapshots) ? body.snapshots : [];
  return snapshots.flatMap((snapshot) => {
    const x = Date.parse(snapshot?.timestamp);
    const y = snapshot?.total_monthly;
    return Number.isFinite(x) && Number.isFinite(y)
      ? [{ x, y, version: snapshot.version, providers: providerCoordinates(snapshot.by_provider) }]
      : [];
  }).sort((a, b) => a.x - b.x || a.version - b.version);
}

function chartThresholds(body) {
  if (Array.isArray(body.thresholds)) {
    return body.thresholds
      .filter((row) => Number.isFinite(row?.amount) && typeof row.display === 'string')
      .map((row) => row.provider || !body.display?.budget ? row : { ...row, display: body.display.budget });
  }
  return Number.isFinite(body.budget) && body.display?.budget
    ? [{ amount: body.budget, display: body.display.budget }]
    : [];
}

function budgetLabel(budget) {
  return budget.provider
    ? `${budget.provider.toUpperCase()} budget: ${budget.display}`
    : `Budget line: ${budget.display}`;
}

function historyChart(points, providers, budgets, deployments, selected, selectDeployment) {
  const width = 640;
  const height = 160;
  const pad = 28;
  const series = providers.length
    ? providers.map((provider) => points.map((point) => point.providers.get(provider) ?? 0))
    : [points.map((point) => point.y)];
  let minY = Infinity;
  let maxY = -Infinity;
  for (const value of series.flat().concat(budgets.map((budget) => budget.amount))) {
    minY = Math.min(minY, value);
    maxY = Math.max(maxY, value);
  }
  for (const point of points) {
    if (!deployments.has(point.version)) continue;
    const value = markerAmount(point, providers);
    minY = Math.min(minY, value);
    maxY = Math.max(maxY, value);
  }
  if (minY === maxY) {
    minY -= 1;
    maxY += 1;
  }
  const minX = points[0].x;
  const maxX = points.at(-1).x;
  const sx = (x) => pad + (maxX === minX ? (width - 2 * pad) / 2 : ((x - minX) / (maxX - minX)) * (width - 2 * pad));
  const sy = (y) => height - pad - ((y - minY) / (maxY - minY)) * (height - 2 * pad);
  const svg = svgEl('svg', {
    class: 'history-chart', viewBox: `0 0 ${width} ${height}`,
    role: 'presentation', 'aria-hidden': 'true', focusable: 'false',
  });
  budgets.forEach((budget) => {
    const y = sy(budget.amount);
    svg.append(svgEl('line', {
      class: `history-budget-line ${budgetStyle(budget, providers)}`,
      'stroke-dasharray': linePattern(providers.indexOf(budget.provider), true),
      x1: pad, x2: width - pad, y1: y, y2: y,
    }));
  });
  series.forEach((values, index) => {
    const style = `history-plot history-plot-${index % 4}`;
    if (points.length === 1) {
      svg.append(svgEl('circle', { class: style, cx: sx(points[0].x), cy: sy(values[0]), r: 3 }));
      return;
    }
    svg.append(svgEl('polyline', {
      class: style,
      'stroke-dasharray': linePattern(index),
      points: values.map((value, i) => `${sx(points[i].x)},${sy(value)}`).join(' '),
      fill: 'none',
    }));
  });
  for (const point of points) {
    const deployment = deployments.get(point.version);
    if (!deployment) continue;
    const x = sx(point.x);
    const y = sy(markerAmount(point, providers));
    const mark = svgEl('g', {
      class: point.version === selected ? 'history-marker history-marker-selected' : 'history-marker',
      'data-version': point.version,
    });
    mark.append(svgEl('circle', { class: 'history-marker-hit', cx: x, cy: y, r: '23' }));
    mark.append(svgEl('circle', { class: 'history-marker-glyph', cx: x, cy: y, r: '10' }));
    const label = svgEl('text', { x, y: y + 3, 'text-anchor': 'middle' });
    label.textContent = String(deployment.number);
    mark.append(label);
    mark.addEventListener('click', () => selectDeployment(deployment));
    svg.append(mark);
  }
  return svg;
}

function budgetStyle(budget, providers) {
  return budget.provider ? `history-plot-${providers.indexOf(budget.provider) % 4}` : 'history-global-budget';
}

function linePattern(index, budget = false) {
  const group = Math.max(0, Math.floor(index / 4));
  if (budget) return `${5 + group * 2} 3 1 3`;
  return group === 0 ? 'none' : `${5 + group * 2} 3`;
}

function legendKey(style, pattern) {
  const key = svgEl('svg', { class: 'history-key', viewBox: '0 0 26 8', 'aria-hidden': 'true' });
  key.append(svgEl('line', {
    class: `history-plot ${style}`, 'stroke-dasharray': pattern, x1: 0, x2: 26, y1: 4, y2: 4,
  }));
  return key;
}

function markerAmount(point, providers) {
  return providers.length ? point.providers.get(providers[0]) ?? 0 : point.y;
}

export function mountHistoryChart(body, displayed, onDeployment) {
  if (typeof document.createElementNS !== 'function') return null;
  const points = chartPoints(body);
  if (points.length === 0) return null;
  const dates = new Map(displayed.map((row) => [row.version, row.date]));
  const positions = new Map(points.map((point, index) => [point.version, index]));
  const providers = [...new Set(points.flatMap((point) => [...point.providers.keys()]))].sort();
  const thresholds = chartThresholds(body);
  const deployments = new Map();
  const choices = el('ol', { class: 'history-annotations' });
  const selection = el('p', { id: 'history-selected-deployment', role: 'status' });
  selection.hidden = true;
  let start = 0;
  let span = points.length;
  let showProviders = false;
  let showBudgets = true;
  let selected;
  const plot = el('div');
  const legend = el('ul', { class: 'history-chart-legend', 'aria-label': 'Chart legend' });
  const range = el('p', { id: 'history-chart-range', role: 'status' });
  const toolbar = el('div', { class: 'toolbar history-chart-controls', role: 'group', 'aria-label': 'Chart controls' });

  function control(id, label, action) {
    const button = el('button', { id, type: 'button', text: label });
    button.addEventListener('click', () => {
      if (!action()) return;
      draw();
    });
    toolbar.append(button);
    return button;
  }

  function resize(next) {
    if (next === span) return false;
    start = Math.max(0, Math.min(points.length - next, start + Math.floor((span - next) / 2)));
    span = next;
    return true;
  }

  function pan(step) {
    const next = Math.max(0, Math.min(points.length - span, start + step));
    if (next === start) return false;
    start = next;
    return true;
  }

  const zoomIn = control('history-zoom-in', 'Zoom in', () => resize(Math.min(points.length, Math.max(2, Math.ceil(span / 2)))));
  const zoomOut = control('history-zoom-out', 'Zoom out', () => resize(Math.min(points.length, span * 2)));
  const earlier = control('history-pan-prev', 'Pan earlier', () => pan(-Math.max(1, Math.floor(span / 2))));
  const later = control('history-pan-next', 'Pan later', () => pan(Math.max(1, Math.floor(span / 2))));
  const reset = control('history-reset-chart', 'Reset chart', () => resize(points.length));
  const split = providers.length ? control('history-show-providers', 'Provider series', () => {
    showProviders = !showProviders;
    return true;
  }) : null;
  const budgetToggle = thresholds.length ? control('history-show-budgets', 'Budget thresholds', () => {
    showBudgets = !showBudgets;
    return true;
  }) : null;

  function selectDeployment(deployment) {
    selected = deployment.version;
    const index = positions.get(selected);
    const previous = points[index - 1];
    const date = dates.get(selected);
    const timing = date ? ` (${date})` : '';
    selection.hidden = false;
    selection.textContent = `Deployment v${selected}${timing}. ${deployment.kind}: ${deployment.message}`;
    if (!previous) selection.textContent += '. No earlier checkpoint in this range.';
    deployment.button.setAttribute('aria-pressed', 'true');
    for (const item of deployments.values()) {
      if (item !== deployment) item.button.setAttribute('aria-pressed', 'false');
    }
    if (index < start || index >= start + span) {
      start = Math.max(0, Math.min(points.length - span, index - Math.floor(span / 2)));
    }
    draw();
    onDeployment(previous?.version, selected);
  }

  for (const item of Array.isArray(body.annotations) ? body.annotations : []) {
    if (!positions.has(item.version)) continue;
    const number = deployments.size + 1;
    const text = `${number}. v${item.version} ${item.kind}: ${item.message}`;
    const button = el('button', {
      type: 'button', class: 'history-deployment', 'aria-pressed': 'false',
      'aria-controls': 'history-selected-deployment history-diff-body', text,
    });
    const deployment = { ...item, number, button };
    button.addEventListener('click', () => selectDeployment(deployment));
    deployments.set(item.version, deployment);
    choices.append(el('li', {}, button));
  }

  function draw() {
    const visible = points.slice(start, start + span);
    const activeProviders = showProviders ? providers : [];
    const activeBudgets = showBudgets
      ? thresholds.filter((budget) => !budget.provider || activeProviders.includes(budget.provider))
      : [];
    plot.replaceChildren(historyChart(visible, activeProviders, activeBudgets, deployments, selected, selectDeployment));
    const seriesLabels = showProviders ? providers.map((provider) => provider.toUpperCase()) : ['Total cost'];
    legend.replaceChildren(...seriesLabels.map((label, index) => el('li', {},
      legendKey(`history-plot-${index % 4}`, linePattern(index)),
      label)), ...activeBudgets.map((budget) => el('li', { class: 'history-budget' },
      legendKey(budgetStyle(budget, activeProviders), linePattern(activeProviders.indexOf(budget.provider), true)),
      budgetLabel(budget))));
    const first = dates.get(visible[0].version) || `v${visible[0].version}`;
    const last = dates.get(visible.at(-1).version) || `v${visible.at(-1).version}`;
    let label = `Chart range ${first} to ${last}`;
    if (first === last) {
      const firstVersion = visible[0].version;
      const lastVersion = visible.at(-1).version;
      const versions = firstVersion === lastVersion ? `v${firstVersion}` : `v${firstVersion} to v${lastVersion}`;
      label = `Chart range ${first} (${versions})`;
    }
    if (range.textContent !== label) range.textContent = label;
    zoomIn.setAttribute('aria-disabled', String(span <= 2));
    zoomOut.setAttribute('aria-disabled', String(span === points.length));
    earlier.setAttribute('aria-disabled', String(start === 0));
    later.setAttribute('aria-disabled', String(start + span === points.length));
    reset.setAttribute('aria-disabled', String(span === points.length));
    split?.setAttribute('aria-pressed', String(showProviders));
    budgetToggle?.setAttribute('aria-pressed', String(showBudgets));
  }

  draw();
  return {
    element: el('div', { class: 'history-chart-panel' },
      toolbar, range, plot, legend, deployments.size ? choices : null, selection),
    clearSelection() {
      if (selected === undefined) return;
      deployments.get(selected).button.setAttribute('aria-pressed', 'false');
      selected = undefined;
      selection.hidden = true;
      selection.textContent = '';
      draw();
    },
  };
}
