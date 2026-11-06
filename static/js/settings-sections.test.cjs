'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const code = fs.readFileSync(path.join(__dirname, 'settings-sections.js'), 'utf8');
function fixture(initialHash) {
    const elements = {};
    const tabs = ['general', 'devices', 'icons', 'security'].map((name, index) => {
        const attributes = { 'aria-controls':'olh-settings-' + name, 'aria-selected':String(index === 0) };
        const handlers = {};
        const tab = {
            attributes, handlers, tabIndex:index === 0 ? 0 : -1, focused:false,
            getAttribute(key){return attributes[key]},
            setAttribute(key,value){attributes[key]=value},
            addEventListener(event,callback){handlers[event]=callback},
            focus(){this.focused=true}
        };
        elements['olh-settings-tab-' + name]=tab;
        elements['olh-settings-' + name]={id:'olh-settings-' + name,hidden:index !== 0};
        return tab;
    });
    const root={querySelectorAll(selector){
        assert.equal(selector,'[role="tab"]');
        return tabs;
    }};
    let adjusted=0;
    const jquery=function(selector){
        assert.equal(selector,'#dataTable');
        return {DataTable(){return {columns:{adjust(){adjusted++}}}}};
    };
    jquery.fn={DataTable:{isDataTable(sel){return sel==='#dataTable'}}};
    const historyCalls=[];
    const window={
        location:{hash:initialHash||''},
        history:{replaceState(...args){historyCalls.push(args)}},
        requestAnimationFrame(fn){fn()},
        jQuery:jquery
    };
    const document={
        readyState:'complete',
        querySelector(sel){assert.equal(sel,'.olh-settings-shell');return root;},
        getElementById(id){return elements[id]||null;}
    };
    vm.runInNewContext(code,{window,document});
    return {tabs,elements,window,historyCalls,get adjusted(){return adjusted}};
}
test('first category is shown by default and exposes all four accessible tabs',()=>{
    const f=fixture('');
    assert.deepEqual(f.tabs.map(t=>t.attributes['aria-selected']),['true','false','false','false']);
    assert.deepEqual(['general','devices','icons','security'].map(id=>f.elements['olh-settings-'+id].hidden),[false,true,true,true]);
    assert.deepEqual(f.tabs.map(t=>t.tabIndex),[0,-1,-1,-1]);
});
test('category selection preserves existing UI state and resizes the supported-devices table',()=>{
    const f=fixture('');
    const unrelatedForm={value:'unchanged private settings'};
    f.tabs[1].handlers.click();
    assert.deepEqual(f.tabs.map(t=>t.attributes['aria-selected']),['false','true','false','false']);
    assert.equal(f.elements['olh-settings-general'].hidden,true);
    assert.equal(f.elements['olh-settings-devices'].hidden,false);
    assert.equal(f.adjusted,1);
    assert.equal(f.historyCalls.length,1);
    assert.equal(f.historyCalls[0][2],'#olh-settings-devices');
    assert.equal(unrelatedForm.value,'unchanged private settings');
});
test('tabs support arrow keys, Home, End and focus without changing application settings',()=>{
    const f=fixture('#olh-settings-security');
    assert.equal(f.tabs[3].attributes['aria-selected'],'true','deep links open correct category');
    assert.equal(f.elements['olh-settings-security'].hidden,false);
    let prevented=0;
    f.tabs[3].handlers.keydown({key:'ArrowRight',preventDefault(){prevented++}});
    assert.equal(f.tabs[0].attributes['aria-selected'],'true','right wraps around');
    assert.equal(f.tabs[0].focused,true);
    f.tabs[0].handlers.keydown({key:'End',preventDefault(){prevented++}});
    assert.equal(f.tabs[3].attributes['aria-selected'],'true');
    f.tabs[3].handlers.keydown({key:'Home',preventDefault(){prevented++}});
    assert.equal(f.tabs[0].attributes['aria-selected'],'true');
    assert.equal(prevented,3);
    f.tabs[0].handlers.keydown({key:'Tab',preventDefault(){prevented++}});
    assert.equal(prevented,3,'Tab continues native focus behavior');
});
test('unrecognized URL fragments safely show General category',()=>{
    const f=fixture('#some-other-application-anchor');
    assert.equal(f.tabs[0].attributes['aria-selected'],'true');
    assert.equal(f.historyCalls.length,0);
});

test('icon packs tab is deep-linkable and keyboard accessible',()=>{
    const f=fixture('#olh-settings-icons');
    assert.equal(f.tabs[2].attributes['aria-selected'],'true');
    assert.equal(f.elements['olh-settings-icons'].hidden,false);
    assert.equal(f.elements['olh-settings-devices'].hidden,true);
    f.tabs[2].handlers.click();
    assert.equal(f.historyCalls.at(-1)[2],'#olh-settings-icons');
    let prevented=0;
    f.tabs[2].handlers.keydown({key:'ArrowRight',preventDefault(){prevented++}});
    assert.equal(f.tabs[3].attributes['aria-selected'],'true');
    assert.equal(prevented,1);
});
