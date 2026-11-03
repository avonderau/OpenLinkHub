'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
const source=fs.readFileSync(path.join(__dirname,'nexus-layout-files.js'),'utf8');
const globals={};vm.runInNewContext(source,globals,{filename:'nexus-layout-files.js'});
const layouts=globals.NexusLayoutFiles;
const template=JSON.parse(fs.readFileSync(path.join(__dirname,'..','nexus','templates','media-deck.json'),'utf8'));
const clone=obj=>JSON.parse(JSON.stringify(obj));

function existingLayout(){
 return {version:1,activeScreenId:'mine',cyclingMode:'automatic',cyclingSeconds:15,
  screens:[{id:'mine',name:'My screen',widgets:[{id:'self',type:'button',x:0,y:0,width:100,height:40,action:'screen',target:'mine'}]}]};
}

test('stock Nexus JSON files are accepted by the import validator',()=>{
 for(const filename of ['media-deck.json','navigation.json','system-overview.json','performance-bars.json','network-and-storage.json']){
  const data=JSON.parse(fs.readFileSync(path.join(__dirname,'..','nexus','templates',filename),'utf8'));
  assert.equal(layouts.validate(data).version,1);
 }
});
test('single-screen export is portable and fixes dangling navigation references',()=>{
 const original=existingLayout();
 original.screens.push({id:'other',name:'Other',widgets:[{id:'switch',type:'button',x:0,y:0,width:120,height:40,action:'screen',target:'mine'}]});
 const exportFile=layouts.exportSingle(original,'other');
 assert.equal(exportFile.screens.length,1);
 assert.equal(exportFile.activeScreenId,'other');
 assert.equal(exportFile.screens[0].widgets[0].target,'other');
 assert.equal(original.screens[1].widgets[0].target,'mine','export mutated source');
});
test('adding an imported screen remaps its ID and leaves existing screens unchanged',()=>{
 const original=existingLayout();
 const next=layouts.applyImport(original,template,'add-screen','mine','imported123');
 assert.equal(next.screens.length,2);
 assert.equal(next.activeScreenId,'imported123');
 assert.equal(next.screens[0].name,'My screen');
 assert.equal(next.screens[1].id,'imported123');
 assert.equal(original.screens.length,1);
});
test('replacing one screen preserves its ID and other navigation targets',()=>{
 const original=existingLayout();
 const next=layouts.applyImport(original,template,'replace-screen','mine','');
 assert.equal(next.screens.length,1);
 assert.equal(next.screens[0].id,'mine');
 assert.equal(next.screens[0].name,'Media Controls');
 assert.equal(original.screens[0].name,'My screen');
});
test('full layout replacement preserves incoming cycling mode until user saves',()=>{
 const next=layouts.applyImport(existingLayout(),clone(template),'replace-layout','mine','');
 assert.equal(next.screens.length,1);
 assert.equal(next.activeScreenId,'starter_media_deck');
});
test('malformed, oversize, unsupported and unsafe data is rejected before import',()=>{
 const bad=[
  null,{}, {version:2,screens:[]},
  {...existingLayout(),activeScreenId:'lost'},
  {...existingLayout(),screens:Array.from({length:33},(_,i)=>({id:'s'+i,name:'x',widgets:[]}))},
 ];
 for(const value of bad)assert.throws(()=>layouts.validate(value));
 const overflow=existingLayout();overflow.screens[0].widgets[0].x=600;
 assert.throws(()=>layouts.validate(overflow));
 const broken=existingLayout();broken.screens[0].widgets[0].target='missing';
 assert.throws(()=>layouts.validate(broken));
 const fake=existingLayout();fake.screens[0].widgets[0].action='shell';
 assert.throws(()=>layouts.validate(fake));
 const asset=existingLayout();asset.screens[0].background='../escape';
 assert.throws(()=>layouts.validate(asset));
 const collision=existingLayout();assert.throws(()=>layouts.applyImport(collision,template,'add-screen','mine','mine'));
 assert.throws(()=>layouts.applyImport(collision,template,'add-screen','mine','../escape'));
 const accepted=layouts.applyImport(collision,template,'add-screen','mine','new');
 assert.equal(accepted.screens.length,2);
});
