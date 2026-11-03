'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const {runInNewContext}=require('node:vm');
const {readFileSync}=require('node:fs');
const path=require('node:path');

const source=readFileSync(path.join(__dirname,'nexus-starter-templates.js'),'utf8');
const context={};
runInNewContext(source,context,{filename:'nexus-starter-templates.js'});
const templates=JSON.parse(JSON.stringify(context.NEXUS_STARTER_TEMPLATES));
const supportedMetrics=new Set([
 'cpu.temperature','cpu.utilization','gpu.temperature','gpu.utilization',
 'memory.used','memory.utilization','network.rx_rate','network.tx_rate',
 'disk.read_rate','disk.write_rate'
]);
const mediaActions=new Set(['play-pause','stop','volume-up','volume-down','next-track','previous-track']);
const actions=new Set(['next','previous','screen','media','macro']);

test('starter screen template collection is present and uniquely named',()=>{
 assert.ok(templates.length>=4);
 assert.equal(new Set(templates.map(t=>t.id)).size,templates.length);
 assert.equal(new Set(templates.map(t=>t.name)).size,templates.length);
});

for(const template of templates){
 test(template.name+' fits the Nexus configuration schema',()=>{
  assert.match(template.id,/^[a-zA-Z0-9_-]{1,64}$/);
  assert.ok(template.name.length>0&&template.name.length<=80);
  assert.ok(template.widgets.length>0&&template.widgets.length<=64);
  for(const widget of template.widgets){
   assert.ok(['metric','text','bar','button'].includes(widget.type));
   for(const prop of ['x','y','width','height'])assert.ok(Number.isInteger(widget[prop]),prop);
   assert.ok(widget.x>=0&&widget.y>=0);
   assert.ok(widget.width>0&&widget.height>0);
   assert.ok(widget.x+widget.width<=640);
   assert.ok(widget.y+widget.height<=48);
   assert.ok(widget.x%8===0&&widget.y%8===0);
   assert.ok(widget.width%8===0&&widget.height%8===0);
   assert.ok(Object.keys(widget.settings||{}).length<=16);
   for(const [key,value] of Object.entries(widget.settings||{})){
    assert.ok(key.length<=64);
    assert.ok(value.length<=256);
    if(['backgroundColor','textColor','borderColor'].includes(key))assert.match(value,/^#[0-9a-fA-F]{6}$/);
   }
   if(['metric','bar'].includes(widget.type))assert.ok(supportedMetrics.has(widget.metric));
   if(widget.type==='button'){
    assert.ok(actions.has(widget.action));
    if(widget.action==='media')assert.ok(mediaActions.has(widget.target));
   }
  }
  for(let i=0;i<template.widgets.length;i++){
   const a=template.widgets[i];
   for(let j=i+1;j<template.widgets.length;j++){
    const b=template.widgets[j];
    const overlap=a.x<b.x+b.width && b.x<a.x+a.width && a.y<b.y+b.height && b.y<a.y+a.height;
    assert.equal(overlap,false,'overlapping widgets '+i+' and '+j);
   }
  }
 });
}
