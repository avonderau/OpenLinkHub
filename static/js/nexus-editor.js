'use strict';
const nexusElementById=id=>document.getElementById(id);
let cfg=null,selected=null,serial='',drag=null,copiedWidget=null;
let nexusPreviewTelemetry=null,nexusTelemetryRequestInFlight=false;
let nexusMetricSourceOptions={};
const nexusMetricLookupKey=widget=>widget?.settings?.metricSource?
  widget.metric+'|'+widget.settings.metricSource:widget?.metric;
async function loadNexusMetricSources(){
 try{
  const response=await fetch('/api/telemetry/sources',{cache:'no-store'});
  if(response.ok){
   const data=await response.json();
   nexusMetricSourceOptions=data?.sources||{};
  }
 }catch(_err){nexusMetricSourceOptions={}}
}
// Bitmap masks are the same per-pixel artwork used by the physical LCD.
const mediaGlyphs={'previous-track':true,'play-pause':true,'next-track':true,'stop':true,
 'volume-down':true,'volume-up':true,'volume-mute':true,'next':true,'previous':true};
const mediaDefaultLabels={'play-pause':'Play/Pause','previous-track':'Previous',
 'next-track':'Next Track',stop:'Stop','volume-up':'Volume +',
 'volume-down':'Volume -','volume-mute':'Mute'};
function nexusActionDefaultLabel(w){
 if(w?.type==='macro')return 'Run Macro';
 if(w?.type!=='button')return '';
 return {media:mediaDefaultLabels[w.target]||'Media',next:'Next Screen',
  previous:'Previous Screen',screen:'Go to Screen',macro:'Run Macro',
  hotkey:'Shortcut',mouse:'Mouse',text:'Type Text',url:'Open URL',
  application:'Launch App',device:'Device'}[w.action]||'Touch';
}
const nexusDisplayLabel=w=>w?.settings?.label||nexusActionDefaultLabel(w);
const metrics=['cpu.temperature','gpu.temperature','cpu.utilization','gpu.utilization','memory.used','memory.utilization','network.rx_rate','network.tx_rate','disk.read_rate','disk.write_rate',"cpu.clock_mhz","cpu.load_1m","cpu.load_5m","cpu.load_15m","disk.utilization","disk.free_gb","fan.rpm","pump.rpm","gpu.vram.used_mb","gpu.vram.utilization","gpu.vram.total_mb","gpu.clock_mhz","gpu.power_w"];
const GRID=8;
let nexusSharedMacros={};
let nexusIconSets=[];
async function loadNexusIconSets(){
 try{
  const response=await fetch('/api/icon-sets',{cache:'no-store'});
  if(response.ok){
   const data=await response.json();
   nexusIconSets=Array.isArray(data?.sets)?data.sets:[];
  }
 }catch(_error){nexusIconSets=[]}
}
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
function selectedScreenID(){return nexusElementById('screens').value}
function refreshActionButtons(){
 const screen=current(),count=cfg?.screens.length||0;
 for(const id of ['renameScreen','duplicateScreen'])if(nexusElementById(id))nexusElementById(id).disabled=!screen||(id==='duplicateScreen'&&count>=NEXUS_MAX_SCREENS);
 if(nexusElementById('deleteScreen'))nexusElementById('deleteScreen').disabled=count<=1;
 const index=cfg?.screens.findIndex(s=>s.id===selectedScreenID())??-1;
 if(nexusElementById('moveScreenUp'))nexusElementById('moveScreenUp').disabled=index<=0;
 if(nexusElementById('moveScreenDown'))nexusElementById('moveScreenDown').disabled=index<0||index>=count-1;
 if(nexusElementById('replaceScreenWithPreset'))nexusElementById('replaceScreenWithPreset').disabled=!screen||nexusElementById('newScreenPreset')?.value==='blank';
}
const snapPosition=(value,max)=>clamp(snap(value),Math.floor(max/GRID)*GRID);
const snapSize=(value,max)=>clamp(Math.max(GRID,snap(value)),max);
function message(t){nexusElementById('message').textContent=t}
function current(){return cfg?.screens.find(s=>s.id===nexusElementById('screens').value)}
function refreshScreens(){const old=nexusElementById('screens').value;nexusElementById('screens').replaceChildren();for(const s of cfg.screens){const opt=new Option(s.name,s.id);nexusElementById('screens').add(opt)}const chosen=cfg.screens.some(s=>s.id===cfg.activeScreenId)?cfg.activeScreenId:old;nexusElementById('screens').value=chosen;render();refreshApplicationScreenMappings()}
// Change only the representation within a widget family. Geometry, colors,
// placement and a metric's source remain unchanged when its display changes.
function changeSelectedWidgetType(widget,type){
 const metric=['metric','bar','graph'],content=['text','image','clock'];
 const allowed=metric.includes(widget.type)?metric:content.includes(widget.type)?content:[];
 if(!allowed.includes(type))return false;
 const previous=widget.type;
 widget.type=type;
 widget.settings??={};
 if(metric.includes(type)){
  widget.metric ||= 'cpu.temperature';
 }else{
  widget.metric='';
  if(previous==='image'&&type!=='image'){
   delete widget.settings.customIcon;
   delete widget.settings.icon;
  }
  if(type==='text')widget.settings.text ||= 'New text';
  if(type==='clock')widget.settings.clockFormat ||= '24h';
  if(type==='image'){
   widget.settings.style ??= 'flat';
   widget.settings.icon='custom';
  }
 }
 return true;
}
function render(){const s=current();nexusElementById('screen').replaceChildren();nexusElementById('properties').replaceChildren();refreshActionButtons();
 const widget=s?.widgets.find(w=>w.id===selected);
 const status=nexusElementById('selectedWidgetStatus'),remove=nexusElementById('deleteWidget');
 const typeMount=nexusElementById('nexusDisplayTypeMount');
 if(typeMount)typeMount.replaceChildren();
 const name=nexusElementById('nexusSelectedName'),subtitle=nexusElementById('nexusSelectedSubtitle');
 const symbol=nexusElementById('nexusSelectedIcon'),headActions=nexusElementById('nexusInspectorActions');
 if(name)name.textContent=widget?(nexusDisplayLabel(widget)||widget.settings?.text||widget.metric||widget.type):'Select a widget';
 if(subtitle)subtitle.textContent=widget?(
  ['metric','bar','graph'].includes(widget.type)?'Metric widget · '+({metric:'Value',bar:'Bar',graph:'Graph'}[widget.type]):
  ['text','image','clock'].includes(widget.type)?'Content widget · '+widget.type:
  'Touch button'
 ):'Choose an item in the screen preview';
 if(symbol){
  const icons={metric:'speedometer2',bar:'bar-chart',graph:'graph-up',text:'type',image:'image',clock:'clock',button:'cursor',macro:'cursor'};
  symbol.className='bi bi-'+(widget?icons[widget.type]||'sliders2':'sliders2');
 }
 // Paste remains available after switching to an empty screen with a copied widget.
 if(headActions)headActions.hidden=!widget&&!copiedWidget;
 if(remove)remove.disabled=!widget;
 if(nexusElementById('duplicateWidget'))nexusElementById('duplicateWidget').disabled=!widget||s.widgets.length>=64||totalNexusWidgets()>=NEXUS_MAX_TOTAL_WIDGETS;
 if(nexusElementById('copyWidget'))nexusElementById('copyWidget').disabled=!widget;
 if(nexusElementById('pasteWidget'))nexusElementById('pasteWidget').disabled=!copiedWidget||!s||s.widgets.length>=64||totalNexusWidgets()>=NEXUS_MAX_TOTAL_WIDGETS;
 if(nexusElementById('saveWidgetPreset'))nexusElementById('saveWidgetPreset').disabled=!widget;
 if(status){
  status.hidden=Boolean(widget);
  status.textContent=widget?'':'Select a widget in the preview to edit its properties.';
 }
 if(!s)return;
 nexusElementById('background').value=s.background||'';
 const display=nexusElementById('screen');display.classList.remove('nexus-bg-preset_aurora','nexus-bg-preset_neon','nexus-bg-preset_circuit','nexus-bg-preset_sunset','nexus-bg-preset_pulse');
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
 }}if(w.type==='graph')el.classList.add('nexus-graph-widget');el.textContent=w.type==='image'?'':w.type==='graph'?(w.settings?.label||'History: '+w.metric):w.type==='clock'?formatNexusClock(w.settings):w.type==='text'?(w.settings?.text||'Text'):((w.type==='button'||w.type==='macro')?nexusDisplayLabel(w):(w.settings?.label||w.metric));if((w.type==='button'||w.type==='macro'||w.type==='image')&&w.settings?.customIcon){
  el.classList.add('nexus-icon-widget');
  const picture=document.createElement('img');
  picture.className='nexus-custom-icon';
  picture.alt='';
  picture.src='/api/nexus/icon?serial='+encodeURIComponent(serial)+'&id='+encodeURIComponent(w.settings.customIcon);
  picture.draggable=false;if(w.type==='image'){picture.style.width='100%';picture.style.height='100%';picture.style.objectFit='contain'}el.prepend(picture);
 }else if((w.type==='button'||w.type==='macro')&&mediaGlyphs[w.settings?.icon]){
  // Position the same 48-pixel glyph and bottom caption as the LCD renderer.
  el.classList.add('nexus-icon-widget','nexus-media-preview-widget');
  const caption=el.textContent;
  el.textContent='';
  const glyph=document.createElement('span');
  glyph.className='nexus-icon nexus-media-glyph';
  glyph.style.maskImage='url("/static/media-glyphs/'+w.settings.icon+'.svg")';
  glyph.style.webkitMaskImage=glyph.style.maskImage;
  glyph.setAttribute('aria-hidden','true');
  el.append(glyph);
  if(caption){
   const text=document.createElement('span');text.className='nexus-media-caption';
   text.textContent=caption;el.append(text)
  }
 }if(w.type==='graph'){
  const plot=document.createElement('canvas');plot.className='nexus-graph-plot';
  plot.width=Math.max(8,w.width-8);plot.height=Math.max(8,w.height-27);
  plot.setAttribute('aria-hidden','true');el.append(plot);
 }el.dataset.id=w.id;nexusElementById('screen').append(el);
 if(w.type==='graph')paintNexusGraph(el,w)
 }
 const w=s.widgets.find(w=>w.id===selected);if(!w)return;
 const properties=nexusElementById('properties');
 const metricTypes=[['metric','Value'],['bar','Bar'],['graph','Graph']];
 const contentTypes=[['text','Text'],['image','Image'],['clock','Clock']];
 const family=['metric','bar','graph'].includes(w.type)?metricTypes:['text','image','clock'].includes(w.type)?contentTypes:null;
 if(family){
  const typeLabel=document.createElement('label');
  typeLabel.className='nexus-selected-widget-type';
  typeLabel.textContent=family===metricTypes?'Metric display ':'Content type ';
  const typeSelect=document.createElement('select');
  typeSelect.setAttribute('aria-label',family===metricTypes?'Selected metric display':'Selected content type');
  for(const [value,label] of family)typeSelect.add(new Option(label,value));
  typeSelect.value=w.type;
  typeSelect.addEventListener('change',()=>{
   changeSelectedWidgetType(w,typeSelect.value);
   render();
  });
  typeLabel.append(typeSelect);
  (typeMount||properties).append(typeLabel);
 }
 const dimensionsRow=document.createElement('div');dimensionsRow.className='nexus-properties-row nexus-dimensions-row';dimensionsRow.setAttribute('aria-label','Widget dimensions');
 const colorsRow=document.createElement('div');colorsRow.className='nexus-properties-row nexus-colors-row';colorsRow.setAttribute('aria-label','Widget colors');
 const styleRow=document.createElement('div');styleRow.className='nexus-properties-row nexus-style-row';styleRow.setAttribute('aria-label','Widget style and settings');
 // Organize the selected widget's controls into concise, accessible groups.
 const section=(title,opened,kind,icon)=>{
  const details=document.createElement('details');
  details.className='nexus-property-section nexus-'+kind+'-section';
  details.open=opened;
  const summary=document.createElement('summary');
  summary.textContent=title;
  const glyph=document.createElement('i');
  glyph.className='bi bi-'+icon;
  glyph.setAttribute('aria-hidden','true');
  summary.prepend(glyph);
  details.append(summary);
  properties.append(details);
  return details;
 };
 const geometrySection=section('Position & size',true,'geometry','bounding-box');
 const appearanceSection=section('Appearance',true,'appearance','palette');
 const contentSection=section('Content & data',true,'data','type');
 const isActionWidget=['button','macro','image'].includes(w.type);
 const iconSection=isActionWidget?section('Icon',true,'icons','image'):null;
 const touchSection=isActionWidget?section('Actions',true,'actions','cursor'):null;
 geometrySection.append(dimensionsRow);
 appearanceSection.append(colorsRow);
 const appearanceRow=document.createElement('div');
 appearanceRow.className='nexus-properties-row nexus-appearance-row';
 appearanceSection.append(appearanceRow);
 contentSection.append(styleRow);
 const iconRow=document.createElement('div');iconRow.className='nexus-properties-row nexus-icon-row';
 if(iconSection)iconSection.append(iconRow);
 const touchRow=document.createElement('div');touchRow.className='nexus-properties-row nexus-touch-row';
 if(touchSection)touchSection.append(touchRow);
 const fields=(w.type==='button'||w.type==='macro')?[['x','number'],['y','number'],['width','number'],['height','number'],['label','text']]:[['x','number'],['y','number'],['width','number'],['height','number'],['label','text'],['text','text'],['metric','select']];
 for(const [key,type] of fields){if(key==='metric'&&(w.type==='text'||w.type==='image'||w.type==='clock'||w.type==='button'||w.type==='macro'))continue;if(key==='text'&&w.type!=='text')continue;
 const label=document.createElement('label');label.textContent=key+' ';
 let input;if(type==='select'){input=document.createElement('select');for(const m of metrics)input.add(new Option(m,m));input.value=w.metric}else{input=document.createElement('input');input.type=type;input.value=(key in w?w[key]:(w.settings||{})[key])??''}
 if(key==='label'){
  // The default is shown as a placeholder, never persisted as custom text.
  if(w.type==='button'||w.type==='macro')input.placeholder=nexusActionDefaultLabel(w);
  input.addEventListener('input',()=>{
   w.settings??={};w.settings.label=input.value;
   const caption=nexusDisplayLabel(w);
   if(name)name.textContent=caption||w.type;
   const el=[...nexusElementById('screen').children].find(el=>el.dataset.id===w.id);
   if(el){
    const icon=el.querySelector('.nexus-icon, .nexus-custom-icon');
    const captionNode=el.querySelector('.nexus-media-caption');
    if(captionNode){captionNode.textContent=caption}
    else{el.textContent=caption;if(icon)el.prepend(icon)}
   }
  });
 }
 input.addEventListener('change',()=>{let v=type==='number'?Number(input.value):input.value;if(['x','y','width','height'].includes(key)){if(!Number.isFinite(v))return message('Enter a valid number');if(key==='width')w.width=Math.max(1,Math.min(640,Math.round(v)));if(key==='height')w.height=Math.max(1,Math.min(48,Math.round(v)));if(key==='x')w.x=clamp(Math.round(v),640-w.width);if(key==='y')w.y=clamp(Math.round(v),48-w.height);w.x=clamp(w.x,640-w.width);w.y=clamp(w.y,48-w.height)}else if(key==='metric'){
   w.metric=v;w.settings??={};delete w.settings.metricSource;
  }else{w.settings??={};w.settings[key]=v}render()});label.append(input);(['x','y','width','height'].includes(key)?dimensionsRow:styleRow).append(label)}
 if(['metric','bar','graph'].includes(w.type)){
   const sources=nexusMetricSourceOptions[w.metric]||[];
   const saved=w.settings?.metricSource||'';
   if(sources.length||saved){
    const label=document.createElement('label');
    label.textContent='Source ';
    const picker=document.createElement('select');
    picker.setAttribute('aria-label','Metric source');
    picker.add(new Option('Automatic / default',''));
    for(const source of sources)picker.add(new Option(source.label,source.id));
    if(saved&&!sources.some(source=>source.id===saved)){
     picker.add(new Option(saved+' (unavailable)',saved));
    }
    picker.value=saved;
    picker.addEventListener('change',()=>{
     w.settings??={};
     if(picker.value)w.settings.metricSource=picker.value;
     else delete w.settings.metricSource;
     render();
    });
    label.append(picker);
    styleRow.append(label);
   }
  }
 renderWidgetColors(w,colorsRow);
 if(['button','macro','metric','bar','graph','text','clock','image'].includes(w.type)){
  // Opacity is an independent percentage, not a preset background style.
  const radiusLabel=document.createElement('label');
  radiusLabel.className='nexus-radius-control';
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
  const radiusInline=document.createElement('span');radiusInline.className='nexus-input-unit';
  const radiusUnit=document.createElement('span');radiusUnit.className='nexus-input-suffix';radiusUnit.textContent='px';
  radiusInline.append(radius,radiusUnit);radiusLabel.append(radiusInline);appearanceRow.append(radiusLabel);
  if(isActionWidget){
   // Fill style is a look, not an action, so it lives under Appearance.
   const styleLabel=document.createElement('label');styleLabel.className='nexus-fill-style-control';styleLabel.textContent='Style ';
   const styleSelect=document.createElement('select');
   for(const [v,t] of [['solid','Solid fill'],['outline','Outline only'],['flat','No fill']])styleSelect.add(new Option(t,v));
   styleSelect.value=w.settings?.style||'solid';
   styleSelect.addEventListener('change',()=>{w.settings??={};w.settings.style=styleSelect.value;render()});
   styleLabel.append(styleSelect);appearanceRow.append(styleLabel);
  }
  const opacityLabel=document.createElement('label');opacityLabel.className='nexus-opacity-control';
  opacityLabel.textContent='Opacity ';
  const opacitySlider=document.createElement('input');
  opacitySlider.type='range';opacitySlider.min='0';opacitySlider.max='100';opacitySlider.step='1';
  const opacityInput=document.createElement('input');
  opacityInput.type='number';opacityInput.min='0';opacityInput.max='100';opacityInput.step='1';
  const initialOpacity=w.settings?.opacity??'100';
  opacitySlider.value=initialOpacity;opacityInput.value=initialOpacity;
  // Custom track fill extends through the 100% endpoint; the native
  // Chromium accent track leaves a misleading unfilled thumb-width gap.
  opacitySlider.style.setProperty('--nexus-opacity-progress',Math.max(0,Math.min(100,Number(initialOpacity)||0))+'%');
  opacitySlider.setAttribute('aria-label','Opacity slider');
  opacityInput.setAttribute('aria-label','Opacity percentage');
  const updateOpacity=value=>{
   if(value.trim()==='')return;
   const n=Number(value);if(!Number.isFinite(n))return;
   const percentage=Math.max(0,Math.min(100,Math.round(n)));
   w.settings??={};w.settings.opacity=String(percentage);
   opacitySlider.value=String(percentage);opacityInput.value=String(percentage);
   opacitySlider.style.setProperty('--nexus-opacity-progress',percentage+'%');
   const preview=[...nexusElementById('screen').children].find(el=>el.dataset.id===w.id);
   if(preview)preview.style.opacity=String(percentage/100);
  };
  opacitySlider.addEventListener('input',()=>updateOpacity(opacitySlider.value));
  opacityInput.addEventListener('change',()=>updateOpacity(opacityInput.value));
  const opacityInline=document.createElement('span');opacityInline.className='nexus-opacity-inline';
  const opacityValue=document.createElement('span');opacityValue.className='nexus-opacity-value';
  const opacityUnit=document.createElement('span');opacityUnit.className='nexus-input-suffix';opacityUnit.textContent='%';
  opacityValue.append(opacityInput,opacityUnit);opacityInline.append(opacitySlider,opacityValue);
  opacityLabel.append(opacityInline);
  appearanceRow.append(opacityLabel);
  const directionLabel=document.createElement('label');directionLabel.className='nexus-gradient-control';directionLabel.textContent='Gradient ';
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
   input.addEventListener('input',()=>{w.settings??={};w.settings.gradientColor=input.value;const el=[...nexusElementById('screen').children].find(el=>el.dataset.id===w.id);if(el)el.style.backgroundImage='linear-gradient('+(direction.value==='horizontal'?'90deg':'180deg')+','+(w.settings.backgroundColor||'#1b2935')+','+input.value+')'});
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
 if(isActionWidget){
  renderNexusIconProperties(w,iconRow);
  renderTouchProperties(w,touchRow);
 }
}
nexusElementById('screen').addEventListener('pointerdown',e=>{const el=e.target.closest('.widget');if(!el)return;selected=el.dataset.id;selectNexusEditorTab('widget');const w=current().widgets.find(w=>w.id===selected);drag={x:e.clientX,y:e.clientY,startX:w.x,startY:w.y,id:e.pointerId};nexusElementById('screen').setPointerCapture(e.pointerId);render()});
nexusElementById('screen').addEventListener('pointermove',e=>{if(!drag||drag.id!==e.pointerId)return;const w=current().widgets.find(w=>w.id===selected);if(!w)return;w.x=snapPosition(drag.startX+e.clientX-drag.x,640-w.width);w.y=snapPosition(drag.startY+e.clientY-drag.y,48-w.height);const el=[...nexusElementById('screen').children].find(el=>el.dataset.id===w.id);if(el){el.style.left=w.x+'px';el.style.top=w.y+'px'}});
nexusElementById('screen').addEventListener('pointerup',()=>{drag=null;render()});nexusElementById('screen').addEventListener('pointercancel',()=>{drag=null;render()});
const editKeys=new Set(['ArrowLeft','ArrowRight','ArrowUp','ArrowDown']);
nexusElementById('screen').addEventListener('keydown',event=>{
 const screen=current();if(!screen)return;
 const focus=event.target;
 if(focus!==nexusElementById('screen')&&focus?.closest?.('input, textarea, select'))return;
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
  event.preventDefault();nexusElementById('copyWidget')?.click();return;
 }
 if((event.ctrlKey||event.metaKey)&&event.key.toLowerCase()==='v'&&copiedWidget){
  event.preventDefault();nexusElementById('pasteWidget')?.click();return;
 }
 if(!widget)return;
 if(event.key==='Delete'&&!event.ctrlKey&&!event.metaKey){
  event.preventDefault();nexusElementById('deleteWidget')?.click();return;
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

async function loadDevices(){try{const response=await fetch('/api/devices/');const json=await response.json();const data=json.devices||json.device||json.data||json;const entries=Object.entries(data);nexusElementById('device').replaceChildren();for(const [key,v] of entries){if(/nexus/i.test(JSON.stringify(v))){nexusElementById('device').add(new Option((v.product||v.Product||'Nexus')+' — '+key,key))}}if(!nexusElementById('device').length)message('No Nexus found automatically. Enter its serial below.')}catch(e){message(e.message)}}
async function load(){serial=nexusElementById('deviceId')?.value||nexusElementById('device')?.value||prompt('Nexus device serial');if(!serial)return;const res=await fetch('/api/nexus/screens?serial='+encodeURIComponent(serial));if(!res.ok)throw Error(await res.text());cfg=await res.json();await Promise.all([loadSharedMacroProfiles(),loadNexusMetricSources(),loadNexusIconSets()]);selected=null;const mode=nexusElementById('cyclingMode');const interval=nexusElementById('cyclingSeconds');if(mode)mode.value=cfg.cyclingMode||'manual';if(interval){const seconds=cfg.cyclingSeconds||15;if(![...interval.options].some(o=>Number(o.value)===seconds))interval.add(new Option(seconds+' seconds (saved)',String(seconds)));interval.value=String(seconds)}syncCycleInterval();if(nexusElementById('displayBrightness'))nexusElementById('displayBrightness').value=String(cfg.brightness||100);refreshScreens();await loadWidgetLibrary().catch(e=>message('Widget library unavailable: '+e.message))}
if(nexusElementById('load'))nexusElementById('load').onclick=()=>load().catch(e=>message(e.message));
nexusElementById('screens').onchange=()=>{cfg.activeScreenId=nexusElementById('screens').value;selected=null;render()};
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
nexusElementById('newScreenPreset')?.addEventListener('change',refreshActionButtons);
nexusElementById('addScreen').onclick=async()=>{
 try{
  if(!cfg)return message('Load a Nexus layout first.');
  if(cfg.screens.length>=NEXUS_MAX_SCREENS)return message('Maximum 256 screens.');
  const kind=nexusElementById('newScreenPreset')?.value||'blank';
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
nexusElementById('replaceScreenWithPreset')?.addEventListener('click',async()=>{
 try{
  if(!cfg)return message('Load a Nexus layout first.');
  const selectedScreen=current(),kind=nexusElementById('newScreenPreset')?.value;
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
nexusElementById('moveScreenUp')?.addEventListener('click',()=>moveSelectedScreen(-1));
nexusElementById('moveScreenDown')?.addEventListener('click',()=>moveSelectedScreen(1));
nexusElementById('renameScreen')?.addEventListener('click',()=>{
 const s=current();if(!s)return;
 const name=prompt('Rename screen',s.name)?.trim();
 if(name===undefined||name===null||!name||name===s.name)return;
 if(name.length>80)return message('Screen name must be 80 characters or fewer');
 s.name=name;refreshScreens();message('Screen renamed. Save Layout to apply.');
});
nexusElementById('duplicateScreen')?.addEventListener('click',()=>{
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
nexusElementById('deleteScreen').onclick=()=>{
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
  const label=document.createElement('label');label.className='nexus-color-control';
  label.textContent=(key==='backgroundColor'?'BG':key==='textColor'?'Text':'Border')+' ';
  const input=document.createElement('input');input.type='color';input.setAttribute('aria-label',name+' color');input.value=/^#[0-9a-fA-F]{6}$/.test(w.settings?.[key]||'')?w.settings[key]:defaultColor;
  input.oninput=()=>{
   w.settings??={};w.settings[key]=input.value;
   const widget=[...nexusElementById('screen').children].find(el=>el.dataset.id===w.id);
   if(widget)widget.style.setProperty(key==='backgroundColor'?'--button-bg':key==='textColor'?'--button-ink':'--button-border',input.value);
  };
  input.onchange=()=>{w.settings[key]=input.value;render()};
  label.append(input);panel.append(label);
 }
}
// Icons belong to the selected widget inspector. Pack management stays in Settings.
const NEXUS_ICON_PAGE_SIZE=24;
function renderNexusIconProperties(w,panel){
 const sourceLabel=document.createElement('label');
 sourceLabel.className='nexus-icon-source';
 sourceLabel.textContent=w.type==='image'?'Image':'Built-in icon';
 const source=document.createElement('select');
 source.setAttribute('aria-label',w.type==='image'?'Image source':'Built-in icon');
 source.add(new Option('None','none'));
 if(w.type!=='image'){
  for(const glyph of Object.keys(mediaGlyphs)){
   source.add(new Option(mediaDefaultLabels[glyph]||glyph,glyph));
  }
 }
 source.add(new Option('Custom artwork','custom'));
 source.value=w.settings?.customIcon?'custom':(w.settings?.icon||'none');
 source.addEventListener('change',()=>{
  w.settings??={};
  if(source.value==='none'){
   delete w.settings.icon;
   delete w.settings.customIcon;
  }else if(source.value==='custom'){
   w.settings.icon='custom';
  }else{
   delete w.settings.customIcon;
   w.settings.icon=source.value;
  }
  render();
 });
 sourceLabel.append(source);
 panel.append(sourceLabel);

 const installed=nexusIconSets.find(pack=>(pack.icons||[]).some(icon=>icon.nexusId===w.settings?.customIcon));
 const chosenIcon=installed?.icons.find(icon=>icon.nexusId===w.settings?.customIcon);
 const library=document.createElement('div');
 library.className='nexus-icon-library';
 const label=document.createElement('label');
 label.className='nexus-icon-pack-label';
 label.textContent='Installed icon pack';
 const packSelect=document.createElement('select');
 packSelect.setAttribute('aria-label','Icon set');
 packSelect.add(new Option('Choose an installed pack',''));
 for(const pack of nexusIconSets)packSelect.add(new Option(pack.name,pack.id));
 packSelect.value=installed?.id||'';
 label.append(packSelect);
 library.append(label);
 const iconSearch=document.createElement('input');
 iconSearch.type='search';
 iconSearch.placeholder='Search icon name or tags';
 iconSearch.setAttribute('aria-label','Search installed pack icons');
 library.append(iconSearch);
 const gallery=document.createElement('div');
 gallery.className='nexus-icon-gallery';
 gallery.setAttribute('role','group');
 gallery.setAttribute('aria-label','Icons in selected pack');
 // Large packs hold thousands of icons. Render one page at a time inside a
 // bounded, scrollable gallery so the inspector stays a manageable size.
 const pager=document.createElement('div');
 pager.className='nexus-icon-pager';
 const prevPage=document.createElement('button');
 prevPage.type='button';prevPage.textContent='‹ Prev';prevPage.setAttribute('aria-label','Previous page of icons');
 const pageInfo=document.createElement('small');
 pageInfo.setAttribute('aria-live','polite');
 const nextPage=document.createElement('button');
 nextPage.type='button';nextPage.textContent='Next ›';nextPage.setAttribute('aria-label','Next page of icons');
 pager.append(prevPage,pageInfo,nextPage);
 let page=0,pageInitialised=false;
 const fillGallery=()=>{
  gallery.replaceChildren();
  pager.hidden=true;
  const pack=nexusIconSets.find(item=>item.id===packSelect.value);
  if(!pack){
   const note=document.createElement('small');
   note.textContent=nexusIconSets.length?'Choose a pack to preview and select its icons.':'No icon packs installed. Add one in Settings.';
   gallery.append(note);
   return;
  }
  const query=iconSearch.value.trim().toLocaleLowerCase();
  const matches=(pack.icons||[]).map((icon,index)=>({icon,index})).filter(({icon})=>
   !query || icon.name.toLocaleLowerCase().includes(query) || (icon.tags||[]).some(tag=>tag.toLocaleLowerCase().includes(query)));
  if(!matches.length){
   const note=document.createElement('small');
   note.textContent=query?'No icons match "'+iconSearch.value.trim()+'".':'This pack has no icons.';
   gallery.append(note);
   return;
  }
  const pages=Math.ceil(matches.length/NEXUS_ICON_PAGE_SIZE);
  if(!pageInitialised){
   // Open on the page that holds the widget's current icon.
   pageInitialised=true;
   const at=matches.findIndex(({icon})=>icon.nexusId===w.settings?.customIcon);
   if(at>=0)page=Math.floor(at/NEXUS_ICON_PAGE_SIZE);
  }
  page=Math.min(Math.max(page,0),pages-1);
  const first=page*NEXUS_ICON_PAGE_SIZE;
  const visible=matches.slice(first,first+NEXUS_ICON_PAGE_SIZE);
  if(pages>1){
   pager.hidden=false;
   pageInfo.textContent=(first+1)+'–'+(first+visible.length)+' of '+matches.length+' icons';
   prevPage.disabled=page===0;
   nextPage.disabled=page>=pages-1;
  }
  for(const {index,icon} of visible){
   const tile=document.createElement('button');
   tile.type='button';
   tile.className='nexus-icon-tile';
   tile.title=icon.name;
   tile.setAttribute('aria-label','Use '+icon.name+' icon');
   tile.setAttribute('aria-pressed',String(icon.nexusId===w.settings?.customIcon));
   const preview=document.createElement('img');
   preview.alt='';
   preview.loading='lazy';
   preview.src='/api/icon-sets/preview?id='+encodeURIComponent(pack.id)+'&index='+index;
   const title=document.createElement('span');
   title.textContent=icon.name;
   tile.append(preview,title);
   tile.addEventListener('click',()=>{
    w.settings??={};
    w.settings.customIcon=icon.nexusId;
    delete w.settings.icon;
    render();
    message('Pack icon assigned. Save Layout to apply.');
   });
   gallery.append(tile);
  }
  gallery.scrollTop=0;
 };
 prevPage.addEventListener('click',()=>{page--;fillGallery();});
 nextPage.addEventListener('click',()=>{page++;fillGallery();});
 packSelect.addEventListener('change',()=>{iconSearch.value='';page=0;fillGallery();});
 iconSearch.addEventListener('input',()=>{page=0;fillGallery();});
 fillGallery();
 library.append(gallery,pager);
 if(w.settings?.customIcon){
  const current=document.createElement('small');
  current.className='nexus-icon-current';
  current.textContent=chosenIcon?'Selected: '+chosenIcon.name+' ('+installed.name+')':'Selected: custom artwork (not in an installed pack)';
  library.append(current);
 }
 const settingsLink=document.createElement('a');
 settingsLink.className='nexus-icon-settings-link';
 settingsLink.href='/settings#olh-settings-icons';
 settingsLink.textContent='Manage icon packs in Settings';
 library.append(settingsLink);
 panel.append(library);

 // Preserve the existing one-off artwork workflow separately from reusable packs.
 if(w.settings?.customIcon || w.settings?.icon==='custom'){
  const wrapper=document.createElement('label');wrapper.className='nexus-custom-icon-upload';
  wrapper.textContent='Upload one-off icon (PNG/JPEG/GIF) ';
  const file=document.createElement('input');file.type='file';file.accept='image/png,image/jpeg,image/gif';
  file.setAttribute('aria-label','Upload custom icon');
  wrapper.append(file);
  const upload=document.createElement('button');upload.type='button';upload.textContent='Upload icon';
  upload.onclick=async()=>{
   const image=file.files?.[0];
   if(!image)return message('Choose a PNG, JPEG or GIF icon first');
   if(image.size>512*1024)return message('Icon exceeds 512 KiB');
   if(!['image/png','image/jpeg','image/gif'].includes(image.type))return message('Select a PNG, JPEG or GIF image');
   upload.disabled=true;
   try{
    const id='icon_'+Date.now()+'_'+Math.floor(Math.random()*1000000);
    const res=await fetch('/api/nexus/icon?serial='+encodeURIComponent(serial)+'&id='+encodeURIComponent(id),{method:'POST',headers:{'Content-Type':image.type},body:image});
    if(!res.ok)throw Error(await res.text());
    w.settings??={};w.settings.customIcon=id;delete w.settings.icon;
    render();message('Custom icon uploaded. Save Layout to apply it.');
   }catch(e){message(e.message);upload.disabled=false}
  };
  panel.append(wrapper,upload);
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
   // The editor preview selects/drags widgets; hardware touches execute them.
   // A deliberate link lets users test a URL in their *current browser*
   // without exposing an unauthenticated remote launch endpoint.
   let browserTestLink=null;
   const refreshBrowserTestLink=()=>{
    if(!browserTestLink)return;
    const raw=w.target||'';
    let safeHref=null;
    try {
     const parsed=typeof URL==='function'?new URL(raw):null;
     if(parsed&&raw.length>=10&&raw.length<=256&&
      !/[\s\x00-\x1f\x7f]/.test(raw)&&
      ['http:','https:'].includes(parsed.protocol)&&
      parsed.hostname&&!parsed.username&&!parsed.password){
      safeHref=parsed.href;
     }
    }catch(_){}
    browserTestLink.hidden=safeHref===null;
    if(safeHref!==null)browserTestLink.href=safeHref;
    else browserTestLink.removeAttribute?.('href');
   };
   input.addEventListener('change',()=>{
    w.target=input.value.trim();
    refreshBrowserTestLink();
    message('Launcher updated. Save Layout to apply.');
   });
   label.append(input);panel.append(label);
   if(w.action==='url'){
    browserTestLink=document.createElement('a');
    browserTestLink.textContent='Test URL in this browser';
    browserTestLink.className='nexus-url-test-link';
    browserTestLink.target='_blank';
    browserTestLink.rel='noopener noreferrer';
    browserTestLink.setAttribute('aria-label','Test URL in this browser (not on the Nexus device)');
    refreshBrowserTestLink();
    panel.append(browserTestLink);
   }
   const note=document.createElement('small');
   note.textContent='The screen preview selects widgets; it does not execute them. Physical Nexus URL and app actions open on the host desktop, not a remote WebUI browser. If OpenLinkHub runs without a desktop session, start the local Nexus desktop agent as your logged-in user. No shell commands.';
   panel.append(note);
  }
  if(w.action==='device')field('Device RGB action',w.target,hardwareActions,v=>w.target=v);
  if(w.action==='media')field('Media action',w.target,media,v=>{const old=w.target;w.target=v;if(!w.settings?.customIcon&&(!w.settings?.icon||w.settings.icon===old)){w.settings??={};w.settings.icon=v}});
 }


}
function addWidget(type){const s=current();if(!s)return;if(s.widgets.length>=64)return message('Maximum 64 widgets');if(totalNexusWidgets()>=NEXUS_MAX_TOTAL_WIDGETS)return message('Maximum 4096 widgets across all screens');const id=uniqueID('widget',new Set(s.widgets.map(w=>w.id)));const w={id,type,metric:(type==='text'||type==='image'||type==='clock'||type==='button'||type==='macro')?'':'cpu.temperature',x:0,y:0,width:152,height:48,settings:type==='image'?{icon:'custom',style:'flat'}:type==='text'?{text:'New text'}:type==='clock'?{clockFormat:'24h',label:''}:{label:((type==='button'||type==='macro')?'':'CPU')},...(type==='button'?{action:'next'}:{}),};s.widgets.push(w);selected=id;selectNexusEditorTab('widget');render()}
nexusElementById('addMetric').onclick=()=>addWidget('metric');
nexusElementById('addContent')?.addEventListener('click',()=>addWidget('text'));
nexusElementById('addButton')?.addEventListener('click',()=>addWidget('button'));
nexusElementById('copyWidget')?.addEventListener('click',()=>{
 const source=current()?.widgets.find(w=>w.id===selected);
 if(!source)return;
 copiedWidget=JSON.parse(JSON.stringify(source));
 render();message('Widget copied. Select a screen and click Paste widget.');
});
nexusElementById('pasteWidget')?.addEventListener('click',()=>{
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
nexusElementById('duplicateWidget')?.addEventListener('click',()=>{
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
nexusElementById('deleteWidget').onclick=()=>{const s=current();if(!s)return;s.widgets=s.widgets.filter(w=>w.id!==selected);selected=null;render()};
nexusElementById('background').onchange=()=>{const s=current();if(s){s.background=nexusElementById('background').value.trim();render()}};
nexusElementById('upload').onclick=async()=>{try{const f=nexusElementById('image').files[0];if(!f||!current())return message('Choose an image and load a screen');if(f.size>4*1024*1024)throw Error('Image exceeds 4 MiB');if(!['image/png','image/jpeg','image/gif'].includes(f.type))throw Error('Choose a PNG, JPEG or GIF background');const id='background_'+Date.now();const res=await fetch('/api/nexus/background?serial='+encodeURIComponent(serial)+'&id='+id,{method:'POST',headers:{'Content-Type':f.type},body:f});if(!res.ok)throw Error(await res.text());current().background=id;render();message('Background uploaded. Save layout to apply it.')}catch(e){message(e.message)}};
function syncCycleInterval(){
 const interval=nexusElementById('cyclingSeconds'),mode=nexusElementById('cyclingMode');
 if(interval&&mode)interval.disabled=mode.value!=='automatic';
}
nexusElementById('cyclingMode')?.addEventListener('change',syncCycleInterval);
syncCycleInterval();
nexusElementById('displayBrightness')?.addEventListener('change',()=>{if(!cfg)return;cfg.brightness=Number(nexusElementById('displayBrightness').value);render()});
nexusElementById('save').onclick=async()=>{try{if(!cfg)return;if(nexusElementById('displayBrightness'))cfg.brightness=Number(nexusElementById('displayBrightness').value);const mode=nexusElementById('cyclingMode');const interval=nexusElementById('cyclingSeconds');if(mode){cfg.cyclingMode=mode.value;cfg.cyclingSeconds=Number(interval?.value||15)}cfg.activeScreenId=nexusElementById('screens').value;const res=await fetch('/api/nexus/screens?serial='+encodeURIComponent(serial),{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(cfg)});if(!res.ok)throw Error(await res.text());message('Saved successfully')}catch(e){message(e.message)}};
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
nexusElementById('exportLayout')?.addEventListener('click',()=>exportNexusJSON(false));
nexusElementById('exportScreen')?.addEventListener('click',()=>exportNexusJSON(true));
nexusElementById('importLayout')?.addEventListener('click',async()=>{
 try{
  if(!cfg)return message('Load a Nexus layout first.');
  const file=nexusElementById('layoutFile')?.files?.[0];
  if(!file)return message('Select a JSON layout file.');
  if(file.size<1||file.size>4*1024*1024)throw Error('JSON file must be 4 MiB or smaller');
  const imported=NexusLayoutFiles.validate(JSON.parse(await file.text()));
  const mode=nexusElementById('layoutImportMode').value;
  if(mode==='replace-layout'&&!confirm('Replace the entire current layout with '+imported.screens.length+' imported screens? Changes become permanent only when Save Layout is clicked.'))return;
  if(mode==='replace-screen'&&!confirm('Replace selected screen "'+(current()?.name||'')+'"? Changes become permanent only when Save Layout is clicked.'))return;
  const id=mode==='add-screen'?uniqueID('screen',new Set(cfg.screens.map(s=>s.id))):'';
  const next=NexusLayoutFiles.applyImport(cfg,imported,mode,nexusElementById('screens').value,id);
  cfg=next;selected=null;
  if(nexusElementById('cyclingMode'))nexusElementById('cyclingMode').value=cfg.cyclingMode||'manual';
  if(nexusElementById('displayBrightness'))nexusElementById('displayBrightness').value=String(cfg.brightness||100);
  if(nexusElementById('cyclingSeconds')){
   const seconds=cfg.cyclingSeconds||15;
   if(![...nexusElementById('cyclingSeconds').options].some(option=>Number(option.value)===seconds))nexusElementById('cyclingSeconds').add(new Option(seconds+' seconds (imported)',String(seconds)));
   nexusElementById('cyclingSeconds').value=String(seconds);
  }
  syncCycleInterval();refreshScreens();
  message('JSON imported into the editor. Review before Save Layout.');
 }catch(e){message('Import failed: '+e.message)}
});


// Reusable widgets live in OpenLinkHub's database/nexus/widget-library.json,
// not in a particular browser's localStorage.
let nexusSavedWidgetEntries=[];
const widgetLibraryEndpoint='/api/nexus/widget-library';
async function widgetLibraryRequest(options={}){
 const response=await fetch(widgetLibraryEndpoint,{cache:'no-store',...options});
 if(!response.ok)throw Error((await response.text()).trim()||'Widget library HTTP '+response.status);
 return response.status===204?null:response.json();
}
function refreshWidgetLibrary(){
 const select=nexusElementById('widgetLibrarySelect');if(!select)return;
 const previouslySelected=select.value;
 select.replaceChildren();
 for(const entry of nexusSavedWidgetEntries)select.add(new Option(entry.name,entry.id));
 if(nexusSavedWidgetEntries.some(entry=>entry.id===previouslySelected))select.value=previouslySelected;
 const selectedEntry=nexusSavedWidgetEntries.find(entry=>entry.id===select.value);
 if(nexusElementById('addWidgetPreset'))nexusElementById('addWidgetPreset').disabled=!selectedEntry||!current()||current().widgets.length>=64||totalNexusWidgets()>=NEXUS_MAX_TOTAL_WIDGETS;
 if(nexusElementById('deleteWidgetPreset'))nexusElementById('deleteWidgetPreset').disabled=!selectedEntry;
}
async function migrateLegacyBrowserWidgets(){
 // Only read the former browser-local library once, to preserve existing
 // personal presets. All subsequent create/delete/list operations use the API.
 if(typeof localStorage==='undefined')return;
 let legacy;
 try{legacy=NexusWidgetLibrary.list(localStorage)}catch(e){
  message('Previous browser widget library could not be migrated: '+e.message);
  return;
 }
 if(!legacy.length)return;
 let imported=0;
 for(const entry of legacy){
  if(nexusSavedWidgetEntries.some(saved=>saved.id===entry.id))continue;
  try{
   await widgetLibraryRequest({method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(entry)});
   nexusSavedWidgetEntries.push(entry);
   imported++;
  }catch(e){
   message('Could not migrate all browser widgets: '+e.message);
   return; // Keep the browser copy intact to prevent data loss.
  }
 }
 localStorage.removeItem?.('openlinkhub:nexus:widgets:v1');
 if(imported)message('Migrated '+imported+' browser widget(s) into the OpenLinkHub library.');
}
async function loadWidgetLibrary(){
 const result=await widgetLibraryRequest();
 if(!Array.isArray(result?.entries))throw Error('Invalid server widget library');
 nexusSavedWidgetEntries=result.entries;
 await migrateLegacyBrowserWidgets();
 refreshWidgetLibrary();
}
nexusElementById('saveWidgetPreset')?.addEventListener('click',async()=>{
 try{
  const widget=current()?.widgets.find(w=>w.id===selected);
  if(!widget)return message('Select a widget to save.');
  const name=prompt('Name for the reusable widget',widget.settings?.label||widget.type)?.trim();
  if(!name)return;
  if(name.length>80)return message('Widget library name must be 80 characters or fewer.');
  const entry={id:uniqueID('preset',new Set(nexusSavedWidgetEntries.map(x=>x.id))),name,widget:JSON.parse(JSON.stringify(widget))};
  // Server revalidates the complete widget and performs an atomic disk write.
  await widgetLibraryRequest({method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(entry)});
  await loadWidgetLibrary();
  nexusElementById('widgetLibrarySelect').value=entry.id;
  refreshWidgetLibrary();
  message('Widget saved in OpenLinkHub. Available across browsers and restarts.');
 }catch(e){message('Unable to save widget: '+e.message)}
});
nexusElementById('addWidgetPreset')?.addEventListener('click',()=>{
 try{
  const screen=current();if(!screen)return;
  if(screen.widgets.length>=64)return message('Maximum 64 widgets');
  if(totalNexusWidgets()>=NEXUS_MAX_TOTAL_WIDGETS)return message('Maximum 4096 widgets across all screens');
  const entry=nexusSavedWidgetEntries.find(x=>x.id===nexusElementById('widgetLibrarySelect').value);
  if(!entry)return;
  const widget=NexusWidgetLibrary.instantiate(entry,uniqueID('widget',new Set(screen.widgets.map(w=>w.id))),screen.id,cfg.screens.map(s=>s.id));
  screen.widgets.push(widget);selected=widget.id;render();
  refreshWidgetLibrary();
  message('Widget added from library. Save Layout to apply.');
 }catch(e){message(e.message)}
});
nexusElementById('deleteWidgetPreset')?.addEventListener('click',async()=>{
 try{
  const id=nexusElementById('widgetLibrarySelect').value;
  const entry=nexusSavedWidgetEntries.find(x=>x.id===id);if(!entry)return;
  if(!confirm('Delete saved widget "'+entry.name+'" from OpenLinkHub?'))return;
  const response=await fetch(widgetLibraryEndpoint+'?id='+encodeURIComponent(id),{method:'DELETE',cache:'no-store'});
  if(!response.ok)throw Error((await response.text()).trim()||'Delete failed');
  await loadWidgetLibrary();
  message('Saved widget removed from the OpenLinkHub library.');
 }catch(e){message('Unable to delete widget: '+e.message)}
});
nexusElementById('widgetLibrarySelect')?.addEventListener('change',refreshWidgetLibrary);

// Integrated device pages load their Nexus automatically without a device selector.
// The standalone editor retains its device dropdown and Load button.
if (nexusElementById('deviceId')) {
  load().catch(e => message(e.message));
} else if (nexusElementById('device')) {
  loadDevices();
}

// Clock preview updates without re-rendering the editor or interrupting drag.
// Live browser plots reuse the physical Nexus's cached 60-second samples.
// The endpoint never polls new sensors; it only returns the last LCD sample.
function paintNexusGraph(element,widget){
 const plot=element.querySelector?.('canvas.nexus-graph-plot');
 const context=plot?.getContext?.('2d');if(!context)return;
 const points=nexusPreviewTelemetry?.history?.[nexusMetricLookupKey(widget)];
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
 for(const element of nexusElementById('screen').children){
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
 for(const el of nexusElementById('screen').children){
  const widget=active.widgets.find(w=>w.id===el.dataset.id);
  if(widget?.type==='clock')el.textContent=formatNexusClock(widget.settings);
 }
}
if(typeof setInterval==='function')setInterval(refreshNexusClockPreviews,1000);

function refreshApplicationScreenMappings(){
 const list=nexusElementById('applicationScreenMappings');
 const select=nexusElementById('applicationScreenTarget');
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
nexusElementById('addApplicationScreen')?.addEventListener('click',()=>{
 if(!cfg)return;
 const id=(nexusElementById('applicationScreenID')?.value||'').trim();
 if(!/^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$/.test(id))return message('Use a valid application ID from OpenLinkHub Application Profiles.');
 cfg.applicationScreens??={};
 if(!Object.hasOwn(cfg.applicationScreens,id)&&Object.keys(cfg.applicationScreens).length>=64)return message('Maximum 64 application bindings.');
 cfg.applicationScreens[id]=nexusElementById('applicationScreenTarget').value;
 refreshApplicationScreenMappings();
 message('Application screen mapping added. Save Layout to apply.');
});




// A visual click indication also works with keyboard activation in the editor.
nexusElementById('screen')?.addEventListener('pointerdown',event=>{
 const target=event.target.closest?.('.widget.nexus-button-style');
 if(!target)return;
 target.classList.add('nexus-pressed-preview');
});
nexusElementById('screen')?.addEventListener('pointerup',()=>{
 for(const item of nexusElementById('screen').children)item.classList?.remove?.('nexus-pressed-preview');
});

/* Five file tabs share a single editor workspace below the screen preview. */
function selectNexusEditorTab(name,focus=false){
 const names=['widget','library','background','layout','applications'];
 const index=names.indexOf(name);
 if(index<0)return;
 for(const [i,id] of names.entries()){
  const tab=nexusElementById('nexus-tab-'+id),panel=nexusElementById('nexus-panel-'+id);
  if(!tab||!panel)return;
  const active=i===index;
  tab.setAttribute('aria-selected',String(active));
  tab.setAttribute('tabindex',active?'0':'-1');
  panel.hidden=!active;
 }
 if(focus)nexusElementById('nexus-tab-'+name).focus();
}
(function setupNexusEditorTabs(){
 const names=['widget','library','background','layout','applications'];
 if(names.some(name=>!nexusElementById('nexus-tab-'+name)||!nexusElementById('nexus-panel-'+name)))return;
 names.forEach((name,index)=>{
  const tab=nexusElementById('nexus-tab-'+name);
  tab.addEventListener('click',()=>selectNexusEditorTab(name));
  tab.addEventListener('keydown',event=>{
   const offset=event.key==='ArrowRight'?1:event.key==='ArrowLeft'?-1:0;
   if(!offset&&event.key!=='Home'&&event.key!=='End')return;
   event.preventDefault();
   const next=event.key==='Home'?0:event.key==='End'?names.length-1:(index+offset+names.length)%names.length;
   selectNexusEditorTab(names[next],true);
  });
 });
 selectNexusEditorTab('widget');
})();
