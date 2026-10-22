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
  addEventListener(k,fn) { this.listeners.set(k,fn); }
  dispatch(k,extra={}) { this.listeners.get(k)?.({target:this,preventDefault(){},...extra}); }
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
 const context=vm.createContext({document,AbortController,console,setTimeout,clearTimeout});
 const app=new vm.SyntheticModule(['el'],function(){this.setExport('el',(tag,attrs={},...children)=>{const node=document.createElement(tag);for(const [k,v]of Object.entries(attrs)){if(k==='text')node.textContent=v;else if(k==='class')node.className=v;else if(k==='dataset')Object.assign(node.dataset,v);else node.setAttribute(k,v);}node.append(...children.flat());return node;});},{context});await app.link(()=>{});await app.evaluate();
 const mod=new vm.SourceTextModule(await readFile(new URL(`../static/views/${view}.js`,import.meta.url),'utf8'),{context});await mod.link(()=>app);await mod.evaluate();
 const calls=[];const api={post(path,body,options){return request(path,body,options);},get(path,options){return request(path,null,options);}};
 function request(path,body,options){let resolve,reject;const promise=new Promise((yes,no)=>{resolve=yes;reject=no;});calls.push({path,body,options,resolve,reject});return promise;}
 const listeners=new Map();const stream={on(name,fn){listeners.set(name,fn);return()=>listeners.delete(name);}};
 const container=document.createElement('div');document.append(container);const mounted=mod.namespace.mount(container,{api,stream});
 return{container,calls,mounted,document,emit:(name,data)=>listeners.get(name)?.(data)};
}
const baseline = (label='base') => ({resource:{id:'urn:real'}, pricingMode:'', pricingModes:[{id:'opaque-mode',label:'Hourly provider',rate:'USD 30/hour',detailsDisplay:'Pricing tiers: server-formatted tier'}],display:{baseline:'USD 60 baseline',modified:'USD 60 modified',change:'$0.00 →',arrow:'→',properties:[{key:'size',originalValue:'2',currentValue:'2',delta:'$0.00 →'},{key:'label',originalValue:'old',currentValue:label,delta:'$0.00 →'}]}});
async function loaded() { const t=await setup('estimate');assert.equal(t.calls[0].path,'/api/estimate/resources');t.calls[0].resolve({resources:[{id:'urn:real',type:'aws:ec2:Instance'}]});await flush();assert.match(t.calls[1].path,/baseline\?urn=urn%3Areal/);t.calls[1].resolve(baseline());await flush();return t; }
test('estimate picker uses a readable resource name while requests retain the full URN',async()=>{
 const t=await setup('estimate');
 const urn='urn:pulumi:dev::initial-30::aws:ec2/instance:Instance::webServer';
 t.calls[0].resolve({resources:[{id:urn,type:'aws:ec2/instance:Instance'}]});await flush();
 const option=t.container.querySelector('#estimate-resource').children[0];
 assert.equal(option.textContent,'webServer (aws:ec2/instance:Instance)');
 assert.equal(option.value,urn);assert.equal(option.attrs.title,urn);
 assert.match(t.calls[1].path,new RegExp(encodeURIComponent(urn)));
 t.calls[1].resolve(baseline());await flush();t.mounted.unmount();
});
test('estimate Enter preserves exact overrides and renders only server display strings',async()=>{
 const t=await loaded();assert.match(t.container.textContent,/server-formatted tier/);const input=t.container.querySelectorAll('.estimate-input').find(n=>n.dataset.key==='size');input.value=' 3 ';input.dispatch('keydown',{key:'Enter'});
 const call=t.calls.at(-1);assert.equal(call.path,'/api/estimate/recalculate');assert.deepEqual(JSON.parse(JSON.stringify(call.body.overrides)),{size:' 3 ',label:'base'});
 call.resolve({display:{...baseline().display,baseline:'precise BASE',modified:'precise MODIFIED',change:'server delta ↑',properties:[{key:'size',originalValue:'2',currentValue:' 3 ',delta:'server property delta'}]}});await flush();assert.match(t.container.textContent,/precise BASE/);assert.match(t.container.textContent,/server delta ↑/);assert.match(t.container.textContent,/server property delta/);assert.equal(t.document.activeElement.dataset.key,'size');t.mounted.unmount();
});
test('estimate mode selects server identifier and stale recalculations cannot win',async()=>{
 const t=await loaded();const mode=t.container.querySelector('#estimate-mode');mode.value='opaque-mode';mode.dispatch('change');assert.equal(t.calls.at(-1).body.pricingMode,'opaque-mode');
 const older=t.calls.at(-1);const input=t.container.querySelectorAll('.estimate-input')[0];input.value='4';input.dispatch('keydown',{key:'Enter'});const newer=t.calls.at(-1);assert.equal(older.options.signal.aborted,true);
 newer.resolve({display:{...baseline().display,modified:'latest result'}});await flush();older.resolve({display:{...baseline().display,modified:'stale result'}});await flush();assert.match(t.container.textContent,/latest result/);assert.doesNotMatch(t.container.textContent,/stale result/);t.mounted.unmount();assert.equal(newer.options.signal.aborted,true);
});
test('estimate empty edits select a fresh baseline and errors remain inline',async()=>{
 const t=await loaded();for(const input of t.container.querySelectorAll('.estimate-input'))input.value=input.dataset.original;
 const mode=t.container.querySelector('#estimate-mode');mode.value='opaque-mode';mode.dispatch('change');assert.match(t.calls.at(-1).path,/pricingMode=opaque-mode/);t.calls.at(-1).reject(new Error('Provider unavailable'));await flush();assert.match(t.container.textContent,/Provider unavailable/);assert.match(t.container.textContent,/Unable to calculate/);t.mounted.unmount();
});
test('a failed baseline keeps rows and modes, then edits and mode changes retry', async () => {
  const t = await setup('estimate');
  t.calls[0].resolve({ resources: [{ id: 'urn:real', type: 'aws:ec2:Instance' }] });
  await flush();
  t.calls[1].resolve({
    status: 'unavailable',
    error: 'estimate data unavailable',
    code: 'estimate_unavailable',
    pricingModes: [{ id: 'hourly', label: 'Hourly provider', rate: 'USD 10/hour' }],
    display: { properties: [{ key: 'size', originalValue: '2', currentValue: '2', delta: '' }] },
  });
  await flush();
  assert.match(t.container.textContent, /estimate data unavailable/);
  assert.match(t.container.textContent, /Unable to calculate/);
  assert.doesNotMatch(t.container.textContent, /Estimate calculated/);
  assert.equal(t.container.querySelector('.totals-bar').textContent, '');
  const input = t.container.querySelector('.estimate-input');
  assert.equal(input.dataset.key, 'size');
  assert.equal(input.value, '2');
  const mode = t.container.querySelector('#estimate-mode');
  assert.equal(mode.querySelectorAll('option').length, 2);
  mode.value = 'hourly';
  mode.dispatch('change');
  assert.match(t.calls.at(-1).path, /pricingMode=hourly/);
  input.value = '4';
  input.dispatch('keydown', { key: 'Enter' });
  const edited = t.calls.at(-1);
  assert.equal(edited.path, '/api/estimate/recalculate');
  assert.equal(edited.body.pricingMode, 'hourly');
  assert.deepEqual(JSON.parse(JSON.stringify(edited.body.overrides)), { size: '4' });
  edited.resolve({
    pricingMode: 'hourly',
    display: {
      baseline: 'USD 20 baseline', modified: 'USD 40 modified', change: '+$20.00 ↑',
      properties: [{ key: 'size', originalValue: '2', currentValue: '4', delta: '+$20.00 ↑' }],
    },
  });
  await flush();
  assert.match(t.container.textContent, /USD 40 modified/);
  assert.match(t.container.textContent, /Estimate calculated/);
  t.mounted.unmount();
});
test('estimate resource loading retries on ready and late responses after unmount are ignored',async()=>{
 const t=await setup('estimate');t.calls[0].reject(Object.assign(new Error('loading'),{status:503}));await flush();t.emit('ready',{});assert.equal(t.calls.length,2);t.mounted.unmount();const before=t.container.textContent;t.calls[1].resolve({resources:[{id:'late',type:'aws:ec2:Instance'}]});await flush();assert.equal(t.container.textContent,before);assert.equal(t.calls[1].options.signal.aborted,true);
});

test('a failed baseline keeps the selected pricing mode', async () => {
  const t = await loaded();
  const mode = t.container.querySelector('#estimate-mode');
  mode.value = 'opaque-mode';
  mode.dispatch('change');
  t.calls.at(-1).resolve({
    status: 'unavailable',
    error: 'estimate data unavailable',
    code: 'estimate_unavailable',
    pricingModes: baseline().pricingModes,
    display: { properties: baseline().display.properties },
  });
  await flush();
  assert.equal(mode.value, 'opaque-mode');
  const input = t.container.querySelectorAll('.estimate-input')[0];
  input.value = '5';
  input.dispatch('keydown', { key: 'Enter' });
  assert.equal(t.calls.at(-1).body.pricingMode, 'opaque-mode');
  t.mounted.unmount();
});

test('a plain failure after a successful estimate clears the stale comparison but keeps rows', async () => {
  const t = await loaded();
  const input = t.container.querySelectorAll('.estimate-input')[0];
  input.value = '4';
  input.dispatch('keydown', { key: 'Enter' });
  t.calls.at(-1).resolve({ display: { ...baseline().display, modified: 'good result' } });
  await flush();
  assert.match(t.container.textContent, /good result/);
  const again = t.container.querySelectorAll('.estimate-input')[0];
  again.value = '5';
  again.dispatch('keydown', { key: 'Enter' });
  t.calls.at(-1).reject(Object.assign(new Error('estimate data unavailable'), { status: 502 }));
  await flush();
  assert.match(t.container.textContent, /estimate data unavailable/);
  assert.equal(t.container.querySelector('.totals-bar').textContent, '');
  assert.ok(t.container.querySelectorAll('.estimate-input').length > 0);
  t.mounted.unmount();
});

test('show all preserves an eligible selection, pricing mode and edits without a new baseline', async () => {
 const t=await loaded();
 const picker=t.container.querySelector('#estimate-resource');
 const toggle=t.container.querySelector('#estimate-show-all');
 assert.ok(toggle);
 const input=t.container.querySelectorAll('.estimate-input')[0];input.value='unsaved edit';
 const mode=t.container.querySelector('#estimate-mode');mode.value='opaque-mode';
 toggle.checked=true;toggle.dispatch('change');
 assert.equal(t.calls.at(-1).path,'/api/estimate/resources?showAll=true');
 const count=t.calls.length;
 t.calls.at(-1).resolve({resources:[{id:'provider',type:'pulumi:providers:aws'},{id:'urn:real',type:'aws:ec2:Instance'}],preferredURN:'urn:real'});
 await flush();
 assert.equal(picker.value,'urn:real');assert.equal(mode.value,'opaque-mode');assert.equal(input.value,'unsaved edit');assert.equal(t.calls.length,count);
 toggle.checked=false;toggle.dispatch('change');
 assert.equal(t.calls.at(-1).path,'/api/estimate/resources');
 t.calls.at(-1).resolve({resources:[{id:'urn:real',type:'aws:ec2:Instance'}]});await flush();
 assert.equal(picker.value,'urn:real');assert.equal(input.value,'unsaved edit');t.mounted.unmount();
});

test('filtered empty state offers show all and uses the server preferred resource', async () => {
 const t=await setup('estimate');
 t.calls[0].resolve({resources:[],hiddenCount:2,preferredURN:''});await flush();
 assert.match(t.container.textContent,/known non-billable/);assert.match(t.container.textContent,/Show all resources/);
 const toggle=t.container.querySelector('#estimate-show-all');toggle.checked=true;toggle.dispatch('change');
 t.calls.at(-1).resolve({resources:[{id:'provider',type:'pulumi:providers:aws'},{id:'candidate',type:'custom:module:Unknown'}],preferredURN:'candidate'});await flush();
 assert.equal(t.container.querySelector('#estimate-resource').value,'candidate');
 assert.match(t.calls.at(-1).path,/baseline\?urn=candidate/);t.mounted.unmount();
});

test('hiding the selected non-billable resource clears stale editor and pricing details', async () => {
 const t=await loaded();
 const toggle=t.container.querySelector('#estimate-show-all');assert.ok(toggle);
 toggle.checked=false;toggle.dispatch('change');
 t.calls.at(-1).resolve({resources:[],hiddenCount:1});await flush();
 assert.equal(t.container.querySelector('#estimate-resource').value,'');
 assert.equal(t.container.querySelectorAll('.estimate-input').length,0);
 assert.equal(t.container.querySelector('.totals-bar').textContent,'');
 assert.equal(t.container.querySelector('#estimate-mode').querySelectorAll('option').length,1);
 assert.doesNotMatch(t.container.textContent,/server-formatted tier/);t.mounted.unmount();
});

test('a stale show-all response cannot replace the latest filtered list', async () => {
 const t=await loaded();const toggle=t.container.querySelector('#estimate-show-all');assert.ok(toggle);
 toggle.checked=true;toggle.dispatch('change');const older=t.calls.at(-1);
 toggle.checked=false;toggle.dispatch('change');const newer=t.calls.at(-1);
 assert.equal(older.options.signal.aborted,true);
 newer.resolve({resources:[{id:'urn:real',type:'aws:ec2:Instance'}]});await flush();
 older.resolve({resources:[{id:'provider',type:'pulumi:providers:aws'}]});await flush();
 assert.equal(t.container.querySelector('#estimate-resource').value,'urn:real');t.mounted.unmount();
});

test('toggling during an initial baseline preserves the pending calculation', async () => {
 const t=await setup('estimate');
 t.calls[0].resolve({resources:[{id:'urn:real',type:'aws:ec2:Instance'}]});await flush();
 const pending=t.calls.at(-1), toggle=t.container.querySelector('#estimate-show-all');
 toggle.checked=true;toggle.dispatch('change');assert.equal(pending.options.signal.aborted,false);
 t.calls.at(-1).resolve({resources:[{id:'urn:real',type:'aws:ec2:Instance'}]});await flush();
 pending.resolve(baseline());await flush();
 assert.match(t.container.textContent,/Estimate calculated/);t.mounted.unmount();
});

test('editor recalculation cannot cancel a pending show-all resource list', async () => {
 const t=await loaded();const toggle=t.container.querySelector('#estimate-show-all');
 toggle.checked=true;toggle.dispatch('change');const listing=t.calls.at(-1);
 const input=t.container.querySelectorAll('.estimate-input')[0];input.value='4';input.dispatch('keydown',{key:'Enter'});
 const calculation=t.calls.at(-1);assert.equal(listing.options.signal.aborted,false);
 calculation.resolve({display:{...baseline().display,modified:'new comparison'}});await flush();
 listing.resolve({resources:[{id:'provider',type:'pulumi:providers:aws'},{id:'urn:real',type:'aws:ec2:Instance'}]});await flush();
 assert.equal(t.container.querySelector('#estimate-resource').children.length,2);
 assert.match(t.container.textContent,/new comparison/);t.mounted.unmount();
});

test('ready refresh preserves edits for unchanged sources and refreshes changed preview properties', async () => {
 const t=await loaded();const input=t.container.querySelectorAll('.estimate-input')[0];input.value='unsaved';
 t.emit('ready',{});const count=t.calls.length;
 t.calls.at(-1).resolve({resources:[{id:'urn:real',type:'aws:ec2:Instance'}]});await flush();
 assert.equal(t.calls.length,count);assert.equal(t.container.querySelectorAll('.estimate-input')[0].value,'unsaved');assert.match(t.container.textContent,/Estimate calculated/);
 t.emit('ready',{});
 t.calls.at(-1).resolve({resources:[{id:'urn:real',type:'aws:ec2:Instance',properties:{instanceType:'changed-by-preview'}}]});await flush();
 assert.match(t.calls.at(-1).path,/baseline\?urn=/);t.mounted.unmount();
});

test('resource-list failures preserve valid estimates and recover independently', async () => {
 const t=await loaded(), toggle=t.container.querySelector('#estimate-show-all');
 const comparison=t.container.querySelector('.totals-bar').textContent;
 toggle.checked=true;toggle.dispatch('change');
 t.calls.at(-1).reject(new Error('List unavailable'));await flush();
 assert.equal(t.container.querySelector('.totals-bar').textContent,comparison);
 assert.match(t.container.textContent,/Estimate calculated/);
 const listError=t.container.querySelector('#estimate-resource-error');
 assert.ok(listError);assert.equal(listError.hidden,false);assert.match(listError.textContent,/List unavailable/);
 toggle.checked=false;toggle.dispatch('change');
 t.calls.at(-1).resolve({resources:[{id:'urn:real',type:'aws:ec2:Instance'}]});await flush();
 assert.equal(listError.hidden,true);assert.equal(t.container.querySelector('.totals-bar').textContent,comparison);
 assert.match(t.container.textContent,/Estimate calculated/);t.mounted.unmount();
});

test('resource-list recovery does not clear a concurrent calculation failure', async () => {
 const t=await loaded(), toggle=t.container.querySelector('#estimate-show-all');
 toggle.checked=true;toggle.dispatch('change');const listing=t.calls.at(-1);
 const input=t.container.querySelectorAll('.estimate-input')[0];input.value='4';input.dispatch('keydown',{key:'Enter'});
 t.calls.at(-1).reject(new Error('Calculation unavailable'));await flush();
 listing.reject(new Error('List unavailable'));await flush();
 const calcError=t.container.querySelector('#estimate-calculation-error');
 const listError=t.container.querySelector('#estimate-resource-error');
 assert.ok(calcError);assert.ok(listError);assert.equal(calcError.hidden,false);assert.equal(listError.hidden,false);
 assert.match(calcError.textContent,/Calculation unavailable/);assert.match(listError.textContent,/List unavailable/);
 toggle.checked=false;toggle.dispatch('change');
 t.calls.at(-1).resolve({resources:[{id:'urn:real',type:'aws:ec2:Instance'}]});await flush();
 assert.equal(listError.hidden,true);assert.equal(calcError.hidden,false);assert.match(calcError.textContent,/Calculation unavailable/);
 assert.match(t.container.textContent,/Unable to calculate estimate/);assert.equal(t.container.querySelector('.totals-bar').textContent,'');t.mounted.unmount();
});

test('a toggle overlapping preview changes refreshes the selected source and baseline', async () => {
 const t=await loaded(), toggle=t.container.querySelector('#estimate-show-all');
 t.container.querySelectorAll('.estimate-input')[0].value='stale edit';
 toggle.checked=true;toggle.dispatch('change');
 const resources=[{id:'urn:real',type:'aws:ec2:Instance',properties:{size:9}}];
 t.calls.at(-1).resolve({resources});await flush();
 const refreshed=t.calls.at(-1);assert.match(refreshed.path,/baseline\?urn=urn%3Areal/);assert.equal(refreshed.body,null);
 const result=baseline();result.display.properties=[{key:'size',originalValue:'9',currentValue:'9',delta:'$0.00 →'}];
 refreshed.resolve(result);await flush();
 const input=t.container.querySelector('.estimate-input');assert.equal(input.dataset.original,'9');assert.equal(input.value,'9');
 t.emit('ready',{});const count=t.calls.length;t.calls.at(-1).resolve({resources});await flush();
 assert.equal(t.calls.length,count);assert.equal(t.container.querySelector('.estimate-input').value,'9');t.mounted.unmount();
});

test('list recovery preserves a baseline unavailable marker from the updated API', async () => {
 const t=await loaded();
 for(const input of t.container.querySelectorAll('.estimate-input'))input.value=input.dataset.original;
 const mode=t.container.querySelector('#estimate-mode');mode.value='opaque-mode';mode.dispatch('change');
 assert.match(t.calls.at(-1).path,/baseline\?urn=/);
 t.calls.at(-1).resolve({status:'unavailable',error:'Provider unavailable',code:'estimate_unavailable',pricingModes:baseline().pricingModes,display:{properties:baseline().display.properties}});
 await flush();
 const calculationError=t.container.querySelector('#estimate-calculation-error');
 assert.equal(calculationError.hidden,false);assert.match(calculationError.textContent,/Provider unavailable/);
 const toggle=t.container.querySelector('#estimate-show-all');toggle.checked=true;toggle.dispatch('change');
 t.calls.at(-1).reject(new Error('List unavailable'));await flush();
 toggle.checked=false;toggle.dispatch('change');t.calls.at(-1).resolve({resources:[{id:'urn:real',type:'aws:ec2:Instance'}]});await flush();
 assert.equal(t.container.querySelector('#estimate-resource-error').hidden,true);
 assert.equal(calculationError.hidden,false);assert.match(calculationError.textContent,/Provider unavailable/);
 assert.equal(mode.value,'opaque-mode');assert.equal(t.container.querySelector('.totals-bar').textContent,'');
 assert.ok(t.container.querySelectorAll('.estimate-input').length>0);t.mounted.unmount();
});
test('reserved terraform metadata is not offered as an editable property',async()=>{
 const t=await setup('estimate');
 t.calls[0].resolve({resources:[{id:'aws_instance.web',type:'aws:ec2/instance:Instance'}]});await flush();
 const base=baseline();base.display.properties=[{key:'terraform:address',originalValue:'aws_instance.web',currentValue:'aws_instance.web',delta:''},{key:'size',originalValue:'2',currentValue:'2',delta:''}];
 t.calls[1].resolve(base);await flush();
 const keys=t.container.querySelectorAll('.estimate-input').map(n=>n.dataset.key);
 assert.deepEqual(keys,['size']);t.mounted.unmount();
});
