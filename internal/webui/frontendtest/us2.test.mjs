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
 const context=vm.createContext({document,AbortController,console,setTimeout,clearTimeout});
 const app=new vm.SyntheticModule(['el'],function(){this.setExport('el',(tag,attrs={},...children)=>{const node=document.createElement(tag);for(const [k,v]of Object.entries(attrs)){if(k==='text')node.textContent=v;else if(k==='class')node.className=v;else if(k==='dataset')Object.assign(node.dataset,v);else node.setAttribute(k,v);}node.append(...children.flat());return node;});},{context});await app.link(()=>{});await app.evaluate();
 const mod=new vm.SourceTextModule(await readFile(new URL(`../static/views/${view}.js`,import.meta.url),'utf8'),{context});await mod.link(()=>app);await mod.evaluate();
 const calls=[];const api={post(path,body,options){return request(path,body,options);},get(path,options){return request(path,null,options);}};
 function request(path,body,options){let resolve,reject;const promise=new Promise((yes,no)=>{resolve=yes;reject=no;});calls.push({path,body,options,resolve,reject});return promise;}
 const listeners=new Map();const stream={on(name,fn){listeners.set(name,fn);return()=>listeners.delete(name);}};
 const container=document.createElement('div');document.append(container);const mounted=mod.namespace.mount(container,{api,stream});
 return{container,calls,mounted,document,emit:(name,data)=>listeners.get(name)?.(data)};
}
const costPage=text=>({rows:[{id:text,type:'aws:ec2:Instance',costDisplay:'$12.34',currency:'USD',trendPoints:'0,28 12,0',canDetail:true}],results:[],totalDisplay:'$12.34',summary:{currency:'USD'},page:1,totalPages:1});
const recPage=text=>({items:[{id:text,resourceId:text,type:'Resize',description:'Smaller',status:'Active'}],itemSavings:['12.34 USD'],summary:{total_count:1},savingsDisplay:'12.34 USD',actions:[],page:1,totalPages:1});

test('cost grouping and tag changes refetch; date alias is absent',async()=>{
 const t=await setup('cost');assert.equal(t.calls[0].path,'/api/cost/actual/query');
 const group=t.container.querySelector('#cost-group');assert.deepEqual(group.children.map(n=>n.attrs.value),['','resource','type','provider','daily','monthly']);
 group.value='provider';group.dispatch('change');assert.equal(t.calls.at(-1).body.groupBy,'provider');
 const tag=t.container.querySelector('#cost-tag');tag.value='env=prod';t.container.querySelector('form').dispatch('submit');assert.equal(t.calls.at(-1).body.tag,'env=prod');
 t.mounted.unmount();assert.equal(t.calls.at(-1).options.signal.aborted,true);
});

test('cost stale results cannot replace newer query and loading retries on ready',async()=>{
 const t=await setup('cost');t.calls[0].reject(Object.assign(new Error('loading'),{status:503}));await flush();t.emit('ready',{});assert.equal(t.calls.length,2);
 const group=t.container.querySelector('#cost-group');group.value='type';group.dispatch('change');
 t.calls.at(-1).resolve(costPage('newest'));await flush();t.calls[1].resolve(costPage('stale'));await flush();assert.match(t.container.textContent,/newest/);assert.doesNotMatch(t.container.textContent,/stale/);t.mounted.unmount();
});

test('recommendations include dismissed refetches and renders server summary',async()=>{
 const t=await setup('recommendations');const toggle=t.container.querySelector('#recommendations-dismissed');toggle.checked=true;toggle.dispatch('change');assert.equal(t.calls.at(-1).body.includeDismissed,true);
 t.calls.at(-1).resolve(recPage('active'));await flush();assert.match(t.container.textContent,/12.34 USD/);assert.equal(t.container.querySelectorAll('button').some(n=>/^(Dismiss|Undismiss)$/.test(n.textContent)),false);t.mounted.unmount();
});

test('recommendation detail restores focus and ignores late results after close',async()=>{
 const t=await setup('recommendations');t.calls[0].resolve(recPage('rec'));await flush();
 const launcher=t.container.querySelector('.detail-button');launcher.focus();launcher.dispatch('click');assert.match(t.calls.at(-1).path,/item\?id=rec/);
 const dialog=t.container.querySelector('dialog');dialog.close();assert.equal(t.document.activeElement,launcher);
 t.calls.at(-1).resolve({item:{description:'late'},savingsDisplay:'10.00 USD'});await flush();assert.doesNotMatch(dialog.textContent,/late/);t.mounted.unmount();
});

for (const view of ['cost','recommendations']) {
 test(`${view} detail close focuses the replacement launcher or filter`,async()=>{
  const t=await setup(view);const makePage=view==='cost'?costPage:recPage;
  t.calls[0].resolve(makePage('same'));await flush();const launcher=t.container.querySelector('.detail-button');launcher.dispatch('click');
  t.emit('ready',{});t.calls.at(-1).resolve(makePage('same'));await flush();const replacement=t.container.querySelector('.detail-button');t.container.querySelector('dialog').close();assert.equal(t.document.activeElement,replacement);
  replacement.dispatch('click');t.emit('ready',{});t.calls.at(-1).resolve(view==='cost'?{...costPage('x'),rows:[]}:{...recPage('x'),items:[]});await flush();t.container.querySelector('dialog').close();assert.equal(t.document.activeElement,t.container.querySelector(`#${view}-filter`));t.mounted.unmount();
 });
}

test('cost detail keeps successful rendered group and tag during pending edits',async()=>{
 const t=await setup('cost');t.calls[0].resolve(costPage('initial'));await flush();
 const group=t.container.querySelector('#cost-group');const tag=t.container.querySelector('#cost-tag');group.value='provider';tag.value='env=prod';group.dispatch('change');t.calls.at(-1).resolve(costPage('grouped'));await flush();
 group.value='type';tag.value='env=next';group.dispatch('change');const launcher=t.container.querySelector('.detail-button');launcher.dispatch('click');
 let path=new URL(t.calls.at(-1).path,'http://localhost');assert.equal(path.searchParams.get('groupBy'),'provider');assert.equal(path.searchParams.get('tag'),'env=prod');
 t.container.querySelector('dialog').close();tag.value='env=uncommitted';launcher.dispatch('click');path=new URL(t.calls.at(-1).path,'http://localhost');assert.equal(path.searchParams.get('tag'),'env=prod');t.mounted.unmount();
});

test('time aggregates offer cost and period sorts only',async()=>{
 const t=await setup('cost');const sort=t.container.querySelector('#cost-sort');sort.value='delta';const group=t.container.querySelector('#cost-group');group.value='daily';group.dispatch('change');assert.equal(t.calls.at(-1).body.sort,'cost');assert.equal(sort.children.find(n=>n.attrs.value==='type').disabled,true);assert.equal(sort.children.find(n=>n.attrs.value==='delta').disabled,true);assert.equal(sort.children.find(n=>n.attrs.value==='name').disabled,false);t.mounted.unmount();
});

test('cost renders shared summary, provider subtotals and complete formatted detail',async()=>{
 const t=await setup('cost');t.calls[0].resolve({...costPage('vm'),summaryDisplay:{totalDisplay:'$12.35',resourceCount:2,recommendationCount:3,providers:[{name:'aws',costDisplay:'$12.35',shareDisplay:'100.0%'}],carbonEquivalency:'Equivalent to 2 miles'},rows:[{...costPage('vm').rows[0],providersDisplay:'aws:$12'}]});await flush();
 for(const text of ['Resources: 2','Recommendations: 3','aws: $12.35 (100.0%)','Equivalent to 2 miles','aws:$12'])assert.ok(t.container.textContent.includes(text),text);
 t.container.querySelector('.detail-button').dispatch('click');t.calls.at(-1).resolve({result:{resourceId:'vm',resourceType:'aws:ec2:Instance'},provider:'aws',periodDisplay:'2026-10-01 - 2026-10-03',monthlyDisplay:'$12.35 USD',hourlyDisplay:'$0.0169 USD',deltaDisplay:'+$2.00 ↑',breakdown:[{name:'compute',costDisplay:'$12.3456'}],sustainabilityDisplay:[{name:'carbon',value:'0.23 kgCO2e'}],recommendations:[{recommendation:{type:'RIGHTSIZE',description:'Smaller',reasoning:['Verify memory']},savingsDisplay:'$2.00 USD'}],notesDisplay:'Provider detail error'});await flush();
 for(const text of ['aws:ec2:Instance','Provider: aws','2026-10-01 - 2026-10-03','$0.0169 USD','+$2.00 ↑','$12.3456','0.23 kgCO2e','RIGHTSIZE','Verify memory','Provider detail error'])assert.ok(t.container.querySelector('dialog').textContent.includes(text),text);t.mounted.unmount();
});

test('recommendations render shared action labels and exact scorer strings',async()=>{
 const t=await setup('recommendations');t.calls[0].resolve({...recPage('rec'),itemActions:['Rightsize'],actions:[{action:'RAW_ENUM',actionDisplay:'Rightsize',count:1,savingsDisplay:'$2.00 USD'}]});await flush();assert.match(t.container.textContent,/Rightsize/);assert.doesNotMatch(t.container.textContent,/RAW_ENUM/);
 t.container.querySelector('.detail-button').dispatch('click');t.calls.at(-1).resolve({item:{description:'Smaller',scores:{risk:0.2}},actionDisplay:'Rightsize',savingsDisplay:'$2.00 USD',scoreDisplay:[{name:'Risk',value:'0.20'},{name:'False positive',value:'-'}]});await flush();assert.match(t.container.querySelector('dialog').textContent,/Risk0.20False positive-/);t.mounted.unmount();
});
