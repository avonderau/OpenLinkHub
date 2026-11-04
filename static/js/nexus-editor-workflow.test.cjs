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
   setAttribute(key,value){this.attributes??={};this.attributes[key]=value},querySelector(){return null}
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


test('Nexus uses existing saved macros and does not expose the old macro builder',async()=>{
 const {context,node}=makeEditor();
 await vm.runInContext('load()',context);
 vm.runInContext("nexusSharedMacros={'7':{name:'Desktop macro'}}",context);
 node('addButton').listeners.click();
 const widget=vm.runInContext('cfg.screens[0].widgets.at(-1)',context);
 const find=(root,predicate)=>{
  if(predicate(root))return root;
  for(const child of root.children||[]){if(child&&typeof child==='object'){const hit=find(child,predicate);if(hit)return hit}}
  return null;
 };
 const action=find(node('properties'),el=>el.options?.some(option=>option.label==='Run saved macro'));
 assert.ok(action,'macro must be a button action');
 action.value='macro';
 (action.listeners.change||action.onchange)();
 assert.equal(widget.type,'button');
 assert.equal(widget.action,'macro');
 assert.equal(widget.target,'7');
 assert.equal(widget.steps,undefined);
 assert.equal(find(node('properties'),el=>el.className==='nexus-macro-editor'),null);
 const choice=find(node('properties'),el=>el.options?.some(option=>option.label==='Desktop macro'));
 assert.ok(choice,'saved OpenLinkHub macro appears in Nexus widget settings');
 const link=find(node('properties'),el=>el.href==='/macros');
 assert.ok(link,'Nexus links to the existing macro editor');
});


test('screen order and cross-screen widget clipboard survive Save Layout',async()=>{
 const {context,node,writes}=makeEditor();
 await vm.runInContext('load()',context);
 node('newScreenPreset').value='media-deck';
 await node('addScreen').onclick();
 const screens=vm.runInContext('cfg.screens',context);
 const added=screens[1].id;
 assert.equal(node('moveScreenUp').disabled,false);
 node('moveScreenUp').listeners.click();
 assert.equal(screens[0].id,added);
 assert.equal(screens[1].id,'existing');
 const sourceScreen=screens[0];const source=sourceScreen.widgets[0];
 vm.runInContext("selected=cfg.screens[0].widgets[0].id;render()",context);
 node('copyWidget').listeners.click();
 node('screens').value='existing';node('screens').onchange();
 assert.equal(node('nexusInspectorActions').hidden,false,'Paste must remain accessible without a selected widget');
 assert.equal(node('pasteWidget').disabled,false);
 node('pasteWidget').listeners.click();
 assert.equal(screens[1].widgets.length,1);
 const pasted=screens[1].widgets[0];
 assert.notEqual(pasted.id,source.id);
 assert.equal(pasted.action,source.action);
 pasted.settings.label='Changed copy';
 assert.notEqual(source.settings.label,'Changed copy');
 await node('save').onclick();
 assert.equal(writes.at(-1).screens[0].id,added);
 assert.equal(writes.at(-1).screens[1].widgets.length,1);
});


test('keyboard layout editing selects, moves, resizes and deselects widgets',async()=>{
 const {context,node}=makeEditor();
 await vm.runInContext('load()',context);
 const screen=vm.runInContext('cfg.screens[0]',context);
 screen.widgets.push({id:'keyboard_widget',type:'text',x:0,y:0,width:120,height:32,settings:{text:'Keyboard'}});
 vm.runInContext('render()',context);
 const key=(name,shiftKey=false)=>{
  let prevented=false;
  node('screen').listeners.keydown({target:node('screen'),key:name,shiftKey,ctrlKey:false,altKey:false,metaKey:false,preventDefault(){prevented=true}});
  return prevented;
 };
 assert.equal(key(']'),true);
 assert.equal(vm.runInContext('selected',context),'keyboard_widget');
 assert.equal(key('ArrowRight'),true);
 assert.equal(screen.widgets[0].x,8);
 assert.equal(key('ArrowDown',true),true);
 assert.equal(screen.widgets[0].height,40);
 assert.equal(key('Escape'),true);
 assert.equal(vm.runInContext('selected',context),null);
 assert.equal(key('ArrowRight'),false);
});


test('software dimming remains full by default and saves a selected level',async()=>{
 const {context,node,writes}=makeEditor();
 await vm.runInContext('load()',context);
 assert.equal(node('displayBrightness').value,'100');
 node('displayBrightness').value='40';
 node('displayBrightness').listeners.change();
 assert.equal(vm.runInContext('cfg.brightness',context),40);
 await node('save').onclick();
 assert.equal(writes.at(-1).brightness,40);
});


test('image widgets are separate from touch buttons and expose image upload',async()=>{
 const {context,node}=makeEditor();
 await vm.runInContext('load()',context);
 node('addContent').listeners.click();
 const selectType=(root)=>{if(root.options?.some(x=>x.label==='Image'))return root;for(const child of root.children||[]){if(child&&typeof child==='object'){const hit=selectType(child);if(hit)return hit}}return null};
 const contentType=selectType(node('nexusDisplayTypeMount'));
 assert.ok(contentType);
 contentType.value='image';
 contentType.listeners.change();
 const widget=vm.runInContext('cfg.screens[0].widgets.at(-1)',context);
 assert.equal(widget.type,'image');
 assert.equal(widget.settings.style,'flat');
 assert.equal(widget.settings.icon,'custom');
 assert.equal(widget.metric,'');
 const find=(root,predicate)=>{
  if(predicate(root))return root;
  for(const child of root.children||[]){const value=find(child,predicate);if(value)return value}
  return null;
 };
 assert.ok(find(node('properties'),element=>element.textContent==='Upload icon'),
  'custom image upload control not available');
 assert.equal(widget.action,undefined,'non-interactive images must not have a touch action');
});

test('selected widget settings group opacity percentage and preserve it on save',async()=>{
 const {context,node,writes}=makeEditor();
 await vm.runInContext('load()',context);
 const screen=vm.runInContext('cfg.screens[0]',context);
 screen.widgets.push({id:'opacity_test',type:'button',action:'next',target:'',x:0,y:0,width:80,height:40,settings:{label:'Demo'}});
 vm.runInContext("selected='opacity_test';render()",context);
 const walk=(root,predicate)=>{
  if(predicate(root))return root;
  for(const child of root.children||[]){if(child&&typeof child==='object'){const found=walk(child,predicate);if(found)return found}}
  return null;
 };
 const properties=node('properties');
 const sections=properties.children.filter(child=>child.className?.includes('nexus-property-section'));
 assert.equal(sections.length,4);
 assert.deepEqual(sections.map(item=>item.children[0].textContent),['Position & size','Appearance','Content & data','Actions & icons']);
 assert.equal(sections[0].open,true);
 assert.equal(sections[1].open,true);
 assert.equal(sections[2].open,true);
 const appearance=sections[1];
 const opacitySlider=walk(appearance,item=>item.type==='range');
 assert.ok(opacitySlider);
 const opacityLabel=walk(appearance,item=>item.className==='nexus-opacity-control');
 assert.ok(opacityLabel);
 const opacityNumber=walk(opacityLabel,item=>item.type==='number');
 assert.ok(opacityNumber);
 assert.equal(opacityNumber.value,'100');
 opacitySlider.value='45';
 opacitySlider.listeners.input();
 assert.equal(screen.widgets[0].settings.opacity,'45');
 assert.equal(opacityNumber.value,'45');
 opacityNumber.value='62';
 opacityNumber.listeners.change();
 assert.equal(screen.widgets[0].settings.opacity,'62');
 assert.equal(opacitySlider.value,'62');
 const noFill=walk(sections[3],item=>item.options?.some(option=>option.label==='No fill'));
 assert.ok(noFill,'style label should distinguish no fill from opacity');
 assert.equal(noFill.options.some(option=>option.label==='Transparent'),false);
 await node('save').onclick();
 assert.equal(writes.at(-1).screens[0].widgets[0].settings.opacity,'62');
});


test('narrow Appearance inspector keeps units inline and positions fields without wrapping',async()=>{
 const {context,node,writes}=makeEditor();
 await vm.runInContext('load()',context);
 const screen=vm.runInContext('cfg.screens[0]',context);
 screen.widgets.push({id:'appearance_compact',type:'metric',metric:'cpu.temperature',x:0,y:0,width:152,height:48,settings:{cornerRadius:'15',opacity:'75'}});
 vm.runInContext("selected='appearance_compact';render()",context);
 const walk=(root,predicate)=>{
  if(predicate(root))return root;
  for(const child of root.children||[]){if(child&&typeof child==='object'){const found=walk(child,predicate);if(found)return found}}
  return null;
 };
 const section=walk(node('properties'),el=>el.className?.includes('nexus-appearance-section'));
 assert.ok(section,'appearance section must be rendered');
 const radius=walk(section,el=>el.className==='nexus-radius-control');
 const inlineRadius=walk(radius,el=>el.className==='nexus-input-unit');
 assert.ok(inlineRadius,'radius value and px suffix must share an inline wrapper');
 assert.equal(inlineRadius.children[0].type,'number');
 assert.equal(inlineRadius.children[1].textContent,'px');
 assert.equal(inlineRadius.children[0].value,'15');
 const opacity=walk(section,el=>el.className==='nexus-opacity-control');
 const inlineOpacity=walk(opacity,el=>el.className==='nexus-opacity-inline');
 const value=walk(opacity,el=>el.className==='nexus-opacity-value');
 assert.ok(inlineOpacity&&value,'opacity range, number and percent need shared row');
 assert.equal(inlineOpacity.children[0].type,'range');
 assert.equal(inlineOpacity.children[1],value);
 assert.equal(value.children[0].type,'number');
 assert.equal(value.children[1].textContent,'%');
 assert.equal(value.children[0].value,'75');
 const gradient=walk(section,el=>el.className==='nexus-gradient-control');
 assert.ok(gradient&&gradient.children.some(ch=>ch.options?.some(o=>o.label==='None')),'gradient remains editable beside radius');
 const colors=[];
 for(const row of section.children){
  for(const child of row.children||[]){
   if(child.className==='nexus-color-control')colors.push(child);
  }
 }
 assert.deepEqual(colors.map(el=>el.textContent.trim()),['BG','Text','Border']);
 assert.deepEqual(colors.map(el=>el.children[0].attributes['aria-label']),['Background color','Text & icon color','Border color']);
 value.children[0].value='64';
 value.children[0].listeners.change();
 assert.equal(screen.widgets[0].settings.opacity,'64');
 inlineRadius.children[0].value='8';
 inlineRadius.children[0].listeners.change();
 assert.equal(screen.widgets[0].settings.cornerRadius,'8');
 await node('save').onclick();
 assert.equal(writes.at(-1).screens[0].widgets[0].settings.opacity,'64');
 assert.equal(writes.at(-1).screens[0].widgets[0].settings.cornerRadius,'8');
});

test('compact Appearance CSS prevents wrapping at narrow card widths',()=>{
 const css=fs.readFileSync(path.join(__dirname,'..','css','nexus-compact-inspector.css'),'utf8');
 assert.match(css,/\.nexus-appearance-row \.nexus-radius-control\s*\{\s*grid-column:1;grid-row:1/);
 assert.match(css,/\.nexus-appearance-row \.nexus-gradient-control\s*\{\s*grid-column:2;grid-row:1/);
 assert.match(css,/label\.nexus-opacity-control\s*\{[^}]*grid-column:1\/-1;grid-row:2/);
 assert.match(css,/\.nexus-opacity-inline\s*\{[^}]*grid-template-columns:minmax\(0,1fr\) auto/);
 assert.match(css,/\.nexus-input-unit,[\s\S]*?\.nexus-opacity-value\s*\{[^}]*flex-wrap:nowrap/);
 assert.match(css,/\.nexus-colors-row label\.nexus-color-control\s*\{[^}]*flex-wrap:nowrap/);
 assert.match(css,/#nexus-editor #nexus-panel-widget #nexus-selected-panel #properties \.nexus-colors-row \.nexus-color-control input\[type=color\]/);
});

test('new widgets use the full 48 pixel Nexus height without modifying existing items',async()=>{
 const {context,node}=makeEditor();
 await vm.runInContext('load()',context);
 const existing=vm.runInContext('cfg.screens[0]',context);
 existing.widgets.push({id:'old',type:'text',x:0,y:0,width:80,height:24,settings:{text:'Existing'}});
 const cases=[['addMetric','metric'],['addContent','text'],['addButton','button']];
 for(const [id,expected] of cases){
  const handler=node(id).listeners.click||node(id).onclick;
  assert.equal(typeof handler,'function',id+' handler');
  handler();
  const added=vm.runInContext('cfg.screens[0].widgets.at(-1)',context);
  assert.equal(added.type,expected);
  assert.equal(added.height,48,id+' height');
  assert.equal(added.y,0,id+' position');
 }
 assert.equal(existing.widgets[0].height,24);
});

test('gradient controls edit a widget without changing its opacity',async()=>{
 const {context,node}=makeEditor();
 await vm.runInContext('load()',context);
 const screen=vm.runInContext('cfg.screens[0]',context);
 screen.widgets.push({id:'gradient_test',type:'text',x:0,y:0,width:80,height:40,settings:{text:'Demo',opacity:'60'}});
 vm.runInContext("selected='gradient_test';render()",context);
 const walk=(root,predicate)=>{
  if(predicate(root))return root;
  for(const child of root.children||[]){if(child&&typeof child==='object'){const found=walk(child,predicate);if(found)return found}}
  return null;
 };
 const select=walk(node('properties'),x=>x.options?.some(option=>option.label==='Left to right'));
 assert.ok(select);
 select.value='horizontal';select.listeners.change();
 assert.equal(screen.widgets[0].settings.gradientDirection,'horizontal');
 assert.equal(screen.widgets[0].settings.opacity,'60');
 assert.equal(screen.widgets[0].settings.gradientColor,'#0b1824');
});

test('preview is above five file-style tabs and only three widget add buttons exist',()=>{
 for(const html of [integrated,standalone]){
  for(const id of ['addMetric','addContent','addButton'])assert.match(html,new RegExp('id="'+id+'"'));
  for(const removed of ['addMacro','addText','addImage','addClock','addBar','addGraph','metricDisplayType','contentWidgetType'])assert.doesNotMatch(html,new RegExp('id="'+removed+'"'));
  const preview=html.indexOf('id="preview"');
  const tabs=html.indexOf('class="nexus-editor-tabs"');
  const add=html.indexOf('id="nexus-add-panel"');
  assert.ok(preview>0&&tabs>preview&&add>tabs,'preview must appear before the tab bar and add-widget controls');
  for(const name of ['widget','library','background','layout','applications']){
   assert.match(html,new RegExp('id="nexus-tab-'+name+'"[^>]*role="tab"'));
   assert.match(html,new RegExp('id="nexus-panel-'+name+'"[^>]*role="tabpanel"'));
  }
  for(const id of ['widgetLibrarySelect','background','layoutImportMode','applicationScreenMappings'])assert.match(html,new RegExp('id="'+id+'"'));
  assert.doesNotMatch(html,/id="nexus-flyout-library"|class="nexus-flyout-toolbar"/);
 }
 assert.match(style,/nexus-editor-tablist.*?border-radius:8px 8px 0 0/s);
 assert.match(style,/nexus-editor-tabcontent/);
});

test('all five file tabs switch exclusively and support keyboard navigation',()=>{
 const {node}=makeEditor();
 const attributes=new Map();
 for(const name of ['widget','library','background','layout','applications']){
  const tab=node('nexus-tab-'+name);
  tab.setAttribute=(key,value)=>attributes.set(name+':'+key,value);
  tab.focus=()=>{tab.focused=true};
 }
 assert.equal(node('nexus-panel-widget').hidden,false);
 assert.equal(node('nexus-panel-library').hidden,true);
 assert.equal(node('nexus-panel-background').hidden,true);
 node('nexus-tab-layout').listeners.click();
 assert.equal(node('nexus-panel-widget').hidden,true);
 assert.equal(node('nexus-panel-layout').hidden,false);
 assert.equal(attributes.get('layout:aria-selected'),'true');
 let prevented=false;
 node('nexus-tab-layout').listeners.keydown({key:'ArrowRight',preventDefault(){prevented=true}});
 assert.equal(prevented,true);
 assert.equal(node('nexus-panel-applications').hidden,false);
 assert.equal(node('nexus-panel-layout').hidden,true);
 assert.equal(node('nexus-tab-applications').focused,true);
});

test('metric display switches Value Bar Graph within Selected Widget and keeps settings',async()=>{
 const {context,node}=makeEditor();
 await vm.runInContext('load()',context);
 node('addMetric').onclick();
 const metric=vm.runInContext('cfg.screens[0].widgets.at(-1)',context);
 metric.settings.opacity='65';metric.metric='gpu.temperature';
 const find=(root,predicate)=>{if(predicate(root))return root;for(const child of root.children||[]){if(child&&typeof child==='object'){const hit=find(child,predicate);if(hit)return hit}}return null};
 for(const expected of ['bar','graph','metric']){
  const select=find(node('nexusDisplayTypeMount'),el=>el.options?.some(option=>option.label==='Graph'&&option.value==='graph'));
  assert.ok(select,'metric display selector missing');
  select.value=expected;select.listeners.change();
  assert.equal(metric.type,expected);
  assert.equal(metric.metric,'gpu.temperature');
  assert.equal(metric.settings.opacity,'65');
  assert.equal(metric.height,48);
 }
});

test('content type changes Text Image Clock from Selected Widget',async()=>{
 const {context,node}=makeEditor();
 await vm.runInContext('load()',context);
 node('addContent').listeners.click();
 const item=vm.runInContext('cfg.screens[0].widgets.at(-1)',context);
 const find=(root,predicate)=>{if(predicate(root))return root;for(const child of root.children||[]){if(child&&typeof child==='object'){const hit=find(child,predicate);if(hit)return hit}}return null};
 for(const expected of ['image','clock','text']){
  const select=find(node('nexusDisplayTypeMount'),el=>el.options?.some(option=>option.label==='Clock'&&option.value==='clock'));
  assert.ok(select,'content type selector missing');
  select.value=expected;select.listeners.change();
  assert.equal(item.type,expected);
  assert.equal(item.metric,'');
  assert.equal(item.height,48);
  if(expected==='text')assert.equal(item.settings.text,'New text');
  if(expected==='clock')assert.equal(item.settings.clockFormat,'24h');
 }
});

test('all secondary Nexus tabs use the shared compact inspector layout and retain control bindings',()=>{
 const css=fs.readFileSync(path.join(__dirname,'..','css','nexus-compact-inspector.css'),'utf8');
 const sections={
  library:{label:'Widget library',controls:['saveWidgetPreset','widgetLibrarySelect','addWidgetPreset','deleteWidgetPreset'],groups:['Saved widgets']},
  background:{label:'Background',controls:['background','image','upload'],groups:['Current background','Upload an image']},
  layout:{label:'Layout files',controls:['exportLayout','exportScreen','layoutFile','layoutImportMode','importLayout'],groups:['Export','Import']},
  applications:{label:'Application screens',controls:['applicationScreenID','applicationScreenTarget','addApplicationScreen','applicationScreenMappings'],groups:['New mapping','Active mappings']}
 };
 for(const html of [integrated,standalone]){
  const content=html.slice(html.indexOf('<div id="nexus-panel-library"'));
  for(const [tab,info] of Object.entries(sections)){
   const i=content.indexOf('id="nexus-panel-'+tab+'"');
   assert.ok(i>=0,tab+' must still exist as a file tab panel');
   const next=content.indexOf('id="nexus-panel-',i+16);
   const block=content.slice(i,next<0?undefined:next);
   assert.match(block,/class="nexus-tool-card"/,tab+' uses the compact card');
   assert.match(block,/class="nexus-tool-hero"/,tab+' has the amber identity header');
   assert.match(block,/class="nexus-tool-icon"/,tab+' has a graphical icon');
   assert.match(block,new RegExp('>'+info.label+'<\\/strong>'));
   assert.match(block,/class="nexus-tool-footer"/,tab+' has a concise informational footer');
   assert.doesNotMatch(block,/<fieldset\b/,tab+' should not use nested fieldsets');
   for(const name of info.groups)assert.ok(block.includes('>'+name+'</h3>'),tab+' missing section '+name);
   for(const id of info.controls){
    assert.equal(html.split('id="'+id+'"').length-1,1,id+' must occur exactly once');
    assert.ok(block.includes('id="'+id+'"'),tab+' must retain '+id+' in its tab');
   }
  }
  for(const tab of ['widget','library','background','layout','applications']){
   assert.equal(html.split('id="nexus-tab-'+tab+'"').length-1,1,tab+' file tab remains present');
  }
  assert.ok(html.indexOf('id="preview"')<html.indexOf('nexus-editor-tablist'), 'screen preview stays above tabs');
  assert.match(html,/id="image" accept="image\/png,image\/jpeg,image\/gif"/);
  assert.match(html,/id="layoutFile" type="file" accept="\.json,application\/json"/);
  for(const value of ['add-screen','replace-screen','replace-layout'])assert.ok(html.includes('value="'+value+'"'));
 }
 assert.match(css,/Shared compact tool-tab language/);
 assert.match(css,/linear-gradient\(110deg,#403729/);
 assert.match(css,/\.nexus-tool-section-heading\s*\{[\s\S]*?linear-gradient\(100deg,rgba\(213,166,90,.18\)/);
 assert.match(css,/@container\(max-width:660px\)/);
 assert.match(css,/@container\(max-width:430px\)/);
 assert.match(css,/\.nexus-app-binding\s*\{/, 'dynamic application mapping rows are styled');
 assert.match(css,/\.nexus-tool-mappings:empty::before\s*\{/, 'empty mappings have a useful state');
});

test('application-screen tab retains add/remove and Save Layout behavior',async()=>{
 const {context,node,writes}=makeEditor();
 await vm.runInContext('load()',context);
 node('applicationScreenID').value='example.app';
 node('applicationScreenTarget').value='existing';
 assert.equal(typeof node('addApplicationScreen').listeners.click,'function');
 node('addApplicationScreen').listeners.click();
 assert.equal(vm.runInContext('cfg.applicationScreens["example.app"]',context),'existing');
 assert.equal(node('applicationScreenMappings').children.length,1);
 await node('save').onclick();
 assert.equal(writes.at(-1).applicationScreens['example.app'],'existing');
 const row=node('applicationScreenMappings').children[0];
 assert.equal(row.className,'nexus-app-binding');
 row.children[1].listeners.click();
 assert.equal(vm.runInContext('Object.keys(cfg.applicationScreens).length',context),0);
 assert.equal(node('applicationScreenMappings').children.length,0);
});

test('compact inspector preserves file tabs and all existing widget action IDs',()=>{
 const compact=fs.readFileSync(path.join(__dirname,'..','css','nexus-compact-inspector.css'),'utf8');
 const page=fs.readFileSync(path.join(__dirname,'..','..','web','nexus.html'),'utf8');
 const expected=['nexusSelectedName','nexusSelectedIcon','nexusSelectedSubtitle','nexusDisplayTypeMount',
  'nexusInspectorActions','addMetric','addContent','addButton','copyWidget','pasteWidget',
  'duplicateWidget','deleteWidget','properties','selectedWidgetStatus'];
 for(const html of [integrated,standalone]){
  for(const id of expected)assert.equal(html.split('id="'+id+'"').length-1,1,id+' must exist once');
  assert.ok(html.indexOf('id="nexus-add-panel"')<html.indexOf('id="nexus-selected-panel"'));
  assert.match(html,/class="nexus-inspector-hero"/);
  assert.match(html,/class="nexus-inspector-actions-menu"/);
  for(const tab of ['widget','library','background','layout','applications']){
   assert.match(html,new RegExp('id="nexus-tab-'+tab+'"'));
  }
 }
 assert.ok(page.indexOf('/static/css/nexus-compact-inspector.css')>page.indexOf('/static/css/nexus-editor.css'));
 assert.match(standalone,/<link rel="stylesheet" href="\/static\/css\/nexus-compact-inspector.css">/);
 assert.match(compact,/#nexus-panel-widget #properties>\.nexus-geometry-section\s*\{[^}]*grid-column:1\/-1/);
 assert.match(compact,/\.nexus-data-section\s*\{grid-column:1;grid-row:2\}/);
 assert.match(compact,/\.nexus-appearance-section\s*\{grid-column:2;grid-row:2\}/);
 assert.match(compact,/linear-gradient\(110deg,#403729/);
 assert.match(compact,/@media\(max-width:580px\)/);
 assert.match(compact,/@container\(max-width:610px\)/,'inspector should adapt to its own column width');
});

test('compact inspector renders widget identity, button label editing, and only relevant sections',async()=>{
 const {context,node,writes}=makeEditor();
 await vm.runInContext('load()',context);
 const screen=vm.runInContext('cfg.screens[0]',context);
 screen.widgets.push({id:'metric_cpu',type:'graph',x:0,y:0,width:152,height:48,metric:'cpu.temperature',settings:{label:'CPU',opacity:'75'}});
 vm.runInContext("selected='metric_cpu';render()",context);
 assert.equal(node('nexusSelectedName').textContent,'CPU');
 assert.match(node('nexusSelectedSubtitle').textContent,/Metric widget/);
 assert.equal(node('nexusInspectorActions').hidden,false);
 const names=node('properties').children.filter(c=>c.className?.includes('nexus-property-section')).map(c=>c.children[0].textContent);
 assert.deepEqual(names,['Position & size','Appearance','Content & data']);
 assert.equal(node('properties').children[0].open,true);
 const metricSelector=node('nexusDisplayTypeMount').children[0].children.at(-1);
 assert.equal(metricSelector.value,'graph');
 screen.widgets.push({id:'button_test',type:'button',x:168,y:0,width:80,height:48,action:'next',settings:{label:'Launch'}});
 vm.runInContext("selected='button_test';render()",context);
 assert.equal(node('nexusSelectedName').textContent,'Launch');
 const actions=node('properties').children.filter(c=>c.className?.includes('nexus-property-section'));
 assert.equal(actions.at(-1).children[0].textContent,'Actions & icons');
 const walk=(root,predicate)=>{
  if(predicate(root))return root;
  for(const child of root.children||[]){if(child&&typeof child==='object'){const found=walk(child,predicate);if(found)return found}}
  return null;
 };
 const label=walk(node('properties'),c=>c.textContent?.startsWith('label ')&&c.children?.some(ch=>ch.type==='text'));
 assert.ok(label,'button label remains editable under Content & data');
 const input=label.children.find(ch=>ch.type==='text');
 input.value='New name';input.listeners.input();
 assert.equal(screen.widgets[1].settings.label,'New name');
 assert.equal(node('nexusSelectedName').textContent,'New name');
 node('copyWidget').listeners.click();
 assert.equal(node('pasteWidget').disabled,false);
 await node('save').onclick();
 assert.equal(writes.at(-1).screens[0].widgets[1].settings.label,'New name');
});

test('Nexus editor leaves jQuery available to initialize the sidebar collapse button',()=>{
 const page=fs.readFileSync(path.join(__dirname,'..','..','web','nexus.html'),'utf8');
 const sidebar=fs.readFileSync(path.join(__dirname,'sidebar.js'),'utf8');
 assert.ok(page.indexOf('/static/js/nexus-editor.js')<page.indexOf('/static/js/sidebar.js'),
  'Nexus editor loads before sidebar initialization on the device page');
 assert.match(sidebar,/\$\(document\)\.ready\(/,'sidebar uses jQuery ready');
 assert.match(sidebar,/\$\('#sidebarToggle'\)\.on\('click'/,'sidebar binds collapse click with jQuery');
 const {context}=makeEditor();
 const jQuery=function(){return {ready(callback){callback()}}};
 context.$=jQuery;
 context.jQuery=jQuery;
 assert.equal(vm.runInContext('$ === jQuery',context),true,
  'editor must not shadow the global jQuery $ binding for scripts that load afterward');
});

test('site-wide fork theme loads after upstream CSS without replacing the upstream theme',()=>{
 const head=fs.readFileSync(path.join(__dirname,'..','..','web','head.html'),'utf8');
 const overlayPath=path.join(__dirname,'..','css','openlinkhub-fork-theme.css');
 const overlay=fs.readFileSync(overlayPath,'utf8');
 const upstream=head.indexOf('/static/css/themes/{{ .Dashboard.Theme }}.css" rel="stylesheet"');
 const fork=head.indexOf('/static/css/openlinkhub-fork-theme.css" rel="stylesheet"');
 assert.ok(upstream>=0&&fork>upstream,'fork CSS should load after upstream theme');
 assert.equal(head.split('/static/css/openlinkhub-fork-theme.css').length-1,1);
 assert.match(overlay,/--olh-accent:\s*#d5a65a/);
 assert.match(overlay,/\.sidebar \.nav-link\.active/);
 assert.match(overlay,/#nexus-editor #nexus-selected-panel/);
 assert.doesNotMatch(overlay,/!important\s*;?\s*\}\s*\*\s*\{/);
});

test('OpenLinkHub Nexus fork branding is present without renaming upstream routes',()=>{
 const head=fs.readFileSync(path.join(__dirname,'..','..','web','head.html'),'utf8');
 const sidebar=fs.readFileSync(path.join(__dirname,'..','..','web','sidebar.html'),'utf8');
 const css=fs.readFileSync(path.join(__dirname,'..','css','openlinkhub-fork-theme.css'),'utf8');
 assert.match(head,/<title>\{\{ \.Title \}\} \| OpenLinkHub Nexus<\/title>/);
 assert.match(sidebar,/class="olh-fork-brand" aria-label="OpenLinkHub Nexus"/);
 assert.doesNotMatch(sidebar,/class="sidebar-title\b|<h6[^>]*>\s*\{\{ \.Title \}\}/,'redundant WebUI title should not render in sidebar');
 assert.match(sidebar,/id="sidebarToggle"/);
 assert.match(sidebar,/\( \.Dict "page" "macros" "url" "\/macros"/);
 assert.match(css,/\.sidebar\.collapsed \.olh-fork-brand \{display:none\}/);
});
