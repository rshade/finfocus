// Config view (specs/628-web-config-page). The server computes every value.
// This module collects edits and renders the preview. It never reads a file
// and never fills a secret input from the response.
import { el } from '../app.js';

const TIERS = ['default', 'global', 'project', 'env'];

function slug(key) {
  return String(key).replace(/[^A-Za-z0-9_-]+/g, '-');
}

function derivedIDs(token) {
  return {
    field: `config-${token}`,
    help: `config-${token}-help`,
    clear: `config-clear-${token}`,
  };
}

// Keys that slug alike (prod/west and prod-west) must still get distinct
// control IDs, and so must a key that slugs onto another control's help or
// clear ID. Rows keep the readable slug unless it is taken.
function assignIDs(settings) {
  const used = new Set();
  const ids = new Map();
  for (const { key } of settings) {
    const base = slug(key);
    let token = base;
    for (let n = 2; Object.values(derivedIDs(token)).some((id) => used.has(id)); n += 1) {
      token = `${base}-${n}`;
    }
    const assigned = derivedIDs(token);
    Object.values(assigned).forEach((id) => used.add(id));
    ids.set(key, assigned);
  }
  return ids;
}

function setHidden(node, hidden) {
  node.hidden = hidden;
  if (hidden) node.setAttribute('hidden', '');
  else node.removeAttribute('hidden');
}

function formatValue(value) {
  if (Array.isArray(value)) return value.join(', ');
  if (typeof value === 'boolean') return value ? 'true' : 'false';
  if (value === undefined || value === null) return '';
  return String(value);
}

function displayValue(value) {
  if (value === undefined || value === null) return 'not set';
  if (Array.isArray(value)) return value.length ? value.join(', ') : '[]';
  return formatValue(value);
}

function tierEntry(setting, name) {
  return setting.tiers?.[name];
}

function tierLabel(setting, name) {
  const entry = tierEntry(setting, name);
  if (!entry?.set) return 'not set';
  if (setting.secret) return 'set';
  return displayValue(entry.value);
}

function effectiveLabel(setting) {
  const source = setting.effective?.source ?? 'default';
  if (setting.secret) return tierEntry(setting, source)?.set ? 'set' : 'not set';
  if (!Object.hasOwn(setting.effective ?? {}, 'value')) return 'not set';
  return displayValue(setting.effective.value);
}

function readOnlyLabel(row) {
  if (row.secret) return row.set ? 'set' : 'not set';
  if (!row.set) return 'not set';
  return displayValue(row.value);
}

function editorInitial(setting, tier) {
  const entry = tierEntry(setting, tier);
  if (!entry?.set || setting.secret) return '';
  if (setting.kind === 'bool') return formatValue(entry.value);
  return formatValue(entry.value);
}

function editorControl(setting, tier, id) {
  const entry = tierEntry(setting, tier);
  const initial = editorInitial(setting, tier);
  if (setting.kind === 'enum' || setting.kind === 'bool') {
    const select = el('select', { id });
    const blank = !entry?.set || setting.kind === 'bool';
    if (blank) {
      const label = entry?.set ? 'Remove from tier' : 'Not set';
      select.append(el('option', { value: '', text: label }));
    }
    const options = setting.kind === 'bool' ? ['true', 'false'] : (setting.enum ?? []);
    for (const option of options) select.append(el('option', { value: option, text: option }));
    select.value = initial;
    return select;
  }
  const attrs = {
    id,
    type: setting.secret ? 'password' : 'text',
    value: setting.secret ? '' : initial,
    autocomplete: setting.secret ? 'off' : 'off',
    spellcheck: 'false',
  };
  if (setting.kind === 'int' || setting.kind === 'number') attrs.inputmode = 'decimal';
  if (setting.secret) {
    attrs.placeholder = entry?.set ? 'Leave blank to keep the current secret' : 'Not set';
  }
  return el('input', attrs);
}

function clearControl(setting, tier, id) {
  const entry = tierEntry(setting, tier);
  if (!entry?.set || setting.kind === 'bool') return null;
  const box = el('input', { id, type: 'checkbox' });
  return el('label', { class: 'config-clear', for: id }, box, 'Remove');
}

function controlValue(input) {
  return input.value ?? '';
}

function changeFor(setting, tier, input, clear) {
  const entry = tierEntry(setting, tier);
  if (clear?.checked) return entry?.set ? { key: setting.key, clear: true } : null;
  if (setting.secret) {
    const next = controlValue(input).trim();
    if (next === '' || next === '[REDACTED]') return null;
    return { key: setting.key, value: controlValue(input) };
  }
  const current = controlValue(input);
  const initial = editorInitial(setting, tier);
  if (current === initial) return null;
  if (current === '') {
    if (!entry?.set) return null;
    if (setting.kind === 'enum' || setting.kind === 'bool') return { key: setting.key, clear: true };
  }
  return { key: setting.key, value: current };
}

function collectChanges(root, page, tier, ids) {
  const changes = [];
  for (const setting of page.settings ?? []) {
    const assigned = ids.get(setting.key);
    const input = assigned && root.querySelector(`#${assigned.field}`);
    if (!input) continue;
    const change = changeFor(setting, tier, input, root.querySelector(`#${assigned.clear}`));
    if (change) changes.push(change);
  }
  return changes;
}

function errorLines(err) {
  const lines = [err?.message || 'Request failed.'];
  for (const field of err?.fields ?? []) {
    const where = field.path || field.key || '';
    const detail = field.message || '';
    if (where && detail) lines.push(`${where}: ${detail}`);
    else if (detail) lines.push(detail);
  }
  return lines;
}

export function mount(container, { api }) {
  const loading = api.get('/api/config');
  let alive = true;
  let page = null;
  let tier = 'project';
  let message = '';
  let loadError = '';
  let pending = null;
  let ids = new Map();
  let dialog = null;
  const root = el('section', { class: 'config-page' });
  container.replaceChildren(root);

  function showError(err) {
    const banner = root.querySelector('#config-error');
    if (!banner) return;
    setMessage('');
    setHidden(banner, false);
    banner.replaceChildren(...errorLines(err).map((text) => el('p', { text })));
  }

  function setMessage(text) {
    message = text;
    const node = root.querySelector('#config-message');
    if (!node) return;
    node.textContent = text;
    setHidden(node, text === '');
  }

  function fillDialog(preview) {
    const cancel = el('button', { id: 'config-cancel', type: 'button', text: 'Cancel' });
    const confirm = el('button', { id: 'config-confirm', class: 'dialog-submit', type: 'button', text: 'Confirm' });
    cancel.addEventListener('click', () => dialog.close());
    confirm.addEventListener('click', () => { void commit(confirm); });
    const rows = (preview.diff ?? []).map((entry) => el('tr', {},
      el('td', { text: entry.key }),
      el('td', { text: entry.before || 'not set' }),
      el('td', { text: entry.after }),
    ));
    dialog.replaceChildren(
      el('h3', { id: 'config-preview-title', text: 'Review changes' }),
      el('p', { text: `Confirm writes these values to the ${pending.tier} configuration file. Cancel writes nothing.` }),
      el('table', { class: 'config-diff' },
        el('thead', {}, el('tr', {},
          el('th', { text: 'Setting' }),
          el('th', { text: 'Before' }),
          el('th', { text: 'After' }),
        )),
        el('tbody', {}, ...rows),
      ),
      el('div', { class: 'dialog-actions' }, cancel, confirm),
    );
  }

  async function commit(button) {
    button.disabled = true;
    try {
      const saved = await api.post('/api/config', pending);
      if (!alive) return;
      page = saved;
      message = saved.message || '';
      pending = null;
      if (dialog?.open) dialog.close();
      render();
    } catch (err) {
      if (!alive) return;
      button.disabled = false;
      showError(err);
      if (dialog?.open) dialog.close();
    }
  }

  async function preview(button) {
    const changes = collectChanges(root, page, tier, ids);
    if (changes.length === 0) {
      setMessage('No changes for this tier.');
      return;
    }
    button.disabled = true;
    const banner = root.querySelector('#config-error');
    if (banner) setHidden(banner, true);
    try {
      const body = { tier, baseVersion: page.versions?.[tier] ?? 'absent', changes };
      const result = await api.post('/api/config/preview', body);
      if (!alive) return;
      if (!result?.diff?.length) {
        setMessage('No changes for this tier.');
        return;
      }
      pending = body;
      fillDialog(result);
      if (dialog.open) dialog.close();
      dialog.showModal();
    } catch (err) {
      if (!alive) return;
      showError(err);
    } finally {
      if (alive && button.isConnected) button.disabled = false;
    }
  }

  async function reload() {
    try {
      const data = await api.get('/api/config');
      if (!alive) return;
      page = data;
      message = '';
      if (!page.projectAvailable && tier === 'project') tier = 'global';
      render();
    } catch (err) {
      if (!alive) return;
      showError(err);
    }
  }

  function settingRow(setting) {
    const assigned = ids.get(setting.key);
    const control = editorControl(setting, tier, assigned.field);
    const remove = clearControl(setting, tier, assigned.clear);
    const helpID = assigned.help;
    control.setAttribute('aria-describedby', helpID);
    return el('tr', {},
      el('th', { scope: 'row' },
        el('label', { for: assigned.field, text: setting.key }),
        el('p', { id: helpID, class: 'config-help', text: setting.help || '' }),
      ),
      el('td', { text: effectiveLabel(setting) }),
      el('td', { text: setting.effective?.source || 'default' }),
      ...TIERS.map((name) => el('td', { text: tierLabel(setting, name) })),
      el('td', { class: 'config-edit' }, control, remove),
    );
  }

  function readOnlyRow(row) {
    return el('tr', {},
      el('th', { scope: 'row', text: row.key }),
      el('td', { text: readOnlyLabel(row) }),
      el('td', { text: row.source || '' }),
    );
  }

  function render() {
    if (!page) {
      root.replaceChildren(
        el('h2', { text: 'Config' }),
        el('p', { class: 'view-loading', role: 'status', text: loadError || 'Loading configuration…' }),
      );
      dialog = null;
      return;
    }
    ids = assignIDs(page.settings ?? []);
    const tierSelect = el('select', { id: 'config-tier' });
    const projectOption = el('option', { value: 'project', text: 'Project' });
    if (!page.projectAvailable) {
      projectOption.disabled = true;
      projectOption.setAttribute('disabled', '');
    }
    tierSelect.append(projectOption, el('option', { value: 'global', text: 'Global' }));
    tierSelect.value = tier;
    tierSelect.addEventListener('change', () => {
      tier = tierSelect.value === 'global' ? 'global' : 'project';
      message = '';
      render();
    });
    const save = el('button', { id: 'config-save', type: 'button', class: 'dialog-submit', text: 'Review changes' });
    save.addEventListener('click', () => { void preview(save); });
    const reloadButton = el('button', { id: 'config-reload', type: 'button', text: 'Reload' });
    reloadButton.addEventListener('click', () => { void reload(); });
    const banner = el('div', { id: 'config-error', class: 'error-banner', role: 'alert' });
    setHidden(banner, true);
    const status = el('p', { id: 'config-message', class: 'config-message', role: 'status', text: message });
    setHidden(status, message === '');
    dialog = el('dialog', { id: 'config-preview-dialog', class: 'passphrase-dialog config-dialog', 'aria-labelledby': 'config-preview-title' });
    const unavailable = page.projectAvailable ? null : el('p', {
      id: 'config-project-unavailable',
      class: 'config-note',
      text: page.projectUnavailableReason || 'no project directory is available',
    });
    const nodes = [
      el('h2', { text: 'Config' }),
      el('p', { class: 'config-note', text: 'Choose a tier, review the diff, then confirm. Environment values still override the file. A save does not reload this session.' }),
      unavailable,
      el('div', { class: 'toolbar' },
        el('label', { class: 'filter-label', text: 'Edit tier' }, tierSelect),
        save,
        reloadButton,
      ),
      status,
      banner,
      el('div', { class: 'table-wrap' },
        el('table', { class: 'config-table' },
          el('caption', { text: 'Known settings' }),
          el('thead', {}, el('tr', {},
            el('th', { text: 'Setting' }),
            el('th', { text: 'Effective' }),
            el('th', { text: 'Source' }),
            el('th', { text: 'Default' }),
            el('th', { text: 'Global' }),
            el('th', { text: 'Project' }),
            el('th', { text: 'Environment' }),
            el('th', { text: 'Edit' }),
          )),
          el('tbody', {}, ...(page.settings ?? []).map(settingRow)),
        ),
      ),
      el('h3', { text: 'Read-only' }),
      el('p', { class: 'config-note', text: 'Alerts, routing, and analyzer settings are shown here and are not editable.' }),
      el('div', { class: 'table-wrap' },
        el('table', { class: 'config-table' },
          el('caption', { text: 'Read-only settings' }),
          el('thead', {}, el('tr', {},
            el('th', { text: 'Setting' }),
            el('th', { text: 'Status' }),
            el('th', { text: 'Source' }),
          )),
          el('tbody', {}, ...(page.readOnly ?? []).map(readOnlyRow)),
        ),
      ),
      dialog,
    ];
    root.replaceChildren(...nodes.filter((node) => node != null));
  }

  render();
  loading.then((data) => {
    if (!alive) return;
    page = data;
    tier = page.projectAvailable ? 'project' : 'global';
    render();
  }).catch((err) => {
    if (!alive) return;
    loadError = err?.message || 'Configuration could not be loaded.';
    render();
  });

  return { unmount() { alive = false; } };
}
