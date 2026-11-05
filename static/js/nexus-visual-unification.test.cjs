'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const repo = path.join(__dirname, '..', '..');
const workspace = fs.readFileSync(path.join(repo,'web','nexus-custom-workspace.html'),'utf8');
const page = fs.readFileSync(path.join(repo,'web','nexus.html'),'utf8');
const css = fs.readFileSync(path.join(repo,'static','css','nexus-editor-unified.css'),'utf8');
const shared = fs.readFileSync(path.join(repo,'static','css','openlinkhub-nexus-design.css'),'utf8');
const deck = fs.readFileSync(path.join(repo,'static','css','deck-editor.css'),'utf8');
const standalone = fs.readFileSync(path.join(repo,'static','nexus-editor.html'),'utf8');
const standaloneCss = fs.readFileSync(path.join(repo,'static','css','nexus-standalone-theme.css'),'utf8');

test('Nexus editor loads additive visual layer after original CSS and inspector, without losing controllers',()=>{
 const base=page.indexOf('/static/css/nexus-editor.css');
 const inspector=page.indexOf('/static/css/nexus-compact-inspector.css');
 const unified=page.indexOf('/static/css/nexus-editor-unified.css');
 assert.ok(base>0 && inspector>base && unified>inspector);
 for(const id of ['save','screens','newScreenPreset','addScreen','replaceScreenWithPreset',
  'renameScreen','duplicateScreen','moveScreenUp','moveScreenDown','deleteScreen',
  'preview','screen','message','nexus-keyboard-hint','nexus-panel-widget',
  'nexus-panel-background','nexus-panel-layout','nexus-panel-library',
  'nexus-panel-applications','nexus-selected-panel','properties','addMetric','addContent','addButton']){
  assert.equal((workspace.match(new RegExp('id="'+id+'"','g'))||[]).length,1,'must preserve '+id);
 }
});

test('Nexus editor and Deck Editor use shared gradient, icon and panel primitives',()=>{
 for(const token of ['--olh-header-gradient','--olh-header-line','--olh-panel-gradient','--olh-icon-bg','--olh-icon-ink']){
  assert.ok(shared.includes(token),'missing shared design token '+token);
 }
 for(const token of ['var(--olh-header-gradient','var(--olh-icon-bg','var(--olh-panel-gradient']){
  assert.ok(css.includes(token),'Nexus missing theme '+token);
  assert.ok(deck.includes(token),'Deck missing theme '+token);
 }
 for(const className of ['olh-page-intro nexus-editor-intro','olh-section-header nexus-editor-toolbar-header',
  'nexus-preview-stage','nexus-preview-chrome']){
  assert.ok(workspace.includes(className),'visual element missing '+className);
 }
 assert.match(css,/grid-template-columns:minmax\(240px,285px\) minmax\(0,1fr\)/);
 assert.match(css,/@media\(max-width:1130px\)/);
 assert.match(css,/@media\(max-width:700px\)/);
 assert.match(css,/prefers-reduced-motion/);
});

test('five existing tab controls retain file-style layout, icons and accessibility',()=>{
 const tabs=['widget','library','background','layout','applications'];
 for(const name of tabs){
  const start=workspace.indexOf('id="nexus-tab-'+name+'"');
  assert.ok(start>=0,'missing '+name+' tab');
  const markup=workspace.slice(start,workspace.indexOf('</button>',start));
  assert.match(markup,/role="tab"/);
  assert.match(markup,new RegExp('aria-controls="nexus-panel-'+name+'"'));
  assert.match(markup,/bi bi-/);
 }
 assert.equal((workspace.match(/role="tab" aria-controls="nexus-panel-/g)||[]).length,5);
 assert.match(css, /\.nexus-editor-tablist \[role=tab\]\[aria-selected=true\]/);
 assert.match(css, /border-top:2px solid var\(--olh-accent/);
});

test('device settings and main touchscreen preview remain operational',()=>{
 assert.match(workspace,/id="save" type="button"/);
 assert.match(workspace,/id="cyclingMode"/);
 assert.match(workspace,/id="cyclingSeconds"/);
 assert.match(workspace,/id="displayBrightness"/);
 assert.match(workspace,/id="screen" tabindex="0" role="group"/);
 assert.match(workspace,/id="nexus-keyboard-hint"/);
 assert.doesNotMatch(css, /#nexus-editor\s+#screen\s*\{[^}]*width\s*:/);
 assert.doesNotMatch(css, /#nexus-editor\s+#screen\s*\{[^}]*height\s*:/);
});

test('standalone editor uses matching chrome and preserves existing IDs',()=>{
 assert.match(standalone, /class="nexus-standalone"/);
 assert.match(standalone, /nexus-standalone-theme\.css\?v=1/);
 assert.match(standalone, /bootstrap-icons\.min\.css/);
 assert.match(standalone, /nexus-preview-stage/);
 for(const id of ['device','save','load','screens','screen','preview','properties',
   'addScreen','addMetric','addContent','addButton','nexus-tab-widget','nexus-tab-layout']){
   assert.equal((standalone.match(new RegExp('id="'+id+'"','g'))||[]).length,1,'standalone missing '+id);
 }
 for(const token of ['#d5a65a','linear-gradient','nexus-editor-tablist','nexus-preview-stage']){
   assert.ok(standaloneCss.includes(token),'standalone design token missing: '+token);
 }
});
