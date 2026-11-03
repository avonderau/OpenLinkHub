'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const path=require('node:path');
const context={};
vm.runInNewContext(fs.readFileSync(path.join(__dirname,'nexus-widget-library.js'),'utf8'),context);
const lib=context.NexusWidgetLibrary;
function storage(){
 const values=new Map();
 return {getItem(key){return values.get(key)||null},setItem(key,value){values.set(key,value)},values};
}
const sample={id:'sample1',name:'CPU meter',widget:{
 id:'cpu',type:'graph',metric:'cpu.utilization',x:40,y:0,width:208,height:48,
 settings:{label:'CPU',style:'outline',textColor:'#00aaff'}
}};
test('widget library persists valid entries across reads without sharing object references',()=>{
 const s=storage();
 lib.add(s,sample);
 let items=lib.list(s);
 assert.equal(items.length,1);
 assert.equal(items[0].name,'CPU meter');
 items[0].widget.settings.label='Mutated';
 assert.equal(lib.list(s)[0].widget.settings.label,'CPU');
 lib.remove(s,'sample1');
 assert.equal(lib.list(s).length,0);
});
test('instantiated widgets get independent ID, offset, and safe screen links',()=>{
 const s=storage();
 lib.add(s,sample);
 const w=lib.instantiate(lib.list(s)[0],'newWidget','screenA',['screenA']);
 assert.equal(w.id,'newWidget');
 assert.equal(w.type,'graph');
 assert.equal(w.x,56);
 assert.equal(w.y,0);
 w.settings.label='Paste';
 assert.equal(lib.list(s)[0].widget.settings.label,'CPU');
 const action={id:'link1',name:'Nav',widget:{id:'old',type:'button',x:0,y:0,width:96,height:48,action:'screen',target:'missing'}};
 lib.add(s,action);
 assert.equal(lib.instantiate(action,'button2','screenA',['screenA']).target,'screenA');
});
test('widget library rejects corrupt, oversized and duplicate entries',()=>{
 const s=storage();
 lib.add(s,sample);
 assert.throws(()=>lib.add(s,sample));
 assert.throws(()=>lib.add(s,{id:'bad/slash',name:'Unsafe',widget:sample.widget}));
 s.setItem('openlinkhub:nexus:widgets:v1','not json');
 assert.throws(()=>lib.list(s));
 s.setItem('openlinkhub:nexus:widgets:v1','a'.repeat(257*1024));
 assert.throws(()=>lib.list(s));
});
