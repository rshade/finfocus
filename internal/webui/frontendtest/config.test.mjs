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
 const calls=[];const api={post(path,body,options){return request(path,body,options);},get(path,options){return request(path,null,options);}};
 function request(path,body,options){let resolve,reject;const promise=new Promise((yes,no)=>{resolve=yes;reject=no;});calls.push({path,body,options,resolve,reject});return promise;}
 const container=document.createElement('div');document.append(container);const mounted=mod.namespace.mount(container,{api,stream:{on(){return()=>{}}}});
 return{container,calls,mounted,document};
}

function configPage() {
  return {
    projectAvailable: true,
    projectUnavailableReason: '',
    versions: { global: 'g1', project: 'p1' },
    settings: [
      {
        key: 'output.precision', kind: 'int', help: 'Decimal places', secret: false,
        effective: { source: 'project', value: 2 },
        tiers: {
          default: { set: true, value: 2 }, global: { set: true, value: 2 },
          project: { set: true, value: 2 }, env: { set: false },
        },
      },
      {
        key: 'plugins.aws.secret_key', kind: 'plugin', help: 'Credential', secret: true,
        effective: { source: 'global' },
        tiers: {
          default: { set: false }, global: { set: true }, project: { set: false }, env: { set: false },
        },
      },
    ],
    readOnly: [{ key: 'routing.plugins[0].name', secret: false, set: true, source: 'global', value: 'aws' }],
  };
}

test('preview then confirm posts precision and omits an empty secret', async () => {
  const t = await setup('config');
  assert.equal(t.calls[0].path, '/api/config');
  t.calls[0].resolve(configPage());
  await flush();
  assert.equal(t.container.querySelector('#config-tier').value, 'project');
  assert.equal(t.container.querySelector('h2').textContent, 'Config');
  assert.doesNotMatch(t.container.textContent, /REDACTED|stored-secret/);
  t.container.querySelector('#config-save').dispatch('click');
  assert.equal(t.calls.length, 1);
  assert.match(t.container.querySelector('#config-message').textContent, /No changes/);
  t.container.querySelector('#config-output-precision').value = '4';
  t.container.querySelector('#config-save').dispatch('click');
  assert.equal(t.calls.at(-1).path, '/api/config/preview');
  assert.deepEqual(JSON.parse(JSON.stringify(t.calls.at(-1).body)), {
    tier: 'project', baseVersion: 'p1', changes: [{ key: 'output.precision', value: '4' }],
  });
  t.calls.at(-1).resolve({ diff: [{ key: 'output.precision', before: '2', after: '4' }], validation: { valid: true, errors: [] } });
  await flush();
  const dialog = t.container.querySelector('#config-preview-dialog');
  assert.equal(dialog.open, true);
  assert.match(dialog.textContent, /2/);
  assert.match(dialog.textContent, /4/);
  t.container.querySelector('#config-confirm').dispatch('click');
  assert.equal(t.calls.at(-1).path, '/api/config');
  assert.deepEqual(JSON.parse(JSON.stringify(t.calls.at(-1).body.changes)), [{ key: 'output.precision', value: '4' }]);
  const saved = configPage();
  saved.message = 'Saved. The running session keeps its startup configuration and does not reload this file.';
  saved.settings[0].tiers.project.value = 4;
  saved.settings[0].effective.value = 4;
  t.calls.at(-1).resolve(saved);
  await flush();
  assert.match(t.container.textContent, /does not reload/);
  assert.equal(t.container.querySelector('#config-output-precision').value, '4');
  assert.equal(dialog.open, false);
  t.mounted.unmount();
});

test('a blank or masked secret is omitted and a replacement is posted', async () => {
  const t = await setup('config');
  t.calls[0].resolve(configPage());
  await flush();
  const tier = t.container.querySelector('#config-tier');
  tier.value = 'global';
  tier.dispatch('change');
  const secret = t.container.querySelector('#config-plugins-aws-secret_key');
  assert.equal(secret.value, '');
  secret.value = '[REDACTED]';
  t.container.querySelector('#config-save').dispatch('click');
  assert.equal(t.calls.length, 1);
  secret.value = 'next-secret';
  t.container.querySelector('#config-save').dispatch('click');
  assert.deepEqual(JSON.parse(JSON.stringify(t.calls.at(-1).body)), {
    tier: 'global', baseVersion: 'g1', changes: [{ key: 'plugins.aws.secret_key', value: 'next-secret' }],
  });
  t.calls.at(-1).resolve({ diff: [{ key: 'plugins.aws.secret_key', before: 'set', after: 'replaced' }] });
  await flush();
  assert.match(t.container.querySelector('#config-preview-dialog').textContent, /replaced/);
  assert.doesNotMatch(t.container.textContent, /next-secret/);
  t.container.querySelector('#config-cancel').dispatch('click');
  const clear = t.container.querySelector('#config-clear-plugins-aws-secret_key');
  clear.checked = true;
  secret.value = 'ignored-secret';
  t.container.querySelector('#config-save').dispatch('click');
  assert.deepEqual(JSON.parse(JSON.stringify(t.calls.at(-1).body.changes)), [{ key: 'plugins.aws.secret_key', clear: true }]);
  t.mounted.unmount();
});

test('an unavailable project tier is disabled and validation fields render', async () => {
  const t = await setup('config');
  const page = configPage();
  page.projectAvailable = false;
  page.projectUnavailableReason = 'no project directory is available';
  t.calls[0].resolve(page);
  await flush();
  const tier = t.container.querySelector('#config-tier');
  assert.equal(tier.value, 'global');
  assert.equal(tier.querySelector('option').disabled, true);
  assert.match(t.container.textContent, /no project directory is available/);
  t.container.querySelector('#config-output-precision').value = 'nope';
  t.container.querySelector('#config-save').dispatch('click');
  t.calls.at(-1).reject(Object.assign(new Error('invalid output.precision'), {
    fields: [{ path: 'output.precision', message: 'precision must be a number' }],
  }));
  await flush();
  assert.equal(t.container.querySelector('#config-preview-dialog').open, undefined);
  assert.match(t.container.querySelector('#config-error').textContent, /output.precision: precision must be a number/);
  t.mounted.unmount();
});

test('settings whose keys slug alike keep separate controls', async () => {
  const t = await setup('config');
  const cluster = (key, value) => ({
    key, kind: 'cluster_context', help: 'Context', secret: false,
    effective: { source: 'project', value },
    tiers: {
      default: { set: false }, global: { set: false },
      project: { set: true, value }, env: { set: false },
    },
  });
  const page = configPage();
  page.settings = [
    cluster('overview.cluster_contexts.prod/west', 'ctx-slash'),
    cluster('overview.cluster_contexts.prod-west', 'ctx-dash'),
  ];
  t.calls[0].resolve(page);
  await flush();

  const inputs = t.container.querySelectorAll('input').filter((n) => n.attrs.type === 'text');
  assert.equal(inputs.length, 2);
  assert.notEqual(inputs[0].id, inputs[1].id);
  const labels = t.container.querySelectorAll('label').filter((n) => n.attrs.for && n.className !== 'config-clear');
  assert.deepEqual(labels.map((n) => n.attrs.for).sort(), inputs.map((n) => n.id).sort());

  t.container.querySelector('#config-save').dispatch('click');
  assert.equal(t.calls.length, 1);
  assert.match(t.container.querySelector('#config-message').textContent, /No changes/);

  inputs[1].value = 'ctx-dash-new';
  t.container.querySelector('#config-save').dispatch('click');
  assert.deepEqual(JSON.parse(JSON.stringify(t.calls.at(-1).body.changes)), [
    { key: 'overview.cluster_contexts.prod-west', value: 'ctx-dash-new' },
  ]);
});

test('Pulumi Cloud history mode is explained without changing its values', async () => {
  const t = await setup('config');
  const page = configPage();
  page.settings = [{
    key: 'cost.history.pulumi_cloud.enabled', kind: 'enum', secret: false,
    help: 'Import resource identity from Pulumi Cloud update history',
    enum: ['false', 'auto', 'true'],
    effective: { source: 'default', value: 'false' },
    tiers: {
      default: { set: true, value: 'false' }, global: { set: false },
      project: { set: false }, env: { set: false },
    },
  }];
  t.calls[0].resolve(page);
  await flush();

  const select = t.container.querySelector('#config-cost-history-pulumi_cloud-enabled');
  const options = select.children.filter((n) => n.tagName === 'option');
  assert.deepEqual(options.filter((n) => n.value !== '').map((n) => n.value), ['false', 'auto', 'true']);
  assert.match(options.find((n) => n.value === 'auto').textContent, /fall back/);
  assert.match(options.find((n) => n.value === 'true').textContent, /fail/);

  const modes = t.container.querySelector('#config-cost-history-pulumi_cloud-enabled-modes');
  assert.match(modes.textContent, /Never contact Pulumi Cloud/);
  assert.match(modes.textContent, /cost\.history\.enabled/);
  assert.equal(select.attrs['aria-describedby'], 'config-cost-history-pulumi_cloud-enabled-help config-cost-history-pulumi_cloud-enabled-modes');
  assert.match(t.container.querySelector('.config-value-note').textContent, /never contact Pulumi Cloud/);

  select.value = 'auto';
  t.container.querySelector('#config-save').dispatch('click');
  assert.deepEqual(JSON.parse(JSON.stringify(t.calls.at(-1).body.changes)), [
    { key: 'cost.history.pulumi_cloud.enabled', value: 'auto' },
  ]);
});
