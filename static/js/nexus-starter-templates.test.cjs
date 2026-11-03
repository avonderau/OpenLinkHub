'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const {readFileSync,readdirSync}=require('node:fs');
const path=require('node:path');
const dir=path.join(__dirname,'..','nexus','templates');
const files=readdirSync(dir).filter(f=>f.endsWith('.json'));
const supportedMetrics=new Set(['cpu.temperature','cpu.utilization','gpu.temperature','gpu.utilization','memory.used','memory.utilization','network.rx_rate','network.tx_rate','disk.read_rate','disk.write_rate']);
const mediaActions=new Set(['play-pause','stop','volume-up','volume-down','next-track','previous-track']);
const presets=new Set(['preset_aurora','preset_neon','preset_circuit','preset_sunset','preset_pulse']);
test('five editable stock screens exist as JSON files',()=>assert.equal(files.length,5));
for(const file of files){
 test(file+' matches the saved Nexus screen schema',()=>{
  const data=JSON.parse(readFileSync(path.join(dir,file),'utf8'));
  assert.equal(data.version,1);
  assert.equal(data.screens.length,1);
  const screen=data.screens[0];
  assert.equal(data.activeScreenId,screen.id);
  assert.match(screen.id,/^[a-zA-Z0-9_-]{1,64}$/);
  assert.ok(screen.name&&screen.name.length<=80);
  assert.ok(presets.has(screen.background));
  assert.ok(screen.widgets.length>0&&screen.widgets.length<=64);
  const ids=new Set();
  for(const w of screen.widgets){
   assert.match(w.id,/^[a-zA-Z0-9_-]{1,64}$/);
   assert.ok(!ids.has(w.id));ids.add(w.id);
   assert.ok(['metric','bar','text','button'].includes(w.type));
   for(const key of ['x','y','width','height'])assert.ok(Number.isInteger(w[key]),key);
   assert.ok(w.x>=0&&w.y>=0&&w.width>0&&w.height>0);
   assert.ok(w.x+w.width<=640&&w.y+w.height<=48);
   assert.equal(w.x%8,0);assert.equal(w.y%8,0);
   assert.equal(w.width%8,0);assert.equal(w.height%8,0);
   assert.ok(Object.keys(w.settings||{}).length<=16);
   for(const [key,value] of Object.entries(w.settings||{})){
    assert.ok(key.length<=64&&value.length<=256);
    if(['backgroundColor','textColor','borderColor'].includes(key))assert.match(value,/^#[0-9a-fA-F]{6}$/);
   }
   if(['metric','bar'].includes(w.type))assert.ok(supportedMetrics.has(w.metric));
   if(w.type==='button'){
    assert.ok(['next','previous','screen','media','macro'].includes(w.action));
    if(w.action==='media')assert.ok(mediaActions.has(w.target));
   }
  }
  screen.widgets.forEach((a,i)=>screen.widgets.slice(i+1).forEach(b=>{
   assert.ok(!(a.x<b.x+b.width&&b.x<a.x+a.width&&a.y<b.y+b.height&&b.y<a.y+a.height),'widgets overlap');
  }));
 });
}
