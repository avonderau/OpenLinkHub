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

function makeEditor(savedWidgetEntries=[]){
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
   if(url.startsWith('/api/nexus/widget-library')){
    if(options.method==='POST'){
     savedWidgetEntries.push(JSON.parse(options.body));
     return {ok:true,status:201,json:async()=>JSON.parse(options.body)};
    }
    if(options.method==='DELETE'){
     const target=new URL('http://localhost'+url).searchParams.get('id');
     const index=savedWidgetEntries.findIndex(item=>item.id===target);
     if(index>=0)savedWidgetEntries.splice(index,1);
     return {ok:true,status:204};
    }
    return {ok:true,status:200,json:async()=>({entries:structuredClone(savedWidgetEntries)})};
   }
   if(url.startsWith('/api/telemetry/sources'))return {
    ok:true,json:async()=>({sources:{
     'disk.free_gb':[{id:'mount:/',label:'/'},{id:'mount:/home',label:'/home'}],
     'disk.read_rate':[{id:'disk:sda',label:'/dev/sda'}],
     'network.rx_rate':[{id:'net:eth0',label:'eth0'}],
    }})
   };
   if(url.startsWith('/api/icon-sets'))return {ok:true,json:async()=>({
    sets:[{id:'aaaaaaaaaaaaaaaaaaaaaaaa',name:'Desktop Pack',icons:[
     {name:'Music',nexusId:'set_aaaaaaaaaaaaaaaaaaaaaaaa',deckId:'a'.repeat(32)}
    ]}]
   })};
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
 vm.runInContext(fs.readFileSync(path.join(__dirname,'nexus-widget-library.js'),'utf8'),context,{filename:'nexus-widget-library.js'});
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
 assert.equal(action.options.some(option=>option.label==='Keyboard shortcut'),true,'standalone shortcut remains available in Nexus');
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
 assert.equal(sections.length,5);
 assert.deepEqual(sections.map(item=>item.children[0].textContent),['Position & size','Appearance','Content & data','Icon','Actions']);
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
 const noFill=walk(sections[4],item=>item.options?.some(option=>option.label==='No fill'));
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
   for(const name of info.groups)assert.ok(block.includes(' '+name+'</h3>'),tab+' missing section '+name);
   for(const id of info.controls){
    assert.equal(html.split('id="'+id+'"').length-1,1,id+' must occur exactly once');
    assert.ok(block.includes('id="'+id+'"'),tab+' must retain '+id+' in its tab');
   }
  }
  for(const tab of ['widget','library','background','layout','applications']){
   assert.equal(html.split('id="nexus-tab-'+tab+'"').length-1,1,tab+' file tab remains present');
  }
  assert.ok(html.indexOf('id="preview"')<html.indexOf('class="nexus-editor-tablist"'), 'screen preview stays above tabs');
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
 assert.equal(actions.at(-1).children[0].textContent,'Actions');
  assert.equal(actions.at(-2).children[0].textContent,'Icon');
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

test('Open URL shows a safe browser test link but does not execute via preview',async()=>{
 const {context,node,writes}=makeEditor();
 context.URL=URL;
 await vm.runInContext('load()',context);
 const screen=vm.runInContext('cfg.screens[0]',context);
 screen.widgets.push({
  id:'url_button',type:'button',x:0,y:0,width:160,height:48,
  action:'url',target:'https://example.com/help',settings:{label:'Help'}
 });
 vm.runInContext("selected='url_button';render()",context);
 const find=(root,predicate)=>{
  if(predicate(root))return root;
  for(const child of root.children||[]){
   if(child&&typeof child==='object'){const found=find(child,predicate);if(found)return found}
  }
  return null;
 };
 const link=find(node('properties'),el=>el.className==='nexus-url-test-link');
 assert.ok(link,'URL action exposes browser-side test link');
 assert.equal(link.href,'https://example.com/help');
 assert.equal(link.target,'_blank');
 assert.equal(link.rel,'noopener noreferrer');
 assert.equal(link.hidden,false);
 const input=find(node('properties'),el=>el.attributes?.['aria-label']==='Open URL');
 assert.ok(input,'URL is editable');
 input.value='javascript:alert(1)';
 input.listeners.change();
 assert.equal(link.hidden,true,'unsafe schemes cannot be opened in browser');
 assert.equal(screen.widgets[0].target,'javascript:alert(1)');
 input.value='https://user:password@example.com/';
 input.listeners.change();
 assert.equal(link.hidden,true,'embedded credentials cannot become a clickable URL');
 input.value='HTTPS://EXAMPLE.COM:443/new';
 input.listeners.change();
 assert.equal(link.hidden,false);
 assert.equal(link.href,'https://example.com/new','link uses the canonical validated URL, not raw input');
 input.value='https://example.com/new';
 input.listeners.change();
 assert.equal(link.hidden,false);
 assert.equal(link.href,'https://example.com/new');
 await node('save').onclick();
 assert.equal(writes.at(-1).screens[0].widgets[0].target,'https://example.com/new');
 assert.ok(find(node('properties'),el=>el.textContent?.includes('preview selects widgets; it does not execute')));
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
 assert.match(head,/<title>OpenLinkHub Nexus<\/title>/);
 assert.match(sidebar,/class="olh-fork-brand" aria-label="OpenLinkHub Nexus"/);
 assert.doesNotMatch(sidebar,/class="sidebar-title\b|<h6[^>]*>\s*\{\{ \.Title \}\}/,'redundant WebUI title should not render in sidebar');
 assert.match(sidebar,/id="sidebarToggle"/);
 assert.match(sidebar,/href="https:\/\/github\.com\/avonderau\/OpenLinkHub-Nexus"[^>]*rel="noopener noreferrer"/);
 assert.doesNotMatch(sidebar,/href="https:\/\/github\.com\/jurkovic-nikola\/OpenLinkHub"/);
 assert.match(sidebar,/\( \.Dict "page" "macros" "url" "\/macros"/);
 assert.match(css,/\.sidebar\.collapsed \.olh-fork-brand \{display:none\}/);
});

test('fork release and installer security defaults are consistent',()=>{
 const root=path.join(__dirname,'..','..');
 const version=fs.readFileSync(path.join(root,'src','version','version.go'),'utf8');
 const remote=fs.readFileSync(path.join(root,'remote-install.sh'),'utf8');
 const system=fs.readFileSync(path.join(root,'install.sh'),'utf8');
 const userInstall=fs.readFileSync(path.join(root,'install-user-space.sh'),'utf8');
 assert.match(version,/var Version = "0\.9\.4N"/);
 assert.match(remote,/REPO_API="https:\/\/api\.github\.com\/repos\/avonderau\/OpenLinkHub-Nexus\/releases\/latest"/);
 assert.match(remote,/sha256sum --check --status/,'installer must check release checksum');
 assert.match(remote,/Release checksum verification failed; refusing to install/);
 for(const script of [remote,system]){
  assert.doesNotMatch(script,/chmod -R 755/,'installer must not make configs world readable');
  assert.match(script,/chmod -R o-rwx/,'installer should close world permissions');
 }
 assert.doesNotMatch(userInstall,/chmod -x "\$PERMISSION_DIR"/,'udev directory must remain traversable');
 assert.match(userInstall,/chmod 644 "\$PERMISSION_DIR\/\$PERMISSION_FILE"/);
});



test('all fork-owned release links and CI guards follow the renamed GitHub repository',()=>{
 const root=path.join(__dirname,'..','..');
 const slug='avonderau/OpenLinkHub-Nexus';
 for(const p of ['README.md','docs/releases/0.9.4N.md','docs/security-review-0.9.4N.md','docs/nexus-fork.md','web/sidebar.html','remote-install.sh','packaging/build-packages.sh','.github/workflows/nexus-fork-release.yml','.github/workflows/nexus-packages.yml']){
  const content=fs.readFileSync(path.join(root,p),'utf8');
  assert.ok(content.includes(slug),p+' must refer to renamed fork');
  assert.ok(!content.includes('avonderau/OpenLinkHub'+'/')&&!content.includes('avonderau/OpenLinkHub"')&&!content.includes("avonderau/OpenLinkHub'"),p+' must not refer to old fork slug');
 }
 const release=fs.readFileSync(path.join(root,'.github/workflows/nexus-fork-release.yml'),'utf8');
 const packages=fs.readFileSync(path.join(root,'.github/workflows/nexus-packages.yml'),'utf8');
 assert.ok(release.includes("github.repository == '"+slug+"'"));
 assert.ok(packages.includes("github.repository == '"+slug+"'"));
 assert.match(packages,/Release asset already exists/,'repeat package publishes must preserve existing checksum-verified files');
 const sidebar=fs.readFileSync(path.join(root,'web/sidebar.html'),'utf8');
 assert.match(sidebar,/rel="noopener noreferrer"/);
 assert.match(fs.readFileSync(path.join(root,'README.md'),'utf8'),/jurkovic-nikola\/OpenLinkHub/,'upstream attribution preserved');
});


test('Nexus design system loads after upstream and keeps Settings styles scoped',()=>{
 const root=path.join(__dirname,'..','..');
 const head=fs.readFileSync(path.join(root,'web','head.html'),'utf8');
 const siteTheme=fs.readFileSync(path.join(root,'static','css','openlinkhub-fork-theme.css'),'utf8');
 const design=fs.readFileSync(path.join(root,'static','css','openlinkhub-nexus-design.css'),'utf8');
 const settingsCss=fs.readFileSync(path.join(root,'static','css','openlinkhub-settings.css'),'utf8');
 const inspector=fs.readFileSync(path.join(root,'static','css','nexus-compact-inspector.css'),'utf8');
 const sources=['/static/css/themes/{{ .Dashboard.Theme }}.css" rel="stylesheet"','/static/css/openlinkhub-fork-theme.css" rel="stylesheet"','/static/css/openlinkhub-nexus-design.css" rel="stylesheet"','/static/css/openlinkhub-settings.css" rel="stylesheet"'];
 let last=-1;
 for(const item of sources){
  const idx=head.indexOf(item);
  assert.ok(idx>last,'CSS cascade must preserve upstream, fork, shared and scoped layering: '+item);
  last=idx;
  assert.equal(head.split(item).length-1,1,'stylesheet included exactly once');
 }
 assert.match(siteTheme,/--olh-accent:\s*#d5a65a/);
 assert.match(design,/--olh-header-gradient:\s*linear-gradient\(110deg,#403729/);
 assert.match(design,/\.olh-section-header \.olh-section-icon/);
 assert.match(design,/\.main-content \.olh-page-intro/);
 assert.match(inspector,/background:var\(--olh-header-gradient,/,'Nexus editor consumes shared token');
 assert.match(settingsCss,/\.olh-settings-page \.settings-row/);
 assert.match(settingsCss,/\.olh-settings-nav/);
 assert.match(settingsCss,/\.olh-settings-category-grid/);
 assert.match(settingsCss,/@media\(max-width:880px\)/);
 assert.doesNotMatch(settingsCss,/^\s*(?:body|html|\.main-content|\.card|\.card-header)\s*[{,]/m,'Settings-only CSS must be scoped, not rewrite other pages');
});

test('Settings pilot preserves actions, localization and all eight section icons',()=>{
 const root=path.join(__dirname,'..','..');
 const page=fs.readFileSync(path.join(root,'web','settings.html'),'utf8');
 const expectedIds=['deviceId','checkbox-celsius','checkbox-temperatureBar','checkbox-deviceLabels',
  'checkbox-addDeviceToDashboard','userLanguage','theme','allDevicesRgb','allDevicesColor',
  'checkbox-rgbOff','keyboardLayout','btnSaveDashboardSettings','btnBackup','restoreForm',
  'backupFile','btnRestore','rgbControl','rgbOff','rgbOn','lcdControl','virtualAudio',
  'outputDevice','checkbox-auth-enabled','auth_username','auth_password','auth_origin',
  'auth_lifetime','btnSaveAuthSettings','dataTable','btnSaveSupportedDevices'];
 for(const id of expectedIds){
  assert.match(page,new RegExp('id="'+id+'"'),'existing Settings UI control preserved: '+id);
 }
 const sectionIds=['txtDashboard','txtBackupRestore','txtRgbScheduler','txtVirtualAudio','txtDisplay','txtDashboardAuth','txtSupportedDevices'];
 const headers=[...page.matchAll(/<div class="card-header olh-section-header">([\s\S]*?)<\/div>/g)];
 assert.equal(headers.length,sectionIds.length+1,'all Settings panels have redesigned, accessible headers');
  assert.ok(headers.some(([markup])=>markup.includes('Icon Pack Library')),'pack manager has matching card header');
 for(const key of sectionIds){
  assert.ok(headers.some(([markup])=>markup.includes('{{ .Lang "'+key+'" }}')),'localized header preserved: '+key);
 }
 assert.equal((page.match(/class="olh-section-icon"/g)||[]).length,8,'one icon per section');
 assert.match(page,/class="main-content p-4 olh-settings-page"/);
 assert.equal((page.match(/class="olh-settings-subheading"/g)||[]).length,3,'Dashboard options are grouped by purpose');
 assert.match(page,/for="keyboardLayout"/,'keyboard layout label targets correct field');
  assert.match(page,/class="olh-settings-shell"/);
 assert.match(page,/class="olh-settings-nav" role="tablist"/);
 assert.equal((page.match(/class="olh-settings-nav-tab"/g)||[]).length,3);
 assert.equal((page.match(/class="olh-settings-tab-panel"/g)||[]).length,3);
 assert.match(page,/id="olh-settings-general" role="tabpanel"/);
 assert.match(page,/id="olh-settings-devices" role="tabpanel"/);
 assert.match(page,/id="olh-settings-security" role="tabpanel"/);
 assert.match(page,/class="olh-page-intro"/);
 assert.match(page,/\{\{ \.Lang "txtSettings" \}\}/,'page heading uses current localization');
 assert.match(page,/src="\/static\/js\/settings\.js(?:\?[^"]*)?"/);
 assert.match(page,/src="\/static\/js\/sidebar\.js"/);
 assert.match(page,/\{\{ if \.Dashboard\.TemperatureBar \}\}/);
});


test('shared Nexus site header replaces the old text-only sidebar title on every page',()=>{
 const root=path.join(__dirname,'..','..');
 const sidebar=fs.readFileSync(path.join(root,'web','sidebar.html'),'utf8');
 const css=fs.readFileSync(path.join(root,'static','css','openlinkhub-nexus-design.css'),'utf8');
 const js=fs.readFileSync(path.join(root,'static','js','sidebar.js'),'utf8');
 const head=fs.readFileSync(path.join(root,'web','head.html'),'utf8');
 assert.match(sidebar,/<header class="olh-site-header">/);
 assert.match(sidebar,/class="olh-site-header-home" href="\/" aria-label="OpenLinkHub Nexus dashboard"/);
 assert.match(sidebar,/class="olh-site-header-mark" aria-hidden="true"/);
 assert.match(sidebar,/class="olh-fork-brand" aria-label="OpenLinkHub Nexus"/);
 assert.match(sidebar,/class="olh-site-header-caption">CONTROL CENTER<\/span>/);
 assert.equal((sidebar.match(/id="sidebarToggle"/g)||[]).length,1,'preserve one sidebar toggle');
 assert.match(sidebar,/aria-controls="olh-primary-navigation"/);
 assert.match(sidebar,/id="olh-primary-navigation" aria-label="Primary navigation"/);
 assert.match(sidebar,/aria-expanded="\{\{ if \.Dashboard\.SidebarCollapsed \}\}false\{\{ else \}\}true\{\{ end \}\}"/);
 assert.match(sidebar,/class="nav flex-column sidebar-nav"/);
 assert.match(sidebar,/\( \.Dict "page" "settings" "url" "\/settings"/, "Settings route remains in the Go template menu");
 assert.match(sidebar,/href="https:\/\/github.com\/avonderau\/OpenLinkHub-Nexus"/);
 assert.doesNotMatch(sidebar,/id="sidebarToggle">☰<\/button>/,'old text-only toggle must be removed');
 assert.match(css,/\.sidebar \.olh-site-header \{/);
 assert.match(css,/background:var\(--olh-header-gradient\)/);
 assert.match(css,/\.sidebar\.collapsed \.olh-site-header \{/);
 assert.match(css,/\.sidebar\.collapsed \.olh-site-header \.olh-fork-brand \{display:none\}/);
 assert.match(css,/prefers-reduced-motion/);
 assert.match(js,/function syncSidebarToggle\(\)/);
 assert.match(js,/sidebarToggle\.setAttribute\("aria-expanded"/);
 assert.match(js,/syncSidebarToggle\(\);/);
 assert.match(js,/\$\('#sidebarToggle'\)\.on\('click'/,'collapse behavior remains bound');
 assert.match(head,/static\/css\/openlinkhub-nexus-design\.css/,'shared header CSS loads on every app page');
 for(const page of ['web/index.html','web/settings.html','web/nexus.html']){
  const body=fs.readFileSync(path.join(root,page),'utf8');
  assert.match(body,/\{\{ template "sidebar" \. \}\}/,page+' uses shared header');
 }
});


test('Settings categorization preserves all original API handlers and section-specific save actions',()=>{
 const root=path.join(__dirname,'..','..');
 const page=fs.readFileSync(path.join(root,'web','settings.html'),'utf8');
 const settings=fs.readFileSync(path.join(root,'static/js/settings.js'),'utf8');
 const css=fs.readFileSync(path.join(root,'static/css/openlinkhub-settings.css'),'utf8');
 const sec=fs.readFileSync(path.join(root,'static/js/settings-sections.js'),'utf8');
 assert.match(page,/src="\/static\/js\/settings-sections\.js"/);
 assert.match(page,/id="btnSaveDashboardSettings"/);
 assert.match(page,/class="system-button saveRgbControl"/);
 assert.match(page,/class="system-button enableVirtualAudio"/);
 assert.match(page,/id="btnSaveAuthSettings"/);
 assert.match(page,/id="btnSaveSupportedDevices"/);
 assert.match(page,/id="btnBackup"/);
 assert.match(page,/id="restoreForm"/);
 assert.match(settings,/\/api\/dashboard\/update/);
 assert.match(settings,/\/api\/scheduler\/rgb/);
 assert.match(settings,/\/api\/setSupportedDevices/);
 assert.match(settings,/\/api\/auth\/update/);
 assert.match(settings,/\/api\/backup/);
 assert.match(settings,/\/api\/restore/);
 assert.match(css,/\.olh-settings-tab-panel\[hidden\]\{display:none!important\}/);
 assert.match(css,/@media\(max-width:560px\)/);
 assert.match(sec,/\.columns\.adjust\(\)/,'table column sizing updated on reveal');
});

test('Nexus media preview uses LCD pixel-shape masks and default captions with custom overrides',async()=>{
 const {context,node}=makeEditor();
 await vm.runInContext('load()',context);
 const screen=vm.runInContext('cfg.screens[0]',context);
 screen.widgets.push({
  id:'media_prev',type:'button',x:0,y:0,width:120,height:48,
  action:'media',target:'previous-track',
  settings:{icon:'previous-track',style:'solid'}
 });
 vm.runInContext('render()',context);
 const preview=node('screen').children[0];
 const glyph=preview.children.find(el=>el.className==='nexus-icon nexus-media-glyph');
 const caption=preview.children.find(el=>el.className==='nexus-media-caption');
 assert.ok(glyph,'hardware-matched preview glyph missing');
 assert.equal(glyph.style.maskImage,'url("/static/media-glyphs/previous-track.svg")');
 assert.equal(caption.textContent,'Previous');
 screen.widgets[0].settings.label='Rewind';
 vm.runInContext('render()',context);
 assert.equal(node('screen').children[0].children.find(el=>el.className==='nexus-media-caption').textContent,
   'Rewind','custom label must replace media default');
 screen.widgets[0].settings.label='';
 vm.runInContext("selected='media_prev';render()",context);
 const find=(el,pred)=>pred(el)?el:(el.children||[]).map(c=>find(c,pred)).find(Boolean);
 const input=find(node('properties'),el=>el.type==='text'&&el.placeholder==='Previous');
 assert.ok(input,'default media caption is visible as inspector placeholder');
 assert.equal(screen.widgets[0].settings.label,'','default caption must not be stored as custom label');
});

test('Nexus metrics expose disk and network sources and retain selections',async()=>{
 const {context,node,writes}=makeEditor();
 await vm.runInContext('load()',context);
 const screen=vm.runInContext('cfg.screens[0]',context);
 screen.widgets.push({id:'disk_metric',type:'metric',metric:'disk.free_gb',
  x:0,y:0,width:145,height:48,settings:{}});
 vm.runInContext("selected='disk_metric';render()",context);
 const find=(el,pred)=>pred(el)?el:(el.children||[]).map(child=>find(child,pred)).find(Boolean);
 let picker=find(node('properties'),el=>el.attributes?.['aria-label']==='Metric source');
 assert.ok(picker,'disk metric needs a source selector');
 assert.equal(picker.options.some(o=>o.value==='mount:/home'),true);
 picker.value='mount:/home';
 picker.listeners.change();
 assert.equal(screen.widgets[0].settings.metricSource,'mount:/home');
 await node('save').onclick();
 assert.equal(writes.at(-1).screens[0].widgets[0].settings.metricSource,'mount:/home');
 const metricChoice=find(node('properties'),el=>el.options?.some(option=>option.value==='cpu.temperature'));
 assert.ok(metricChoice,'metric dropdown missing');
 metricChoice.value='cpu.temperature';
 metricChoice.listeners.change();
 assert.equal(screen.widgets[0].settings.metricSource,undefined,'changing metrics clears previous source');
 picker=find(node('properties'),el=>el.attributes?.['aria-label']==='Metric source');
 assert.equal(picker,undefined,'single-source CPU metrics should not display the selector');
});

test('all six media keys retain absolute positioning and their saved preview slots',async()=>{
 // Media icon wrappers must NEVER override .widget positioning. Relative
 // positioning stacked the buttons and clipped most of them offscreen.
 assert.match(style,/#nexus-editor \.widget\s*\{[^}]*position:\s*absolute/s);
 assert.doesNotMatch(style,/#nexus-editor \.widget\.nexus-media-preview-widget\s*\{[^}]*position\s*:/s);
 const {context,node}=makeEditor();
 await vm.runInContext('load()',context);
 node('newScreenPreset').value='media-deck';
 await node('addScreen').onclick();
 const screen=vm.runInContext('cfg.screens.find(s=>s.name==="Media Controls")',context);
 assert.equal(screen.widgets.length,6);
 const visible=node('screen').children;
 assert.equal(visible.length,6,'the preview must render every media control');
 assert.equal(visible.filter(el=>el.classList?.add && el.children.some(c=>c.className==='nexus-icon nexus-media-glyph')).length,6,
  'all six media buttons should have hardware-matched glyph previews');
 const positions=Array.from(screen.widgets,w=>Number(w.x));
 assert.deepEqual(positions,[0,104,208,312,416,520]);
 for(let i=0;i<screen.widgets.length;i++){
  assert.ok(visible[i].style.cssText.includes('left:'+positions[i]+'px;'),
   'button '+i+' must retain its original LCD X coordinate');
  assert.equal(visible[i].dataset.id,screen.widgets[i].id);
 }
});

test('Widget library saves to OpenLinkHub and survives a fresh editor/browser context',async()=>{
 const shared=[];
 const first=makeEditor(shared);
 await vm.runInContext('load()',first.context);
 first.node('addMetric').onclick();
 await first.node('saveWidgetPreset').listeners.click();
 assert.equal(shared.length,1);
 assert.equal(shared[0].name,'demo-nexus');
 assert.equal(shared[0].widget.type,'metric');
 assert.match(first.node('message').textContent,/saved in OpenLinkHub/i);

 const second=makeEditor(shared);
 await vm.runInContext('load()',second.context);
 assert.equal(second.node('widgetLibrarySelect').options.length,1);
 second.node('widgetLibrarySelect').value=shared[0].id;
 second.node('addWidgetPreset').listeners.click();
 const widgets=vm.runInContext('cfg.screens[0].widgets',second.context);
 assert.equal(widgets.length,1);
 assert.equal(widgets[0].type,'metric');
 assert.notEqual(widgets[0].id,shared[0].widget.id);
 await second.node('deleteWidgetPreset').listeners.click();
 assert.equal(shared.length,0);
 assert.match(second.node('message').textContent,/removed from the OpenLinkHub library/);
});

test('Nexus Icon and Actions inspector cards are adjacent columns with responsive stacking',()=>{
 const css=fs.readFileSync(path.join(__dirname,'..','css','nexus-compact-inspector.css'),'utf8');
 assert.match(css,/#nexus-panel-widget #properties>\.nexus-icons-section\{grid-column:1;grid-row:3\}/);
 assert.match(css,/#nexus-panel-widget #properties>\.nexus-actions-section\{grid-column:2;grid-row:3\}/);
 assert.match(css,/@container\(max-width:610px\)/);
 assert.match(editorSource,/section\('Actions',true,'actions','cursor'\)/);
 assert.doesNotMatch(editorSource,/storage\.setItem\(/);
});

test('Nexus icon gallery lives in the selected widget inspector and saves the chosen image ID',async()=>{
 const {context,node,writes}=makeEditor();
 await vm.runInContext('load()',context);
 const screen=vm.runInContext('cfg.screens[0]',context);
 screen.widgets.push({id:'icon_btn',type:'button',action:'next',target:'',
  x:0,y:0,width:90,height:40,settings:{}});
 vm.runInContext("selected='icon_btn';render()",context);
 const find=(root,pred)=>pred(root)?root:(root.children||[]).map(child=>find(child,pred)).find(Boolean);
 const icons=find(node('properties'),el=>el.className==='nexus-property-section nexus-icons-section');
 assert.ok(icons,'icon selection needs its own existing-style inspector section');
 assert.equal(icons.open,true,'icon selection should be visible without nested disclosure menus');
 assert.ok(find(node('properties'),el=>el.className==='nexus-property-section nexus-actions-section'),
  'button actions should stay in their own inspector section');
 assert.equal(find(node('properties'),el=>el.className==='nexus-icon-set-picker'),undefined,
  'the separate nested pack picker must be removed');
 const pack=find(icons,el=>el.attributes?.['aria-label']==='Icon set');
 assert.ok(pack,'installed packs should be selectable in the icon inspector');
 pack.value='aaaaaaaaaaaaaaaaaaaaaaaa';
 pack.listeners.change();
 const gallery=find(icons,el=>el.className==='nexus-icon-gallery');
 assert.ok(gallery,'pack icons must have an image gallery');
 const tile=find(icons,el=>el.attributes?.['aria-label']==='Use Music icon');
 assert.ok(tile,'pack icon must be selectable directly from its preview');
 assert.ok(find(tile,el=>el.src?.includes('/api/icon-sets/preview?id=')),
  'the gallery must render the exact Settings preview artwork');
 assert.equal(tile.attributes['aria-pressed'],'false');
 tile.listeners.click();
 assert.equal(screen.widgets[0].settings.customIcon,'set_aaaaaaaaaaaaaaaaaaaaaaaa');
 const reselected=find(node('properties'),el=>el.attributes?.['aria-label']==='Icon set');
 assert.equal(reselected.value,'aaaaaaaaaaaaaaaaaaaaaaaa','selected pack stays visible after apply');
 const chosen=find(node('properties'),el=>el.attributes?.['aria-label']==='Use Music icon');
 assert.equal(chosen.attributes['aria-pressed'],'true','selected icon must be visually identified');
 await node('save').onclick();
 assert.equal(writes.at(-1).screens[0].widgets[0].settings.customIcon,'set_aaaaaaaaaaaaaaaaaaaaaaaa');
});

test('Nexus can replace a library icon with built-in artwork and retain image widget selection',async()=>{
 const {context,node}=makeEditor();
 await vm.runInContext('load()',context);
 const screen=vm.runInContext('cfg.screens[0]',context);
 screen.widgets.push({id:'icon_button',type:'button',action:'media',target:'play-pause',
  x:0,y:0,width:90,height:40,settings:{customIcon:'set_aaaaaaaaaaaaaaaaaaaaaaaa'}});
 vm.runInContext("selected='icon_button';render()",context);
 const find=(root,pred)=>pred(root)?root:(root.children||[]).map(child=>find(child,pred)).find(Boolean);
 const iconSection=find(node('properties'),el=>el.className==='nexus-property-section nexus-icons-section');
 const source=find(iconSection,el=>el.attributes?.['aria-label']==='Built-in icon');
 assert.ok(source);
 source.value='next-track';
 source.listeners.change();
 assert.equal(screen.widgets[0].settings.customIcon,undefined);
 assert.equal(screen.widgets[0].settings.icon,'next-track');
 screen.widgets.push({id:'image_card',type:'image',x:100,y:0,width:90,height:40,settings:{customIcon:'set_aaaaaaaaaaaaaaaaaaaaaaaa'}});
 vm.runInContext("selected='image_card';render()",context);
 const imageSection=find(node('properties'),el=>el.className==='nexus-property-section nexus-icons-section');
 assert.ok(imageSection);
 assert.equal(find(imageSection,el=>el.attributes?.['aria-label']==='Icon set').value,'aaaaaaaaaaaaaaaaaaaaaaaa');
 assert.ok(find(imageSection,el=>el.attributes?.['aria-label']==='Use Music icon'));
});

test('icon set imports are managed in Settings, not device editors',()=>{
 assert.doesNotMatch(editorSource,/fetch\('\/api\/icon-sets\?name=/);
 assert.match(editorSource,/href='\/settings#olh-settings-devices'/);
});
