'use strict';
const $=id=>document.getElementById(id);
let cfg=null,selected=null,serial='',drag=null,copiedWidget=null;
let nexusPreviewTelemetry=null,nexusTelemetryRequestInFlight=false;
const mediaGlyphs={'previous-track':'⏮','play-pause':'⏯','next-track':'⏭','stop':'■','volume-down':'◖−','volume-up':'◖+','next':'▶','previous':'◀'};
const metrics=['cpu.temperature','gpu.temperature','cpu.utilization','gpu.utilization','memory.used','memory.utilization','network.rx_rate','network.tx_rate','disk.read_rate','disk.write_rate',"cpu.clock_mhz","cpu.load_1m","cpu.load_5m","cpu.load_15m","disk.utilization","disk.free_gb","fan.rpm","pump.rpm","gpu.vram.used_mb","gpu.vram.utilization","gpu.vram.total_mb","gpu.clock_mhz","gpu.power_w"];
const GRID=8;
let nexusSharedMacros={};
const NEXUS_MAX_SCREENS=256, NEXUS_MAX_TOTAL_WIDGETS=4096;
const totalNexusWidgets=()=>cfg?.screens.reduce((sum,s)=>sum+s.widgets.length,0)||0;
function formatNexusClock(settings={},now=new Date()){
 const value=settings.clockFormat||'24h';
 if(value==='12h')return now.toLocaleTimeString(undefined,{hour:'numeric',minute:'2-digit',hour12:true});
 if(value==='date')return now.toLocaleDateString(undefined,{weekday:'short',month:'short',day:'numeric'});
 if(value==='datetime')return now.toLocaleDateString(undefined,{month:'short',day:'numeric'})+' '+now.toLocaleTimeString(undefined,{hour:'2-digit',minute:'2-digit',hour12:false});
 return now.toLocaleTimeString(undefined,{hour:'2-digit',minute:'2-digit',hour12:false});
}
const snap=(value)=>Math.round(value/GRID)*GRID;
const clamp=(value,max)=>Math.max(0,Math.min(value,max));
function uniqueID(prefix,used){
 let candidate,seq=0;
 do {candidate=prefix+'_'+Date.now().toString(36)+(seq?'_'+seq:'');seq++}while(used.has(candidate));
 return candidate;
}
function selectedScreenID(){return $('screens').value}
function refreshActionButtons(){
 const screen=current(),count=cfg?.screens.length||0;
 for(const id of ['renameScreen','duplicateScreen'])if($(id))$(id).disabled=!screen||(id==='duplicateScreen'&&count>=NEXUS_MAX_SCREENS);
 if($('deleteScreen'))$('deleteScreen').disabled=count<=1;
 const index=cfg?.screens.findIndex(s=>s.id===selectedScreenID())??-1;
 if($('moveScreenUp'))$('moveScreenUp').disabled=index<=0;
 if($('moveScreenDown'))$('moveScreenDown').disabled=index<0||index>=count-1;
 if($('replaceScreenWithPreset'))$('replaceScreenWithPreset').disabled=!screen||$('newScreenPreset')?.value==='blank';
}
const snapPosition=(value,max)=>clamp(snap(value),Math.floor(max/GRID)*GRID);
const snapSize=(value,max)=>clamp(Math.max(GRID,snap(value)),max);
function message(t){$('message').textContent=t}
function current(){return cfg?.screens.find(s=>s.id===$('screens').value)}
function refreshScreens(){const old=$('screens').value;$('screens').replaceChildren();for(const s of cfg.screens){const opt=new Option(s.name,s.id);$('screens').add(opt)}const chosen=cfg.screens.some(s=>s.id===cfg.activeScreenId)?cfg.activeScreenId:old;$('screens').value=chosen;render();refreshApplicationScreenMappings()}
function render(){const s=current();$('screen').replaceChildren();$('properties').replaceChildren();refreshActionButtons();
 const widget=s?.widgets.find(w=>w.id===selected);
 const status=$('selectedWidgetStatus'),remove=$('deleteWidget');
 if(remove)remove.disabled=!widget;
 if($('duplicateWidget'))$('duplicateWidget').disabled=!widget||s.widgets.length>=64||totalNexusWidgets()>=NEXUS_MAX_TOTAL_WIDGETS;
 if($('copyWidget'))$('copyWidget').disabled=!widget;
 if($('pasteWidget'))$('pasteWidget').disabled=!copiedWidget||!s||s.widgets.length>=64||totalNexusWidgets()>=NEXUS_MAX_TOTAL_WIDGETS;
 if($('saveWidgetPreset'))$('saveWidgetPreset').disabled=!widget;
 if(status){
  status.replaceChildren();
  if(widget && (widget.type==='button'||widget.type==='macro')){
   const label=document.createElement('label');label.textContent='Button Label ';
   const input=document.createElement('input');input.type='text';input.maxLength=256;
   input.value=widget.settings?.label||'';input.placeholder='Button label';
   input.setAttribute('aria-label','Button Label');
   input.addEventListener('input',()=>{
    widget.settings??={};widget.settings.label=input.value;
    const el=[...$('screen').children].find(el=>el.dataset.id===widget.id);
    if(el){const icon=el.querySelector('.nexus-icon, .nexus-custom-icon');el.textContent=input.value;if(icon)el.prepend(icon)}
   });
   label.append(input);status.append(label);
  }else{
   status.textContent=widget?(widget.settings?.label||widget.settings?.text||widget.type):'Select a widget in the preview to edit it.';
  }
 }
 if(!s)return;
 $('background').value=s.background||'';
 const display=$('screen');display.classList.remove('nexus-bg-preset_aurora','nexus-bg-preset_neon','nexus-bg-preset_circuit','nexus-bg-preset_sunset','nexus-bg-preset_pulse');
 if(s.background?.startsWith('preset_'))display.classList.add('nexus-bg-'+s.background);
 // The same GET endpoint serves static PNG/JPEG and animated GIF; browsers
 // animate GIF previews automatically while the physical Nexus uses frames.
 const uploaded=Boolean(s.background&&!s.background.startsWith('preset_'));
 display.style.backgroundImage=uploaded?'url("/api/nexus/background?serial='+encodeURIComponent(serial)+'&id='+encodeURIComponent(s.background)+'")':'';
 display.style.backgroundSize=uploaded?'100% 100%':'';
 display.style.backgroundRepeat=uploaded?'no-repeat':'';
 display.style.filter='brightness('+(cfg.brightness||100)+'%)';
 for(const w of s.widgets){const el=document.createElement('div');el.className='widget'+(selected===w.id?' selected':'');el.style.cssText=`left:${w.x}px;top:${w.y}px;width:${w.width}px;height:${w.height}px`;if(['button','macro','metric','bar','graph','text','clock','image'].includes(w.type)){const st=w.settings||{};if(w.type==='button'||w.type==='macro')el.classList.add('nexus-button-style');else el.classList.add('nexus-colored-widget');el.classList.add('nexus-style-'+(st.style||'solid'));el.style.setProperty('--button-bg',st.backgroundColor||'#1b2935');if((st.style||'solid')==='solid'&&['horizontal','vertical'].includes(st.gradientDirection))el.style.backgroundImage='linear-gradient('+(st.gradientDirection==='horizontal'?'90deg':'180deg')+','+(st.backgroundColor||'#1b2935')+','+(st.gradientColor||'#1b2935')+')';el.style.borderRadius=Math.min(Number(st.cornerRadius)||0,Math.floor(Math.min(w.width,w.height)/2))+'px';el.style.opacity=String((st.opacity===undefined?100:Number(st.opacity))/100);el.style.setProperty('--button-ink',st.textColor||'#ebf5fc');el.style.setProperty('--button-border',st.borderColor||'#80bdd9');el.style.setProperty('--button-border-visibility',st.borderColor?'solid':'solid');el.style.fontSize=st.fontSize==='small'?'10px':st.fontSize==='large'?'17px':'11px';el.style.textAlign=st.textAlign||'left';el.style.textShadow=st.textShadow==='soft'?'1px 1px 1px rgba(0,0,0,.7)':st.textShadow==='strong'?'2px 2px 2px rgba(0,0,0,.85)':'none';if((w.type==='text'||((w.type==='button'||w.type==='macro')&&!st.icon&&!st.customIcon))&&st.verticalAlign&&st.verticalAlign!=='top'){
 el.style.display='flex';el.style.alignItems=st.verticalAlign==='middle'?'center':'flex-end';el.style.justifyContent=st.textAlign==='right'?'flex-end':st.textAlign==='center'?'center':'flex-start'
 }}if(w.type==='graph')el.classList.add('nexus-graph-widget');el.textContent=w.type==='image'?'':w.type==='graph'?(w.settings?.label||'History: '+w.metric):w.type==='clock'?formatNexusClock(w.settings):w.type==='text'?(w.settings?.text||'Text'):((w.type==='button'||w.type==='macro')?(w.settings?.label||''):(w.settings?.label||w.metric));if((w.type==='button'||w.type==='macro'||w.type==='image')&&w.settings?.customIcon){
  el.classList.add('nexus-icon-widget');
  const picture=document.createElement('img');
  picture.className='nexus-custom-icon';
  picture.alt='';
  picture.src='/api/nexus/icon?serial='+encodeURIComponent(serial)+'&id='+encodeURIComponent(w.settings.customIcon);
  picture.draggable=false;if(w.type==='image'){picture.style.width='100%';picture.style.height='100%';picture.style.objectFit='contain'}el.prepend(picture);
 }else if((w.type==='button'||w.type==='macro')&&mediaGlyphs[w.settings?.icon]){
  el.classList.add('nexus-icon-widget');
  const glyph=document.createElement('span');glyph.className='nexus-icon';
  glyph.textContent=mediaGlyphs[w.settings.icon];el.prepend(glyph)
 }if(w.type==='graph'){
  const plot=document.createElement('canvas');plot.className='nexus-graph-plot';
  plot.width=Math.max(8,w.width-8);plot.height=Math.max(8,w.height-27);
  plot.setAttribute('aria-hidden','true');el.append(plot);
 }el.dataset.id=w.id;$('screen').append(el);
 if(w.type==='graph')paintNexusGraph(el,w)
 }
 const w=s.widgets.find(w=>w.id===selected);if(!w)return;
 const properties=$('properties');
 const dimensionsRow=document.createElement('div');dimensionsRow.className='nexus-properties-row nexus-dimensions-row';dimensionsRow.setAttribute('aria-label','Widget dimensions');
 const colorsRow=document.createElement('div');colorsRow.className='nexus-properties-row nexus-colors-row';colorsRow.setAttribute('aria-label','Widget colors');
 const styleRow=document.createElement('div');styleRow.className='nexus-properties-row nexus-style-row';styleRow.setAttribute('aria-label','Widget style and settings');
 // Organize the selected widget's controls into concise, accessible groups.
 const section=(title,opened)=>{
  const details=document.createElement('details');
  details.className='nexus-property-section';
  details.open=opened;
  const summary=document.createElement('summary');
  summary.textContent=title;
  details.append(summary);
  properties.append(details);
  return details;
 };
 const geometrySection=section('Position & size',true);
 const appearanceSection=section('Appearance',true);
 const contentSection=section('Text & data',false);
 const touchSection=section('Actions & icons',false);
 geometrySection.append(dimensionsRow);
 appearanceSection.append(colorsRow);
 const appearanceRow=document.createElement('div');
 appearanceRow.className='nexus-properties-row nexus-appearance-row';
 appearanceSection.append(appearanceRow);
 contentSection.append(styleRow);
 const touchRow=document.createElement('div');touchRow.className='nexus-properties-row nexus-touch-row';
 touchSection.append(touchRow);
 const fields=(w.type==='button'||w.type==='macro')?[['x','number'],['y','number'],['width','number'],['height','number']]:[['x','number'],['y','number'],['width','number'],['height','number'],['label','text'],['text','text'],['metric','select']];
 for(const [key,type] of fields){if(key==='metric'&&(w.type==='text'||w.type==='image'||w.type==='clock'||w.type==='button'||w.type==='macro'))continue;if(key==='text'&&w.type!=='text')continue;
 const label=document.createElement('label');label.textContent=key+' ';
 let input;if(type==='select'){input=document.createElement('select');for(const m of metrics)input.add(new Option(m,m));input.value=w.metric}else{input=document.createElement('input');input.type=type;input.value=(key in w?w[key]:(w.settings||{})[key])??''}
 input.addEventListener('change',()=>{let v=type==='number'?Number(input.value):input.value;if(['x','y','width','height'].includes(key)){if(!Number.isFinite(v))return message('Enter a valid number');if(key==='width')w.width=Math.max(1,Math.min(640,Math.round(v)));if(key==='height')w.height=Math.max(1,Math.min(48,Math.round(v)));if(key==='x')w.x=clamp(Math.round(v),640-w.width);if(key==='y')w.y=clamp(Math.round(v),48-w.height);w.x=clamp(w.x,640-w.width);w.y=clamp(w.y,48-w.height)}else if(key==='metric')w.metric=v;else{w.settings??={};w.settings[key]=v}render()});label.append(input);(['x','y','width','height'].includes(key)?dimensionsRow:styleRow).append(label)}
 renderWidgetColors(w,colorsRow);
 if(['button','macro','metric','bar','graph','text','clock','image'].includes(w.type)){
  // Opacity is an independent percentage, not a preset background style.
  const radiusLabel=document.createElement('label');
  radiusLabel.textContent='Corner radius ';
  const radius=document.createElement('input');
  radius.type='number';radius.min='0';radius.max='24';radius.step='1';
  radius.value=w.settings?.cornerRadius??'0';
  radius.setAttribute('aria-label','Corner radius');
  radius.addEventListener('change',()=>{
   const v=Number(radius.value);if(!Number.isFinite(v))return;
   w.settings??={};w.settings.cornerRadius=String(Math.max(0,Math.min(24,Math.round(v))));
   render();
  });
  radiusLabel.append(radius,' px');appearanceRow.append(radiusLabel);
  const opacityLabel=document.createElement('label');opacityLabel.className='nexus-opacity-control';
  opacityLabel.textContent='Opacity ';
  const opacitySlider=document.createElement('input');
  opacitySlider.type='range';opacitySlider.min='0';opacitySlider.max='100';opacitySlider.step='1';
  const opacityInput=document.createElement('input');
  opacityInput.type='number';opacityInput.min='0';opacityInput.max='100';opacityInput.step='1';
  const initialOpacity=w.settings?.opacity??'100';
  opacitySlider.value=initialOpacity;opacityInput.value=initialOpacity;
  opacitySlider.setAttribute('aria-label','Opacity slider');
  opacityInput.setAttribute('aria-label','Opacity percentage');
  const updateOpacity=value=>{
   if(value.trim()==='')return;
   const n=Number(value);if(!Number.isFinite(n))return;
   const percentage=Math.max(0,Math.min(100,Math.round(n)));
   w.settings??={};w.settings.opacity=String(percentage);
   opacitySlider.value=String(percentage);opacityInput.value=String(percentage);
   const preview=[...$('screen').children].find(el=>el.dataset.id===w.id);
   if(preview)preview.style.opacity=String(percentage/100);
  };
  opacitySlider.addEventListener('input',()=>updateOpacity(opacitySlider.value));
  opacityInput.addEventListener('change',()=>updateOpacity(opacityInput.value));
  opacityLabel.append(opacitySlider,opacityInput,'%');
  appearanceRow.append(opacityLabel);
  const directionLabel=document.createElement('label');directionLabel.textContent='Gradient ';
  const direction=document.createElement('select');direction.setAttribute('aria-label','Background gradient direction');
  for(const [value,name] of [['none','None'],['horizontal','Left to right'],['vertical','Top to bottom']])direction.add(new Option(name,value));
  direction.value=w.settings?.gradientDirection||'none';
  direction.addEventListener('change',()=>{w.settings??={};w.settings.gradientDirection=direction.value;if(direction.value!=='none'&&!w.settings.gradientColor)w.settings.gradientColor='#0b1824';render()});
  directionLabel.append(direction);appearanceRow.append(directionLabel);
  if(direction.value!=='none'){
   const label=document.createElement('label');label.textContent='Gradient end color ';
   const input=document.createElement('input');input.type='color';
   const candidate=w.settings?.gradientColor;
   input.value=/^#[0-9a-fA-F]{6}$/.test(candidate||'')?candidate:'#0b1824';
   input.setAttribute('aria-label','Gradient end color');
   input.addEventListener('input',()=>{w.settings??={};w.settings.gradientColor=input.value;const el=[...$('screen').children].find(el=>el.dataset.id===w.id);if(el)el.style.backgroundImage='linear-gradient('+(direction.value==='horizontal'?'90deg':'180deg')+','+(w.settings.backgroundColor||'#1b2935')+','+input.value+')'});
   input.addEventListener('change',()=>{w.settings.gradientColor=input.value;render()});
   label.append(input);appearanceRow.append(label);
  }
 }
 if(['text','clock','metric','bar','graph','button','macro'].includes(w.type)){
  const control=(name,key,options)=>{
   const label=document.createElement('label');label.textContent=name+' ';
   const select=document.createElement('select');
   for(const [id,title] of options)select.add(new Option(title,id));
   select.value=w.settings?.[key]||options[0][0];
   select.addEventListener('change',()=>{w.settings??={};w.settings[key]=select.value;render()});
   label.append(select);styleRow.append(label);
  };
  control('Font size','fontSize',[['normal','Normal'],['small','Small'],['large','Large']]);
  control('Text alignment','textAlign',[['left','Left'],['center','Center'],['right','Right']]);
  control('Text shadow','textShadow',[['none','None'],['soft','Soft'],['strong','Strong']]);
  if(w.type==='text'||w.type==='button'||w.type==='macro')control('Vertical alignment','verticalAlign',[['top','Top'],['middle','Middle'],['bottom','Bottom']]);
 }
 if(w.type==='clock'){
  const label=document.createElement('label');label.textContent='Clock format ';
  const input=document.createElement('select');
  for(const [value,title] of [['24h','24-hour time'],['12h','12-hour time'],['date','Date'],['datetime','Date and time']])input.add(new Option(title,value));
  input.value=w.settings?.clockFormat||'24h';
  input.addEventListener('change',()=>{w.settings??={};w.settings.clockFormat=input.value;render()});
  label.append(input);styleRow.append(label);
 }
 if(w.type==='button'||w.type==='macro'||w.type==='image')renderTouchProperties(w,touchRow);
}
$('screen').addEventListener('pointerdown',e=>{const el=e.target.closest('.widget');if(!el)return;selected=el.dataset.id;const w=current().widgets.find(w=>w.id===selected);drag={x:e.clientX,y:e.clientY,startX:w.x,startY:w.y,id:e.pointerId};$('screen').setPointerCapture(e.pointerId);render()});
$('screen').addEventListener('pointermove',e=>{if(!drag||drag.id!==e.pointerId)return;const w=current().widgets.find(w=>w.id===selected);if(!w)return;w.x=snapPosition(drag.startX+e.clientX-drag.x,640-w.width);w.y=snapPosition(drag.startY+e.clientY-drag.y,48-w.height);const el=[...$('screen').children].find(el=>el.dataset.id===w.id);if(el){el.style.left=w.x+'px';el.style.top=w.y+'px'}});
$('screen').addEventListener('pointerup',()=>{drag=null;render()});$('screen').addEventListener('pointercancel',()=>{drag=null;render()});
const editKeys=new Set(['ArrowLeft','ArrowRight','ArrowUp','ArrowDown']);
$('screen').addEventListener('keydown',event=>{
 const screen=current();if(!screen)return;
 const focus=event.target;
 if(focus!==$('screen')&&focus?.closest?.('input, textarea, select'))return;
 const widgets=screen.widgets;
 if(event.key==='['||event.key===']'){
  if(!widgets.length)return;
  event.preventDefault();
  const at=widgets.findIndex(widget=>widget.id===selected);
  const shift=event.key===']'?1:-1;
  const index=at<0?(shift===1?0:widgets.length-1):(at+shift+widgets.length)%widgets.length;
  selected=widgets[index].id;render();return;
 }
 if(event.key==='Escape'){event.preventDefault();selected=null;render();return}
 const widget=widgets.find(item=>item.id===selected);
 if((event.ctrlKey||event.metaKey)&&event.key.toLowerCase()==='c'&&widget){
  event.preventDefault();$('copyWidget')?.click();return;
 }
 if((event.ctrlKey||event.metaKey)&&event.key.toLowerCase()==='v'&&copiedWidget){
  event.preventDefault();$('pasteWidget')?.click();return;
 }
 if(!widget)return;
 if(event.key==='Delete'&&!event.ctrlKey&&!event.metaKey){
  event.preventDefault();$('deleteWidget')?.click();return;
 }
 if(!editKeys.has(event.key)||event.ctrlKey||event.altKey||event.metaKey)return;
 event.preventDefault();
 const horizontal=event.key==='ArrowLeft'||event.key==='ArrowRight';
 const positive=event.key==='ArrowRight'||event.key==='ArrowDown';
 const delta=positive?GRID:-GRID;
 if(event.shiftKey){
  if(horizontal){widget.width=snapSize(widget.width+delta,640-widget.x)}
  else{widget.height=snapSize(widget.height+delta,48-widget.y)}
 }else{
  if(horizontal){widget.x=snapPosition(widget.x+delta,640-widget.width)}
  else{widget.y=snapPosition(widget.y+delta,48-widget.height)}
 }
 render();
 message('Widget position updated. Save Layout to apply.');
});

async function loadDevices(){try{const response=await fetch('/api/devices/');const json=await response.json();const data=json.devices||json.device||json.data||json;const entries=Object.entries(data);$('device').replaceChildren();for(const [key,v] of entries){if(/nexus/i.test(JSON.stringify(v))){$('device').add(new Option((v.product||v.Product||'Nexus')+' — '+key,key))}}if(!$('device').length)message('No Nexus found automatically. Enter its serial below.')}catch(e){message(e.message)}}
async function load(){serial=$('deviceId')?.value||$('device')?.value||prompt('Nexus device serial');if(!serial)return;const res=await fetch('/api/nexus/screens?serial='+encodeURIComponent(serial));if(!res.ok)throw Error(await res.text());cfg=await res.json();await loadSharedMacroProfiles();selected=null;const mode=$('cyclingMode');const interval=$('cyclingSeconds');if(mode)mode.value=cfg.cyclingMode||'manual';if(interval){const seconds=cfg.cyclingSeconds||15;if(![...interval.options].some(o=>Number(o.value)===seconds))interval.add(new Option(seconds+' seconds (saved)',String(seconds)));interval.value=String(seconds)}syncCycleInterval();if($('displayBrightness'))$('displayBrightness').value=String(cfg.brightness||100);refreshScreens();refreshWidgetLibrary();message('')}
if($('load'))$('load').onclick=()=>load().catch(e=>message(e.message));
$('screens').onchange=()=>{cfg.activeScreenId=$('screens').value;selected=null;render()};
// Starter designs are simply saved-screen JSON, copied into the regular
// screen list. Every resulting screen is edited and saved like any other one.
const builtInScreens=new Set(['system-overview','performance-bars','network-and-storage','media-deck','navigation']);
async function loadExampleScreen(id){
 if(!builtInScreens.has(id))throw Error('Unknown built-in screen');
 const response=await fetch('/static/nexus/templates/'+id+'.json',{cache:'no-cache'});
 if(!response.ok)throw Error('Could not load example screen ('+response.status+')');
 const config=await response.json();
 if(config.version!==1||!Array.isArray(config.screens)||config.screens.length!==1)throw Error('Unsupported screen JSON');
 const screen=config.screens[0];
 if(!screen||typeof screen.name!=='string'||!screen.name||screen.name.length>80||!Array.isArray(screen.widgets)||screen.widgets.length>64)throw Error('Invalid example screen');
 return JSON.parse(JSON.stringify(screen));
}
function relinkScreenID(screen, newID){
 const previousID=screen.id;screen.id=newID;
 for(const widget of screen.widgets){
  if(widget.type==='button'&&widget.action==='screen'&&widget.target===previousID)widget.target=newID;

 }
 return screen;
}
$('newScreenPreset')?.addEventListener('change',refreshActionButtons);
$('addScreen').onclick=async()=>{
 try{
  if(!cfg)return message('Load a Nexus layout first.');
  if(cfg.screens.length>=NEXUS_MAX_SCREENS)return message('Maximum 256 screens.');
  const kind=$('newScreenPreset')?.value||'blank';
  const id=uniqueID('screen',new Set(cfg.screens.map(screen=>screen.id)));
  let screen;
  if(kind==='blank'){
   const name=prompt('Screen name','New screen')?.trim();
   if(!name)return;
   if(name.length>80)return message('Screen name must be 80 characters or fewer.');
   screen={id,name,widgets:[]};
  }else{
   screen=relinkScreenID(await loadExampleScreen(kind),id);
   const used=cfg.screens.filter(item=>item.name===screen.name||item.name.startsWith(screen.name+' (')).length;
   if(used){const suffix=' ('+(used+1)+')';screen.name=screen.name.slice(0,80-suffix.length)+suffix}
  }
  cfg.screens.push(screen);cfg.activeScreenId=id;selected=null;refreshScreens();
  message('Screen added. Edit and Save Layout when ready.');
 }catch(e){message(e.message)}
};
$('replaceScreenWithPreset')?.addEventListener('click',async()=>{
 try{
  if(!cfg)return message('Load a Nexus layout first.');
  const selectedScreen=current(),kind=$('newScreenPreset')?.value;
  if(!selectedScreen||!builtInScreens.has(kind))return;
  const replacement=await loadExampleScreen(kind);
  if(!confirm('Replace "'+selectedScreen.name+'" with "'+replacement.name+'"? The existing widgets and background will be discarded when saved.'))return;
  relinkScreenID(replacement,selectedScreen.id);
  const index=cfg.screens.findIndex(screen=>screen.id===selectedScreen.id);
  cfg.screens[index]=replacement;cfg.activeScreenId=replacement.id;selected=null;refreshScreens();
  message('Screen replaced in the editor. Click Save Layout to apply.');
 }catch(e){message(e.message)}
});
function moveSelectedScreen(direction){
 if(!cfg)return;
 const index=cfg.screens.findIndex(screen=>screen.id===selectedScreenID());
 const to=index+direction;
 if(index<0||to<0||to>=cfg.screens.length)return;
 [cfg.screens[index],cfg.screens[to]]=[cfg.screens[to],cfg.screens[index]];
 cfg.activeScreenId=selectedScreenID();
 refreshScreens();message('Screen order changed. Save Layout to apply.');
}
$('moveScreenUp')?.addEventListener('click',()=>moveSelectedScreen(-1));
$('moveScreenDown')?.addEventListener('click',()=>moveSelectedScreen(1));
$('renameScreen')?.addEventListener('click',()=>{
 const s=current();if(!s)return;
 const name=prompt('Rename screen',s.name)?.trim();
 if(name===undefined||name===null||!name||name===s.name)return;
 if(name.length>80)return message('Screen name must be 80 characters or fewer');
 s.name=name;refreshScreens();message('Screen renamed. Save Layout to apply.');
});
$('duplicateScreen')?.addEventListener('click',()=>{
 const source=current();if(!source||!cfg)return;
 if(cfg.screens.length>=NEXUS_MAX_SCREENS)return message('Maximum 256 screens');
 const newID=uniqueID('screen',new Set(cfg.screens.map(s=>s.id)));
 const name=(source.name.slice(0,73)+' copy').slice(0,80);
 // JSON round trip gives the duplicate fully independent widget settings and macro steps.
 const duplicate=JSON.parse(JSON.stringify(source));
 duplicate.id=newID;duplicate.name=name;
 for(const widget of duplicate.widgets){
  if(widget.type==='button'&&widget.action==='screen'&&widget.target===source.id)widget.target=newID;

 }
 const at=cfg.screens.findIndex(s=>s.id===source.id);
 cfg.screens.splice(at+1,0,duplicate);
 cfg.activeScreenId=newID;selected=null;refreshScreens();
 message('Screen duplicated. Save Layout to apply.');
});
$('deleteScreen').onclick=()=>{
 if(!cfg||cfg.screens.length<=1)return message('Keep at least one screen');
 const removed=selectedScreenID();
 cfg.screens=cfg.screens.filter(s=>s.id!==removed);
 const fallback=cfg.screens[0].id;
 // Keep existing screen-navigation actions valid when their destination disappears.
 for(const screen of cfg.screens)for(const w of screen.widgets){
  if(w.type==='button'&&w.action==='screen'&&w.target===removed)w.target=fallback;

 }
 for(const [appID,target] of Object.entries(cfg.applicationScreens||{}))if(target===removed)cfg.applicationScreens[appID]=fallback;
 cfg.activeScreenId=fallback;selected=null;refreshScreens();
 message('Screen removed. Navigation links to it now point to '+cfg.screens[0].name+'. Save Layout to apply.');
};
async function loadSharedMacroProfiles(){
 try{
  const response=await fetch('/api/macro/');
  if(!response.ok)throw Error('Unable to load saved macros');
  const json=await response.json();
  nexusSharedMacros=(json?.status===1&&json.data&&typeof json.data==='object')?json.data:{};
 }catch(_err){nexusSharedMacros={}}
}
function renderWidgetColors(w,panel){
 if(!['button','macro','metric','bar','graph','text','clock','image'].includes(w.type))return;
 for(const [key,name,defaultColor] of [['backgroundColor','Background','#1b2935'],['textColor','Text & icon','#ebf5fc'],['borderColor','Border','#80bdd9']]){
  const label=document.createElement('label');label.textContent=name+' ';
  const input=document.createElement('input');input.type='color';input.value=/^#[0-9a-fA-F]{6}$/.test(w.settings?.[key]||'')?w.settings[key]:defaultColor;
  input.oninput=()=>{
   w.settings??={};w.settings[key]=input.value;
   const widget=[...$('screen').children].find(el=>el.dataset.id===w.id);
   if(widget)widget.style.setProperty(key==='backgroundColor'?'--button-bg':key==='textColor'?'--button-ink':'--button-border',input.value);
  };
  input.onchange=()=>{w.settings[key]=input.value;render()};
  label.append(input);panel.append(label);
 }
}
function renderTouchProperties(w,panel){
 const field=(name,value,choices,change)=>{
  const label=document.createElement('label');label.textContent=name+' ';
  const input=document.createElement('select');
  for(const [v,t] of choices)input.add(new Option(t,v));
  input.value=value;input.onchange=()=>{change(input.value);render()};
  label.append(input);panel.append(label);
 };
 const actions=[['next','Next screen'],['previous','Previous screen'],['screen','Go to screen'],['media','Media control'],['device','Device RGB control'],['macro','Run saved macro'],['hotkey','Keyboard shortcut'],['mouse','Mouse action'],['url','Open URL'],['application','Launch desktop app'],['text','Type text']];
 const hardwareActions=[['rgb-on','Enable device RGB'],['rgb-off','Disable device RGB']];
 const media=[['play-pause','Play/Pause'],['stop','Stop'],['volume-up','Volume +'],['volume-down','Volume -'],['next-track','Next track'],['previous-track','Previous track']];
 const targets=cfg.screens.map(s=>[s.id,s.name]);
 const mouseActions=[['left','Left click'],['double-left','Double left click'],['right','Right click'],['middle','Middle click'],['back','Back button'],['forward','Forward button'],['scroll-up','Scroll up'],['scroll-down','Scroll down'],['scroll-left','Scroll left'],['scroll-right','Scroll right']];
 const textInput=(name,value,update)=>{
  const label=document.createElement('label');label.textContent=name+' ';
  const input=document.createElement('input');input.type='text';input.maxLength=80;
  input.placeholder='Type this text';input.value=value||'';
  input.setAttribute('aria-label',name);
  input.addEventListener('change',()=>{update(input.value);message('Text entry updated. Save Layout to apply.')});
  label.append(input);panel.append(label);
 };
 const shortcutInput=(name,value,update)=>{
  const label=document.createElement('label');label.textContent=name+' ';
  const input=document.createElement('input');input.type='text';input.maxLength=80;
  input.placeholder='Ctrl+Alt+T';input.value=value||'';
  input.setAttribute('aria-label',name);
  input.addEventListener('change',()=>{update(input.value.trim());message('Shortcut updated. Save Layout to apply.')});
  label.append(input);panel.append(label);
 };
 if(w.type==='button'||w.type==='macro'||w.type==='image'){
  field('Style',w.settings?.style||'solid',[['solid','Solid fill'],['outline','Outline only'],['flat','No fill']],v=>{w.settings??={};w.settings.style=v});
 }
 if(w.type==='image'){
  field('Image',w.settings?.customIcon?'custom':'none',[['none','No image'],['custom','Upload image']],v=>{
   w.settings??={};if(v==='none'){delete w.settings.customIcon;delete w.settings.icon}else{w.settings.icon='custom'}
  });
 }else{
 field('Icon',w.settings?.customIcon?'custom':w.settings?.icon||'none',[['none','No icon'],...Object.keys(mediaGlyphs).map(id=>[id,id]),['custom','Custom image']],v=>{
  w.settings??={};
  if(v!=='custom')delete w.settings.customIcon;
  if(v==='none')delete w.settings.icon;else w.settings.icon=v;
 });
 }
 if(w.settings?.customIcon || w.settings?.icon==='custom'){
  const wrapper=document.createElement('label');wrapper.className='nexus-custom-icon-upload';
  wrapper.textContent='Custom icon (PNG/JPEG/GIF) ';
  const file=document.createElement('input');file.type='file';file.accept='image/png,image/jpeg,image/gif';
  file.setAttribute('aria-label','Upload custom icon');
  wrapper.append(file);
  const upload=document.createElement('button');upload.type='button';upload.textContent='Upload icon';
  upload.onclick=async()=>{
   const source=file.files?.[0];
   if(!source)return message('Choose a PNG, JPEG or GIF icon first');
   if(source.size>512*1024)return message('Icon exceeds 512 KiB');
   if(!['image/png','image/jpeg','image/gif'].includes(source.type))return message('Select a PNG, JPEG or GIF image');
   upload.disabled=true;
   try{
    const id='icon_'+Date.now()+'_'+Math.floor(Math.random()*1000000);
    const res=await fetch('/api/nexus/icon?serial='+encodeURIComponent(serial)+'&id='+encodeURIComponent(id),{method:'POST',headers:{'Content-Type':source.type},body:source});
    if(!res.ok)throw Error(await res.text());
    w.settings??={};w.settings.customIcon=id;delete w.settings.icon;
    render();message('Custom icon uploaded. Save Layout to apply it.');
   }catch(e){message(e.message);upload.disabled=false}
  };
  panel.append(wrapper,upload);
 }
 if(w.type==='button'){
  field('Action',w.action||'next',actions,v=>{w.action=v;w.target=v==='screen'?cfg.screens[0].id:v==='media'?'play-pause':v==='device'?'rgb-on':v==='hotkey'?'Ctrl+Alt+T':v==='mouse'?'left':v==='url'?'https://example.com':v==='application'?'org.gnome.Terminal.desktop':v==='text'?'Hello':'';if(v==='media'&&!w.settings?.customIcon){w.settings??={};w.settings.icon='play-pause'}if(v==='macro')w.target=Object.keys(nexusSharedMacros)[0]||'1'});
  if(w.action==='macro'){
   const macros=Object.entries(nexusSharedMacros).map(([id,m])=>[id,m.name||('Macro #'+id)]);
   if(macros.length)field('Saved macro',w.target,macros,v=>w.target=v);
   else {const note=document.createElement('small');note.textContent='No saved macros found. Create one in OpenLinkHub Macros.';panel.append(note)}
   const link=document.createElement('a');link.href='/macros';link.textContent='Open macro editor';link.target='_blank';link.rel='noopener noreferrer';panel.append(link);
  }
  if(w.action==='screen')field('Target screen',w.target,targets,v=>w.target=v);
  if(w.action==='hotkey')shortcutInput('Shortcut',w.target,v=>w.target=v);
  if(w.action==='text'){
   textInput('Text to type',w.target,v=>w.target=v);
   const note=document.createElement('small');note.textContent='US keyboard layout, printable ASCII only (1–80 characters). Saved in plain text: do not enter passwords.';panel.append(note);
  }
  if(w.action==='mouse')field('Mouse command',w.target,mouseActions,v=>w.target=v);
  if(w.action==='url'||w.action==='application'){
   const label=document.createElement('label');label.textContent=w.action==='url'?'Web URL ':'Desktop entry ID ';
   const input=document.createElement('input');input.type='text';input.maxLength=w.action==='url'?256:128;
   input.value=w.target||'';input.placeholder=w.action==='url'?'https://example.com':'org.gnome.Terminal.desktop';
   input.setAttribute('aria-label',w.action==='url'?'Open URL':'Desktop entry ID');
   input.addEventListener('change',()=>{w.target=input.value.trim();message('Launcher updated. Save Layout to apply.')});
   label.append(input);panel.append(label);
   const note=document.createElement('small');note.textContent='Requires a non-root Linux desktop session; desktop apps use gtk-launch, URLs use xdg-open. No shell commands.';
   panel.append(note);
  }
  if(w.action==='device')field('Device RGB action',w.target,hardwareActions,v=>w.target=v);
  if(w.action==='media')field('Media action',w.target,media,v=>{const old=w.target;w.target=v;if(!w.settings?.customIcon&&(!w.settings?.icon||w.settings.icon===old)){w.settings??={};w.settings.icon=v}});
 }


}
function addWidget(type){const s=current();if(!s)return;if(s.widgets.length>=64)return message('Maximum 64 widgets');if(totalNexusWidgets()>=NEXUS_MAX_TOTAL_WIDGETS)return message('Maximum 4096 widgets across all screens');const id=uniqueID('widget',new Set(s.widgets.map(w=>w.id)));const w={id,type,metric:(type==='text'||type==='image'||type==='clock'||type==='button'||type==='macro')?'':'cpu.temperature',x:0,y:0,width:152,height:48,settings:type==='image'?{icon:'custom',style:'flat'}:type==='text'?{text:'New text'}:type==='clock'?{clockFormat:'24h',label:''}:{label:(type==='button'?'Button':type==='macro'?'Macro':'CPU')},...(type==='button'?{action:'next'}:{}),};s.widgets.push(w);selected=id;render()}
$('addMetric').onclick=()=>addWidget('metric');$('addClock')?.addEventListener('click',()=>addWidget('clock'));$('addText').onclick=()=>addWidget('text');$('addImage')?.addEventListener('click',()=>addWidget('image'));$('addBar').onclick=()=>addWidget('bar');$('addGraph')?.addEventListener('click',()=>addWidget('graph'));$('addButton')?.addEventListener('click',()=>addWidget('button'));$('addMacro')?.addEventListener('click',()=>{
 if(!Object.keys(nexusSharedMacros).length)return message('Create a macro in OpenLinkHub Macros first.');
 addWidget('button');
 const w=current()?.widgets.find(x=>x.id===selected);
 if(w){w.action='macro';w.target=Object.keys(nexusSharedMacros)[0];w.settings.label='Macro';render()}
});
$('copyWidget')?.addEventListener('click',()=>{
 const source=current()?.widgets.find(w=>w.id===selected);
 if(!source)return;
 copiedWidget=JSON.parse(JSON.stringify(source));
 render();message('Widget copied. Select a screen and click Paste widget.');
});
$('pasteWidget')?.addEventListener('click',()=>{
 const screen=current();
 if(!screen||!copiedWidget)return;
 if(screen.widgets.length>=64)return message('Maximum 64 widgets');
 if(totalNexusWidgets()>=NEXUS_MAX_TOTAL_WIDGETS)return message('Maximum 4096 widgets across all screens');
 const copy=JSON.parse(JSON.stringify(copiedWidget));
 copy.id=uniqueID('widget',new Set(screen.widgets.map(w=>w.id)));
 copy.x=snapPosition(copy.x+16,640-copy.width);
 copy.y=snapPosition(copy.y+8,48-copy.height);
 screen.widgets.push(copy);selected=copy.id;render();
 message('Widget pasted. Save Layout to apply.');
});
$('duplicateWidget')?.addEventListener('click',()=>{
 const screen=current();if(!screen)return;
 const source=screen.widgets.find(w=>w.id===selected);
 if(!source)return;
 if(screen.widgets.length>=64)return message('Maximum 64 widgets');
 if(totalNexusWidgets()>=NEXUS_MAX_TOTAL_WIDGETS)return message('Maximum 4096 widgets across all screens');
 const duplicate=JSON.parse(JSON.stringify(source));
 duplicate.id=uniqueID('widget',new Set(screen.widgets.map(w=>w.id)));
 // Shift one grid cell when space allows; otherwise keep position, with the copy on top.
 duplicate.x=source.x+GRID<=640-source.width?source.x+GRID:source.x;
 duplicate.y=source.y+GRID<=48-source.height?source.y+GRID:source.y;
 const index=screen.widgets.findIndex(w=>w.id===source.id);
 screen.widgets.splice(index+1,0,duplicate);
 selected=duplicate.id;render();message('Widget duplicated. Save Layout to apply.');
});
$('deleteWidget').onclick=()=>{const s=current();if(!s)return;s.widgets=s.widgets.filter(w=>w.id!==selected);selected=null;render()};
$('background').onchange=()=>{const s=current();if(s){s.background=$('background').value.trim();render()}};
$('upload').onclick=async()=>{try{const f=$('image').files[0];if(!f||!current())return message('Choose an image and load a screen');if(f.size>4*1024*1024)throw Error('Image exceeds 4 MiB');if(!['image/png','image/jpeg','image/gif'].includes(f.type))throw Error('Choose a PNG, JPEG or GIF background');const id='background_'+Date.now();const res=await fetch('/api/nexus/background?serial='+encodeURIComponent(serial)+'&id='+id,{method:'POST',headers:{'Content-Type':f.type},body:f});if(!res.ok)throw Error(await res.text());current().background=id;render();message('Background uploaded. Save layout to apply it.')}catch(e){message(e.message)}};
function syncCycleInterval(){
 const interval=$('cyclingSeconds'),mode=$('cyclingMode');
 if(interval&&mode)interval.disabled=mode.value!=='automatic';
}
$('cyclingMode')?.addEventListener('change',syncCycleInterval);
syncCycleInterval();
$('displayBrightness')?.addEventListener('change',()=>{if(!cfg)return;cfg.brightness=Number($('displayBrightness').value);render()});
$('save').onclick=async()=>{try{if(!cfg)return;if($('displayBrightness'))cfg.brightness=Number($('displayBrightness').value);const mode=$('cyclingMode');const interval=$('cyclingSeconds');if(mode){cfg.cyclingMode=mode.value;cfg.cyclingSeconds=Number(interval?.value||15)}cfg.activeScreenId=$('screens').value;const res=await fetch('/api/nexus/screens?serial='+encodeURIComponent(serial),{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(cfg)});if(!res.ok)throw Error(await res.text());message('Saved successfully')}catch(e){message(e.message)}};
function exportNexusJSON(single){
 try{
  if(!cfg)return message('Load a Nexus layout first.');
  const selectedScreen=current();
  if(single&&!selectedScreen)return message('Select a screen to export.');
  const data=single?NexusLayoutFiles.exportSingle(cfg,selectedScreen.id):NexusLayoutFiles.validate(cfg);
  const name=single?'nexus-screen-'+selectedScreen.id+'.json':'nexus-layout.json';
  const file=new Blob([JSON.stringify(data,null,2)+'\\n'],{type:'application/json'});
  const url=URL.createObjectURL(file);
  const link=document.createElement('a');link.href=url;link.download=name;
  link.click();
  setTimeout(()=>URL.revokeObjectURL(url),0);
  message((single?'Screen':'Layout')+' exported.');
 }catch(e){message(e.message)}
}
$('exportLayout')?.addEventListener('click',()=>exportNexusJSON(false));
$('exportScreen')?.addEventListener('click',()=>exportNexusJSON(true));
$('importLayout')?.addEventListener('click',async()=>{
 try{
  if(!cfg)return message('Load a Nexus layout first.');
  const file=$('layoutFile')?.files?.[0];
  if(!file)return message('Select a JSON layout file.');
  if(file.size<1||file.size>4*1024*1024)throw Error('JSON file must be 4 MiB or smaller');
  const imported=NexusLayoutFiles.validate(JSON.parse(await file.text()));
  const mode=$('layoutImportMode').value;
  if(mode==='replace-layout'&&!confirm('Replace the entire current layout with '+imported.screens.length+' imported screens? Changes become permanent only when Save Layout is clicked.'))return;
  if(mode==='replace-screen'&&!confirm('Replace selected screen "'+(current()?.name||'')+'"? Changes become permanent only when Save Layout is clicked.'))return;
  const id=mode==='add-screen'?uniqueID('screen',new Set(cfg.screens.map(s=>s.id))):'';
  const next=NexusLayoutFiles.applyImport(cfg,imported,mode,$('screens').value,id);
  cfg=next;selected=null;
  if($('cyclingMode'))$('cyclingMode').value=cfg.cyclingMode||'manual';
  if($('displayBrightness'))$('displayBrightness').value=String(cfg.brightness||100);
  if($('cyclingSeconds')){
   const seconds=cfg.cyclingSeconds||15;
   if(![...$('cyclingSeconds').options].some(option=>Number(option.value)===seconds))$('cyclingSeconds').add(new Option(seconds+' seconds (imported)',String(seconds)));
   $('cyclingSeconds').value=String(seconds);
  }
  syncCycleInterval();refreshScreens();
  message('JSON imported into the editor. Review before Save Layout.');
 }catch(e){message('Import failed: '+e.message)}
});


function widgetLibraryStorage(){
 if(typeof localStorage==='undefined')throw Error('This browser does not provide persistent widget storage');
 return localStorage;
}
function refreshWidgetLibrary(){
 const select=$('widgetLibrarySelect');if(!select)return;
 const previouslySelected=select.value;
 select.replaceChildren();
 try{
  const entries=NexusWidgetLibrary.list(widgetLibraryStorage());
  for(const entry of entries)select.add(new Option(entry.name,entry.id));
  if(entries.some(entry=>entry.id===previouslySelected))select.value=previouslySelected;
  if($('addWidgetPreset'))$('addWidgetPreset').disabled=!entries.length||!current()||current().widgets.length>=64||totalNexusWidgets()>=NEXUS_MAX_TOTAL_WIDGETS;
  if($('deleteWidgetPreset'))$('deleteWidgetPreset').disabled=!entries.length;
 }catch(e){
  if($('addWidgetPreset'))$('addWidgetPreset').disabled=true;
  if($('deleteWidgetPreset'))$('deleteWidgetPreset').disabled=true;
  message(e.message);
 }
}
$('saveWidgetPreset')?.addEventListener('click',()=>{
 try{
  const widget=current()?.widgets.find(w=>w.id===selected);
  if(!widget)return message('Select a widget to save.');
  const name=prompt('Name for the reusable widget',widget.settings?.label||widget.type)?.trim();
  if(!name)return;
  if(name.length>80)return message('Widget library name must be 80 characters or fewer.');
  const storage=widgetLibraryStorage();
  const entry={id:uniqueID('preset',new Set(NexusWidgetLibrary.list(storage).map(x=>x.id))),name,widget:JSON.parse(JSON.stringify(widget))};
  NexusWidgetLibrary.add(storage,entry);
  refreshWidgetLibrary();
  $('widgetLibrarySelect').value=entry.id;message('Widget saved in this browser.');
 }catch(e){message(e.message)}
});
$('addWidgetPreset')?.addEventListener('click',()=>{
 try{
  const screen=current();if(!screen)return;
  if(screen.widgets.length>=64)return message('Maximum 64 widgets');
  if(totalNexusWidgets()>=NEXUS_MAX_TOTAL_WIDGETS)return message('Maximum 4096 widgets across all screens');
  const storage=widgetLibraryStorage();
  const entry=NexusWidgetLibrary.list(storage).find(x=>x.id===$('widgetLibrarySelect').value);
  if(!entry)return;
  const widget=NexusWidgetLibrary.instantiate(entry,uniqueID('widget',new Set(screen.widgets.map(w=>w.id))),screen.id,cfg.screens.map(s=>s.id));
  screen.widgets.push(widget);selected=widget.id;render();
  refreshWidgetLibrary();
  message('Widget added from library. Save Layout to apply.');
 }catch(e){message(e.message)}
});
$('deleteWidgetPreset')?.addEventListener('click',()=>{
 try{
  const storage=widgetLibraryStorage(),id=$('widgetLibrarySelect').value;
  const entry=NexusWidgetLibrary.list(storage).find(x=>x.id===id);if(!entry)return;
  if(!confirm('Delete saved widget "'+entry.name+'" from this browser?'))return;
  NexusWidgetLibrary.remove(storage,id);refreshWidgetLibrary();
  message('Saved widget removed from the browser library.');
 }catch(e){message(e.message)}
});
$('widgetLibrarySelect')?.addEventListener('change',refreshWidgetLibrary);

// Integrated device pages load their Nexus automatically without a device selector.
// The standalone editor retains its device dropdown and Load button.
if ($('deviceId')) {
  load().catch(e => message(e.message));
} else if ($('device')) {
  loadDevices();
}

// Clock preview updates without re-rendering the editor or interrupting drag.
// Live browser plots reuse the physical Nexus's cached 60-second samples.
// The endpoint never polls new sensors; it only returns the last LCD sample.
function paintNexusGraph(element,widget){
 const plot=element.querySelector?.('canvas.nexus-graph-plot');
 const context=plot?.getContext?.('2d');if(!context)return;
 const points=nexusPreviewTelemetry?.history?.[widget.metric];
 const width=plot.width,height=plot.height;
 context.clearRect(0,0,width,height);
 if(!Array.isArray(points)||points.length<2)return;
 const valid=points.filter(v=>typeof v==='number'&&Number.isFinite(v));
 if(!valid.length)return;
 const utilization=['cpu.utilization','gpu.utilization','memory.utilization','disk.utilization','gpu.vram.utilization'].includes(widget.metric);
 let minimum=utilization?0:Math.min(...valid),maximum=utilization?100:Math.max(...valid);
 if(maximum-minimum<1){minimum-=0.5;maximum+=0.5}
 const trace=widget.settings?.textColor||'#5cd5af';
 context.strokeStyle=trace;context.lineWidth=1.5;
 context.beginPath();let drawing=false;
 for(let i=0;i<points.length;i++){
  const value=points[i];
  if(typeof value!=='number'||!Number.isFinite(value)){drawing=false;continue}
  const x=i*(width-1)/(points.length-1);
  const normalized=Math.max(0,Math.min(1,(value-minimum)/(maximum-minimum)));
  const y=height-1-normalized*(height-1);
  if(!drawing){context.moveTo(x,y);drawing=true}
  else context.lineTo(x,y)
 }
 context.stroke();
}
function updateNexusGraphPlots(){
 const screen=current();if(!screen)return;
 for(const element of $('screen').children){
  const widget=screen.widgets.find(w=>w.id===element.dataset.id);
  if(widget?.type==='graph')paintNexusGraph(element,widget)
 }
}
async function pollNexusPreviewTelemetry(){
 if(!serial||!cfg||nexusTelemetryRequestInFlight||document.hidden)return;
 if(!cfg.screens.some(screen=>screen.widgets.some(widget=>widget.type==='graph')))return;
 nexusTelemetryRequestInFlight=true;
 try{
  const response=await fetch('/api/nexus/telemetry?serial='+encodeURIComponent(serial),{cache:'no-store'});
  if(!response.ok)throw Error('telemetry unavailable');
  const data=await response.json();
  if(data&&typeof data==='object'&&data.history&&typeof data.history==='object'){
   nexusPreviewTelemetry=data;
   updateNexusGraphPlots();
  }
 }catch(_err){
  // No synthetic readings: physical device/display may be offline.
  nexusPreviewTelemetry=null;updateNexusGraphPlots();
 }finally{nexusTelemetryRequestInFlight=false}
}
if(typeof setInterval==='function')setInterval(pollNexusPreviewTelemetry,1500);

function refreshNexusClockPreviews(){
 const active=current();if(!active)return;
 for(const el of $('screen').children){
  const widget=active.widgets.find(w=>w.id===el.dataset.id);
  if(widget?.type==='clock')el.textContent=formatNexusClock(widget.settings);
 }
}
if(typeof setInterval==='function')setInterval(refreshNexusClockPreviews,1000);

function refreshApplicationScreenMappings(){
 const list=$('applicationScreenMappings');
 const select=$('applicationScreenTarget');
 if(!list||!select||!cfg)return;
 const prior=select.value;
 select.replaceChildren();
 for(const screen of cfg.screens)select.add(new Option(screen.name,screen.id));
 select.value=cfg.screens.some(screen=>screen.id===prior)?prior:cfg.screens[0].id;
 list.replaceChildren();
 for(const [id,target] of Object.entries(cfg.applicationScreens||{})){
  const row=document.createElement('div');row.className='nexus-app-binding';
  const name=document.createElement('span');
  name.textContent=id+' → '+(cfg.screens.find(screen=>screen.id===target)?.name||target);
  const remove=document.createElement('button');remove.type='button';remove.textContent='Remove';
  remove.addEventListener('click',()=>{delete cfg.applicationScreens[id];refreshApplicationScreenMappings()});
  row.append(name,remove);list.append(row);
 }
}
$('addApplicationScreen')?.addEventListener('click',()=>{
 if(!cfg)return;
 const id=($('applicationScreenID')?.value||'').trim();
 if(!/^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$/.test(id))return message('Use a valid application ID from OpenLinkHub Application Profiles.');
 cfg.applicationScreens??={};
 if(!Object.hasOwn(cfg.applicationScreens,id)&&Object.keys(cfg.applicationScreens).length>=64)return message('Maximum 64 application bindings.');
 cfg.applicationScreens[id]=$('applicationScreenTarget').value;
 refreshApplicationScreenMappings();
 message('Application screen mapping added. Save Layout to apply.');
});

(function setupNexusRightFlyouts(){
 const backdrop=document.getElementById('nexus-flyout-backdrop');
 if(!backdrop)return;
 const triggers=[...(document.querySelectorAll?.('.nexus-flyout-toolbar [data-nexus-flyout]')||[])];
 if(!triggers.length)return;
 const panels=triggers.map(button=>document.getElementById(button.getAttribute('aria-controls'))).filter(Boolean);
 let active=null,origin=null;
 function close(){
  if(!active)return;
  active.hidden=true;active.classList.remove('nexus-flyout-docked');
  active.style.right='';active.style.top='';active.style.width='';
  active.setAttribute('aria-modal','true');
  backdrop.hidden=true;
  for(const trigger of triggers)trigger.setAttribute('aria-expanded','false');
  active=null;
  const focus=origin;origin=null;
  focus?.focus?.();
 }
 function place(){
  if(!active)return;
  const editor=document.getElementById('nexus-editor')||document.getElementById('preview');
  const rect=editor?.getBoundingClientRect?.();
  const rightSpace=rect?window.innerWidth-rect.right:0;
  const dock=rightSpace>=340;
  active.classList.toggle('nexus-flyout-docked',dock);
  active.setAttribute('aria-modal',dock?'false':'true');
  backdrop.hidden=dock;
  if(dock){
   const width=Math.min(420,rightSpace-20);
   active.style.width=width+'px';
   active.style.right='12px';
   active.style.top=Math.max(12,Math.min(rect.top,window.innerHeight-160))+'px';
  }else{
   active.style.width='';active.style.right='';active.style.top='';
  }
 }
 for(const trigger of triggers)trigger.addEventListener('click',()=>{
  const panel=document.getElementById(trigger.getAttribute('aria-controls'));
  if(!panel)return;
  if(active===panel){close();return}
  close();
  origin=trigger;active=panel;active.hidden=false;
  trigger.setAttribute('aria-expanded','true');
  place();active.focus();
 });
 for(const panel of panels){
  panel.querySelector('.nexus-flyout-close')?.addEventListener('click',close);
  panel.addEventListener('keydown',event=>{
   if(event.key!=='Tab'||panel.classList.contains('nexus-flyout-docked'))return;
   const elements=[...panel.querySelectorAll('button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),a[href]')].filter(el=>!el.hidden);
   if(!elements.length){event.preventDefault();return}
   const first=elements[0],last=elements[elements.length-1];
   if(event.shiftKey&&document.activeElement===first){event.preventDefault();last.focus()}
   else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first.focus()}
  });
 }
 backdrop.addEventListener('click',close);
 document.addEventListener('keydown',event=>{if(active&&event.key==='Escape'){event.preventDefault();close()}});
 window.addEventListener('resize',place);
})();


// A visual click indication also works with keyboard activation in the editor.
$('screen')?.addEventListener('pointerdown',event=>{
 const target=event.target.closest?.('.widget.nexus-button-style');
 if(!target)return;
 target.classList.add('nexus-pressed-preview');
});
$('screen')?.addEventListener('pointerup',()=>{
 for(const item of $('screen').children)item.classList?.remove?.('nexus-pressed-preview');
});
