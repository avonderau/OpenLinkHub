'use strict';
// Nexus layout files use exactly the same JSON schema as the saved device
// configuration; importing only modifies editor state until Save Layout.
globalThis.NexusLayoutFiles=(()=>{
 const safeID=/^[A-Za-z0-9_-]{1,64}$/;
 const hex=/^#[0-9A-Fa-f]{6}$/;
 const types=new Set(['text','metric','bar','graph','clock','button','macro']);
 const metrics=new Set(['cpu.temperature','cpu.utilization','gpu.temperature','gpu.utilization','memory.used','memory.utilization','network.rx_rate','network.tx_rate','disk.read_rate','disk.write_rate']);
 const actions=new Set(['next','previous','screen','media','hotkey','mouse','url','application','macro']);
 const media=new Set(['play-pause','stop','volume-up','volume-down','next-track','previous-track']);
 const mouse=new Set(['left','double-left','right','middle','back','forward','scroll-up','scroll-down','scroll-left','scroll-right']);
 const stepTypes=new Set(['next','previous','screen','media','hotkey','mouse','url','application','delay']);
 const clone=value=>JSON.parse(JSON.stringify(value));
 const object=x=>x!==null&&typeof x==='object'&&!Array.isArray(x);
 function checkStep(step,screenIDs){
  if(!object(step)||!stepTypes.has(step.type))throw Error('Unsupported macro step');
  if(step.type==='delay'){
   if(!Number.isInteger(step.delayMs)||step.delayMs<0||step.delayMs>5000)throw Error('Invalid macro delay');
  }else if(step.type==='screen'){
   if(!screenIDs.has(step.value))throw Error('Macro links to a missing screen');
  }else if(step.type==='media'){
   if(!media.has(step.value))throw Error('Invalid macro media action');
  }else if(step.type==='mouse'){
   if(!mouse.has(step.value))throw Error('Invalid macro mouse action');
  }else if(step.type==='url'||step.type==='application'){
   if(typeof step.value!=='string'||step.value.length<1||step.value.length>(step.type==='url'?256:128))throw Error('Invalid macro launch value');
  }else if(step.type==='hotkey'){
   if(typeof step.value!=='string'||step.value.length<1||step.value.length>80)throw Error('Invalid macro shortcut');
  }
 }
 function validate(value){
  if(!object(value)||value.version!==1||!Array.isArray(value.screens)||value.screens.length<1||value.screens.length>32)
   throw Error('Unsupported Nexus layout file');
  if(!['',undefined,'manual','automatic','off'].includes(value.cyclingMode))throw Error('Invalid screen cycling mode');
  if(value.cyclingMode==='automatic'&&(!Number.isInteger(value.cyclingSeconds)||value.cyclingSeconds<5||value.cyclingSeconds>3600))
   throw Error('Invalid automatic cycling interval');
  const screenIDs=new Set();
  for(const screen of value.screens){
   if(!object(screen)||typeof screen.id!=='string'||!safeID.test(screen.id)||screenIDs.has(screen.id))
    throw Error('Invalid or duplicate screen ID');
   if(typeof screen.name!=='string'||screen.name.length<1||screen.name.length>80||!Array.isArray(screen.widgets)||screen.widgets.length>64)
    throw Error('Invalid screen name or widget count');
   if(screen.background!==undefined&&screen.background!==''&&(!safeID.test(screen.background)))throw Error('Invalid background ID');
   screenIDs.add(screen.id);
  }
  if(!screenIDs.has(value.activeScreenId))throw Error('Active screen not found');
  for(const screen of value.screens){
   const widgetIDs=new Set();
   for(const widget of screen.widgets){
    if(!object(widget)||!safeID.test(widget.id)||widgetIDs.has(widget.id)||!types.has(widget.type))
     throw Error('Invalid or duplicate widget');
    widgetIDs.add(widget.id);
    if(!['x','y','width','height'].every(key=>Number.isInteger(widget[key])))throw Error('Invalid widget geometry');
    if(widget.x<0||widget.y<0||widget.width<1||widget.height<1||widget.x+widget.width>640||widget.y+widget.height>48)
     throw Error('Widget outside 640 × 48 display');
    if(['metric','bar','graph'].includes(widget.type)&&!metrics.has(widget.metric))throw Error('Unsupported metric');
    const settings=widget.settings||{};
    if(!object(settings)||Object.keys(settings).length>16)throw Error('Invalid widget settings');
    for(const [key,text] of Object.entries(settings)){
     if(typeof text!=='string'||key.length>64||text.length>256)throw Error('Invalid widget setting');
    }
    if(!['',undefined,'solid','outline','flat'].includes(settings.style))throw Error('Unsupported widget style');
    for(const key of ['textColor','backgroundColor','borderColor'])if(settings[key]&&!hex.test(settings[key]))throw Error('Invalid widget color');
    if(settings.customIcon&&(!safeID.test(settings.customIcon)||!['button','macro'].includes(widget.type)))throw Error('Invalid custom icon');
    if(widget.type==='clock'&&!['',undefined,'24h','12h','date','datetime'].includes(settings.clockFormat))throw Error('Invalid clock format');
    if(widget.type==='button'){
     if(!actions.has(widget.action))throw Error('Unsupported touch action');
     if(widget.action==='screen'&&!screenIDs.has(widget.target))throw Error('Button links to missing screen');
     if(widget.action==='media'&&!media.has(widget.target))throw Error('Invalid media action');
     if(widget.action==='mouse'&&!mouse.has(widget.target))throw Error('Invalid mouse action');
     if((widget.action==='url'||widget.action==='application')&&!(typeof widget.target==='string'&&widget.target.length>0&&widget.target.length<=(widget.action==='url'?256:128)))throw Error('Invalid launch action');
     if(widget.action==='hotkey'&&!(typeof widget.target==='string'&&widget.target.length>0&&widget.target.length<=80))throw Error('Invalid shortcut');
    }
    if(widget.type==='macro'||(widget.type==='button'&&widget.action==='macro')){
     if(!Array.isArray(widget.steps)||widget.steps.length<1||widget.steps.length>12)throw Error('Macro must have 1–12 steps');
     let delay=0;
     for(const step of widget.steps){checkStep(step,screenIDs);if(step.type==='delay')delay+=step.delayMs}
     if(delay>15000)throw Error('Macro waits exceed 15 seconds');
    }
   }
  }
  return value;
 }
 function remapScreenLinks(screen,oldID,newID){
  screen.id=newID;
  for(const w of screen.widgets){
   if(w.type==='button'&&w.action==='screen'&&w.target===oldID)w.target=newID;
   if(w.type==='macro'||(w.type==='button'&&w.action==='macro')){
    for(const step of w.steps||[])if(step.type==='screen'&&step.value===oldID)step.value=newID;
   }
  }
  return screen;
 }
 function exportSingle(layout,screenID){
  validate(layout);
  const screen=layout.screens.find(s=>s.id===screenID);
  if(!screen)throw Error('Screen not found');
  const copy=clone(screen);
  // Standalone exports must not contain dangling references to other screens.
  for(const w of copy.widgets){
   if(w.type==='button'&&w.action==='screen'&&w.target!==screenID)w.target=screenID;
   if(w.type==='macro'||(w.type==='button'&&w.action==='macro')){
    for(const step of w.steps||[])if(step.type==='screen'&&step.value!==screenID)step.value=screenID;
   }
  }
  const data={version:1,activeScreenId:copy.id,cyclingMode:'manual',cyclingSeconds:15,screens:[copy]};
  return validate(data);
 }
 function applyImport(current,incoming,mode,screenID,newID){
  validate(current);validate(incoming);
  if(mode==='replace-layout')return clone(incoming);
  if(!['add-screen','replace-screen'].includes(mode)||incoming.screens.length!==1)
   throw Error('Use a single-screen JSON file for this import mode');
  const next=clone(current),newScreen=clone(incoming.screens[0]);
  if(mode==='add-screen'){
   if(next.screens.length>=32)throw Error('Maximum 32 screens');
   if(!safeID.test(newID)||next.screens.some(s=>s.id===newID))throw Error('Invalid new screen ID');
   remapScreenLinks(newScreen,newScreen.id,newID);
   next.screens.push(newScreen);next.activeScreenId=newID;
  }else{
   const i=next.screens.findIndex(s=>s.id===screenID);
   if(i<0)throw Error('Select a screen to replace');
   remapScreenLinks(newScreen,newScreen.id,screenID);
   next.screens[i]=newScreen;next.activeScreenId=screenID;
  }
  return validate(next);
 }
 return Object.freeze({validate,exportSingle,applyImport});
})();
