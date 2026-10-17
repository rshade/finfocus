// Plugins view (specs/629-web-plugin-page). The server plans installs and
// owns setting values. This module confirms those plans and never copies a
// secret value into the page.
import { el } from '../app.js';

function setHidden(node, hidden) {
  node.hidden = hidden;
  if (hidden) node.setAttribute('hidden', '');
  else node.removeAttribute('hidden');
}

function versionSlug(version) {
  const slug = String(version).replace(/[^A-Za-z0-9_-]+/g, '-').replace(/^-+|-+$/g, '');
  return slug || 'version';
}

function uniqueId(used, base) {
  let id = base;
  for (let n = 2; used.has(id); n += 1) id = `${base}-${n}`;
  used.add(id);
  return id;
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

function progressText(event) {
  const parts = [];
  if (event.phase) parts.push(String(event.phase));
  if (event.message) parts.push(String(event.message));
  const total = Number(event.total);
  if (Number.isFinite(total) && total > 0) parts.push(`${event.bytes ?? 0} / ${total} bytes`);
  if (event.warning) parts.push('warning');
  return parts.join(' ');
}

export function applyProgress(root, event) {
  const node = root?.querySelector?.('#plugin-progress');
  if (!node || event == null || typeof event !== 'object') return;
  if (event.status === 'failed') {
    node.textContent = event.error || 'The operation failed.';
    node.className = 'config-note';
    setHidden(node, false);
    return;
  }
  if (event.status === 'succeeded') {
    node.textContent = 'The operation finished.';
    node.className = 'config-note';
    setHidden(node, false);
    return;
  }
  if (Array.isArray(event.events)) {
    const last = event.events[event.events.length - 1];
    if (last) applyProgress(root, last);
    return;
  }
  const text = progressText(event);
  node.textContent = text;
  node.className = event.warning ? 'config-note detail-warnings' : 'config-note';
  setHidden(node, text === '');
}

function shortKey(name, key) {
  const prefix = `plugins.${name}.`;
  return typeof key === 'string' && key.startsWith(prefix) ? key.slice(prefix.length) : key;
}

function storageKey(name, key) {
  return `plugins.${name}.${shortKey(name, key)}`;
}

function tierSide(row, tier) {
  const side = tier === 'global' ? row?.global : row?.project;
  return { set: Boolean(side?.set), value: side?.value };
}

function statusText(field, tier) {
  const side = tierSide(field.row, tier);
  if (field.secret) return side.set ? 'set' : 'not set';
  if (!side.set) return 'not set';
  if (side.value === undefined || side.value === null) return 'not set';
  return String(side.value);
}

function describePlugin(plugin) {
  const parts = [];
  if (plugin.description) parts.push(plugin.description);
  if (plugin.providers?.length) parts.push(`Providers: ${plugin.providers.join(', ')}`);
  if (plugin.capabilities?.length) parts.push(`Capabilities: ${plugin.capabilities.join(', ')}`);
  if (plugin.repository) parts.push(`Repository: ${plugin.repository}`);
  return parts.join('. ');
}

function statusFlags(plugin, installed) {
  const flags = [];
  if (plugin.inCatalog !== true) flags.push('Outside catalog');
  if (plugin.updateAvailable) flags.push('Update available');
  if (installed.length === 0) flags.push('Not installed');
  else if (!plugin.updateAvailable) flags.push('Installed');
  return flags.join('. ');
}

function operationSummary(plan) {
  const lines = [
    `${plan.action} ${plan.name}`,
    `Version ${plan.version}`,
    `Repository ${plan.repository ?? ''}`,
    `Security level ${plan.securityLevel ?? ''}`,
  ];
  if (plan.checksumFile === 'missing') lines.push('This release has no checksum file.');
  if (plan.fallbackFrom) {
    lines.push(`Version ${plan.fallbackFrom} has no files. Confirm to use version ${plan.version} instead.`);
  }
  return lines.join(' ');
}

export function mount(container, { api }) {
  let alive = true;
  let plugins = [];
  let settingsView = null;
  let settingsTier = 'project';
  let pending = null;
  let holdPending = false;
  let operationSource = null;
  let catalogGeneration = 0;
  let settingsGeneration = 0;
  let planGeneration = 0;
  const usedIds = new Set();

  const message = el('p', { id: 'plugin-message', class: 'config-message', role: 'status' });
  const errorBanner = el('div', { id: 'plugin-error', class: 'error-banner', role: 'alert' });
  const progress = el('p', { id: 'plugin-progress', class: 'config-note', role: 'status' });
  const list = el('div', { id: 'plugins-list' });
  const settings = el('section', { id: 'plugin-settings' });
  const summary = el('div', { id: 'plugin-confirm-summary' });
  const title = el('h3', { id: 'plugin-confirm-title', text: 'Confirm' });
  const accept = el('button', { id: 'plugin-confirm-accept', class: 'dialog-submit', type: 'button', text: 'Confirm' });
  const cancel = el('button', { id: 'plugin-confirm-cancel', type: 'button', text: 'Cancel' });
  const dialog = el('dialog', {
    id: 'plugin-confirm-dialog',
    class: 'passphrase-dialog config-dialog',
    'aria-labelledby': 'plugin-confirm-title',
  },
  title,
  summary,
  el('div', { class: 'dialog-actions' }, cancel, accept));
  const root = el('section', { class: 'config-page' },
    el('h2', { text: 'Plugins' }),
    el('p', {
      class: 'config-note',
      text: 'Install, update, and remove catalog plugins after confirmation. Settings apply the next time that plugin is launched.',
    }),
    message,
    errorBanner,
    progress,
    list,
    settings,
    dialog);
  container.replaceChildren(root);
  setHidden(message, true);
  setHidden(errorBanner, true);
  setHidden(progress, true);
  setHidden(settings, true);
  accept.addEventListener('click', () => { void confirmPending(); });
  cancel.addEventListener('click', () => { if (dialog.open) dialog.close(); });
  dialog.addEventListener('close', () => { if (!holdPending) pending = null; });

  function setMessage(text) {
    message.textContent = text;
    setHidden(message, text === '');
  }

  function hideError() {
    setHidden(errorBanner, true);
    errorBanner.replaceChildren();
  }

  function showError(err) {
    setMessage('');
    setHidden(errorBanner, false);
    errorBanner.replaceChildren(...errorLines(err).map((text) => el('p', { text })));
  }

  function showDialog() {
    holdPending = true;
    if (dialog.open) dialog.close();
    holdPending = false;
    dialog.showModal();
  }

  function closeOperation() {
    if (!operationSource) return;
    operationSource.close();
    operationSource = null;
  }

  function listen(operationId) {
    closeOperation();
    const Source = globalThis.EventSource;
    if (typeof Source !== 'function') return;
    const source = new Source(`/api/plugins/operations/${encodeURIComponent(operationId)}/stream`);
    operationSource = source;
    let sawEvent = false;
    const onNamed = (ev) => {
      if (!alive || operationSource !== source) return;
      let data;
      try {
        data = JSON.parse(ev.data);
      } catch {
        return;
      }
      sawEvent = true;
      applyProgress(container, data);
      if (ev.type !== 'done') return;
      closeOperation();
      if (data.status === 'succeeded') loadCatalog(false);
    };
    source.addEventListener('snapshot', onNamed);
    source.addEventListener('progress', onNamed);
    source.addEventListener('done', onNamed);
    source.addEventListener('error', () => {
      if (operationSource !== source || source.readyState !== Source.CLOSED) return;
      closeOperation();
      if (!sawEvent) setMessage('Plugin progress is unavailable.');
    });
  }

  function loadCatalog(initial) {
    const generation = ++catalogGeneration;
    api.get('/api/plugins/catalog').then((data) => {
      if (!alive || generation !== catalogGeneration) return;
      plugins = Array.isArray(data?.plugins) ? data.plugins : [];
      renderList();
    }).catch((err) => {
      if (!alive || generation !== catalogGeneration) return;
      showError(err);
      if (initial) {
        list.replaceChildren(el('p', { class: 'view-loading', text: 'Plugins could not be loaded.' }));
      }
    });
  }

  function actionButton(id, label, run) {
    const button = el('button', { id, type: 'button', text: label });
    button.addEventListener('click', () => { void run(button); });
    return button;
  }

  function pluginRow(plugin) {
    const installed = Array.isArray(plugin.installedVersions) ? plugin.installedVersions : [];
    const inCatalog = plugin.inCatalog === true;
    const actions = [];
    if (inCatalog && installed.length === 0) {
      actions.push(actionButton(`plugin-install-${plugin.name}`, 'Install', (button) => (
        startPlan(plugin, 'install', plugin.latestVersion || '', button)
      )));
    }
    if (inCatalog && plugin.updateAvailable && installed.length > 0) {
      actions.push(actionButton(`plugin-update-${plugin.name}`, 'Update', (button) => (
        startPlan(plugin, 'update', plugin.latestVersion || '', button)
      )));
    }
    for (const version of installed) {
      const removeId = uniqueId(usedIds, `plugin-remove-${plugin.name}-${versionSlug(version)}`);
      actions.push(actionButton(removeId, `Remove ${version}`, (button) => (
        startPlan(plugin, 'remove', version, button)
      )));
    }
    if (installed.length > 0) {
      actions.push(actionButton(`plugin-validate-${plugin.name}`, 'Validate', (button) => validatePlugin(plugin, button)));
    }
    if (installed.length > 0) {
      actions.push(actionButton(`plugin-configure-${plugin.name}`, 'Settings', (button) => openSettings(plugin, button)));
    }
    return el('tr', {},
      el('th', { scope: 'row', text: plugin.name }),
      el('td', { text: describePlugin(plugin) }),
      el('td', { text: plugin.securityLevel || '' }),
      el('td', { text: installed.length ? installed.join(', ') : 'Not installed' }),
      el('td', { text: plugin.latestVersion || '' }),
      el('td', {},
        el('p', { text: statusFlags(plugin, installed) }),
        plugin.warning ? el('p', { class: 'detail-warnings', text: plugin.warning }) : null,
      ),
      el('td', { class: 'plugin-actions' }, ...actions),
    );
  }

  function renderList() {
    usedIds.clear();
    list.replaceChildren(el('div', { class: 'table-wrap' },
      el('table', { class: 'config-table' },
        el('caption', { text: 'Catalog and installed plugins' }),
        el('thead', {}, el('tr', {},
          el('th', { text: 'Plugin' }),
          el('th', { text: 'Description' }),
          el('th', { text: 'Security level' }),
          el('th', { text: 'Installed' }),
          el('th', { text: 'Latest' }),
          el('th', { text: 'Status' }),
          el('th', { text: 'Actions' }),
        )),
        el('tbody', {}, ...plugins.map(pluginRow)),
      )));
    setHidden(list, settingsView != null);
  }

  async function startPlan(plugin, action, version, button) {
    const generation = ++planGeneration;
    button.disabled = true;
    hideError();
    try {
      const body = { action };
      if (version) body.version = version;
      const plan = await api.post(`/api/plugins/${encodeURIComponent(plugin.name)}/plan`, body);
      if (!alive || generation !== planGeneration) return;
      pending = { kind: 'operation', plan, action: plan?.action || action };
      title.textContent = `Confirm ${pending.action}`;
      summary.textContent = operationSummary(plan);
      showDialog();
    } catch (err) {
      if (!alive) return;
      showError(err);
    } finally {
      if (alive && button.isConnected) button.disabled = false;
    }
  }

  async function validatePlugin(plugin, button) {
    button.disabled = true;
    hideError();
    try {
      const result = await api.post(`/api/plugins/${encodeURIComponent(plugin.name)}/validate`, {});
      if (!alive) return;
      if (result?.passed) setMessage('Validation passed.');
      else setMessage(result?.reason || 'Validation failed.');
    } catch (err) {
      if (!alive) return;
      showError(err);
    } finally {
      if (alive && button.isConnected) button.disabled = false;
    }
  }

  function fields() {
    if (!settingsView) return [];
    const name = settingsView.name;
    if (settingsView.mode === 'generic') {
      return (settingsView.values ?? []).map((row) => ({
        key: shortKey(name, row.key),
        type: 'string',
        secret: Boolean(row.secret),
        required: false,
        description: '',
        env: '',
        enumValues: [],
        row,
      }));
    }
    const byKey = new Map((settingsView.values ?? []).map((row) => [shortKey(name, row.key), row]));
    return (settingsView.descriptors ?? []).map((descriptor) => {
      const key = shortKey(name, descriptor.key);
      const row = byKey.get(key);
      return {
        key,
        type: descriptor.type || 'string',
        secret: Boolean(descriptor.secret || row?.secret),
        required: Boolean(descriptor.required),
        description: descriptor.description || '',
        env: descriptor.env || '',
        enumValues: descriptor.enumValues ?? [],
        row,
      };
    });
  }

  function controlFor(field) {
    const id = `plugin-setting-${field.key}`;
    const side = tierSide(field.row, settingsTier);
    const initial = field.secret || !side.set ? '' : String(side.value ?? '');
    if (!field.secret && (field.type === 'bool' || field.type === 'enum')) {
      const select = el('select', { id });
      select.append(el('option', { value: '', text: side.set ? 'Remove from tier' : 'Not set' }));
      const options = field.type === 'bool' ? ['true', 'false'] : field.enumValues;
      for (const option of options) select.append(el('option', { value: option, text: option }));
      select.value = initial;
      return select;
    }
    const attrs = {
      id,
      type: field.secret ? 'password' : 'text',
      value: field.secret ? '' : initial,
      autocomplete: 'off',
      spellcheck: 'false',
    };
    if (field.type === 'int') attrs.inputmode = 'decimal';
    if (field.secret) attrs.placeholder = side.set ? 'Replace secret' : 'Not set';
    return el('input', attrs);
  }

  function clearControl(field) {
    const side = tierSide(field.row, settingsTier);
    const rendersAsSelect = !field.secret && (field.type === 'bool' || field.type === 'enum');
    if (!side.set || rendersAsSelect) return null;
    const id = `plugin-clear-${field.key}`;
    const box = el('input', { id, type: 'checkbox' });
    return el('label', { class: 'config-clear', for: id }, box, 'Remove');
  }

  function changeFor(field, input, clear) {
    const side = tierSide(field.row, settingsTier);
    const key = storageKey(settingsView.name, field.key);
    if (clear?.checked) return side.set ? { key, clear: true } : null;
    if (field.secret) {
      const next = String(input?.value ?? '').trim();
      if (next === '' || next === '[REDACTED]') return null;
      return { key, value: input.value };
    }
    const current = String(input?.value ?? '');
    const initial = side.set ? String(side.value ?? '') : '';
    if (current === initial) return null;
    if (current === '') return side.set ? { key, clear: true } : null;
    return { key, value: current };
  }

  const controls = new Map();

  function collectChanges() {
    const changes = [];
    for (const field of fields()) {
      const held = controls.get(field.key);
      if (!held?.input) continue;
      const change = changeFor(field, held.input, held.clear);
      if (change) changes.push(change);
    }
    return changes;
  }

  function fieldHelp(field) {
    const parts = [];
    if (field.description) parts.push(field.description);
    if (field.env) parts.push(`Environment variable ${field.env}.`);
    if (field.required) parts.push('Required.');
    if (field.row?.envSet) parts.push('An environment value is set and overrides the file.');
    return parts.join(' ');
  }

  function fieldRow(field) {
    const input = controlFor(field);
    const clear = clearControl(field);
    controls.set(field.key, { input, clear: clear?.querySelector('input') ?? null });
    const helpID = `plugin-help-${field.key}`;
    input.setAttribute('aria-describedby', helpID);
    return el('tr', {},
      el('th', { scope: 'row' },
        el('label', { for: input.id, text: field.key }),
        el('p', { id: helpID, class: 'config-help', text: fieldHelp(field) }),
      ),
      el('td', { text: statusText(field, settingsTier) }),
      el('td', { class: 'config-edit' }, input, clear),
    );
  }

  function renderSettings() {
    controls.clear();
    if (!settingsView) {
      settings.replaceChildren();
      setHidden(settings, true);
      setHidden(list, false);
      return;
    }
    if (!settingsView.projectAvailable && settingsTier === 'project') settingsTier = 'global';
    const tierSelect = el('select', { id: 'plugin-tier' });
    const projectOption = el('option', { value: 'project', text: 'Project' });
    if (!settingsView.projectAvailable) {
      projectOption.disabled = true;
      projectOption.setAttribute('disabled', '');
    }
    tierSelect.append(projectOption, el('option', { value: 'global', text: 'Global' }));
    tierSelect.value = settingsTier;
    tierSelect.addEventListener('change', () => {
      settingsTier = tierSelect.value === 'global' ? 'global' : 'project';
      setMessage('');
      renderSettings();
    });
    const save = el('button', { id: 'plugin-save', type: 'button', class: 'dialog-submit', text: 'Review changes' });
    save.addEventListener('click', () => { void reviewSettings(save); });
    const back = el('button', { id: 'plugin-settings-back', type: 'button', text: 'Back' });
    back.addEventListener('click', () => {
      settingsView = null;
      setMessage('');
      renderSettings();
    });
    const unavailable = settingsView.projectAvailable ? null : el('p', {
      class: 'config-note',
      text: 'no project directory is available',
    });
    const modeNote = settingsView.mode === 'generic'
      ? 'This plugin does not describe its settings. Each stored name is a text value. Credential names stay write-only.'
      : 'Choose a tier, review the diff, then confirm. A save does not change a plugin that is already running.';
    settings.replaceChildren(...[
      el('h3', { text: `Settings for ${settingsView.name}` }),
      el('p', { class: 'config-note', text: modeNote }),
      unavailable,
      el('div', { class: 'toolbar' },
        el('label', { class: 'filter-label', text: 'Edit tier' }, tierSelect),
        save,
        back,
      ),
      el('div', { class: 'table-wrap' },
        el('table', { class: 'config-table' },
          el('caption', { text: settingsView.mode === 'generic' ? 'Stored settings' : 'Plugin settings' }),
          el('thead', {}, el('tr', {},
            el('th', { text: 'Setting' }),
            el('th', { text: 'Status' }),
            el('th', { text: 'Edit' }),
          )),
          el('tbody', {}, ...fields().map(fieldRow)),
        ),
      ),
    ].filter((node) => node != null));
    setHidden(settings, false);
    setHidden(list, true);
  }

  async function openSettings(plugin, button) {
    const generation = ++settingsGeneration;
    button.disabled = true;
    hideError();
    try {
      const view = await api.get(`/api/plugins/${encodeURIComponent(plugin.name)}/settings`);
      if (!alive || generation !== settingsGeneration) return;
      settingsView = view;
      settingsTier = view?.projectAvailable ? 'project' : 'global';
      setMessage('');
      renderSettings();
    } catch (err) {
      if (!alive) return;
      showError(err);
    } finally {
      if (alive && button.isConnected) button.disabled = false;
    }
  }

  function fillSettingsSummary(preview) {
    const rows = (preview.diff ?? []).map((entry) => el('tr', {},
      el('td', { text: entry.key }),
      el('td', { text: entry.before || 'not set' }),
      el('td', { text: entry.after ?? '' }),
    ));
    summary.replaceChildren(
      el('p', { text: `Confirm writes these values to the ${pending.body.tier} configuration file. Settings apply the next time this plugin is launched.` }),
      el('table', { class: 'config-diff' },
        el('thead', {}, el('tr', {},
          el('th', { text: 'Setting' }),
          el('th', { text: 'Before' }),
          el('th', { text: 'After' }),
        )),
        el('tbody', {}, ...rows),
      ),
    );
  }

  async function reviewSettings(button) {
    const changes = collectChanges();
    if (changes.length === 0) {
      setMessage('No changes for this tier.');
      return;
    }
    button.disabled = true;
    hideError();
    try {
      const body = {
        tier: settingsTier,
        baseVersion: settingsView.versions?.[settingsTier] ?? 'absent',
        changes,
      };
      const preview = await api.post(`/api/plugins/${encodeURIComponent(settingsView.name)}/settings/preview`, body);
      if (!alive) return;
      if (!preview?.diff?.length) {
        setMessage('No changes for this tier.');
        return;
      }
      pending = { kind: 'settings', name: settingsView.name, body };
      title.textContent = 'Review changes';
      fillSettingsSummary(preview);
      showDialog();
    } catch (err) {
      if (!alive) return;
      showError(err);
    } finally {
      if (alive && button.isConnected) button.disabled = false;
    }
  }

  async function confirmPending() {
    const current = pending;
    if (!current || !alive) return;
    accept.disabled = true;
    try {
      if (current.kind === 'settings') {
        const saved = await api.post(`/api/plugins/${encodeURIComponent(current.name)}/settings`, current.body);
        if (!alive) return;
        settingsView = saved;
        setMessage(saved?.message || '');
        pending = null;
        if (dialog.open) dialog.close();
        renderSettings();
        return;
      }
      const plan = current.plan;
      const name = encodeURIComponent(plan.name);
      let accepted;
      if (current.action === 'remove') {
        accepted = await api.del(`/api/plugins/${name}/${encodeURIComponent(plan.version)}`, {
          repository: plan.repository ?? '',
          securityLevel: plan.securityLevel ?? '',
          acceptMissingChecksum: false,
          acceptFallbackFrom: '',
        });
      } else {
        const action = current.action === 'update' ? 'update' : 'install';
        accepted = await api.post(`/api/plugins/${name}/${action}`, {
          version: plan.version,
          repository: plan.repository ?? '',
          securityLevel: plan.securityLevel ?? '',
          acceptMissingChecksum: plan.checksumFile === 'missing',
          acceptFallbackFrom: plan.fallbackFrom ?? '',
        });
      }
      if (!alive) return;
      pending = null;
      if (dialog.open) dialog.close();
      if (accepted?.operationId) {
        applyProgress(container, { phase: 'resolve', message: 'Operation started.' });
        listen(accepted.operationId);
      } else {
        setMessage('The operation did not return an id.');
      }
    } catch (err) {
      if (!alive) return;
      showError(err);
    } finally {
      if (alive) accept.disabled = false;
    }
  }

  list.replaceChildren(el('p', { class: 'view-loading', role: 'status', text: 'Loading plugins…' }));
  loadCatalog(true);

  return {
    unmount() {
      alive = false;
      closeOperation();
    },
  };
}
