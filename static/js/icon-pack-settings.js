/* Settings owns importing and managing packs; device editors only select. */
(function () {
 'use strict';
 function initialize() {
  const root = document.getElementById('olh-icon-pack-library');
  if (!root) return;
  const name = document.getElementById('olh-icon-pack-name');
  const file = document.getElementById('olh-icon-pack-file');
  const importer = document.getElementById('olh-icon-pack-import');
  const status = document.getElementById('olh-icon-pack-status');
  let working = false;
  function message(text, error) {
   status.textContent = text;
   status.classList.toggle('is-error', !!error);
  }
  async function request(url, options) {
   const response = await fetch(url, {cache:'no-store',...options});
   if (!response.ok) {
    const reason = await response.text();
    throw new Error(reason.trim() || 'Icon pack request failed (' + response.status + ')');
   }
   return response.status === 204 ? null : response.json();
  }
  const make = (tag,cls,text) => {
   const el = document.createElement(tag);
   if (cls) el.className = cls;
   if (text !== undefined) el.textContent = text;
   return el;
  };
  function showPacks(packs) {
   root.replaceChildren();
   if (!packs.length) {
    root.appendChild(make('p','olh-icon-pack-description','No icon packs imported yet.'));
    return;
   }
   for (const pack of packs) {
    const card = make('article','olh-icon-pack-item');
    const title = make('div','olh-icon-pack-item-title');
    const edit = make('input');
    edit.type = 'text';
    edit.value = pack.name;
    edit.maxLength = 60;
    edit.setAttribute('aria-label','Pack name for ' + pack.name);
    title.appendChild(edit);
    const counter = make('small','olh-icon-pack-description',(pack.icons || []).length + ' icon(s)');
    const actions = make('div','olh-icon-pack-item-actions');
    const rename = make('button','system-button','Save name');
    rename.type='button';
    rename.addEventListener('click',async()=>{
     if (working || !edit.value.trim()) return;
     working=true;rename.disabled=true;
     try {
      await request('/api/icon-sets?id='+encodeURIComponent(pack.id),{
       method:'PUT',headers:{'Content-Type':'application/json'},
       body:JSON.stringify({name:edit.value.trim()})
      });
      message('Icon pack renamed.');
      await refresh();
     } catch(e){ message(e.message,true); }
     finally {working=false;rename.disabled=false;}
    });
    const remove = make('button','system-button danger','Delete pack');
    remove.type='button';
    remove.addEventListener('click',async()=>{
     if (working || !window.confirm('Delete icon pack "'+pack.name+'"? Icons already used in layouts will be retained.')) return;
     working=true;remove.disabled=true;
     try {
      await request('/api/icon-sets?id='+encodeURIComponent(pack.id),{method:'DELETE'});
      message('Icon pack deleted. Icons already used in layouts remain available.');
      await refresh();
     } catch(e){message(e.message,true);}
     finally {working=false;remove.disabled=false;}
    });
    actions.append(rename,remove);
    const tiles=make('div','olh-icon-pack-thumbnails');
    for (const [index, icon] of (pack.icons || []).entries()) {
     const tile=make('div','olh-icon-pack-tile');
     const img=make('img');
     img.src='/api/icon-sets/preview?id='+encodeURIComponent(pack.id)+'&index='+index;
     img.alt='';
     img.loading='lazy';
     const caption=make('span','',icon.name);
     caption.title=[icon.name,...(icon.tags||[])].join(' · ');
     tile.append(img,caption);
     tiles.appendChild(tile);
    }
    card.append(title,counter,actions,tiles);
    root.appendChild(card);
   }
  }
  async function refresh() {
   try {
    const response=await request('/api/icon-sets');
    showPacks(Array.isArray(response?.sets)?response.sets:[]);
    if (response?.svgAvailable === false)
     message('PNG/JPEG imports are ready. For SVG, install librsvg2-bin on the host.');
   } catch(e){message('Unable to load icon packs: '+e.message,true);}
  }
  importer.addEventListener('click',async()=>{
   const zip=file.files?.[0];
   if (working) return;
   if (!zip || zip.size<1 || zip.size>32*1024*1024) {
    message('Select a ZIP or .streamDeckIconPack file no larger than 32 MiB.',true);return;
   }
   const streamDeck=/\.streamDeckIconPack$/i.test(zip.name);
   const label=name.value.trim() || (streamDeck ? '' : zip.name.replace(/\.zip$/i,''));
   working=true;importer.disabled=true;
   try {
    await request('/api/icon-sets?name='+encodeURIComponent(label),{
     method:'POST',headers:{'Content-Type':'application/zip'},body:zip
    });
    name.value='';file.value='';
    message('Icon pack imported. Select its icons from the Nexus or Deck editor.');
    await refresh();
   } catch(e){message('Import failed: '+e.message,true);}
   finally{working=false;importer.disabled=false;}
  });
  refresh();
 }
 if (document.readyState==='loading') document.addEventListener('DOMContentLoaded',initialize);
 else initialize();
})();
