'use strict';
// Reusable Nexus widgets are deliberately browser-local. No user scripts,
// shell commands or remote services are involved.
globalThis.NexusWidgetLibrary=(()=>{
 const storageKey='openlinkhub:nexus:widgets:v1';
 const limit=50;
 const idPattern=/^[A-Za-z0-9_-]{1,64}$/;
 const types=new Set(['metric','bar','graph','clock','text','image','button','macro']);
 const clone=obj=>JSON.parse(JSON.stringify(obj));
 function validateEntry(entry){
  if(!entry||typeof entry!=='object'||Array.isArray(entry)||!idPattern.test(entry.id))
   throw Error('Invalid library entry ID');
  if(typeof entry.name!=='string'||!entry.name.trim()||entry.name.length>80)
   throw Error('Invalid library entry name');
  const w=entry.widget;
  if(!w||typeof w!=='object'||Array.isArray(w)||!types.has(w.type)||!idPattern.test(w.id))
   throw Error('Invalid library widget');
  for(const field of ['x','y','width','height'])if(!Number.isInteger(w[field]))throw Error('Invalid widget geometry');
  if(w.x<0||w.y<0||w.width<1||w.height<1||w.x+w.width>640||w.y+w.height>48)
   throw Error('Widget outside Nexus display');
  if(w.settings!==undefined){
   if(!w.settings||typeof w.settings!=='object'||Array.isArray(w.settings)||Object.keys(w.settings).length>16)
    throw Error('Invalid widget settings');
   for(const [key,val] of Object.entries(w.settings))if(key.length>64||typeof val!=='string'||val.length>256)
    throw Error('Invalid widget setting value');
  }
  if(w.settings?.textShadow&&!['none','soft','strong'].includes(w.settings.textShadow))throw Error('Invalid text shadow');
  if(w.settings?.verticalAlign&&!['top','middle','bottom'].includes(w.settings.verticalAlign))throw Error('Invalid vertical text alignment');
  if(w.steps!==undefined&&(!Array.isArray(w.steps)||w.steps.length>12))throw Error('Invalid macro steps');
  return entry;
 }
 function list(storage){
  const value=storage.getItem(storageKey);
  if(!value)return [];
  if(value.length>256*1024)throw Error('Widget library exceeds storage limit');
  const parsed=JSON.parse(value);
  if(!Array.isArray(parsed)||parsed.length>limit)throw Error('Invalid widget library format');
  const ids=new Set();
  for(const entry of parsed){
   validateEntry(entry);
   if(ids.has(entry.id))throw Error('Duplicate widget library ID');
   ids.add(entry.id);
  }
  return clone(parsed);
 }
 function write(storage,entries){
  if(!Array.isArray(entries)||entries.length>limit)throw Error('Widget library is full');
  entries.forEach(validateEntry);
  const data=JSON.stringify(entries);
  if(data.length>256*1024)throw Error('Widget library exceeds storage limit');
  storage.setItem(storageKey,data);
 }
 function add(storage,entry){
  validateEntry(entry);
  const items=list(storage);
  if(items.some(item=>item.id===entry.id))throw Error('Widget library entry ID already exists');
  write(storage,[...items,clone(entry)]);
 }
 function remove(storage,id){write(storage,list(storage).filter(entry=>entry.id!==id))}
 function instantiate(entry,newID,screenID,knownScreenIDs){
  validateEntry(entry);
  if(!idPattern.test(newID))throw Error('Invalid new widget ID');
  const widget=clone(entry.widget);widget.id=newID;
  widget.x=Math.min(640-widget.width,Math.max(0,widget.x+16));
  widget.y=Math.min(48-widget.height,Math.max(0,widget.y+8));
  if(widget.type==='button'&&widget.action==='screen'&&!knownScreenIDs.includes(widget.target))
   widget.target=screenID;
  if(widget.type==='macro'||(widget.type==='button'&&widget.action==='macro')){
   for(const step of widget.steps||[])
    if(step.type==='screen'&&!knownScreenIDs.includes(step.value))step.value=screenID;
  }
  return widget;
 }
 return Object.freeze({list,add,remove,instantiate});
})();
