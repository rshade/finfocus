import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

class Node {
  constructor(tag, doc) { this.tagName=tag; this.doc=doc; this.children=[]; this.listeners=new Map(); this.attrs={}; this.dataset={}; this.hidden=false; this.value=''; }
  append(...children) { for (const child of children) { if (typeof child !== 'string') child.parent=this; this.children.push(child); } }
  replaceChildren(...children) { for(const child of this.children)if(typeof child!=='string')child.parent=null; this.children=[]; this.append(...children); }
  set textContent(text) { this.text=String(text); this.children=[]; }
  get textContent() { return (this.text??'')+this.children.map(c=>typeof c==='string'?c:c.textContent).join(''); }
  setAttribute(k,v) { this.attrs[k]=String(v); if(k==='id')this.id=v; if(k==='value')this.value=String(v); if(k==='class')this.className=v; }
  removeAttribute(k) { delete this.attrs[k]; }
  addEventListener(k,fn) { this.listeners.set(k,fn); }
  dispatch(k) { this.listeners.get(k)?.({target:this,preventDefault(){}}); }
  focus() { this.doc.activeElement=this; }
  showModal() { this.open=true; }
  close() { this.open=false; this.dispatch('close'); }
  get isConnected() { return this===this.doc || Boolean(this.parent?.isConnected); }
  querySelector(selector) { return this.querySelectorAll(selector)[0]; }
  querySelectorAll(selector) { const match=n=>selector[0]==='#'?n.id===selector.slice(1):selector[0]==='.'?n.className?.split(' ').includes(selector.slice(1)):n.tagName===selector; return this.children.filter(c=>typeof c!=='string').flatMap(c=>[...(match(c)?[c]:[]),...c.querySelectorAll(selector)]); }
}
const flush=()=>new Promise(resolve=>setImmediate(resolve));
async function setup(view) {
 const document=new Node('document'); document.doc=document; document.createElement=tag=>new Node(tag,document); document.createElementNS=(_ns,tag)=>new Node(tag,document);
 const context=vm.createContext({document,AbortController,console,setTimeout,clearTimeout,Object});
 const app=new vm.SyntheticModule(['el'],function(){this.setExport('el',(tag,attrs={},...children)=>{const node=document.createElement(tag);for(const [k,v]of Object.entries(attrs)){if(v===undefined||v===null)continue;if(k==='text')node.textContent=v;else if(k==='class')node.className=v;else if(k==='dataset')Object.assign(node.dataset,v);else node.setAttribute(k,v);}node.append(...children.flat().filter(c=>c!=null));return node;});},{context});await app.link(()=>{});await app.evaluate();
 const mod=new vm.SourceTextModule(await readFile(new URL(`../static/views/${view}.js`,import.meta.url),'utf8'),{context});await mod.link(()=>app);await mod.evaluate();
 const calls=[];const api={post(path,body,options){return request(path,body,options);},get(path,options){return request(path,null,options);},del(path,body,options){return request(path,body,options);}};
 function request(path,body,options){let resolve,reject;const promise=new Promise((yes,no)=>{resolve=yes;reject=no;});calls.push({path,body,options,resolve,reject});return promise;}
 const container=document.createElement('div');document.append(container);const mounted=mod.namespace.mount(container,{api,stream:{on(){return()=>{}}},el:app.namespace.el,navigate(){}});
 return{container,calls,mounted,document,applyProgress:mod.namespace.applyProgress};
}

function catalog() {
  return {
    plugins: [
      {
        name: 'recorder',
        description: 'Records plugin requests',
        providers: ['aws'],
        capabilities: ['projected'],
        securityLevel: 'community',
        repository: 'rshade/finfocus-plugin-recorder',
        inCatalog: true,
        installedVersions: [],
        latestVersion: '1.2.3',
        updateAvailable: false,
      },
      {
        name: 'aws',
        description: 'AWS public pricing',
        providers: ['aws'],
        capabilities: ['projected', 'actual'],
        securityLevel: 'official',
        repository: 'rshade/finfocus-plugin-aws-public',
        inCatalog: true,
        installedVersions: ['1.0.0'],
        latestVersion: '1.4.0',
        updateAvailable: true,
      },
      {
        name: 'jev',
        description: 'Deprecated. Use scorer',
        providers: [],
        capabilities: ['recommendation_scoring'],
        securityLevel: 'official',
        repository: 'rshade/finfocus',
        inCatalog: true,
        installedVersions: [],
        latestVersion: '0.1.4',
        updateAvailable: false,
        deprecated: true,
        replacement: 'scorer',
      },
      {
        name: 'local',
        description: '',
        providers: [],
        capabilities: [],
        securityLevel: '',
        repository: '',
        inCatalog: false,
        installedVersions: ['0.4.0'],
        latestVersion: '',
        updateAvailable: false,
      },
    ],
  };
}

function descriptorSettings() {
  return {
    name: 'aws',
    mode: 'descriptor',
    projectAvailable: true,
    versions: { global: 'g1', project: 'p1' },
    descriptors: [
      { key: 'api_key', type: 'string', secret: true, required: false, description: 'API key', env: 'RECORDER_API_KEY' },
      { key: 'base_url', type: 'url', secret: false, required: true, description: 'Base URL' },
      { key: 'timeout', type: 'int', secret: false, description: 'Timeout' },
      { key: 'enabled', type: 'bool', secret: false, description: 'Enabled' },
      { key: 'mock_response', type: 'string', secret: false, description: 'Mock body' },
    ],
    values: [
      { key: 'api_key', secret: true, project: { set: true }, global: { set: false }, envSet: false, effectiveSource: 'project' },
      { key: 'base_url', secret: false, project: { set: true, value: 'https://old.example' }, global: { set: false }, effectiveSource: 'project' },
      { key: 'timeout', secret: false, project: { set: true, value: '30' }, global: { set: false }, effectiveSource: 'project' },
      { key: 'enabled', secret: false, project: { set: true, value: 'true' }, global: { set: false }, effectiveSource: 'project' },
      { key: 'mock_response', secret: false, project: { set: false }, global: { set: false }, effectiveSource: '' },
    ],
  };
}

function plan(overrides = {}) {
  return {
    action: 'install',
    name: 'recorder',
    version: '1.2.3',
    repository: 'rshade/finfocus-plugin-recorder',
    securityLevel: 'community',
    fallbackFrom: '',
    checksumFile: 'present',
    inCatalog: true,
    ...overrides,
  };
}

function body(call) {
  return JSON.parse(JSON.stringify(call.body));
}

async function open() {
  const t = await setup('plugins');
  assert.equal(t.calls[0].path, '/api/plugins/catalog');
  t.calls[0].resolve(catalog());
  await flush();
  return t;
}

test('confirm dialog names the plan before install is posted', async () => {
  const t = await open();
  assert.equal(t.container.querySelector('h2').textContent, 'Plugins');
  assert.match(t.container.textContent, /Update available/);
  assert.match(t.container.textContent, /Outside catalog/);
  assert.match(t.container.textContent, /Deprecated\. Use scorer\. /);
  assert.doesNotMatch(t.container.textContent, /scorer\.\./);
  assert.match(t.container.textContent, /1\.0\.0/);
  assert.match(t.container.textContent, /community/);
  assert.equal(t.container.querySelector('#plugin-install-local'), undefined);
  assert.equal(t.container.querySelector('#plugin-update-local'), undefined);
  assert.ok(t.container.querySelector('#plugin-validate-local'));
  assert.ok(t.container.querySelector('#plugin-configure-local'));
  assert.equal(t.container.querySelector('#plugin-configure-recorder'), undefined);
  assert.ok(t.container.querySelector('#plugin-configure-aws'));
  assert.ok(t.container.querySelector('#plugin-remove-local-0-4-0'));
  assert.ok(t.container.querySelector('#plugin-update-aws'));

  t.container.querySelector('#plugin-install-recorder').dispatch('click');
  assert.equal(t.calls.at(-1).path, '/api/plugins/recorder/plan');
  assert.deepEqual(body(t.calls.at(-1)), { action: 'install', version: '1.2.3' });
  assert.equal(t.calls.some((call) => call.path === '/api/plugins/recorder/install'), false);
  t.calls.at(-1).resolve(plan());
  await flush();
  const dialog = t.container.querySelector('#plugin-confirm-dialog');
  assert.equal(dialog.open, true);
  const summary = t.container.querySelector('#plugin-confirm-summary').textContent;
  assert.match(summary, /recorder/);
  assert.match(summary, /1\.2\.3/);
  assert.match(summary, /rshade\/finfocus-plugin-recorder/);
  assert.match(summary, /community/);
  assert.equal(t.calls.some((call) => call.path === '/api/plugins/recorder/install'), false);

  t.container.querySelector('#plugin-confirm-accept').dispatch('click');
  assert.equal(t.calls.at(-1).path, '/api/plugins/recorder/install');
  assert.deepEqual(body(t.calls.at(-1)), {
    version: '1.2.3',
    repository: 'rshade/finfocus-plugin-recorder',
    securityLevel: 'community',
    acceptMissingChecksum: false,
    acceptFallbackFrom: '',
  });
  t.mounted.unmount();
});

test('a missing checksum and a version fallback are named before confirm', async () => {
  const t = await open();
  t.container.querySelector('#plugin-install-recorder').dispatch('click');
  t.calls.at(-1).resolve(plan({ checksumFile: 'missing' }));
  await flush();
  assert.match(t.container.querySelector('#plugin-confirm-summary').textContent, /no checksum file/);
  t.container.querySelector('#plugin-confirm-accept').dispatch('click');
  assert.equal(body(t.calls.at(-1)).acceptMissingChecksum, true);
  assert.equal(t.calls.at(-1).path, '/api/plugins/recorder/install');

  t.container.querySelector('#plugin-update-aws').dispatch('click');
  assert.deepEqual(body(t.calls.at(-1)), { action: 'update', version: '1.4.0' });
  t.calls.at(-1).resolve(plan({
    action: 'update',
    name: 'aws',
    version: '1.3.0',
    repository: 'rshade/finfocus-plugin-aws-public',
    securityLevel: 'official',
    fallbackFrom: '1.4.0',
    checksumFile: 'present',
  }));
  await flush();
  const summary = t.container.querySelector('#plugin-confirm-summary').textContent;
  assert.match(summary, /1\.4\.0/);
  assert.match(summary, /1\.3\.0/);
  assert.match(summary, /aws/);
  assert.match(summary, /official/);
  t.container.querySelector('#plugin-confirm-accept').dispatch('click');
  assert.equal(t.calls.at(-1).path, '/api/plugins/aws/update');
  assert.deepEqual(body(t.calls.at(-1)), {
    version: '1.3.0',
    repository: 'rshade/finfocus-plugin-aws-public',
    securityLevel: 'official',
    acceptMissingChecksum: false,
    acceptFallbackFrom: '1.4.0',
  });
  t.mounted.unmount();
});

test('remove confirms the installed version then deletes it', async () => {
  const t = await open();
  t.container.querySelector('#plugin-remove-local-0-4-0').dispatch('click');
  assert.deepEqual(body(t.calls.at(-1)), { action: 'remove', version: '0.4.0' });
  t.calls.at(-1).resolve(plan({
    action: 'remove',
    name: 'local',
    version: '0.4.0',
    repository: '',
    securityLevel: '',
    fallbackFrom: '',
    checksumFile: 'unknown',
    inCatalog: false,
  }));
  await flush();
  assert.match(t.container.querySelector('#plugin-confirm-summary').textContent, /local/);
  assert.match(t.container.querySelector('#plugin-confirm-summary').textContent, /0\.4\.0/);
  assert.equal(t.calls.some((call) => call.path.includes('/0.4.0')), false);
  t.container.querySelector('#plugin-confirm-accept').dispatch('click');
  assert.equal(t.calls.at(-1).path, '/api/plugins/local/0.4.0');
  assert.deepEqual(body(t.calls.at(-1)), {
    repository: '',
    securityLevel: '',
    acceptMissingChecksum: false,
    acceptFallbackFrom: '',
  });
  t.mounted.unmount();
});

test('applyProgress shows phase, bytes, and a warning', async () => {
  const t = await open();
  const progress = t.container.querySelector('#plugin-progress');
  t.applyProgress(t.container, {
    phase: 'download',
    message: 'archive is incomplete',
    bytes: 1200,
    total: 4096,
    warning: true,
  });
  assert.match(progress.textContent, /download/);
  assert.match(progress.textContent, /1200/);
  assert.match(progress.textContent, /4096/);
  assert.match(progress.textContent, /warning/);
  t.applyProgress(t.container, { phase: 'verify', message: 'checking archive', bytes: 10, total: 0, warning: false });
  assert.match(progress.textContent, /verify/);
  assert.doesNotMatch(progress.textContent, /10/);
  assert.doesNotMatch(progress.textContent, /warning/);
  t.applyProgress(t.container, { status: 'failed', error: 'checksum did not match' });
  assert.match(progress.textContent, /checksum did not match/);
  t.applyProgress(t.container, { status: 'succeeded', error: '' });
  assert.match(progress.textContent, /finished/);
  t.mounted.unmount();
});

test('validate shows pass or the reason without treating a failed check as transport', async () => {
  const t = await open();
  t.container.querySelector('#plugin-validate-aws').dispatch('click');
  assert.equal(t.calls.at(-1).path, '/api/plugins/aws/validate');
  assert.deepEqual(body(t.calls.at(-1)), {});
  t.calls.at(-1).resolve({ passed: false, reason: 'binary is missing' });
  await flush();
  assert.match(t.container.querySelector('#plugin-message').textContent, /binary is missing/);
  assert.equal(t.container.querySelector('#plugin-error').hidden, true);
  t.container.querySelector('#plugin-validate-aws').dispatch('click');
  t.calls.at(-1).resolve({ passed: true, reason: '' });
  await flush();
  assert.match(t.container.querySelector('#plugin-message').textContent, /Validation passed/);
  t.mounted.unmount();
});

function statusCellOf(container, key) {
  const help = container.querySelector(`#plugin-help-${key}`);
  return help.parent.parent.children.find((child) => child.tagName === 'td');
}

test('ESC status names the environment and not the value', async () => {
  const t = await open();
  t.container.querySelector('#plugin-configure-aws').dispatch('click');
  const settings = descriptorSettings();
  settings.values[0] = {
    key: 'api_key', secret: true, project: { set: false }, global: { set: false },
    envSet: true, effectiveSource: 'esc', escEnvironment: 'billing/vantage@stable',
    value: 'leaked-esc-secret',
  };
  t.calls.at(-1).resolve(settings);
  await flush();
  const help = t.container.querySelector('#plugin-help-api_key');
  assert.equal(help.querySelector('code').textContent, 'billing/vantage@stable');
  assert.match(help.textContent, /Supplied by Pulumi ESC environment billing\/vantage@stable, which overrides the shell and both files/);
  assert.match(help.textContent, /An environment value is set and overrides the file/);
  assert.doesNotMatch(help.textContent, /`/);
  const status = statusCellOf(t.container, 'api_key');
  assert.equal(status.className, 'setting-source');
  assert.equal(status.textContent, 'from ESC (billing/vantage@stable)');
  assert.equal(status.querySelector('code').textContent, 'billing/vantage@stable');
  assert.equal(status.querySelector('.setting-source-label').attrs['aria-label'], 'from ESC (billing/vantage@stable)');
  assert.equal(statusCellOf(t.container, 'base_url').textContent, 'https://old.example');
  assert.equal(statusCellOf(t.container, 'mock_response').textContent, 'not set');
  assert.doesNotMatch(t.container.textContent, /leaked-esc-secret/);
  assert.doesNotMatch(t.container.textContent, /stored-secret/);

  const tier = t.container.querySelector('#plugin-tier');
  tier.value = 'global';
  tier.dispatch('change');
  assert.equal(statusCellOf(t.container, 'api_key').textContent, 'from ESC (billing/vantage@stable)');
  t.mounted.unmount();
});

test('ESC status without a reference does not invent one', async () => {
  const t = await open();
  t.container.querySelector('#plugin-configure-aws').dispatch('click');
  const settings = descriptorSettings();
  settings.values[1] = {
    key: 'base_url', secret: false, project: { set: true, value: 'https://old.example' },
    global: { set: false }, effectiveSource: 'esc', escEnvironment: '   ',
  };
  t.calls.at(-1).resolve(settings);
  await flush();
  const status = statusCellOf(t.container, 'base_url');
  assert.equal(status.textContent, 'from ESC');
  assert.equal(status.querySelector('code'), undefined);
  assert.equal(status.querySelector('.setting-source-label').attrs['aria-label'], 'from ESC');
  const help = t.container.querySelector('#plugin-help-base_url');
  assert.match(help.textContent, /Supplied by a Pulumi ESC environment, which overrides the shell and both files/);
  assert.equal(help.querySelector('code'), undefined);
  assert.doesNotMatch(help.textContent, /`/);
  assert.doesNotMatch(status.textContent, /old\.example/);
  assert.equal(t.container.querySelector('#plugin-setting-base_url').value, 'https://old.example');
  t.mounted.unmount();
});

test('descriptor form previews typed controls and omits an empty secret', async () => {
  const t = await open();
  t.container.querySelector('#plugin-configure-aws').dispatch('click');
  assert.equal(t.calls.at(-1).path, '/api/plugins/aws/settings');
  t.calls.at(-1).resolve(descriptorSettings());
  await flush();
  const enabled = t.container.querySelector('#plugin-setting-enabled');
  const base = t.container.querySelector('#plugin-setting-base_url');
  const timeout = t.container.querySelector('#plugin-setting-timeout');
  const secret = t.container.querySelector('#plugin-setting-api_key');
  const mock = t.container.querySelector('#plugin-setting-mock_response');
  assert.equal(enabled.tagName, 'select');
  assert.deepEqual(enabled.querySelectorAll('option').map((option) => option.value).filter(Boolean), ['true', 'false']);
  assert.equal(enabled.value, 'true');
  assert.equal(base.tagName, 'input');
  assert.equal(base.value, 'https://old.example');
  assert.equal(timeout.tagName, 'input');
  assert.equal(timeout.value, '30');
  assert.equal(mock.tagName, 'input');
  assert.equal(secret.attrs.type, 'password');
  assert.equal(secret.value, '');
  assert.equal(secret.attrs.placeholder, 'Replace secret');
  assert.equal(t.container.querySelector('#plugin-tier').value, 'project');
  assert.doesNotMatch(t.container.textContent, /stored-secret/);

  base.value = 'https://new.example';
  timeout.value = '15';
  enabled.value = 'false';
  t.container.querySelector('#plugin-save').dispatch('click');
  assert.equal(t.calls.at(-1).path, '/api/plugins/aws/settings/preview');
  assert.deepEqual(body(t.calls.at(-1)), {
    tier: 'project',
    baseVersion: 'p1',
    changes: [
      { key: 'plugins.aws.base_url', value: 'https://new.example' },
      { key: 'plugins.aws.timeout', value: '15' },
      { key: 'plugins.aws.enabled', value: 'false' },
    ],
  });
  t.calls.at(-1).resolve({
    diff: [{ key: 'plugins.aws.base_url', before: 'https://old.example', after: 'https://new.example' }],
    validation: { valid: true, errors: [] },
  });
  await flush();
  assert.equal(t.container.querySelector('#plugin-confirm-dialog').open, true);

  secret.value = 'next-secret';
  t.container.querySelector('#plugin-save').dispatch('click');
  assert.deepEqual(body(t.calls.at(-1)), {
    tier: 'project',
    baseVersion: 'p1',
    changes: [
      { key: 'plugins.aws.api_key', value: 'next-secret' },
      { key: 'plugins.aws.base_url', value: 'https://new.example' },
      { key: 'plugins.aws.timeout', value: '15' },
      { key: 'plugins.aws.enabled', value: 'false' },
    ],
  });
  const preview = body(t.calls.at(-1));
  t.calls.at(-1).resolve({
    diff: [{ key: 'plugins.aws.api_key', before: 'set', after: 'replaced' }],
  });
  await flush();
  assert.match(t.container.querySelector('#plugin-confirm-dialog').textContent, /replaced/);
  assert.doesNotMatch(t.container.textContent, /next-secret/);
  t.container.querySelector('#plugin-confirm-accept').dispatch('click');
  assert.equal(t.calls.at(-1).path, '/api/plugins/aws/settings');
  assert.deepEqual(body(t.calls.at(-1)), preview);
  const saved = descriptorSettings();
  saved.message = 'Saved. These settings apply the next time this plugin is launched.';
  t.calls.at(-1).resolve(saved);
  await flush();
  assert.match(t.container.querySelector('#plugin-message').textContent, /next time this plugin is launched/);
  assert.equal(t.container.querySelector('#plugin-confirm-dialog').open, false);
  t.mounted.unmount();
});

test('generic mode renders a stored key and hides a secret that was not sent', async () => {
  const t = await open();
  t.container.querySelector('#plugin-configure-local').dispatch('click');
  t.calls.at(-1).resolve({
    name: 'local',
    mode: 'generic',
    projectAvailable: false,
    versions: { global: 'g1', project: 'absent' },
    descriptors: [],
    values: [
      { key: 'mock_response', secret: false, project: { set: false }, global: { set: true, value: 'visible-mock' }, effectiveSource: 'global' },
      { key: 'api_key', secret: true, project: { set: false }, global: { set: true }, effectiveSource: 'global' },
    ],
  });
  await flush();
  const tier = t.container.querySelector('#plugin-tier');
  assert.equal(tier.value, 'global');
  assert.equal(tier.querySelectorAll('option').find((option) => option.value === 'project').disabled, true);
  assert.match(t.container.textContent, /no project directory is available/);
  const stored = t.container.querySelector('#plugin-setting-mock_response');
  const secret = t.container.querySelector('#plugin-setting-api_key');
  assert.equal(stored.tagName, 'input');
  assert.equal(stored.value, 'visible-mock');
  assert.match(t.container.querySelector('#plugin-settings').textContent, /mock_response/);
  assert.equal(secret.attrs.type, 'password');
  assert.equal(secret.value, '');
  assert.equal(secret.attrs.placeholder, 'Replace secret');
  assert.match(t.container.textContent, /set/);
  assert.doesNotMatch(t.container.textContent, /not-sent-secret/);
  assert.doesNotMatch(t.container.textContent, /visible-mock-secret/);
  t.mounted.unmount();
});

test('an empty secret is omitted and a typed replacement is sent', async () => {
  const t = await open();
  t.container.querySelector('#plugin-configure-aws').dispatch('click');
  t.calls.at(-1).resolve(descriptorSettings());
  await flush();
  const secret = t.container.querySelector('#plugin-setting-api_key');
  assert.equal(secret.value, '');
  t.container.querySelector('#plugin-save').dispatch('click');
  assert.equal(t.calls.length, 2);
  assert.match(t.container.querySelector('#plugin-message').textContent, /No changes/);

  secret.value = 'next-secret';
  t.container.querySelector('#plugin-save').dispatch('click');
  assert.deepEqual(body(t.calls.at(-1)), {
    tier: 'project',
    baseVersion: 'p1',
    changes: [{ key: 'plugins.aws.api_key', value: 'next-secret' }],
  });
  t.calls.at(-1).resolve({ diff: [{ key: 'plugins.aws.api_key', before: 'set', after: 'replaced' }] });
  await flush();
  assert.match(t.container.querySelector('#plugin-confirm-dialog').textContent, /replaced/);
  assert.doesNotMatch(t.container.textContent, /next-secret/);
  t.container.querySelector('#plugin-confirm-cancel').dispatch('click');
  assert.equal(t.container.querySelector('#plugin-confirm-dialog').open, false);

  secret.value = 'ignored-secret';
  t.container.querySelector('#plugin-clear-api_key').checked = true;
  t.container.querySelector('#plugin-save').dispatch('click');
  assert.deepEqual(body(t.calls.at(-1)), {
    tier: 'project',
    baseVersion: 'p1',
    changes: [{ key: 'plugins.aws.api_key', clear: true }],
  });
  assert.doesNotMatch(JSON.stringify(t.calls.at(-1).body), /ignored-secret/);
  t.mounted.unmount();
});

test('a stored secret bool can be removed even though it renders as a masked input', async () => {
  const t = await open();
  t.container.querySelector('#plugin-configure-aws').dispatch('click');
  const settings = descriptorSettings();
  settings.descriptors.push({ key: 'allow_credentials', type: 'bool', secret: true, description: 'Allow credentials' });
  settings.values.push({
    key: 'allow_credentials',
    secret: true,
    project: { set: true },
    global: { set: false },
    effectiveSource: 'project',
  });
  t.calls.at(-1).resolve(settings);
  await flush();

  assert.equal(t.container.querySelector('#plugin-setting-allow_credentials').attrs.type, 'password');
  t.container.querySelector('#plugin-clear-allow_credentials').checked = true;
  t.container.querySelector('#plugin-save').dispatch('click');
  assert.deepEqual(body(t.calls.at(-1)).changes, [{ key: 'plugins.aws.allow_credentials', clear: true }]);
  t.mounted.unmount();
});
