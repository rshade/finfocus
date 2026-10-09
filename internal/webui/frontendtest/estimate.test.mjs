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
test('estimate resource loading retries on ready and late responses after unmount are ignored',async()=>{
 const t=await setup('estimate');t.calls[0].reject(Object.assign(new Error('loading'),{status:503}));await flush();t.emit('ready',{});assert.equal(t.calls.length,2);t.mounted.unmount();const before=t.container.textContent;t.calls[1].resolve({resources:[{id:'late',type:'aws:ec2:Instance'}]});await flush();assert.equal(t.container.textContent,before);assert.equal(t.calls[1].options.signal.aborted,true);
});
