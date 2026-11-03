'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');

const editorSource=fs.readFileSync(path.join(__dirname,'nexus-editor.js'),'utf8');
const integrated=fs.readFileSync(path.join(__dirname,'..','..','web','nexus-custom-workspace.html'),'utf8');
const standalone=fs.readFileSync(path.join(__dirname,'..','nexus-editor.html'),'utf8');
const style=fs.readFileSync(path.join(__dirname,'..','css','nexus-editor.css'),'utf8');

function makeEditor(){
 const nodes=new Map();
 const writes=[];
 const persisted={version:1,activeScreenId:'existing',cyclingMode:'manual',cyclingSeconds:15,screens:[{id:'existing',name:'Existing layout',widgets:[]}]};
 function makeNode(){
  return {
   value:'',disabled:false,textContent:'',options:[],children:[],dataset:{},style:{setProperty(){}},classList:{add(){},remove(){}},listeners:{},
   addEventListener(type,callback){this.listeners[type]=callback},
   replaceChildren(...children){this.children=[...children];this.options=[]},
   append(...children){this.children.push(...children)},
   prepend(child){this.children.unshift(child)},
   add(option){this.options.push(option)},
   setAttribute(){},querySelector(){return null}
  };
 }
 function node(id){
  if(!nodes.has(id))nodes.set(id,makeNode());
  return nodes.get(id);
 }
 node('newScreenPreset').value='blank';
 node('cyclingMode').value='manual';
 node('cyclingSeconds').value='15';
 node('cyclingSeconds').options=[5,10,15,30,60,120,300,600,1800,3600].map(v=>({value:String(v)}));
 const context=vm.createContext({
  document:{
   getElementById(id){return id==='deviceId'||id==='device'?null:node(id)},
   createElement(){return makeNode()}
  },
  Option:function(label,value){this.label=label;this.value=value},
  prompt(){return 'demo-nexus'},
  confirm(){return true},
  fetch:async(url,options={})=>{
   if(url.startsWith('/static/nexus/templates/')){
    const id=url.split('/').pop();
    const file=path.join(__dirname,'..','nexus','templates',id);
    return {ok:true,json:async()=>JSON.parse(fs.readFileSync(file,'utf8'))};
   }
   if(options.method==='PUT'){
    writes.push(JSON.parse(options.body));
    return {ok:true};
   }
   return {ok:true,json:async()=>JSON.parse(JSON.stringify(persisted))};
  }
 });
 vm.runInContext(editorSource,context,{filename:'nexus-editor.js'});
 return {context,node,writes};
}

test('the integrated and standalone editors have no separate starter template section',()=>{
 for(const html of [integrated,standalone]){
  assert.doesNotMatch(html,/id="starterTemplate"|id="addStarterTemplate"|Starter Templates/);
  assert.match(html,/id="screens"/);
  assert.match(html,/id="newScreenPreset"/);
  assert.match(html,/id="replaceScreenWithPreset"/);
 }
});

test('Save Layout belongs to the integrated device settings card',()=>{
 const left=integrated.indexOf('nexus-device-column');
 const right=integrated.indexOf('nexus-editor-column');
 const save=integrated.indexOf('id="save"');
 assert.ok(left>=0 && save>left && save<right);
 assert.equal(integrated.split('id="save"').length-1,1);
 assert.match(style,/nexus-cycle-settings.*?--text-muted/s);
});

test('cycling interval is disabled except in Automatic mode and preserves saved value',async()=>{
 const {context,node,writes}=makeEditor();
 await vm.runInContext('load()',context);
 assert.equal(node('cyclingMode').value,'manual');
 assert.equal(node('cyclingSeconds').disabled,true);
 node('cyclingMode').value='automatic';
 node('cyclingMode').listeners.change();
 assert.equal(node('cyclingSeconds').disabled,false);
 node('cyclingSeconds').value='120';
 await node('save').onclick();
 assert.equal(writes.at(-1).cyclingMode,'automatic');
 assert.equal(writes.at(-1).cyclingSeconds,120);
 node('cyclingMode').value='off';
 node('cyclingMode').listeners.change();
 assert.equal(node('cyclingSeconds').disabled,true);
 await node('save').onclick();
 assert.equal(writes.at(-1).cyclingMode,'off');
 assert.equal(writes.at(-1).cyclingSeconds,120);
});

test('pre-made JSON screens are normal screens to edit, add, replace and save',async()=>{
 const {context,node,writes}=makeEditor();
 await vm.runInContext('load()',context);
 node('newScreenPreset').value='media-deck';
 node('newScreenPreset').listeners.change();
 assert.equal(node('replaceScreenWithPreset').disabled,false);
 await node('addScreen').onclick();
 let screens=vm.runInContext('cfg.screens',context);
 assert.equal(screens.length,2);
 assert.equal(screens[0].name,'Existing layout');
 assert.equal(screens[1].name,'Media Controls');
 assert.equal(screens[1].widgets.length,6);
 const addedID=screens[1].id;
 node('newScreenPreset').value='navigation';
 node('newScreenPreset').listeners.change();
 await node('replaceScreenWithPreset').listeners.click();
 screens=vm.runInContext('cfg.screens',context);
 assert.equal(screens.length,2);
 assert.equal(screens[1].id,addedID);
 assert.equal(screens[1].background,'preset_pulse');
 assert.equal(screens[1].widgets.length,3);
 await node('save').onclick();
 assert.equal(writes.at(-1).screens[1].id,addedID);
 assert.equal(writes.at(-1).screens[1].background,'preset_pulse');
});
