/* OpenLinkHub Nexus fork visual shell.
 * Progressive enhancement only: use existing page/sidebar DOM and routes.
 * No changes to upstream page templates or application APIs are required.
 */
(function(){
 'use strict';
 function setup(){
  if(document.querySelector('.olh-app-topbar'))return;
  const main=document.querySelector('main.main-content');
  if(!main)return;
  const sidebar=document.querySelector('aside.sidebar');
  const active=sidebar?.querySelector('.nav-link.active');
  const page=main.querySelector('#nexus-editor')?'Editor':(active?.textContent||document.title.split('|')[0]||'Dashboard').trim();
  const header=document.createElement('header');
  header.className='olh-app-topbar';
  const identity=document.createElement('div');identity.className='olh-app-topbar-title';
  const mark=document.createElement('span');mark.className='olh-app-topbar-icon';mark.setAttribute('aria-hidden','true');
  const logo=document.createElement('i');logo.className='bi bi-link-45deg';mark.append(logo);
  const brand=document.createElement('span');brand.className='olh-app-topbar-name';brand.textContent='OpenLinkHub ';
  const suffix=document.createElement('strong');suffix.className='olh-app-topbar-suffix';suffix.textContent='Nexus';brand.append(suffix);
  const divider=document.createElement('span');divider.className='olh-app-topbar-divider';divider.setAttribute('aria-hidden','true');
  const pageName=document.createElement('span');pageName.className='olh-app-topbar-page';pageName.textContent=page;
  identity.append(mark,brand,divider,pageName);
  const actions=document.createElement('nav');actions.className='olh-app-topbar-actions';actions.setAttribute('aria-label','Quick navigation');
  const github=sidebar?.querySelector('a[href*="github.com/"]');
  if(github){
   const link=document.createElement('a');
   link.href=github.href;link.target='_blank';link.rel='noopener noreferrer';link.textContent='GitHub';
   const icon=document.createElement('i');icon.className='bi bi-github';icon.setAttribute('aria-hidden','true');link.prepend(icon);
   actions.append(link);
  }
  const settings=sidebar?.querySelector('a[href="/settings"]');
  if(settings){
   const link=document.createElement('a');link.href=settings.href;link.textContent='Settings';
   const icon=document.createElement('i');icon.className='bi bi-gear';icon.setAttribute('aria-hidden','true');link.prepend(icon);
   actions.append(link);
  }
  header.append(identity,actions);
  document.body.prepend(header);
 }
 if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',setup,{once:true});
 else setup();
})();
