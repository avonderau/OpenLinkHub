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
    'deck-key-action', 'deck-key-target-page', 'deck-target-property',
    'deck-key-shortcut', 'deck-hotkey-property', 'deck-key-media', 'deck-media-property',
    'deck-launch-property', 'deck-launch-label', 'deck-key-launch-target', 'deck-launch-help', 'deck-launcher-status',
    'deck-macro-property', 'deck-key-macro', 'deck-macro-status', 'deck-refresh-macros',
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
    if (type === '/api/macro/') {
      return { ok: true, status: 200, json: async () => ({
        status: 1, data: { '7': { id: 7, name: 'Window Manager' }, '12': { id: 12, name: 'Editor Macro' } }
      }) };
    }
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
      ? { connected: true, status: 'Connected; layout applied to display',
          desktopLauncherReady: false, desktopLauncherStatus: 'Desktop agent disconnected' }
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
  assert.equal(nodes['deck-launcher-status'].dataset.ready, 'false');
  assert.match(nodes['deck-launcher-status'].textContent, /Desktop agent disconnected/);

  grid.children[0].dispatch('click');
  const label = nodes['deck-key-label'];
  assert.equal(label.disabled, false, 'selecting a key enables the inspector');
  label.value = 'test';
  label.dispatch('input');
  assert.equal(grid.children[0].children[1].textContent, 'test', 'live preview should show typed labels');
  const action = nodes['deck-key-action'];
  action.value = 'page-next';
  action.dispatch('change');
  assert.equal(nodes['deck-target-property'].hidden, true, 'next-page action does not need a destination');
  assert.match(nodes['deck-message'].textContent, /Unsaved changes/);
  await nodes['deck-save'].dispatch('click');
  assert.equal(received.length, 1, 'saving sends a PUT request');
  assert.equal(received[0].pages[0].keys[0].label, 'test');
  assert.equal(received[0].pages[0].keys[0].action, 'page-next', 'physical action must be saved with label');
  assert.match(nodes['deck-message'].textContent, /applied to Mirabox/);
  assert.equal(nodes['deck-save'].disabled, false);

  nodes['deck-tab-pages'].dispatch('click');
  assert.equal(nodes['deck-panel-key'].hidden, true);
  assert.equal(nodes['deck-panel-pages'].hidden, false);
  nodes['deck-add-page'].dispatch('click');
  assert.equal(nodes['deck-page'].value, 'page-2');
  assert.equal(nodes['deck-key-grid'].children.length, 18);
  grid.children[0].dispatch('click');
  action.value = 'page-go';
  action.dispatch('change');
  assert.equal(nodes['deck-target-property'].hidden, false, 'go-to-page shows destination picker');
  const target = nodes['deck-key-target-page'];
  assert.equal(target.children.length, 2, 'all destinations available');
  target.value = 'home';
  target.dispatch('change');
  await nodes['deck-save'].dispatch('click');
  assert.equal(received[1].pages.length, 2, 'new pages must persist');
  assert.equal(received[1].pages[1].keys[0].action, 'page-go');
  assert.equal(received[1].pages[1].keys[0].targetPage, 'home');

  // A keyboard shortcut uses a validated text target, never a command line.
  grid.children[1].dispatch('click');
  action.value = 'hotkey';
  action.dispatch('change');
  assert.equal(nodes['deck-hotkey-property'].hidden, false);
  assert.equal(nodes['deck-media-property'].hidden, true);
  const shortcut = nodes['deck-key-shortcut'];
  shortcut.value = 'Ctrl+Alt+T';
  shortcut.dispatch('input');
  await nodes['deck-save'].dispatch('click');
  assert.equal(received[2].pages[1].keys.find(key => key.index === 1).action, 'hotkey');
  assert.equal(received[2].pages[1].keys.find(key => key.index === 1).target, 'Ctrl+Alt+T');

  // Switching from a shortcut to media must not leave a stale hotkey target.
  action.value = 'media';
  action.dispatch('change');
  assert.equal(nodes['deck-hotkey-property'].hidden, true);
  assert.equal(nodes['deck-media-property'].hidden, false);
  const media = nodes['deck-key-media'];
  assert.equal(media.value, 'play-pause');
  media.value = 'volume-mute';
  media.dispatch('change');
  await nodes['deck-save'].dispatch('click');
  assert.equal(received[3].pages[1].keys.find(key => key.index === 1).action, 'media');
  assert.equal(received[3].pages[1].keys.find(key => key.index === 1).target, 'volume-mute');

  // Desktop launcher is a URL or safe desktop ID, never an arbitrary shell.
  grid.children[2].dispatch('click');
  action.value = 'url';
  action.dispatch('change');
  assert.equal(nodes['deck-launch-property'].hidden, false);
  assert.equal(nodes['deck-key-launch-target'].maxLength, 256);
  const launch = nodes['deck-key-launch-target'];
  launch.value = 'https://example.com/test';
  launch.dispatch('input');
  await nodes['deck-save'].dispatch('click');
  assert.equal(received[4].pages[1].keys.find(key => key.index === 2).action, 'url');
  assert.equal(received[4].pages[1].keys.find(key => key.index === 2).target, 'https://example.com/test');

  action.value = 'application';
  action.dispatch('change');
  assert.equal(launch.value, '', 'changing action type must discard stale URL');
  assert.equal(launch.maxLength, 128);
  launch.value = 'org.gnome.Terminal.desktop';
  launch.dispatch('input');
  await nodes['deck-save'].dispatch('click');
  assert.equal(received[5].pages[1].keys.find(key => key.index === 2).action, 'application');
  assert.equal(received[5].pages[1].keys.find(key => key.index === 2).target, 'org.gnome.Terminal.desktop');

  // Saved macro options come from existing OpenLinkHub /api/macro/ profiles.
  grid.children[3].dispatch('click');
  action.value = 'macro';
  action.dispatch('change');
  assert.equal(nodes['deck-macro-property'].hidden, false);
  const macros = nodes['deck-key-macro'];
  assert.equal(macros.disabled, false);
  assert.equal(macros.children.length, 2);
  assert.equal(macros.value, '7');
  macros.value = '12';
  macros.dispatch('change');
  await nodes['deck-save'].dispatch('click');
  assert.equal(received[6].pages[1].keys.find(key => key.index === 3).action, 'macro');
  assert.equal(received[6].pages[1].keys.find(key => key.index === 3).target, '12');

  console.log('Deck Editor interaction test passed');
}

run().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
