'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

class FakeElement {
  constructor(tag = 'div') {
    this.tagName = tag.toUpperCase();
    this.children = [];
    this.dataset = {};
    this.listeners = {};
    this.attrs = {};
    this.style = {};
    this.value = '';
    this.textContent = '';
    this.disabled = false;
    this.hidden = false;
    this.tabIndex = 0;
  }
  addEventListener(type, listener) { this.listeners[type] = listener; }
  dispatch(type, event = {}) {
    assert.ok(this.listeners[type], `missing listener: ${type}`);
    return this.listeners[type](event);
  }
  setAttribute(name, value) { this.attrs[name] = String(value); }
  getAttribute(name) { return this.attrs[name]; }
  append(...elements) { this.children.push(...elements); }
  appendChild(element) { this.children.push(element); }
  replaceChildren(...elements) { this.children = elements; }
  focus() { this.focused = true; }
  get selectedOptions() {
    return this.children.filter(child => child.value === this.value);
  }
}

async function run() {
  const ids = [
    'deck-device-id', 'deck-key-grid', 'deck-page', 'deck-key-label',
    'deck-page-name', 'deck-selected-key', 'deck-save', 'deck-add-page',
    'deck-remove-page', 'deck-message', 'deck-key-count', 'deck-page-count',
    'deck-connection-state', 'deck-tab-key', 'deck-tab-pages',
    'deck-panel-key', 'deck-panel-pages',
  ];
  const nodes = Object.fromEntries(ids.map(id => [id, new FakeElement()]));
  nodes['deck-device-id'].value = 'deck-test';
  nodes['deck-key-grid'].dataset.keys = '18';
  const received = [];
  const documentListeners = {};
  const document = {
    getElementById: id => nodes[id],
    createElement: tag => new FakeElement(tag),
    addEventListener: (name, fn) => { documentListeners[name] = fn; },
  };
  const fetch = async (url, options = {}) => {
    const type = url.split('?')[0];
    if (type === '/api/deck/layout' && options.method === 'PUT') {
      received.push(JSON.parse(options.body));
      return {
        ok: true, status: 200,
        json: async () => ({
          saved: true, applied: true,
          message: 'Saved and applied to Mirabox display.',
        }),
      };
    }
    const data = type === '/api/deck/status'
      ? { connected: true, status: 'Connected; layout applied to display' }
      : { version: 1, activePage: 'home', pages: [{ id: 'home', name: 'Home', keys: null }] };
    return { ok: true, status: 200, json: async () => structuredClone(data) };
  };

  const code = fs.readFileSync(path.join(__dirname, 'deck-editor.js'), 'utf8');
  vm.runInNewContext(code, {
    document, fetch,
    window: { confirm: () => true },
    setInterval: () => 1,
    console,
  }, { filename: 'deck-editor.js' });

  // load() fetches data asynchronously and builds the preview.
  await new Promise(resolve => setImmediate(resolve));
  const grid = nodes['deck-key-grid'];
  assert.equal(grid.children.length, 18, 'all LCD keys should be previewed');
  assert.equal(nodes['deck-save'].disabled, false, 'Save & Apply must work even without edits');
  assert.equal(nodes['deck-connection-state'].dataset.connected, 'true');

  grid.children[0].dispatch('click');
  const label = nodes['deck-key-label'];
  assert.equal(label.disabled, false, 'selecting a key enables the inspector');
  label.value = 'test';
  label.dispatch('input');
  assert.equal(grid.children[0].children[1].textContent, 'test', 'live preview should show typed labels');
  assert.match(nodes['deck-message'].textContent, /Unsaved changes/);
  await nodes['deck-save'].dispatch('click');
  assert.equal(received.length, 1, 'saving sends a PUT request');
  assert.equal(received[0].pages[0].keys[0].label, 'test');
  assert.match(nodes['deck-message'].textContent, /applied to Mirabox/);
  assert.equal(nodes['deck-save'].disabled, false);

  nodes['deck-tab-pages'].dispatch('click');
  assert.equal(nodes['deck-panel-key'].hidden, true);
  assert.equal(nodes['deck-panel-pages'].hidden, false);
  nodes['deck-add-page'].dispatch('click');
  assert.equal(nodes['deck-page'].value, 'page-2');
  assert.equal(nodes['deck-key-grid'].children.length, 18);
  await nodes['deck-save'].dispatch('click');
  assert.equal(received[1].pages.length, 2, 'new pages must persist');
  console.log('Deck Editor interaction test passed');
}

run().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
