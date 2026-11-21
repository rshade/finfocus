import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';
import { linkView } from './view_linker.mjs';

class Node {
  constructor(tag, doc) { this.tagName=tag; this.doc=doc; this.children=[]; this.listeners=new Map(); this.attrs={}; this.dataset={}; this.hidden=false; this.value=''; }
  // Browsers strip CR and LF from a text input's value (HTML value sanitization).
  get value() { return this.rawValue; }
  set value(v) { this.rawValue=this.tagName==='input'&&(this.attrs.type??'text')==='text'?String(v).replace(/[\r\n]/g,''):v; }
  append(...children) { for (const child of children) { if (typeof child !== 'string') child.parent=this; this.children.push(child); } }
  replaceChildren(...children) { for(const child of this.children)if(typeof child!=='string')child.parent=null; this.children=[]; this.append(...children); }
  set textContent(text) { this.text=String(text); this.children=[]; }
  get textContent() { return (this.text??'')+this.children.map(c=>typeof c==='string'?c:c.textContent).join(''); }
  setAttribute(k,v) { this.attrs[k]=String(v); if(k==='id')this.id=v; if(k==='value')this.value=String(v); if(k==='class')this.className=v; }
  addEventListener(k,fn) { this.listeners.set(k,fn); }
  dispatch(k,extra={}) { this.listeners.get(k)?.({target:this,preventDefault(){},...extra}); }
  focus() { this.doc.activeElement=this; }
  setSelectionRange(start,end) { this.selectionStart=start; this.selectionEnd=end; }
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
 const mod=new vm.SourceTextModule(await readFile(new URL(`../static/views/${view}.js`,import.meta.url),'utf8'),{context});await linkView(mod,app,context,view);await mod.evaluate();
 const calls=[];const api={post(path,body,options){return request(path,body,options);},get(path,options){return request(path,null,options);}};
 function request(path,body,options){let resolve,reject;const promise=new Promise((yes,no)=>{resolve=yes;reject=no;});calls.push({path,body,options,resolve,reject});return promise;}
 const listeners=new Map();const stream={on(name,fn){listeners.set(name,fn);return()=>listeners.delete(name);}};
 const container=document.createElement('div');document.append(container);const mounted=mod.namespace.mount(container,{api,stream});
 return{container,calls,mounted,document,emit:(name,data)=>listeners.get(name)?.(data)};
}
const baseline = (label='base') => ({resource:{id:'urn:real'}, pricingMode:'', pricingModes:[{id:'opaque-mode',label:'Hourly provider',rate:'USD 30/hour',detailsDisplay:'Pricing tiers: server-formatted tier'}],display:{baseline:'USD 60 baseline',modified:'USD 60 modified',change:'$0.00 →',arrow:'→',properties:[{key:'size',originalValue:'2',currentValue:'2',delta:'$0.00 →',defaultVisible:true},{key:'label',originalValue:'old',currentValue:label,delta:'$0.00 →',defaultVisible:true}]}});
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
 call.resolve({display:{...baseline().display,baseline:'precise BASE',modified:'precise MODIFIED',change:'server delta ↑',properties:[{key:'size',originalValue:'2',currentValue:' 3 ',delta:'server property delta',defaultVisible:true}]}});await flush();assert.match(t.container.textContent,/precise BASE/);assert.match(t.container.textContent,/server delta ↑/);assert.match(t.container.textContent,/server property delta/);assert.equal(t.document.activeElement.dataset.key,'size');t.mounted.unmount();
});
test('estimate sends only the edited property when another value has newlines a text input drops',async()=>{
 const t=await setup('estimate');t.calls[0].resolve({resources:[{id:'urn:real',type:'aws:ec2:Instance'}]});await flush();
 const script='#!/bin/bash\n\nyum update -y\n';
 t.calls[1].resolve({...baseline(),display:{...baseline().display,properties:[{key:'instanceType',originalValue:'t3.micro',currentValue:'t3.micro',delta:'$0.00 →',defaultVisible:true},{key:'userData',originalValue:script,currentValue:script,delta:'$0.00 →',defaultVisible:true}]}});await flush();
 const input=t.container.querySelectorAll('.estimate-input').find(n=>n.dataset.key==='instanceType');input.value='t3.large';input.dispatch('keydown',{key:'Enter'});
 assert.deepEqual(JSON.parse(JSON.stringify(t.calls.at(-1).body.overrides)),{instanceType:'t3.large'});t.mounted.unmount();
});
test('estimate leaving an edited value recalculates once, and leaving an unchanged value does not',async()=>{
 const t=await loaded();const count=t.calls.length;const size=t.container.querySelectorAll('.estimate-input').find(n=>n.dataset.key==='size');
 size.dispatch('blur');assert.equal(t.calls.length,count);
 size.value='3';size.dispatch('blur');assert.equal(t.calls.length,count+1);
 assert.deepEqual(JSON.parse(JSON.stringify(t.calls.at(-1).body.overrides)),{size:'3',label:'base'});
 size.dispatch('blur');assert.equal(t.calls.length,count+1);
 size.value='4';size.dispatch('keydown',{key:'Enter'});size.dispatch('blur');assert.equal(t.calls.length,count+2);t.mounted.unmount();
});
test('estimate result keeps focus and unsent typing in the field the person moved to',async()=>{
 const t=await loaded();const inputs=t.container.querySelectorAll('.estimate-input');
 const size=inputs.find(n=>n.dataset.key==='size'),label=inputs.find(n=>n.dataset.key==='label');
 size.value='3';size.dispatch('blur');label.focus();label.value='typing';
 t.calls.at(-1).resolve({display:{...baseline().display,properties:[{key:'size',originalValue:'2',currentValue:'3',delta:'server size delta',defaultVisible:true},{key:'label',originalValue:'old',currentValue:'base',delta:'$0.00 →',defaultVisible:true}]}});await flush();
 const after=t.container.querySelectorAll('.estimate-input');
 assert.equal(after.find(n=>n.dataset.key==='label').value,'typing');assert.equal(after.find(n=>n.dataset.key==='size').value,'3');
 assert.equal(t.document.activeElement.dataset.key,'label');assert.match(t.container.textContent,/server size delta/);
 const count=t.calls.length;after.find(n=>n.dataset.key==='label').dispatch('blur');assert.equal(t.calls.length,count+1);
 assert.deepEqual(JSON.parse(JSON.stringify(t.calls.at(-1).body.overrides)),{size:'3',label:'typing'});t.mounted.unmount();
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
    display: { properties: [{ key: 'size', originalValue: '2', currentValue: '2', delta: '', defaultVisible: true }] },
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
      properties: [{ key: 'size', originalValue: '2', currentValue: '4', delta: '+$20.00 ↑', defaultVisible: true }],
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
 const result=baseline();result.display.properties=[{key:'size',originalValue:'9',currentValue:'9',delta:'$0.00 →',defaultVisible:true}];
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
test('estimate shows the server-formatted SPOT tag beside the priced side only',async()=>{
 const t=await setup('estimate');
 t.calls[0].resolve({resources:[{id:'urn:real',type:'aws:ec2:Instance'}]});await flush();
 const base=baseline();base.display.modifiedSpot='SPOT · risk 0.12 · -62% vs on-demand';
 t.calls[1].resolve(base);await flush();
 const badges=t.container.querySelectorAll('.spot-badge');
 assert.equal(badges.length,1);
 assert.equal(badges[0].textContent,'SPOT · risk 0.12 · -62% vs on-demand');
 assert.equal(badges[0].parent.textContent.startsWith('Modified:'),true);t.mounted.unmount();
});
const row=(key,value,extra={})=>({key,originalValue:value,currentValue:value,delta:'',class:'other',editable:true,defaultVisible:false,overridden:false,...extra});
async function withProperties(properties,display={}){
 const t=await setup('estimate');
 t.calls[0].resolve({resources:[{id:'urn:real',type:'aws:ec2/instance:Instance'}]});await flush();
 const base=baseline();base.display={...base.display,properties,details:[],curated:true,note:'',...display};
 t.calls[1].resolve(base);await flush();return t;
}
const renderedKeys=t=>t.container.querySelectorAll('.estimate-input').map(n=>n.dataset.key);
test('estimate renders only default-visible rows, in the server order',async()=>{
 const t=await withProperties([
  row('instanceType','t3.micro',{class:'pricing',provenance:'sku',defaultVisible:true}),
  row('tenancy','default',{class:'pricing',provenance:'plugin-input',defaultVisible:true}),
  row('ami','ami-1'),row('subnetId','subnet-1'),
 ]);
 assert.deepEqual(renderedKeys(t),['instanceType','tenancy']);t.mounted.unmount();
});
test('estimate renders a terraform-prefixed row exactly as the server sent it',async()=>{
 const t=await withProperties([row('terraform:address','aws_instance.web',{defaultVisible:true}),row('size','2',{defaultVisible:true})]);
 assert.deepEqual(renderedKeys(t),['terraform:address','size']);t.mounted.unmount();
});
test('Show all properties reveals the other rows with no request and hides them again',async()=>{
 const t=await withProperties([
  row('instanceType','t3.micro',{class:'pricing',defaultVisible:true}),row('ami','ami-1'),row('subnetId','subnet-1'),
 ]);
 const toggle=t.container.querySelector('#estimate-show-advanced');assert.ok(toggle);
 assert.match(t.container.textContent,/Show all properties/);
 const count=t.calls.length;
 toggle.checked=true;toggle.dispatch('change');
 assert.deepEqual(renderedKeys(t),['instanceType','ami','subnetId']);assert.equal(t.calls.length,count);
 toggle.checked=false;toggle.dispatch('change');
 assert.deepEqual(renderedKeys(t),['instanceType']);assert.equal(t.calls.length,count);t.mounted.unmount();
});
test('the fallback note shows in the status line only when the type is not curated',async()=>{
 const note='Pricing inputs are not curated for this resource type; all safe properties are shown.';
 const fallback=await withProperties([row('engine','mysql',{defaultVisible:true})],{curated:false,note});
 assert.match(fallback.container.textContent,/not curated for this resource type/);fallback.mounted.unmount();
 const curated=await withProperties([row('instanceType','t3.micro',{class:'pricing',defaultVisible:true})]);
 assert.doesNotMatch(curated.container.textContent,/not curated/);
 assert.match(curated.container.textContent,/Estimate calculated/);curated.mounted.unmount();
});
test('a usage row sends a value and sends nothing while empty', async () => {
 const t = await withProperties([
  row('instanceType', 't3.micro', { class: 'pricing', defaultVisible: true }),
  row('requestsPerMonth', '', { class: 'pricing', provenance: 'plugin-input', defaultVisible: true, usage: true }),
 ]);
 const usage = t.container.querySelectorAll('.estimate-input').find((n) => n.dataset.key === 'requestsPerMonth');
 assert.equal(usage.value, '');
 usage.dispatch('keydown', { key: 'Enter' });
 assert.equal(t.calls.at(-1).body, null, 'an empty usage row sends no override');
 assert.equal(String(t.calls.at(-1).path).includes('requestsPerMonth'), false);
 usage.value = '1000000';
 usage.dispatch('keydown', { key: 'Enter' });
 assert.deepEqual(JSON.parse(JSON.stringify(t.calls.at(-1).body.overrides)), { requestsPerMonth: '1000000' });
 usage.value = '';
 usage.dispatch('keydown', { key: 'Enter' });
 assert.equal(t.calls.at(-1).path.startsWith('/api/estimate/baseline'), true);
 t.mounted.unmount();
});
test('a mode change keeps a removed usage value on the returned row', async () => {
 const t = await withProperties([
  row('instanceType', 't3.micro', { class: 'pricing', defaultVisible: true }),
  row('tenancy', '', { class: 'pricing', provenance: 'plugin-input', defaultVisible: true, usage: true }),
 ]);
 const tenancy = t.container.querySelectorAll('.estimate-input').find((n) => n.dataset.key === 'tenancy');
 tenancy.value = 'dedicated';
 tenancy.focus();
 const mode = t.container.querySelector('#estimate-mode');
 mode.value = 'opaque-mode';
 mode.dispatch('change');
 t.calls.at(-1).resolve({
  display: {
   ...baseline().display,
   properties: [
    row('instanceType', 't3.micro', { class: 'pricing', defaultVisible: true }),
    row('tenancy', '', { class: 'other', defaultVisible: true, overridden: true, currentValue: 'dedicated' }),
   ],
  },
  pricingMode: 'opaque-mode',
 });
 await flush();
 const kept = t.container.querySelectorAll('.estimate-input').find((n) => n.dataset.key === 'tenancy');
 assert.equal(kept.value, 'dedicated');
 assert.equal(t.document.activeElement.dataset.key, 'tenancy');
 t.mounted.unmount();
});
test('an uncurated response shows every row and the note without another request', async () => {
 const note = 'Pricing inputs are not curated for this resource type; all safe properties are shown.';
 const t = await withProperties([
  row('acl', 'private', { defaultVisible: true }),
 ], { curated: false, note });
 assert.deepEqual(renderedKeys(t), ['acl']);
 assert.match(t.container.textContent, /not curated/);
 const count = t.calls.length;
 t.container.querySelector('#estimate-show-advanced').dispatch('change');
 assert.equal(t.calls.length, count);
 t.mounted.unmount();
});
test('Show all properties is separate from Show all resources',async()=>{
 const t=await withProperties([row('instanceType','t3.micro',{class:'pricing',defaultVisible:true}),row('ami','ami-1')]);
 const advanced=t.container.querySelector('#estimate-show-advanced'),resources=t.container.querySelector('#estimate-show-all');
 assert.notEqual(advanced,resources);const count=t.calls.length;
 advanced.checked=true;advanced.dispatch('change');assert.equal(t.calls.length,count);
 resources.checked=true;resources.dispatch('change');assert.equal(t.calls.at(-1).path,'/api/estimate/resources?showAll=true');t.mounted.unmount();
});
const detailsOf=t=>t.container.querySelector('.estimate-details');
test('identity details render as a read-only list with no input, and never enter overrides',async()=>{
 const t=await withProperties([row('instanceType','t3.micro',{class:'pricing',defaultVisible:true})],
  {details:[{key:'arn',value:'arn:aws:ec2:us-east-1:1:instance/i-1'},{key:'pulumi:cloudId',value:'i-1'}]});
 const details=detailsOf(t);assert.ok(details);assert.equal(details.tagName,'dl');
 assert.equal(details.querySelectorAll('input').length,0);
 assert.deepEqual(details.querySelectorAll('dt').map(n=>n.textContent),['arn','pulumi:cloudId']);
 assert.deepEqual(details.querySelectorAll('dd').map(n=>n.textContent),['arn:aws:ec2:us-east-1:1:instance/i-1','i-1']);
 assert.deepEqual(renderedKeys(t),['instanceType']);
 const input=t.container.querySelector('.estimate-input');input.value='t3.large';input.dispatch('keydown',{key:'Enter'});
 assert.deepEqual(JSON.parse(JSON.stringify(t.calls.at(-1).body.overrides)),{instanceType:'t3.large'});t.mounted.unmount();
});
test('the identity details list is hidden when the resource has none',async()=>{
 const t=await withProperties([row('instanceType','t3.micro',{class:'pricing',defaultVisible:true})]);
 assert.equal(detailsOf(t).hidden,true);t.mounted.unmount();
});
test('a failed baseline still shows identity details',async()=>{
 const t=await setup('estimate');
 t.calls[0].resolve({resources:[{id:'urn:real',type:'aws:ec2/instance:Instance'}]});await flush();
 t.calls[1].resolve({status:'unavailable',error:'estimate data unavailable',code:'estimate_unavailable',pricingModes:[],
  display:{properties:[row('instanceType','t3.micro',{class:'pricing',defaultVisible:true})],details:[{key:'id',value:'i-1'}],curated:true,note:''}});await flush();
 assert.deepEqual(detailsOf(t).querySelectorAll('dt').map(n=>n.textContent),['id']);t.mounted.unmount();
});
const twoEditable=()=>withProperties([
 row('instanceType','t3.micro',{class:'pricing',defaultVisible:true}),row('ami','ami-1'),row('subnetId','subnet-1'),
]);
const inputFor=(t,key)=>t.container.querySelectorAll('.estimate-input').find(n=>n.dataset.key===key);
test('a toggle round trip keeps values, overrides and deltas',async()=>{
 const t=await twoEditable();const toggle=t.container.querySelector('#estimate-show-advanced');
 toggle.checked=true;toggle.dispatch('change');
 inputFor(t,'ami').value='ami-2';inputFor(t,'ami').dispatch('keydown',{key:'Enter'});
 t.calls.at(-1).resolve({display:{...baseline().display,properties:[
  row('instanceType','t3.micro',{class:'pricing',defaultVisible:true,delta:'+$5.00 ↑'}),
  row('ami','ami-1',{currentValue:'ami-2',overridden:true,defaultVisible:true,delta:'+$1.00 ↑'}),row('subnetId','subnet-1')],details:[],curated:true,note:''}});await flush();
 toggle.checked=false;toggle.dispatch('change');toggle.checked=true;toggle.dispatch('change');toggle.checked=false;toggle.dispatch('change');
 assert.deepEqual(renderedKeys(t),['instanceType','ami']);
 assert.equal(inputFor(t,'ami').value,'ami-2');
 assert.match(t.container.textContent,/\+\$5\.00 ↑/);assert.match(t.container.textContent,/\+\$1\.00 ↑/);
 inputFor(t,'instanceType').value='t3.large';inputFor(t,'instanceType').dispatch('keydown',{key:'Enter'});
 assert.deepEqual(JSON.parse(JSON.stringify(t.calls.at(-1).body.overrides)),{ami:'ami-2',instanceType:'t3.large'});t.mounted.unmount();
});
test('a row with an edit in flight stays visible when the toggle goes off, before the server answers',async()=>{
 const t=await twoEditable();const toggle=t.container.querySelector('#estimate-show-advanced');
 toggle.checked=true;toggle.dispatch('change');
 inputFor(t,'ami').value='ami-2';inputFor(t,'ami').dispatch('blur');
 toggle.checked=false;toggle.dispatch('change');
 assert.deepEqual(renderedKeys(t),['instanceType','ami']);assert.equal(inputFor(t,'ami').value,'ami-2');
 inputFor(t,'instanceType').value='t3.large';inputFor(t,'instanceType').dispatch('keydown',{key:'Enter'});
 assert.deepEqual(JSON.parse(JSON.stringify(t.calls.at(-1).body.overrides)),{ami:'ami-2',instanceType:'t3.large'});t.mounted.unmount();
});
test('unsent typing in an advanced row survives toggling off',async()=>{
 const t=await twoEditable();const toggle=t.container.querySelector('#estimate-show-advanced');
 toggle.checked=true;toggle.dispatch('change');inputFor(t,'subnetId').value='typing';
 toggle.checked=false;toggle.dispatch('change');
 assert.equal(inputFor(t,'subnetId').value,'typing');t.mounted.unmount();
});
test('focus and selection stay on the same key across a toggle and a recalculation',async()=>{
 const t=await twoEditable();const toggle=t.container.querySelector('#estimate-show-advanced');
 const first=inputFor(t,'instanceType');first.focus();first.selectionStart=1;first.selectionEnd=3;
 toggle.checked=true;toggle.dispatch('change');
 assert.equal(t.document.activeElement.dataset.key,'instanceType');
 assert.deepEqual([t.document.activeElement.selectionStart,t.document.activeElement.selectionEnd],[1,3]);
 const ami=inputFor(t,'ami');ami.focus();ami.selectionStart=0;ami.selectionEnd=2;
 inputFor(t,'instanceType').value='t3.large';inputFor(t,'instanceType').dispatch('blur');
 t.calls.at(-1).resolve({display:{...baseline().display,properties:[
  row('instanceType','t3.micro',{class:'pricing',defaultVisible:true,currentValue:'t3.large',overridden:true}),row('ami','ami-1'),row('subnetId','subnet-1')],details:[],curated:true,note:''}});await flush();
 assert.equal(t.document.activeElement.dataset.key,'ami');
 assert.deepEqual([t.document.activeElement.selectionStart,t.document.activeElement.selectionEnd],[0,2]);t.mounted.unmount();
});
test('a late response after a toggle keeps the toggle and rows the person chose',async()=>{
 const t=await twoEditable();const toggle=t.container.querySelector('#estimate-show-advanced');
 inputFor(t,'instanceType').value='t3.large';inputFor(t,'instanceType').dispatch('keydown',{key:'Enter'});const pending=t.calls.at(-1);
 toggle.checked=true;toggle.dispatch('change');
 pending.resolve({display:{...baseline().display,properties:[
  row('instanceType','t3.micro',{class:'pricing',defaultVisible:true,currentValue:'t3.large',overridden:true}),row('ami','ami-1'),row('subnetId','subnet-1')],details:[],curated:true,note:''}});await flush();
 assert.equal(toggle.checked,true);assert.deepEqual(renderedKeys(t),['instanceType','ami','subnetId']);t.mounted.unmount();
});
