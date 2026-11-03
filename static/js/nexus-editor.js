'use strict';
const $=id=>document.getElementById(id);
let cfg=null,selected=null,serial='',drag=null;
const metrics=['cpu.temperature','gpu.temperature','cpu.utilization','gpu.utilization','memory.used','memory.utilization','network.rx_rate','network.tx_rate','disk.read_rate','disk.write_rate'];
function message(t){$('message').textContent=t}
function current(){return cfg?.screens.find(s=>s.id===$('screens').value)}
function refreshScreens(){const old=$('screens').value;$('screens').replaceChildren();for(const s of cfg.screens){const opt=new Option(s.name,s.id);$('screens').add(opt)}$('screens').value=cfg.screens.some(s=>s.id===old)?old:cfg.activeScreenId;render()}
function render(){const s=current();$('screen').replaceChildren();$('properties').replaceChildren();if(!s)return;
 $('background').value=s.background||'';
 for(const w of s.widgets){const el=document.createElement('div');el.className='widget'+(selected===w.id?' selected':'');el.style.cssText=`left:${w.x}px;top:${w.y}px;width:${w.width}px;height:${w.height}px`;el.textContent=w.type==='text'?(w.settings?.text||'Text'):((w.type==='button'||w.type==='macro')?(w.settings?.label||'Touch'):(w.settings?.label||w.metric));el.dataset.id=w.id;$('screen').append(el)}
 const w=s.widgets.find(w=>w.id===selected);if(!w)return;
 const fields=[['x','number'],['y','number'],['width','number'],['height','number'],['label','text'],['text','text'],['metric','select']];
 for(const [key,type] of fields){if(key==='metric'&&(w.type==='text'||w.type==='button'||w.type==='macro'))continue;if(key==='text'&&w.type!=='text')continue;
 const label=document.createElement('label');label.textContent=key+' ';
 let input;if(type==='select'){input=document.createElement('select');for(const m of metrics)input.add(new Option(m,m));input.value=w.metric}else{input=document.createElement('input');input.type=type;input.value=(key in w?w[key]:(w.settings||{})[key])??''}
 input.addEventListener('change',()=>{let v=type==='number'?Number(input.value):input.value;if(['x','y','width','height'].includes(key)){w[key]=v;w.width=Math.max(1,Math.min(640,w.width));w.height=Math.max(1,Math.min(48,w.height));w.x=Math.max(0,Math.min(640-w.width,w.x));w.y=Math.max(0,Math.min(48-w.height,w.y))}else if(key==='metric')w.metric=v;else{w.settings??={};w.settings[key]=v}render()});label.append(input);$('properties').append(label)}
 if(w.type==='button'||w.type==='macro')renderTouchProperties(w);
}
$('screen').addEventListener('pointerdown',e=>{const el=e.target.closest('.widget');if(!el)return;selected=el.dataset.id;const w=current().widgets.find(w=>w.id===selected);drag={x:e.clientX,y:e.clientY,startX:w.x,startY:w.y,id:e.pointerId};$('screen').setPointerCapture(e.pointerId);render()});
$('screen').addEventListener('pointermove',e=>{if(!drag||drag.id!==e.pointerId)return;const w=current().widgets.find(w=>w.id===selected);if(!w)return;w.x=Math.max(0,Math.min(640-w.width,Math.round(drag.startX+e.clientX-drag.x)));w.y=Math.max(0,Math.min(48-w.height,Math.round(drag.startY+e.clientY-drag.y)));const el=[...$('screen').children].find(el=>el.dataset.id===w.id);if(el){el.style.left=w.x+'px';el.style.top=w.y+'px'}});
$('screen').addEventListener('pointerup',()=>{drag=null;render()});$('screen').addEventListener('pointercancel',()=>{drag=null;render()});
async function loadDevices(){try{const response=await fetch('/api/devices/');const json=await response.json();const data=json.devices||json.device||json.data||json;const entries=Object.entries(data);$('device').replaceChildren();for(const [key,v] of entries){if(/nexus/i.test(JSON.stringify(v))){$('device').add(new Option((v.product||v.Product||'Nexus')+' — '+key,key))}}if(!$('device').length)message('No Nexus found automatically. Enter its serial below.')}catch(e){message(e.message)}}
async function load(){serial=$('deviceId')?.value||$('device')?.value||prompt('Nexus device serial');if(!serial)return;const res=await fetch('/api/nexus/screens?serial='+encodeURIComponent(serial));if(!res.ok)throw Error(await res.text());cfg=await res.json();selected=null;const mode=$('cyclingMode');const interval=$('cyclingSeconds');if(mode)mode.value=cfg.cyclingMode||'manual';if(interval){interval.value=cfg.cyclingSeconds||15;interval.disabled=mode?.value!=='automatic'}refreshScreens();message('Layout loaded')}
if($('load'))$('load').onclick=()=>load().catch(e=>message(e.message));
$('screens').onchange=()=>{cfg.activeScreenId=$('screens').value;selected=null;render()};
$('addScreen').onclick=()=>{if(!cfg)return;if(cfg.screens.length>=32)return message('Maximum 32 screens');const name=prompt('Screen name','New screen');if(!name)return;const id='screen_'+Date.now();cfg.screens.push({id,name,widgets:[]});cfg.activeScreenId=id;refreshScreens()};
$('deleteScreen').onclick=()=>{if(!cfg||cfg.screens.length<=1)return message('Keep at least one screen');cfg.screens=cfg.screens.filter(s=>s.id!==$('screens').value);cfg.activeScreenId=cfg.screens[0].id;selected=null;refreshScreens()};
function renderTouchProperties(w){
 const panel=$('properties');
 const field=(name,value,choices,change)=>{
  const label=document.createElement('label');label.textContent=name+' ';
  const input=document.createElement('select');
  for(const [v,t] of choices)input.add(new Option(t,v));
  input.value=value;input.onchange=()=>{change(input.value);render()};
  label.append(input);panel.append(label);
 };
 const actions=[['next','Next screen'],['previous','Previous screen'],['screen','Go to screen'],['media','Media control']];
 const media=[['play-pause','Play/Pause'],['stop','Stop'],['volume-up','Volume +'],['volume-down','Volume -'],['next-track','Next track'],['previous-track','Previous track']];
 const targets=cfg.screens.map(s=>[s.id,s.name]);
 if(w.type==='button'){
  field('Action',w.action||'next',actions,v=>{w.action=v;w.target=v==='screen'?cfg.screens[0].id:v==='media'?'play-pause':''});
  if(w.action==='screen')field('Target screen',w.target,targets,v=>w.target=v);
  if(w.action==='media')field('Media action',w.target,media,v=>w.target=v);
 }else{
  const label=document.createElement('label');label.textContent='Macro steps (JSON) ';
  const area=document.createElement('textarea');area.rows=5;area.cols=55;area.value=JSON.stringify(w.steps||[],null,2);
  area.setAttribute('aria-label','Macro steps JSON');
  const apply=document.createElement('button');apply.textContent='Apply steps';apply.onclick=()=>{
   try{const steps=JSON.parse(area.value);if(!Array.isArray(steps)||steps.length<1||steps.length>12)throw Error('Use 1–12 steps');w.steps=steps;message('Macro steps updated. Save layout to apply.')}catch(e){message(e.message)}
  };
  label.append(area);panel.append(label);panel.append(apply);
  const help=document.createElement('small');help.textContent='Step types: next, previous, screen (value: screen ID), media (value: play-pause, stop, volume-up, volume-down, next-track, previous-track), delay (delayMs: 0–5000). Max 15 seconds total delays.';panel.append(help);
 }
}
function addWidget(type){const s=current();if(!s)return;if(s.widgets.length>=64)return message('Maximum 64 widgets');const id='widget_'+Date.now();const w={id,type,metric:(type==='text'||type==='button'||type==='macro')?'':'cpu.temperature',x:0,y:0,width:150,height:40,settings:type==='text'?{text:'New text'}:{label:(type==='button'?'Button':type==='macro'?'Macro':'CPU')},...(type==='button'?{action:'next'}:{}),...(type==='macro'?{steps:[{type:'next'}]}:{})};s.widgets.push(w);selected=id;render()}
$('addMetric').onclick=()=>addWidget('metric');$('addText').onclick=()=>addWidget('text');$('addBar').onclick=()=>addWidget('bar');$('addButton')?.addEventListener('click',()=>addWidget('button'));$('addMacro')?.addEventListener('click',()=>addWidget('macro'));
$('deleteWidget').onclick=()=>{const s=current();if(!s)return;s.widgets=s.widgets.filter(w=>w.id!==selected);selected=null;render()};
$('background').onchange=()=>{const s=current();if(s)s.background=$('background').value.trim()};
$('upload').onclick=async()=>{try{const f=$('image').files[0];if(!f||!current())return message('Choose an image and load a screen');if(f.size>4*1024*1024)throw Error('Image exceeds 4 MiB');const id='background_'+Date.now();const res=await fetch('/api/nexus/background?serial='+encodeURIComponent(serial)+'&id='+id,{method:'POST',headers:{'Content-Type':f.type},body:f});if(!res.ok)throw Error(await res.text());current().background=id;render();message('Background uploaded. Save layout to apply it.')}catch(e){message(e.message)}};
$('cyclingMode')?.addEventListener('change',()=>{const interval=$('cyclingSeconds');if(interval)interval.disabled=$('cyclingMode').value!=='automatic'});
$('save').onclick=async()=>{try{if(!cfg)return;const mode=$('cyclingMode');const interval=$('cyclingSeconds');if(mode){cfg.cyclingMode=mode.value;cfg.cyclingSeconds=Number(interval?.value||15)}cfg.activeScreenId=$('screens').value;const res=await fetch('/api/nexus/screens?serial='+encodeURIComponent(serial),{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(cfg)});if(!res.ok)throw Error(await res.text());message('Saved successfully')}catch(e){message(e.message)}};
// Integrated device pages load their Nexus automatically without a device selector.
// The standalone editor retains its device dropdown and Load button.
if ($('deviceId')) {
  load().catch(e => message(e.message));
} else if ($('device')) {
  loadDevices();
}
