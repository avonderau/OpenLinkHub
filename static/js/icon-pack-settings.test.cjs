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
function fixture(){
 class Element {
  constructor(){this.children=[];this.listeners={};this.value='';this.files=[];this.classList={toggle(){}};this.disabled=false;this.attributes={};}
  append(...children){this.children.push(...children)}
  appendChild(child){this.children.push(child)}
  replaceChildren(...children){this.children=[...children]}
  setAttribute(name,value){this.attributes[name]=value}
  addEventListener(type,fn){this.listeners[type]=fn}
 }
 const elements=Object.fromEntries([
  'olh-icon-pack-library','olh-icon-pack-name','olh-icon-pack-file',
  'olh-icon-pack-import','olh-icon-pack-status'
 ].map(id=>[id,new Element()]));
 const calls=[];
 const sets=[{id:'a'.repeat(24),name:'Media',icons:[{name:'Play',deckId:'b'.repeat(32),nexusId:'set_cc'}]}];
 const fetch=async(url,opts={})=>{
  calls.push({url,method:opts.method||'GET',body:opts.body});
  const method=opts.method||'GET';
  if(method==='DELETE'){sets.length=0;return {ok:true,status:204}}
  if(method==='PUT'){sets[0].name=JSON.parse(opts.body).name;return {ok:true,status:200,json:async()=>({set:sets[0]})}}
  if(method==='POST'){sets.push({id:'d'.repeat(24),name:'New Pack',icons:[]});return {ok:true,status:201,json:async()=>({set:sets.at(-1)})}}
  return {ok:true,status:200,json:async()=>({sets:[...sets],svgAvailable:true})};
 };
 const ctx=vm.createContext({
  document:{readyState:'complete',getElementById:id=>elements[id],createElement:()=>new Element()},
  window:{confirm:()=>true},fetch
 });
 vm.runInContext(source,ctx);
 return {elements,calls};
}
const flush=async()=>{for(let i=0;i<8;i++)await Promise.resolve()};
test('Settings owns icon pack installation and management, not either editor',()=>{
 assert.match(markup,/id="olh-icon-pack-library"/);
 assert.match(markup,/id="olh-icon-pack-import"/);
 assert.match(markup,/id="olh-settings-devices"/);
 assert.doesNotMatch(deck,/method:\s*'POST'\s*,\s*headers:\s*\{\s*'Content-Type':\s*'application\/zip'/);
 assert.doesNotMatch(nexus,/fetch\('\/api\/icon-sets\?name=/);
 assert.match(deck,/\/api\/icon-sets/);
 assert.match(nexus,/\/api\/icon-sets/);
});
test('Settings lists, renames and removes packs without touching device layouts',async()=>{
 const f=fixture();await flush();
 const card=f.elements['olh-icon-pack-library'].children[0];
 assert.ok(card,'imported pack must be visible');
 assert.equal(card.children[0].children[0].value,'Media');
 const name=card.children[0].children[0];
 name.value='Navigation';
 await card.children[2].children[0].listeners.click();await flush();
 assert.equal(f.elements['olh-icon-pack-library'].children[0].children[0].children[0].value,'Navigation');
 assert.equal(f.calls.some(c=>c.method==='PUT'&&JSON.parse(c.body).name==='Navigation'),true);
 const next=f.elements['olh-icon-pack-library'].children[0];
 await next.children[2].children[1].listeners.click();await flush();
 assert.equal(f.calls.some(c=>c.method==='DELETE'),true);
 assert.equal(f.elements['olh-icon-pack-library'].children[0].textContent,'No icon packs imported yet.');
});
