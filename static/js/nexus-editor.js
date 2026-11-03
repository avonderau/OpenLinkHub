'use strict';
const $=id=>document.getElementById(id);
let cfg=null,selected=null,serial='',drag=null;
const mediaGlyphs={'previous-track':'⏮','play-pause':'⏯','next-track':'⏭','stop':'■','volume-down':'◖−','volume-up':'◖+','next':'▶','previous':'◀'};
const metrics=['cpu.temperature','gpu.temperature','cpu.utilization','gpu.utilization','memory.used','memory.utilization','network.rx_rate','network.tx_rate','disk.read_rate','disk.write_rate'];
const GRID=8;
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
 for(const id of ['renameScreen','duplicateScreen'])if($(id))$(id).disabled=!screen||(id==='duplicateScreen'&&count>=32);
 if($('deleteScreen'))$('deleteScreen').disabled=count<=1;
 if($('replaceScreenWithPreset'))$('replaceScreenWithPreset').disabled=!screen||$('newScreenPreset')?.value==='blank';
}
const snapPosition=(value,max)=>clamp(snap(value),Math.floor(max/GRID)*GRID);
const snapSize=(value,max)=>clamp(Math.max(GRID,snap(value)),max);
function message(t){$('message').textContent=t}
function current(){return cfg?.screens.find(s=>s.id===$('screens').value)}
function refreshScreens(){const old=$('screens').value;$('screens').replaceChildren();for(const s of cfg.screens){const opt=new Option(s.name,s.id);$('screens').add(opt)}const chosen=cfg.screens.some(s=>s.id===cfg.activeScreenId)?cfg.activeScreenId:old;$('screens').value=chosen;render()}
function render(){const s=current();$('screen').replaceChildren();$('properties').replaceChildren();refreshActionButtons();
 const widget=s?.widgets.find(w=>w.id===selected);
 const status=$('selectedWidgetStatus'),remove=$('deleteWidget');
 if(remove)remove.disabled=!widget;
 if($('duplicateWidget'))$('duplicateWidget').disabled=!widget||s.widgets.length>=64;
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
 for(const w of s.widgets){const el=document.createElement('div');el.className='widget'+(selected===w.id?' selected':'');el.style.cssText=`left:${w.x}px;top:${w.y}px;width:${w.width}px;height:${w.height}px`;if(['button','macro','metric','bar','text','clock'].includes(w.type)){const st=w.settings||{};if(w.type==='button'||w.type==='macro')el.classList.add('nexus-button-style');else el.classList.add('nexus-colored-widget');el.classList.add('nexus-style-'+(st.style||'solid'));el.style.setProperty('--button-bg',st.backgroundColor||'#1b2935');el.style.setProperty('--button-ink',st.textColor||'#ebf5fc');el.style.setProperty('--button-border',st.borderColor||'#80bdd9');el.style.setProperty('--button-border-visibility',st.borderColor?'solid':'solid')}el.textContent=w.type==='clock'?formatNexusClock(w.settings):w.type==='text'?(w.settings?.text||'Text'):((w.type==='button'||w.type==='macro')?(w.settings?.label||''):(w.settings?.label||w.metric));if((w.type==='button'||w.type==='macro')&&w.settings?.customIcon){
  el.classList.add('nexus-icon-widget');
  const picture=document.createElement('img');
  picture.className='nexus-custom-icon';
  picture.alt='';
  picture.src='/api/nexus/icon?serial='+encodeURIComponent(serial)+'&id='+encodeURIComponent(w.settings.customIcon);
  picture.draggable=false;el.prepend(picture);
 }else if((w.type==='button'||w.type==='macro')&&mediaGlyphs[w.settings?.icon]){
  el.classList.add('nexus-icon-widget');
  const glyph=document.createElement('span');glyph.className='nexus-icon';
  glyph.textContent=mediaGlyphs[w.settings.icon];el.prepend(glyph)
 }el.dataset.id=w.id;$('screen').append(el)}
 const w=s.widgets.find(w=>w.id===selected);if(!w)return;
 const properties=$('properties');
 const dimensionsRow=document.createElement('div');dimensionsRow.className='nexus-properties-row nexus-dimensions-row';dimensionsRow.setAttribute('aria-label','Widget dimensions');
 const colorsRow=document.createElement('div');colorsRow.className='nexus-properties-row nexus-colors-row';colorsRow.setAttribute('aria-label','Widget colors');
 const styleRow=document.createElement('div');styleRow.className='nexus-properties-row nexus-style-row';styleRow.setAttribute('aria-label','Widget style and settings');
 properties.append(dimensionsRow,colorsRow,styleRow);
 const fields=(w.type==='button'||w.type==='macro')?[['x','number'],['y','number'],['width','number'],['height','number']]:[['x','number'],['y','number'],['width','number'],['height','number'],['label','text'],['text','text'],['metric','select']];
 for(const [key,type] of fields){if(key==='metric'&&(w.type==='text'||w.type==='clock'||w.type==='button'||w.type==='macro'))continue;if(key==='text'&&w.type!=='text')continue;
 const label=document.createElement('label');label.textContent=key+' ';
 let input;if(type==='select'){input=document.createElement('select');for(const m of metrics)input.add(new Option(m,m));input.value=w.metric}else{input=document.createElement('input');input.type=type;input.value=(key in w?w[key]:(w.settings||{})[key])??''}
 input.addEventListener('change',()=>{let v=type==='number'?Number(input.value):input.value;if(['x','y','width','height'].includes(key)){if(!Number.isFinite(v))return message('Enter a valid number');if(key==='width')w.width=snapSize(v,640);if(key==='height')w.height=snapSize(v,48);if(key==='x')w.x=snapPosition(v,640-w.width);if(key==='y')w.y=snapPosition(v,48-w.height);w.x=snapPosition(w.x,640-w.width);w.y=snapPosition(w.y,48-w.height)}else if(key==='metric')w.metric=v;else{w.settings??={};w.settings[key]=v}render()});label.append(input);(['x','y','width','height'].includes(key)?dimensionsRow:styleRow).append(label)}
 renderWidgetColors(w,colorsRow);
 if(w.type==='clock'){
  const label=document.createElement('label');label.textContent='Clock format ';
  const input=document.createElement('select');
  for(const [value,title] of [['24h','24-hour time'],['12h','12-hour time'],['date','Date'],['datetime','Date and time']])input.add(new Option(title,value));
  input.value=w.settings?.clockFormat||'24h';
  input.addEventListener('change',()=>{w.settings??={};w.settings.clockFormat=input.value;render()});
  label.append(input);styleRow.append(label);
 }
 if(w.type==='button'||w.type==='macro')renderTouchProperties(w,styleRow);
}
$('screen').addEventListener('pointerdown',e=>{const el=e.target.closest('.widget');if(!el)return;selected=el.dataset.id;const w=current().widgets.find(w=>w.id===selected);drag={x:e.clientX,y:e.clientY,startX:w.x,startY:w.y,id:e.pointerId};$('screen').setPointerCapture(e.pointerId);render()});
$('screen').addEventListener('pointermove',e=>{if(!drag||drag.id!==e.pointerId)return;const w=current().widgets.find(w=>w.id===selected);if(!w)return;w.x=snapPosition(drag.startX+e.clientX-drag.x,640-w.width);w.y=snapPosition(drag.startY+e.clientY-drag.y,48-w.height);const el=[...$('screen').children].find(el=>el.dataset.id===w.id);if(el){el.style.left=w.x+'px';el.style.top=w.y+'px'}});
$('screen').addEventListener('pointerup',()=>{drag=null;render()});$('screen').addEventListener('pointercancel',()=>{drag=null;render()});
async function loadDevices(){try{const response=await fetch('/api/devices/');const json=await response.json();const data=json.devices||json.device||json.data||json;const entries=Object.entries(data);$('device').replaceChildren();for(const [key,v] of entries){if(/nexus/i.test(JSON.stringify(v))){$('device').add(new Option((v.product||v.Product||'Nexus')+' — '+key,key))}}if(!$('device').length)message('No Nexus found automatically. Enter its serial below.')}catch(e){message(e.message)}}
async function load(){serial=$('deviceId')?.value||$('device')?.value||prompt('Nexus device serial');if(!serial)return;const res=await fetch('/api/nexus/screens?serial='+encodeURIComponent(serial));if(!res.ok)throw Error(await res.text());cfg=await res.json();selected=null;const mode=$('cyclingMode');const interval=$('cyclingSeconds');if(mode)mode.value=cfg.cyclingMode||'manual';if(interval){const seconds=cfg.cyclingSeconds||15;if(![...interval.options].some(o=>Number(o.value)===seconds))interval.add(new Option(seconds+' seconds (saved)',String(seconds)));interval.value=String(seconds)}syncCycleInterval();refreshScreens();message('')}
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
  if(widget.type==='macro'||(widget.type==='button'&&widget.action==='macro'))
   for(const step of widget.steps||[])if(step.type==='screen'&&step.value===previousID)step.value=newID;
 }
 return screen;
}
$('newScreenPreset')?.addEventListener('change',refreshActionButtons);
$('addScreen').onclick=async()=>{
 try{
  if(!cfg)return message('Load a Nexus layout first.');
  if(cfg.screens.length>=32)return message('Maximum 32 screens.');
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
$('renameScreen')?.addEventListener('click',()=>{
 const s=current();if(!s)return;
 const name=prompt('Rename screen',s.name)?.trim();
 if(name===undefined||name===null||!name||name===s.name)return;
 if(name.length>80)return message('Screen name must be 80 characters or fewer');
 s.name=name;refreshScreens();message('Screen renamed. Save Layout to apply.');
});
$('duplicateScreen')?.addEventListener('click',()=>{
 const source=current();if(!source||!cfg)return;
 if(cfg.screens.length>=32)return message('Maximum 32 screens');
 const newID=uniqueID('screen',new Set(cfg.screens.map(s=>s.id)));
 const name=(source.name.slice(0,73)+' copy').slice(0,80);
 // JSON round trip gives the duplicate fully independent widget settings and macro steps.
 const duplicate=JSON.parse(JSON.stringify(source));
 duplicate.id=newID;duplicate.name=name;
 for(const widget of duplicate.widgets){
  if(widget.type==='button'&&widget.action==='screen'&&widget.target===source.id)widget.target=newID;
  if(widget.type==='macro'||(widget.type==='button'&&widget.action==='macro')){
   for(const step of widget.steps||[])if(step.type==='screen'&&step.value===source.id)step.value=newID;
  }
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
  if(w.type==='macro'||(w.type==='button'&&w.action==='macro')){
   for(const step of w.steps||[])if(step.type==='screen'&&step.value===removed)step.value=fallback;
  }
 }
 cfg.activeScreenId=fallback;selected=null;refreshScreens();
 message('Screen removed. Navigation links to it now point to '+cfg.screens[0].name+'. Save Layout to apply.');
};
function renderWidgetColors(w,panel){
 if(!['button','macro','metric','bar','text','clock'].includes(w.type))return;
 for(const [key,name,defaultColor] of [['backgroundColor','Background','#1b2935'],['textColor','Text & icon','#ebf5fc'],['borderColor','Border','#80bdd9']]){
  const label=document.createElement('label');label.textContent=name+' ';
  const input=document.createElement('input');input.type='color';input.value=w.settings?.[key]||defaultColor;
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
 const actions=[['next','Next screen'],['previous','Previous screen'],['screen','Go to screen'],['media','Media control'],['macro','Run macro'],['hotkey','Keyboard shortcut'],['mouse','Mouse action']];
 const media=[['play-pause','Play/Pause'],['stop','Stop'],['volume-up','Volume +'],['volume-down','Volume -'],['next-track','Next track'],['previous-track','Previous track']];
 const targets=cfg.screens.map(s=>[s.id,s.name]);
 const mouseActions=[['left','Left click'],['double-left','Double left click'],['right','Right click'],['middle','Middle click'],['back','Back button'],['forward','Forward button'],['scroll-up','Scroll up'],['scroll-down','Scroll down'],['scroll-left','Scroll left'],['scroll-right','Scroll right']];
 const shortcutInput=(name,value,update)=>{
  const label=document.createElement('label');label.textContent=name+' ';
  const input=document.createElement('input');input.type='text';input.maxLength=80;
  input.placeholder='Ctrl+Alt+T';input.value=value||'';
  input.setAttribute('aria-label',name);
  input.addEventListener('change',()=>{update(input.value.trim());message('Shortcut updated. Save Layout to apply.')});
  label.append(input);panel.append(label);
 };
 if(w.type==='button'||w.type==='macro'){
  field('Style',w.settings?.style||'solid',[['solid','Solid'],['outline','Outline'],['flat','Transparent']],v=>{w.settings??={};w.settings.style=v});
 }
 field('Icon',w.settings?.customIcon?'custom':w.settings?.icon||'none',[['none','No icon'],...Object.keys(mediaGlyphs).map(id=>[id,id]),['custom','Custom image']],v=>{
  w.settings??={};
  if(v!=='custom')delete w.settings.customIcon;
  if(v==='none')delete w.settings.icon;else w.settings.icon=v;
 });
 if(w.settings?.customIcon || w.settings?.icon==='custom'){
  const wrapper=document.createElement('label');wrapper.className='nexus-custom-icon-upload';
  wrapper.textContent='Custom icon (PNG/JPEG) ';
  const file=document.createElement('input');file.type='file';file.accept='image/png,image/jpeg';
  file.setAttribute('aria-label','Upload custom icon');
  wrapper.append(file);
  const upload=document.createElement('button');upload.type='button';upload.textContent='Upload icon';
  upload.onclick=async()=>{
   const source=file.files?.[0];
   if(!source)return message('Choose a PNG or JPEG icon first');
   if(source.size>512*1024)return message('Icon exceeds 512 KiB');
   if(!['image/png','image/jpeg'].includes(source.type))return message('Select a PNG or JPEG image');
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
  field('Action',w.action||'next',actions,v=>{w.action=v;w.target=v==='screen'?cfg.screens[0].id:v==='media'?'play-pause':v==='hotkey'?'Ctrl+Alt+T':v==='mouse'?'left':'';if(v==='media'&&!w.settings?.customIcon){w.settings??={};w.settings.icon='play-pause'}if(v==='macro'&&(!w.steps||!w.steps.length))w.steps=[{type:'next'}]});
  if(w.action==='screen')field('Target screen',w.target,targets,v=>w.target=v);
  if(w.action==='hotkey')shortcutInput('Shortcut',w.target,v=>w.target=v);
  if(w.action==='mouse')field('Mouse command',w.target,mouseActions,v=>w.target=v);
  if(w.action==='media')field('Media action',w.target,media,v=>{const old=w.target;w.target=v;if(!w.settings?.customIcon&&(!w.settings?.icon||w.settings.icon===old)){w.settings??={};w.settings.icon=v}});
 }
 if(w.type==='macro'||w.action==='macro'){
  const label=document.createElement('label');label.textContent='Macro steps (JSON) ';
  const area=document.createElement('textarea');area.rows=5;area.cols=55;area.value=JSON.stringify(w.steps||[],null,2);
  area.setAttribute('aria-label','Macro steps JSON');
  const apply=document.createElement('button');apply.textContent='Apply steps';apply.onclick=()=>{
   try{const steps=JSON.parse(area.value);if(!Array.isArray(steps)||steps.length<1||steps.length>12)throw Error('Use 1–12 steps');w.steps=steps;message('Macro steps updated. Save layout to apply.')}catch(e){message(e.message)}
  };
  label.append(area);panel.append(label);panel.append(apply);
  const help=document.createElement('small');help.textContent='Step types: next, previous, screen (value: screen ID), media (value: play-pause, stop, volume-up, volume-down, next-track, previous-track), hotkey (value: Ctrl+Alt+T), mouse (value: left, double-left, right, middle, back, forward, scroll-up, scroll-down, scroll-left, scroll-right), delay (delayMs: 0–5000). Max 15 seconds total delays.';panel.append(help);
 }
}
function addWidget(type){const s=current();if(!s)return;if(s.widgets.length>=64)return message('Maximum 64 widgets');const id=uniqueID('widget',new Set(s.widgets.map(w=>w.id)));const w={id,type,metric:(type==='text'||type==='clock'||type==='button'||type==='macro')?'':'cpu.temperature',x:0,y:0,width:152,height:40,settings:type==='text'?{text:'New text'}:type==='clock'?{clockFormat:'24h',label:''}:{label:(type==='button'?'Button':type==='macro'?'Macro':'CPU')},...(type==='button'?{action:'next'}:{}),...(type==='macro'?{steps:[{type:'next'}]}:{})};s.widgets.push(w);selected=id;render()}
$('addMetric').onclick=()=>addWidget('metric');$('addClock')?.addEventListener('click',()=>addWidget('clock'));$('addText').onclick=()=>addWidget('text');$('addBar').onclick=()=>addWidget('bar');$('addButton')?.addEventListener('click',()=>addWidget('button'));
$('duplicateWidget')?.addEventListener('click',()=>{
 const screen=current();if(!screen)return;
 const source=screen.widgets.find(w=>w.id===selected);
 if(!source)return;
 if(screen.widgets.length>=64)return message('Maximum 64 widgets');
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
$('save').onclick=async()=>{try{if(!cfg)return;const mode=$('cyclingMode');const interval=$('cyclingSeconds');if(mode){cfg.cyclingMode=mode.value;cfg.cyclingSeconds=Number(interval?.value||15)}cfg.activeScreenId=$('screens').value;const res=await fetch('/api/nexus/screens?serial='+encodeURIComponent(serial),{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(cfg)});if(!res.ok)throw Error(await res.text());message('Saved successfully')}catch(e){message(e.message)}};
// Integrated device pages load their Nexus automatically without a device selector.
// The standalone editor retains its device dropdown and Load button.
if ($('deviceId')) {
  load().catch(e => message(e.message));
} else if ($('device')) {
  loadDevices();
}

// Clock preview updates without re-rendering the editor or interrupting drag.
function refreshNexusClockPreviews(){
 const active=current();if(!active)return;
 for(const el of $('screen').children){
  const widget=active.widgets.find(w=>w.id===el.dataset.id);
  if(widget?.type==='clock')el.textContent=formatNexusClock(widget.settings);
 }
}
if(typeof setInterval==='function')setInterval(refreshNexusClockPreviews,1000);
