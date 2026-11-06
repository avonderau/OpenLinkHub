'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
const root=path.join(__dirname,'..','..');
const source=fs.readFileSync(path.join(__dirname,'icon-pack-settings.js'),'utf8');
const markup=fs.readFileSync(path.join(root,'web','settings.html'),'utf8');
const deck=fs.readFileSync(path.join(__dirname,'deck-editor.js'),'utf8');
const nexus=fs.readFileSync(path.join(__dirname,'nexus-editor.js'),'utf8');
const css=fs.readFileSync(path.join(root,'static','css','openlinkhub-settings.css'),'utf8');

function fixture(){
 class Element {
  constructor(tag='div'){
   this.tag=tag;this.children=[];this.listeners={};this.value='';
   this.files=[];this.classList={toggle(){}};this.disabled=false;
   this.attributes={};this.hidden=false;this.textContent='';this.className='';
  }
  append(...children){this.children.push(...children)}
  appendChild(child){this.children.push(child)}
  replaceChildren(...children){this.children=[...children]}
  setAttribute(name,value){this.attributes[name]=value}
  addEventListener(type,fn){this.listeners[type]=fn}
 }
 const elements=Object.fromEntries([
  'olh-icon-pack-library','olh-icon-pack-file','olh-icon-pack-file-name',
  'olh-icon-pack-import','olh-icon-pack-status'
 ].map(id=>[id,new Element()]));
 const calls=[];
 const many=Array.from({length:130},(_,i)=>({
  name:i===129?'Target icon':`Icon ${i}`,tags:i===129?['special']:['generic'],
  deckId:String(i).padStart(32,'a'),nexusId:'set_'+i
 }));
 const sets=[
  {id:'a'.repeat(24),name:'Media',icons:[{name:'Play',tags:['media'],deckId:'b'.repeat(32),nexusId:'set_cc'}]},
  {id:'c'.repeat(24),name:'System',icons:many}
 ];
 const fetch=async(url,opts={})=>{
  calls.push({url,method:opts.method||'GET',body:opts.body});
  const method=opts.method||'GET';
  const id=new URL(url,'http://localhost').searchParams.get('id');
  if(method==='DELETE'){
   const index=sets.findIndex(s=>s.id===id);
   if(index<0)throw new Error('missing pack');
   sets.splice(index,1);
   return {ok:true,status:204};
  }
  if(method==='PUT'){
   const item=sets.find(s=>s.id===id);
   item.name=JSON.parse(opts.body).name;
   return {ok:true,status:200,json:async()=>({set:item})};
  }
  if(method==='POST'){
   sets.push({id:'d'.repeat(24),name:'New Pack',icons:[]});
   return {ok:true,status:201,json:async()=>({sets:[sets.at(-1)]})};
  }
  return {ok:true,status:200,json:async()=>({sets:[...sets],svgAvailable:true})};
 };
 const ctx=vm.createContext({
  document:{readyState:'complete',getElementById:id=>elements[id],createElement:tag=>new Element(tag)},
  window:{confirm:()=>true},fetch
 });
 vm.runInContext(source,ctx);
 return {elements,calls,sets};
}
const flush=async()=>{for(let i=0;i<12;i++)await Promise.resolve()};
const panes=f=>f.elements['olh-icon-pack-library'].children;
const packList=f=>panes(f)[0].children[2];
const detail=f=>panes(f)[1];
const controls=f=>detail(f).children[1];

test('Settings owns pack installation and management, not editors',()=>{
 assert.match(markup,/id="olh-icon-pack-library"/);
 assert.match(markup,/id="olh-icon-pack-import"/);
 assert.match(markup,/class="system-button olh-icon-pack-choose"/);
 assert.match(markup,/for="olh-icon-pack-file"/);
 assert.match(markup,/id="olh-icon-pack-file-name"/);
 assert.doesNotMatch(markup,/Stream Deck packs use their embedded names/);
 assert.doesNotMatch(markup,/id="olh-icon-pack-name"/);
 assert.match(markup,/id="olh-settings-devices"/);
 assert.match(markup,/id="olh-settings-tab-icons"[^>]*aria-controls="olh-settings-icons"/);
 assert.match(markup,/id="olh-settings-icons"[^>]*aria-labelledby="olh-settings-tab-icons"/);
 assert.match(nexus,/\/settings#olh-settings-icons/);
 assert.match(fs.readFileSync(path.join(root,'web','deck.html'),'utf8'),/\/settings#olh-settings-icons/);
 assert.match(css,/olh-icon-pack-sidebar/);
 assert.match(css,/olh-icon-pack-detail/);
 assert.doesNotMatch(deck,/method:\s*'POST'\s*,\s*headers:\s*\{\s*'Content-Type':\s*'application\/zip'/);
 assert.doesNotMatch(nexus,/fetch\('\/api\/icon-sets\?name=/);
 assert.match(deck,/\/api\/icon-sets/);
 assert.match(nexus,/\/api\/icon-sets/);
});
test('Manager uses left pack picker and right selected-pack details',async()=>{
 const f=fixture();await flush();
 assert.equal(panes(f).length,2);
 assert.equal(packList(f).children.length,2);
 assert.equal(packList(f).children[0].attributes['aria-current'],'true');
 assert.equal(controls(f).children[0].value,'Media');
 assert.equal(detail(f).children[0].children[0].textContent,'Media');
 await packList(f).children[1].listeners.click();
 assert.equal(packList(f).children[1].attributes['aria-current'],'true');
 assert.equal(controls(f).children[0].value,'System');
 assert.equal(detail(f).children[0].children[1].textContent,'130 icons');
});
test('Rename and delete act only on the selected pack',async()=>{
 const f=fixture();await flush();
 await packList(f).children[1].listeners.click();
 controls(f).children[0].value='Navigation';
 await controls(f).children[1].listeners.click();await flush();
 assert.equal(f.sets[1].name,'Navigation');
 assert.equal(f.sets[0].name,'Media');
 assert.equal(controls(f).children[0].value,'Navigation');
 await controls(f).children[2].listeners.click();await flush();
 assert.equal(f.sets.length,1);
 assert.equal(f.sets[0].name,'Media');
 assert.equal(controls(f).children[0].value,'Media');
 assert.ok(f.calls.some(c=>c.method==='PUT'&&c.url.includes('c'.repeat(24))));
 assert.ok(f.calls.some(c=>c.method==='DELETE'&&c.url.includes('c'.repeat(24))));
});
test('Selected pack previews are searchable, lazy, and progressively loaded',async()=>{
 const f=fixture();await flush();
 await packList(f).children[1].listeners.click();
 const search=detail(f).children[2];
 const summary=detail(f).children[3];
 const tiles=detail(f).children[4];
 const more=detail(f).children[5];
 assert.equal(tiles.children.length,120);
 assert.equal(more.hidden,false);
 assert.equal(tiles.children[0].children[0].loading,'eager');
 assert.equal(tiles.children[16].children[0].loading,'lazy');
 const loading=detail(f).children[6];
 assert.match(loading.textContent,/Loading icon previews \(0\/16\)/);
 for(let i=0;i<16;i++)tiles.children[i].children[0].listeners.load();
 assert.equal(loading.hidden,true,'first visible batch should hide spinner once loaded');
 await more.listeners.click();
 assert.equal(tiles.children.length,130);
 assert.equal(more.hidden,true);
 search.value='special';
 search.listeners.input();
 assert.equal(tiles.children.length,1);
 assert.equal(tiles.children[0].children[1].textContent,'Target icon');
 assert.match(summary.textContent,/1 of 1/);
});

test('Import omits name parameter so the backend chooses embedded or NewPackN name',async()=>{
 const f=fixture();await flush();
 const file=f.elements['olh-icon-pack-file'];
 file.files=[{name:'icons.zip',size:1024}];
 await f.elements['olh-icon-pack-import'].listeners.click();await flush();
 const post=f.calls.find(call=>call.method==='POST');
 assert.ok(post,'import should POST a ZIP');
 assert.equal(post.url,'/api/icon-sets');
 assert.equal(post.body.name,'icons.zip');
 assert.equal(file.value,'');
});

test('Choose file shows selected archive name and resets after successful import',async()=>{
 const f=fixture();await flush();
 const selected=f.elements['olh-icon-pack-file'];
 const filename=f.elements['olh-icon-pack-file-name'];
 selected.files=[{name:'fluent-ui.zip',size:4096}];
 selected.listeners.change();
 assert.equal(filename.textContent,'fluent-ui.zip');
 await f.elements['olh-icon-pack-import'].listeners.click();await flush();
 assert.equal(filename.textContent,'No file selected');
 assert.ok(f.calls.some(c=>c.method==='POST'&&c.url==='/api/icon-sets'));
});

test('Import reports progress while upload is pending and returns to idle',async()=>{
 const f=fixture();await flush();
 const file=f.elements['olh-icon-pack-file'];
 const importer=f.elements['olh-icon-pack-import'];
 const status=f.elements['olh-icon-pack-status'];
 file.files=[{name:'large-icons.zip',size:4096}];
 const operation=importer.listeners.click();
 assert.match(status.textContent,/Importing and processing icon images/);
 assert.equal(importer.disabled,true);
 assert.equal(importer.attributes['aria-busy'],'true');
 await operation;await flush();
 assert.equal(importer.disabled,false);
 assert.equal(importer.attributes['aria-busy'],'false');
 assert.match(status.textContent,/Imported New Pack/);
});
test('A new icon search resets pending load counters',async()=>{
 const f=fixture();await flush();
 await packList(f).children[1].listeners.click();
 const before=detail(f).children[6];
 assert.match(before.textContent,/0\/16/);
 const search=detail(f).children[2];
 search.value='special';
 search.listeners.input();
 const after=detail(f).children[6];
 assert.match(after.textContent,/0\/1/);
 detail(f).children[4].children[0].children[0].listeners.load();
 assert.equal(after.hidden,true);
});
