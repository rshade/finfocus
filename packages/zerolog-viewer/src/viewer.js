import { createNDJSONParser } from './ndjson.js';
import { normalizeEntry, LEVELS } from './normalize.js';
import { compileFilter, matchesFilter } from './filter.js';

const DEFAULT_MAX_ROWS = 2000;
const FOLLOW_SLACK_PX = 4;
let detailSerial = 0;
const KINDS = [
  ['', 'All sources'],
  ['core', 'Core'],
  ['plugin', 'Plugins'],
];

function pad(value, width) {
  return String(value).padStart(width, '0');
}

function fallbackDisplay(entry) {
  const parsed = new Date(entry.time);
  const time = Number.isNaN(parsed.getTime())
    ? String(entry.time ?? '')
    : `${pad(parsed.getHours(), 2)}:${pad(parsed.getMinutes(), 2)}:${pad(parsed.getSeconds(), 2)}.${pad(parsed.getMilliseconds(), 3)}`;
  return { time, level: String(entry.level ?? '').toUpperCase(), source: entry.source };
}

function fieldText(value) {
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function exportLine(entry) {
  const out = {
    time: entry.time,
    level: entry.level,
    source: entry.source,
    sourceKind: entry.sourceKind,
    message: entry.message ?? '',
  };
  if (entry.fields && Object.keys(entry.fields).length > 0) out.fields = entry.fields;
  return JSON.stringify(out);
}

/**
 * Renders log entries into container. All text is set through textContent.
 * options: document, maxRows, filters, follow, fieldMap, sourceFields,
 * pluginFields, sources (names to offer as chips before any line arrives).
 */
export function createLogViewer(container, options = {}) {
  const doc = options.document ?? container.ownerDocument;
  const maxRows = options.maxRows ?? DEFAULT_MAX_ROWS;
  const normalizeOptions = {
    fieldMap: options.fieldMap,
    sourceFields: options.sourceFields,
    pluginFields: options.pluginFields,
  };
  const defaultFilters = { sources: [], kind: '', minLevel: '', text: '', regex: false };
  let raw = { ...defaultFilters, ...options.filters };
  let filter = compileFilter(raw);
  let shown = 0;
  let following = options.follow !== false;
  let pending = 0;
  let dropped = 0;
  let localSeq = 0;
  let items = [];
  const sourceNames = [];
  const chips = new Map();
  const parser = createNDJSONParser();

  const make = (tag, className, text) => {
    const node = doc.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
  };

  const makeButton = (className, label) => {
    const button = make('button', className, label);
    button.setAttribute('type', 'button');
    return button;
  };

  const makeSelect = (className, label, choices) => {
    const select = make('select', className);
    select.setAttribute('aria-label', label);
    for (const [value, text] of choices) {
      const option = make('option', '', text);
      option.setAttribute('value', value);
      select.appendChild(option);
    }
    return select;
  };

  const root = make('div', 'zlv');
  const toolbar = make('div', 'zlv-toolbar');
  const chipBox = make('div', 'zlv-chips');
  chipBox.setAttribute('role', 'group');
  chipBox.setAttribute('aria-label', 'Sources');
  const kindSelect = makeSelect('zlv-kind-select', 'Source kind', KINDS);
  const levelSelect = makeSelect('zlv-level-select', 'Minimum level', [
    ['', 'All levels'],
    ...LEVELS.map((level) => [level, `${level}+`]),
  ]);
  const search = make('input', 'zlv-search');
  search.setAttribute('type', 'search');
  search.setAttribute('placeholder', 'Filter message and field values');
  search.setAttribute('aria-label', 'Filter log lines');
  search.setAttribute('aria-invalid', 'false');
  const regexLabel = make('label', 'zlv-regex-label');
  const regexBox = make('input', 'zlv-regex');
  regexBox.setAttribute('type', 'checkbox');
  regexLabel.appendChild(regexBox);
  regexLabel.appendChild(make('span', '', 'Regex'));
  const followButton = makeButton('zlv-follow', 'Pause');
  const clearButton = makeButton('zlv-clear', 'Clear');
  const downloadButton = makeButton('zlv-download', 'Download NDJSON');
  const filterError = make('span', 'zlv-filter-error');
  filterError.setAttribute('role', 'status');
  for (const node of [chipBox, kindSelect, levelSelect, search, regexLabel, followButton, clearButton, downloadButton, filterError]) {
    toolbar.appendChild(node);
  }
  const status = make('div', 'zlv-status');
  status.setAttribute('aria-live', 'off');
  status.setAttribute('aria-atomic', 'true');
  const list = make('div', 'zlv-list');
  list.setAttribute('role', 'log');
  list.setAttribute('aria-live', 'polite');
  root.appendChild(toolbar);
  root.appendChild(status);
  root.appendChild(list);
  container.appendChild(root);

  const lineNoun = (count) => (count === 1 ? 'line' : 'lines');

  const updateStatus = () => {
    const parts = [`Showing ${shown} of ${items.length} ${lineNoun(items.length)}`];
    if (!following && pending > 0) parts.push(`${pending} new ${lineNoun(pending)} while paused`);
    if (dropped > 0) {
      parts.push(`${dropped} older ${lineNoun(dropped)} dropped (showing the newest ${maxRows})`);
    }
    status.textContent = parts.join(', ');
  };

  const updateCount = updateStatus;

  const updateFollowButton = () => {
    followButton.textContent = following ? 'Pause' : 'Resume';
    followButton.setAttribute('aria-pressed', following ? 'true' : 'false');
    // While following, the log region announces each new line. While paused,
    // that region is quiet and the status line announces the pending count.
    list.setAttribute('aria-live', following ? 'polite' : 'off');
    status.setAttribute('aria-live', following ? 'off' : 'polite');
  };

  const scrollToEnd = () => {
    if (following) list.scrollTop = list.scrollHeight;
  };

  const setFollowing = (value) => {
    following = value;
    if (following) {
      pending = 0;
      list.scrollTop = list.scrollHeight;
    }
    updateFollowButton();
    updateStatus();
  };

  let ignoreScroll = false;
  const onScroll = () => {
    if (ignoreScroll) return;
    const atEnd = list.scrollHeight - list.scrollTop - list.clientHeight <= FOLLOW_SLACK_PX;
    if (atEnd !== following) setFollowing(atEnd);
  };
  list.addEventListener('scroll', onScroll);

  const syncChips = () => {
    for (const [name, chip] of chips) {
      chip.setAttribute('aria-pressed', raw.sources.includes(name) ? 'true' : 'false');
    }
  };

  const addSource = (name) => {
    if (typeof name !== 'string' || name === '' || chips.has(name)) return;
    sourceNames.push(name);
    const chip = makeButton('zlv-chip', name);
    chip.setAttribute('aria-pressed', raw.sources.includes(name) ? 'true' : 'false');
    chip.addEventListener('click', () => {
      const selected = raw.sources.includes(name) ? raw.sources.filter((s) => s !== name) : [...raw.sources, name];
      applyFilters({ ...raw, sources: selected });
    });
    chips.set(name, chip);
    chipBox.appendChild(chip);
  };

  const buildRow = (entry) => {
    const display = { ...fallbackDisplay(entry), ...entry.display };
    const row = make('div', `zlv-row zlv-level-${entry.level}`);
    row.setAttribute('data-level', entry.level);
    row.setAttribute('data-source', entry.source);
    const fields = entry.fields && Object.keys(entry.fields).length > 0 ? entry.fields : null;
    const line = fields ? makeButton('zlv-line') : make('div', 'zlv-line');
    const levelText = String(display.level).trim();
    line.appendChild(make('span', 'zlv-time', display.time));
    line.appendChild(make('span', 'zlv-level', levelText));
    const badge = make('span', 'zlv-source', display.source);
    badge.setAttribute('data-kind', entry.sourceKind ?? 'core');
    line.appendChild(badge);
    line.appendChild(make('span', 'zlv-message', entry.message ?? ''));
    row.appendChild(line);

    if (fields) {
      row.setAttribute('data-expandable', 'true');
      const detailId = `zlv-detail-${++detailSerial}`;
      const name = [display.time, levelText, display.source, entry.message ?? ''].filter((part) => part !== '').join(' ');
      if (name !== '') line.setAttribute('aria-label', name);
      line.setAttribute('aria-expanded', 'false');
      line.setAttribute('aria-controls', detailId);
      let detail = null;
      line.addEventListener('click', () => {
        if (!detail) {
          detail = make('dl', 'zlv-fields');
          detail.setAttribute('id', detailId);
          for (const [key, value] of Object.entries(fields)) {
            detail.appendChild(make('dt', '', key));
            detail.appendChild(make('dd', '', fieldText(value)));
          }
          row.appendChild(detail);
        } else {
          detail.hidden = !detail.hidden;
        }
        const open = !detail.hidden;
        line.setAttribute('aria-expanded', open ? 'true' : 'false');
        if (!open) return;
        // Keep the opened fields on screen. A tail row is pinned to the bottom,
        // so the fields start below the fold, and the next line would cover
        // them while follow is still on.
        ignoreScroll = true;
        if (typeof detail.scrollIntoView === 'function') detail.scrollIntoView({ block: 'nearest' });
        setFollowing(false);
        const view = doc.defaultView;
        if (view && typeof view.requestAnimationFrame === 'function') {
          view.requestAnimationFrame(() => {
            view.requestAnimationFrame(() => { ignoreScroll = false; });
          });
        } else {
          ignoreScroll = false;
        }
      });
    }
    return row;
  };

  const evictOldest = () => {
    const oldest = items.shift();
    if (oldest.row) {
      const active = doc.activeElement;
      const focused = Boolean(active) && typeof oldest.row.contains === 'function' && oldest.row.contains(active);
      const neighbor = focused
        ? oldest.row.nextElementSibling?.querySelector?.('button.zlv-line')
          ?? oldest.row.previousElementSibling?.querySelector?.('button.zlv-line')
        : null;
      list.removeChild(oldest.row);
      shown -= 1;
      if (focused) {
        const target = neighbor ?? followButton;
        if (typeof target.focus === 'function') target.focus();
      }
    }
    dropped += 1;
  };

  const push = (entry) => {
    const stored = entry.seq === undefined ? { ...entry, seq: ++localSeq } : entry;
    addSource(stored.source);
    const item = { entry: stored, row: null };
    if (matchesFilter(stored, filter)) {
      item.row = buildRow(stored);
      list.appendChild(item.row);
      shown += 1;
      if (!following) pending += 1;
    }
    items.push(item);
    if (items.length > maxRows) evictOldest();
    updateCount();
    scrollToEnd();
  };

  const pushItems = (parsed) => {
    for (const value of parsed) push(normalizeEntry(value, normalizeOptions));
  };

  const rerender = () => {
    list.textContent = '';
    shown = 0;
    pending = 0;
    for (const item of items) {
      item.row = matchesFilter(item.entry, filter) ? buildRow(item.entry) : null;
      if (item.row) {
        list.appendChild(item.row);
        shown += 1;
      }
    }
    updateCount();
    scrollToEnd();
  };

  function applyFilters(next) {
    raw = { ...defaultFilters, ...next };
    for (const name of raw.sources) addSource(name);
    syncChips();
    const compiled = compileFilter(raw);
    filterError.textContent = compiled.error ? `Invalid pattern: ${compiled.error}` : '';
    search.setAttribute('aria-invalid', compiled.error ? 'true' : 'false');
    if (compiled.error) return;
    filter = compiled;
    rerender();
  }

  const syncControls = () => {
    search.value = raw.text;
    regexBox.checked = Boolean(raw.regex);
    kindSelect.value = raw.kind;
    levelSelect.value = raw.minLevel;
  };

  const exportNDJSON = () => {
    let text = '';
    for (const item of items) if (item.row) text += `${exportLine(item.entry)}\n`;
    return text;
  };

  const download = () => {
    const view = doc.defaultView ?? globalThis;
    const urlApi = options.url ?? view.URL ?? globalThis.URL;
    const BlobCtor = view.Blob ?? globalThis.Blob;
    if (!BlobCtor || !urlApi || typeof urlApi.createObjectURL !== 'function') return;
    const href = urlApi.createObjectURL(new BlobCtor([exportNDJSON()], { type: 'application/x-ndjson' }));
    const link = make('a');
    link.href = href;
    link.className = 'zlv-save-link';
    link.setAttribute('download', `logs-${new Date().toISOString().replace(/[:.]/g, '-')}.ndjson`);
    link.setAttribute('tabindex', '-1');
    link.setAttribute('aria-hidden', 'true');
    // Safari ignores a download on an anchor that is not in the document, and
    // revoking the blob URL in the same turn cancels the save. The link stays
    // out of the tab order and out of the flex layout.
    root.appendChild(link);
    if (typeof link.click === 'function') link.click();
    setTimeout(() => {
      if (link.parentNode === root) root.removeChild(link);
      urlApi.revokeObjectURL?.(href);
    }, 1500);
  };

  const onSearch = () => applyFilters({ ...raw, text: search.value, regex: regexBox.checked });
  const onKind = () => applyFilters({ ...raw, kind: kindSelect.value });
  const onLevel = () => applyFilters({ ...raw, minLevel: levelSelect.value });
  const onFollow = () => setFollowing(!following);
  const clear = () => {
    items = [];
    dropped = 0;
    shown = 0;
    pending = 0;
    list.textContent = '';
    updateCount();
  };
  search.addEventListener('input', onSearch);
  regexBox.addEventListener('change', onSearch);
  kindSelect.addEventListener('change', onKind);
  levelSelect.addEventListener('change', onLevel);
  followButton.addEventListener('click', onFollow);
  clearButton.addEventListener('click', clear);
  downloadButton.addEventListener('click', download);

  for (const name of options.sources ?? []) addSource(name);
  syncControls();
  updateFollowButton();
  syncChips();
  updateCount();

  return {
    push,
    pushLine(text) {
      pushItems(createNDJSONParser().push(`${text}\n`));
    },
    pushChunk(text) {
      pushItems(parser.push(text));
    },
    clear,
    setSources(names) {
      for (const name of names ?? []) addSource(name);
    },
    setFilters(next) {
      applyFilters(next ?? {});
      syncControls();
    },
    setFollow(value) {
      setFollowing(Boolean(value));
    },
    exportNDJSON,
    download,
    destroy() {
      list.removeEventListener('scroll', onScroll);
      search.removeEventListener('input', onSearch);
      regexBox.removeEventListener('change', onSearch);
      kindSelect.removeEventListener('change', onKind);
      levelSelect.removeEventListener('change', onLevel);
      followButton.removeEventListener('click', onFollow);
      clearButton.removeEventListener('click', clear);
      downloadButton.removeEventListener('click', download);
      container.removeChild(root);
      items = [];
    },
  };
}
