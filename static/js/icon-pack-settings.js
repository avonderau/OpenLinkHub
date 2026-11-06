/* Settings owns importing and managing packs; device editors only select. */
(function () {
 'use strict';
 function initialize() {
  const root = document.getElementById('olh-icon-pack-library');
  if (!root) return;
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
  let selectedPackId = '';
  let iconQuery = '';
  let visibleIcons = 120;
  const previewBatch = 120;
  function showPacks(packs) {
   root.replaceChildren();
   if (!packs.length) {
    selectedPackId = '';
    root.appendChild(make('p','olh-icon-pack-description','No icon packs imported yet.'));
    return;
   }
   if (!packs.some(pack=>pack.id===selectedPackId)) selectedPackId=packs[0].id;
   const selected=packs.find(pack=>pack.id===selectedPackId);
   const sidebar=make('nav','olh-icon-pack-sidebar');
   sidebar.setAttribute('aria-label','Installed icon packs');
   const heading=make('div','olh-icon-pack-sidebar-title','Installed packs');
   const count=make('small','olh-icon-pack-description',packs.length+' pack(s)');
   sidebar.append(heading,count);
   const packList=make('div','olh-icon-pack-list');
   for (const pack of packs) {
    const entry=make('button','olh-icon-pack-option');
    entry.type='button';
    entry.setAttribute('aria-current',String(pack.id===selectedPackId));
    entry.append(make('span','olh-icon-pack-option-name',pack.name));
    entry.append(make('small','olh-icon-pack-option-count',(pack.icons||[]).length+' icons'));
    entry.addEventListener('click',()=>{
     if (working || selectedPackId===pack.id) return;
     selectedPackId=pack.id;
     iconQuery='';
     visibleIcons=previewBatch;
     showPacks(packs);
    });
    packList.append(entry);
   }
   sidebar.append(packList);
   const details=make('section','olh-icon-pack-detail');
   details.setAttribute('aria-label','Selected icon pack');
   const headingRow=make('div','olh-icon-pack-detail-heading');
   headingRow.append(make('h3','olh-icon-pack-detail-title',selected.name));
   headingRow.append(make('small','olh-icon-pack-description',(selected.icons||[]).length+' icons'));
   details.append(headingRow);
   const toolbar=make('div','olh-icon-pack-item-title');
   const edit=make('input','form-control');
   edit.type='text';
   edit.value=selected.name;
   edit.maxLength=60;
   edit.setAttribute('aria-label','Pack name for '+selected.name);
   const rename=make('button','system-button','Save name');
   rename.type='button';
   rename.addEventListener('click',async()=>{
    if (working || !edit.value.trim()) return;
    working=true;rename.disabled=true;
    try {
     await request('/api/icon-sets?id='+encodeURIComponent(selected.id),{
      method:'PUT',headers:{'Content-Type':'application/json'},
      body:JSON.stringify({name:edit.value.trim()})
     });
     message('Icon pack renamed.');
     await refresh();
    } catch(e){message(e.message,true);}
    finally {working=false;rename.disabled=false;}
   });
   const remove=make('button','system-button danger','Delete pack');
   remove.type='button';
   remove.addEventListener('click',async()=>{
    if (working || !window.confirm('Delete icon pack "'+selected.name+'"? Icons already used in layouts will be retained.')) return;
    working=true;remove.disabled=true;
    try {
     await request('/api/icon-sets?id='+encodeURIComponent(selected.id),{method:'DELETE'});
     selectedPackId='';
     iconQuery='';
     visibleIcons=previewBatch;
     message('Icon pack deleted. Icons already used in layouts remain available.');
     await refresh();
    } catch(e){message(e.message,true);}
    finally {working=false;remove.disabled=false;}
   });
   toolbar.append(edit,rename,remove);
   details.append(toolbar);
   if (selected.author || selected.version) {
    details.append(make('small','olh-icon-pack-description',
      [selected.author,selected.version && 'Version '+selected.version].filter(Boolean).join(' · ')));
   }
   const search=make('input','form-control olh-icon-pack-search');
   search.type='search';
   search.value=iconQuery;
   search.placeholder='Search icon names and tags';
   search.setAttribute('aria-label','Search icons in '+selected.name);
   details.append(search);
   const summary=make('small','olh-icon-pack-description');
   details.append(summary);
   const tiles=make('div','olh-icon-pack-thumbnails');
   details.append(tiles);
   const loadMore=make('button','system-button olh-icon-pack-load-more','Show more icons');
   loadMore.type='button';
   details.append(loadMore);
   const paintIcons=()=>{
    tiles.replaceChildren();
    const query=iconQuery.trim().toLocaleLowerCase();
    const matching=(selected.icons||[]).map((icon,index)=>({icon,index}))
     .filter(({icon})=>!query || icon.name.toLocaleLowerCase().includes(query) ||
       (icon.tags||[]).some(tag=>tag.toLocaleLowerCase().includes(query)));
    const visible=matching.slice(0,visibleIcons);
    summary.textContent='Showing '+visible.length+' of '+matching.length+' matching icons';
    for (const {icon,index} of visible) {
     const tile=make('div','olh-icon-pack-tile');
     const img=make('img');
     img.src='/api/icon-sets/preview?id='+encodeURIComponent(selected.id)+'&index='+index;
     img.alt='';
     img.loading='lazy';
     const caption=make('span','',icon.name);
     caption.title=[icon.name,...(icon.tags||[])].join(' · ');
     tile.append(img,caption);
     tiles.append(tile);
    }
    loadMore.hidden=visible.length>=matching.length;
   };
   search.addEventListener('input',()=>{
    iconQuery=search.value;
    visibleIcons=previewBatch;
    paintIcons();
   });
   loadMore.addEventListener('click',()=>{
    visibleIcons+=previewBatch;
    paintIcons();
   });
   paintIcons();
   root.append(sidebar,details);
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
   working=true;importer.disabled=true;
   try {
    await request('/api/icon-sets',{
     method:'POST',headers:{'Content-Type':'application/zip'},body:zip
    });
    file.value='';
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
