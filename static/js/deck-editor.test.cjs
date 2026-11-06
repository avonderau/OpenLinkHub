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
  querySelector(selector) {
    return this.children.find(child => selector === '.deck-live-value' && child.className === 'deck-live-value') || null;
  }
  focus() { this.focused = true; }
  get selectedOptions() {
    return this.children.filter(child => child.value === this.value);
  }
}

async function run() {
  const ids = [
    'deck-device-id', 'deck-key-grid', 'deck-page', 'deck-key-label',
    'deck-key-image', 'deck-upload-image', 'deck-remove-image', 'deck-icon-status',
    'deck-icon-set', 'deck-icon-set-item', 'deck-icon-set-search', 'deck-apply-icon-set',
    'deck-key-settings', 'deck-empty-state', 'deck-selected-summary', 'deck-selected-hero', 'deck-artwork-preview',
    'deck-metric-details', 'deck-metric-indicator', 'deck-action-indicator',
    'deck-key-control', 'deck-control-property', 'deck-key-brightness', 'deck-brightness-property',
    'deck-folder-target-property', 'deck-key-folder', 'deck-create-folder', 'deck-page-parent',
    'deck-key-metric', 'deck-metric-source', 'deck-metric-source-property',
    'deck-metric-interval', 'deck-metric-style',
    'deck-metric-color', 'deck-metric-color-reset', 'deck-metric-status',
    'deck-key-action', 'deck-key-target-page', 'deck-target-property',
    'deck-display-only-note',
    'deck-key-shortcut', 'deck-hotkey-property',
    'deck-macro-property', 'deck-key-macro', 'deck-macro-status', 'deck-refresh-macros',
    'deck-plugin-property', 'deck-key-plugin', 'deck-plugin-status', 'deck-refresh-plugins',
    'deck-tab-plugins', 'deck-panel-plugins', 'deck-plugin-manager-refresh', 'deck-plugin-host',
    'deck-plugin-count', 'deck-plugin-file', 'deck-plugin-install', 'deck-plugin-install-status', 'deck-plugin-list',
    'deck-plugin-settings', 'deck-plugin-inspector', 'deck-plugin-inspector-frame', 'deck-plugin-inspector-close',
    'deck-key-media', 'deck-media-property',
    'deck-url-property', 'deck-key-url', 'deck-application-property', 'deck-key-application',
    'deck-page-name', 'deck-selected-key', 'deck-save', 'deck-add-page',
    'deck-remove-page', 'deck-message', 'deck-key-count', 'deck-page-count',
    'deck-connection-state', 'deck-tab-key', 'deck-tab-pages',
    'deck-panel-key', 'deck-panel-pages',
  ];
  const nodes = Object.fromEntries(ids.map(id => [id, new FakeElement()]));
  nodes['deck-device-id'].value = 'deck-test';
  nodes['deck-key-grid'].dataset.keys = '18';
  nodes['deck-key-grid'].dataset.columns = '6';
  nodes['deck-key-grid'].dataset.family = 'mirabox';
  const received = [];
  const pluginCalls = [];
  const intervals = [];
  let cpuReading = 55.6;
  const documentListeners = {};
  const document = {
    getElementById: id => nodes[id],
    createElement: tag => new FakeElement(tag),
    addEventListener: (name, fn) => { documentListeners[name] = fn; },
  };
  const fetch = async (url, options = {}) => {
    const type = url.split('?')[0];
    if (type === '/api/telemetry/sources') {
      return { ok: true, status: 200, json: async () => ({
        sources: {
          'disk.utilization': [{ id: 'mount:/', label: '/' }, { id: 'mount:/home', label: '/home' }],
          'disk.free_gb': [{ id: 'mount:/', label: '/' }],
          'network.rx_rate': [{ id: 'net:eth0', label: 'eth0' }],
          'disk.read_rate': [{ id: 'disk:sda', label: '/dev/sda' }],
        },
      }) };
    }
    if (type === '/api/icon-sets') {
      return { ok: true, status: 200, json: async () => ({ sets: [{
        id: 'c'.repeat(24), name: 'Test icons',
        icons: [{ name: 'Sample arrow', deckId: 'c'.repeat(32), tags: ['navigation'] }]
      }] }) };
    }
    if (type === '/api/macro/') {
      return { ok: true, status: 200, json: async () => ({
        status: 1,
        data: { '7': { id: 7, name: 'Window Manager' }, '12': { id: 12, name: 'Editor Macro' } },
      }) };
    }
    if (type === '/api/deck/plugins') {
      pluginCalls.push({ url, method: options.method || 'GET', body: options.body });
      if (options.method === 'POST') return { ok: true, status: 201, json: async () => ({ plugin: 'com.example.counter' }) };
      if (options.method === 'DELETE') return { ok: true, status: 204, json: async () => null };
      return { ok: true, status: 200, json: async () => ({
        connected: true, canInstall: true, maxUploadBytes: 1024,
        plugins: [{ uuid: 'com.example.counter', name: 'Counter', version: '1.0', author: 'Example',
          actions: [{ uuid: 'com.example.counter.increment', name: 'Increment' }] }],
      }) };
    }
    if (type === '/api/deck/icon' && options.method === 'POST') {
      assert.equal(options.headers['Content-Type'], 'image/png');
      return { ok: true, status: 201, json: async () => ({ id: 'a'.repeat(32) }) };
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
          metrics: { 'cpu.temperature': cpuReading, 'memory.utilization': 48.2 } }
      : { version: 1, activePage: 'home', pages: [{ id: 'home', name: 'Home', keys: null }] };
    return { ok: true, status: 200, json: async () => structuredClone(data) };
  };

  const code = fs.readFileSync(path.join(__dirname, 'deck-editor.js'), 'utf8');
  vm.runInNewContext(code, {
    document, fetch,
    window: { confirm: () => true },
    setInterval: handler => { intervals.push(handler); return intervals.length; },
    console,
  }, { filename: 'deck-editor.js' });

  assert.match(fs.readFileSync(path.join(__dirname,'deck-editor.js'),'utf8'), /metricSource/);
  const iconSetMarkup = fs.readFileSync(path.join(__dirname, '..', '..', 'web', 'deck.html'), 'utf8');
  assert.match(iconSetMarkup, /id="deck-icon-set"/);
  assert.match(iconSetMarkup, /id="deck-icon-set-item"/);
  assert.doesNotMatch(iconSetMarkup, /id="deck-import-icon-set"|id="deck-icon-set-import"/);
  assert.ok(iconSetMarkup.includes('/settings#olh-settings-icons'));
  assert.match(iconSetMarkup, /class="deck-icon-set-library" open/);
  assert.match(iconSetMarkup, /<summary><i class="bi bi-grid-3x3-gap"[^>]*><\/i> Icons<\/summary>/);
  assert.match(iconSetMarkup, /class="deck-artwork-upload-panel"/);
  assert.doesNotMatch(iconSetMarkup, /Reusable icon sets/);
  assert.ok(iconSetMarkup.indexOf('class="deck-icon-set-library" open') < iconSetMarkup.indexOf('id="deck-artwork-preview"'));
  assert.ok(iconSetMarkup.indexOf('id="deck-artwork-preview"') < iconSetMarkup.indexOf('id="deck-icon-set"'),
    'inline selected-icon preview must sit immediately before the Icon Set picker');
  assert.doesNotMatch(iconSetMarkup, /class="deck-artwork-selected"/);
  assert.doesNotMatch(iconSetMarkup, /<label>Artwork<\/label>/);
  assert.ok(iconSetMarkup.indexOf('class="deck-icon-set-library" open') < iconSetMarkup.indexOf('class="deck-artwork-upload-panel"'));
  assert.ok(iconSetMarkup.indexOf('id="deck-remove-image"') < iconSetMarkup.indexOf('class="deck-artwork-upload-panel"'),
    'remove artwork and its selected preview remain accessible even while uploads are collapsed');
  assert.match(fs.readFileSync(path.join(__dirname, 'deck-editor.js'), 'utf8'), /\/api\/icon-sets/);
  // Nonessential helper copy should stay out of the Deck inspector.
  const html = fs.readFileSync(path.join(__dirname, '..', '..', 'web', 'deck.html'), 'utf8');
  for (const hiddenCopy of [
    'Choose a metric to show live sensor data on the LCD.',
    'Changes appear in the preview immediately',
    'Actions trigger when you press the physical Mirabox key',
    'Uses the same validated virtual-keyboard backend',
    'More button actions are coming',
    'Saved macros reuse the existing Nexus macro engine',
  ]) {
    assert.ok(!html.includes(hiddenCopy), `obsolete Deck helper copy: ${hiddenCopy}`);
  }

  // Contextual inspector hides inactive controls until a physical key is selected.
  assert.equal((html.match(/class="deck-inspector-section" aria-labelledby=/g) || []).length, 3);
  for (const heading of ['Appearance', 'Live data', 'Button behavior']) {
    assert.ok(html.includes('>'+heading+'</strong>'), heading + ' card title missing');
  }
  assert.match(html, /id="deck-key-settings" hidden/);
  assert.match(html, /id="deck-empty-state"/);
  assert.match(html, /id="deck-metric-details" hidden/);
  assert.match(html, /id="deck-artwork-preview"/);
  assert.doesNotMatch(html, /Action plugins \(planned\)|aria-label="Action plugins/);
  const sectionCss = fs.readFileSync(path.join(__dirname, '..', 'css', 'deck-editor.css'), 'utf8');
  assert.match(sectionCss, /#deck-metric-details\[hidden\]/);
  assert.match(sectionCss, /\.deck-inspector-section-header/);
  assert.match(sectionCss, /grid-template-columns:minmax\(0,1fr\) minmax\(0,1\.15fr\)/,
    'retain the compact two-column metric controls instead of PR 77 layout');

  assert.match(html, /<select class="form-select" id="deck-metric-style"/);
  assert.match(html, /class="deck-metric-options">\s*<div class="deck-metric-option">/);
  assert.equal((html.match(/class="deck-metric-option"/g) || []).length, 3,
    'Source is separate; Refresh and Visualization remain two aligned columns');
  const css = fs.readFileSync(path.join(__dirname, '..', 'css', 'deck-editor.css'), 'utf8');
  assert.match(css,/\.deck-key\[data-has-image="true"\]\[data-has-caption="true"\]/);
  assert.match(css,/background-size:auto 74%/);
  assert.match(css,/height:26%/);
  assert.match(css, /\.deck-metric-options\s*\{[^}]*grid-template-columns:\s*minmax\(0,1fr\) minmax\(0,1\.15fr\)/,
    'metric options remain on the same row in the narrow inspector');
  assert.ok(!html.includes("Mini graph"), "metric UI must use visualization names");
  assert.match(html, /id="deck-metric-color" type="color"/);

  // load() fetches data asynchronously and builds the preview.
  await new Promise(resolve => setImmediate(resolve));
  const grid = nodes['deck-key-grid'];
  assert.equal(grid.children.length, 18, 'all LCD positions should be previewed');
  assert.match(nodes['deck-key-count'].textContent, /15 buttons · 3 display-only LCDs/,
    'Mirabox preview must not describe three passive displays as buttons');
  assert.equal(nodes['deck-save'].disabled, false, 'Save & Apply must work even without edits');
  assert.equal(nodes['deck-connection-state'].dataset.connected, 'true');
  assert.equal(nodes['deck-metric-status'].textContent, '', 'empty metric selection has no helper sentence');
  assert.equal(nodes['deck-key-settings'].hidden, true, 'no selected key should hide inactive fields');
  assert.equal(nodes['deck-selected-hero'].hidden, true);
  assert.equal(nodes['deck-empty-state'].hidden, false);
  assert.equal(nodes['deck-metric-details'].hidden, true);
  assert.equal(nodes['deck-metric-indicator'].textContent, 'Off');
  assert.equal(nodes['deck-action-indicator'].textContent, 'None');
  assert.equal(nodes['deck-control-property'].hidden, true);
  assert.match(html, /<option value="deck-control">Deck controls<\/option>/);
  assert.doesNotMatch(html, /id="deck-control-heading"/);

  grid.children[0].dispatch('click');
  assert.equal(nodes['deck-key-settings'].hidden, false, 'key selection reveals grouped controls');
  assert.equal(nodes['deck-selected-hero'].hidden, false);
  assert.equal(nodes['deck-empty-state'].hidden, true);
  assert.match(nodes['deck-selected-summary'].textContent, /Unlabeled key/);
  const label = nodes['deck-key-label'];
  assert.equal(label.disabled, false, 'selecting a key enables the inspector');
  label.value = 'test';
  label.dispatch('input');
  assert.equal(grid.children[0].children[1].textContent, 'test', 'live preview should show typed labels');
  assert.match(nodes['deck-selected-summary'].textContent, /test/);
  const action = nodes['deck-key-action'];
  const deckControl = nodes['deck-key-control'];
  action.value = 'deck-control';
  action.dispatch('change');
  assert.equal(action.value, 'deck-control');
  assert.equal(deckControl.value, 'page-next');
  assert.equal(nodes['deck-control-property'].hidden, false,
    'Deck controls appear as a nested menu just like media');
  assert.equal(nodes['deck-action-indicator'].textContent, 'Assigned');
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
  const controls = nodes['deck-key-control'];
  action.value = 'deck-control';
  action.dispatch('change');
  controls.value = 'page-go';
  controls.dispatch('change');
  assert.equal(action.value, 'deck-control', 'Deck controls remain the selected action family');
  assert.equal(nodes['deck-target-property'].hidden, false, 'go-to-page shows destination picker');
  const target = nodes['deck-key-target-page'];
  assert.equal(target.children.length, 2, 'all destinations available');
  target.value = 'home';
  target.dispatch('change');
  await nodes['deck-save'].dispatch('click');
  assert.equal(received[1].pages.length, 2, 'new pages must persist');
  assert.equal(received[1].pages[1].keys[0].action, 'page-go');
  assert.equal(received[1].pages[1].keys[0].targetPage, 'home');

  // Brightness actions live with navigation under Deck controls.
  controls.value = 'brightness-set';
  controls.dispatch('change');
  assert.equal(action.value, 'deck-control');
  assert.equal(nodes['deck-brightness-property'].hidden, false);
  assert.equal(nodes['deck-key-brightness'].value, '50');
  nodes['deck-key-brightness'].value = '75';
  nodes['deck-key-brightness'].dispatch('change');
  await nodes['deck-save'].dispatch('click');
  const brightSaved = received.at(-1).pages[1].keys.find(key => key.index === 0);
  assert.equal(brightSaved.action, 'brightness-set');
  assert.equal(brightSaved.target, '75');
  controls.value = 'brightness-up';
  controls.dispatch('change');
  await nodes['deck-save'].dispatch('click');
  const upSaved = received.at(-1).pages[1].keys.find(key => key.index === 0);
  assert.equal(upSaved.action, 'brightness-up');
  assert.equal(upSaved.target, undefined, 'incremental brightness has no target');
  assert.equal(nodes['deck-brightness-property'].hidden, true);

  // Saved macros are shared with the existing OpenLinkHub macro library.
  grid.children[1].dispatch('click');
  action.value = 'macro';
  action.dispatch('change');
  assert.equal(nodes['deck-control-property'].hidden, true,
    'selecting another button action hides the Deck control submenu');
  assert.equal(nodes['deck-macro-property'].hidden, false);
  assert.equal(nodes['deck-media-property'].hidden, true);
  const macros = nodes['deck-key-macro'];
  assert.equal(macros.disabled, false);
  assert.equal(macros.children.length, 2);
  assert.equal(macros.value, '7');
  macros.value = '12';
  macros.dispatch('change');
  await nodes['deck-save'].dispatch('click');
  assert.equal(received[4].pages[1].keys.find(key => key.index === 1).action, 'macro');
  assert.equal(received[4].pages[1].keys.find(key => key.index === 1).target, '12');

  // Switching to media must not reuse the old macro identifier.
  action.value = 'media';
  action.dispatch('change');
  assert.equal(nodes['deck-macro-property'].hidden, true);
  assert.equal(nodes['deck-media-property'].hidden, false);
  assert.equal(nodes['deck-control-property'].hidden, true,
    'media command and Deck command use the same nested-control pattern');
  const media = nodes['deck-key-media'];
  assert.equal(media.value, 'play-pause');
  for (const command of [
    'play-pause', 'previous-track', 'next-track', 'stop',
    'volume-up', 'volume-down', 'volume-mute',
  ]) {
    media.value = command;
    media.dispatch('change');
    const preview = grid.children[1];
    assert.equal(preview.dataset.hasMediaIcon, 'true', command);
    assert.equal(preview.children[1].className, 'deck-media-glyph', command);
    assert.equal(preview.children[1].style.maskImage,
      `url("/static/media-glyphs/${command}.svg")`, command);
    const expected = {
      'play-pause':'Play/Pause', 'previous-track':'Previous',
      'next-track':'Next Track', stop:'Stop', 'volume-up':'Volume +',
      'volume-down':'Volume -', 'volume-mute':'Mute',
    };
    assert.equal(preview.children[2].textContent, expected[command], command);
  }
  media.value = 'volume-mute';
  media.dispatch('change');
  assert.equal(label.placeholder, 'Mute', 'default should not populate custom label field');
  label.value = 'Silence';
  label.dispatch('input');
  assert.equal(grid.children[1].children[2].textContent, 'Silence', 'custom label overrides default');
  await nodes['deck-save'].dispatch('click');
  assert.equal(received[5].pages[1].keys.find(key => key.index === 1).action, 'media');
  assert.equal(received[5].pages[1].keys.find(key => key.index === 1).target, 'volume-mute');

  // Image-only keys must survive the editor's assignment filtering and
  // show their artwork in the preview without interpreting a file as HTML.
  grid.children[3].dispatch('click');
  const iconPack = nodes['deck-icon-set'];
  const iconChoice = nodes['deck-icon-set-item'];
  iconPack.value = 'c'.repeat(24);
  iconPack.dispatch('change');
  iconChoice.value = 'c'.repeat(32);
  iconChoice.dispatch('change');
  assert.equal(nodes['deck-artwork-preview'].dataset.hasArtwork, 'true',
    'choosing a library icon previews it before applying');
  assert.match(nodes['deck-artwork-preview'].style.backgroundImage, /c{32}/);
  assert.equal(nodes['deck-apply-icon-set'].disabled, false);
  assert.equal(grid.children[3].dataset.hasImage, undefined,
    'preview selection must not alter the key until Use selected icon');
  nodes['deck-apply-icon-set'].dispatch('click');
  assert.match(grid.children[3].style.backgroundImage, /c{32}/,
    'applying the chosen icon updates the device layout preview');
  const image = nodes['deck-key-image'];
  image.files = [{ type: 'image/png', size: 150, name: 'icon.png' }];
  await nodes['deck-upload-image'].dispatch('click');
  assert.equal(nodes['deck-remove-image'].disabled, false);
  assert.equal(nodes['deck-remove-image'].hidden, false);
  assert.equal(grid.children[3].dataset.hasImage, 'true');
  assert.equal(nodes['deck-artwork-preview'].dataset.hasArtwork, 'true');
  assert.match(nodes['deck-artwork-preview'].style.backgroundImage, /\/api\/deck\/icon/);
  assert.ok(grid.children[3].style.backgroundImage.includes('/api/deck/icon'));
  assert.equal(grid.children[3].dataset.hasCaption,undefined,
    'icon-only keys must not display a placeholder label on top of artwork');
  label.value = 'My app';
  label.dispatch('input');
  assert.equal(grid.children[3].dataset.hasCaption,'true',
    'a custom key label creates the same reserved caption region as hardware');
  assert.equal(grid.children[3].children[1].className,'deck-key-caption');
  assert.equal(grid.children[3].children[1].textContent,'My app');
  label.value = '';
  label.dispatch('input');
  assert.equal(grid.children[3].dataset.hasCaption,undefined,
    'removing a label restores full-size icon rendering');
  await nodes['deck-save'].dispatch('click');
  assert.equal(received[6].pages[1].keys.find(key => key.index === 3).icon, 'a'.repeat(32));

  nodes['deck-remove-image'].dispatch('click');
  assert.equal(nodes['deck-remove-image'].disabled, true);
  assert.equal(nodes['deck-remove-image'].hidden, true);
  assert.equal(grid.children[3].dataset.hasImage, undefined);
  assert.equal(nodes['deck-artwork-preview'].dataset.hasArtwork, 'false');
  await nodes['deck-save'].dispatch('click');
  assert.equal(received[7].pages[1].keys.some(key => key.index === 3), false);
  // Metric-only keys retain configuration, update the browser preview from
  // the device's live metric snapshot, and save refresh + visualization options.
  grid.children[4].dispatch('click');
  const metric = nodes['deck-key-metric'];
  metric.value = 'cpu.temperature';
  metric.dispatch('change');
  assert.equal(nodes['deck-metric-details'].hidden, false);
  assert.equal(nodes['deck-metric-indicator'].textContent, 'Active');
  assert.equal(nodes['deck-metric-interval'].disabled, false);
  assert.equal(nodes['deck-metric-style'].disabled, false);
  const picker = nodes['deck-metric-color'];
  const colorReset = nodes['deck-metric-color-reset'];
  assert.equal(picker.disabled, false);
  assert.equal(picker.value, '#d5a65a');
  assert.equal(colorReset.disabled, true);
  picker.value = '#ff0000';
  picker.dispatch('input');
  assert.equal(colorReset.disabled, false);
  assert.equal(grid.children[4].children[2].style.color, '#ff0000');
  const interval = nodes['deck-metric-interval'];
  interval.value = '10';
  interval.dispatch('change');
  const visual = nodes['deck-metric-style'];
  visual.value = 'line';
  visual.dispatch('change');
  assert.equal(grid.children[4].dataset.hasMetric, 'true');
  assert.equal(grid.children[4].children[2].textContent, '56 C');
  const existingMetricKey = grid.children[4];
  cpuReading = 62.4;
  await intervals[0]();
  assert.equal(grid.children[4], existingMetricKey, 'status poll should preserve physical preview DOM');
  assert.equal(grid.children[4].children[2].textContent, '62 C');
  await nodes['deck-save'].dispatch('click');
  const metricKey = received[8].pages[1].keys.find(key => key.index === 4);
  assert.equal(metricKey.metric, 'cpu.temperature');
  assert.equal(metricKey.refreshSeconds, 10);
  assert.equal(metricKey.metricStyle, 'line');
  assert.equal(metricKey.metricColor, '#ff0000');
  assert.equal(metricKey.showGraph, false);
  metric.value = '';
  metric.dispatch('change');
  assert.equal(grid.children[4].dataset.hasMetric, undefined);
  assert.equal(picker.disabled, true);
  assert.equal(colorReset.disabled, true);
  assert.equal(nodes['deck-metric-details'].hidden, true);
  assert.equal(nodes['deck-metric-status'].textContent, '', 'clearing metric hides generic copy');
  await nodes['deck-save'].dispatch('click');
  assert.equal(received[9].pages[1].keys.some(key => key.index === 4), false);
  // Every visualization is saved independently, without replacing the metric.
  metric.value = 'memory.utilization';
  metric.dispatch('change');
  for (const style of ['value', 'line', 'bars', 'fuel', 'ring']) {
    visual.value = style;
    visual.dispatch('change');
    assert.equal(nodes['deck-metric-style'].value, style);
    await nodes['deck-save'].dispatch('click');
    const saved = received.at(-1).pages[1].keys.find(key => key.index === 4);
    assert.equal(saved.metric, 'memory.utilization');
    assert.equal(saved.metricStyle, style);
  }
  picker.value = '#00ff66';
  picker.dispatch('input');
  await nodes['deck-save'].dispatch('click');
  assert.equal(received.at(-1).pages[1].keys.find(key => key.index === 4).metricColor, '#00ff66');
  colorReset.dispatch('click');
  await nodes['deck-save'].dispatch('click');
  assert.equal(received.at(-1).pages[1].keys.find(key => key.index === 4).metricColor, undefined,
    'Reset uses default accent without adding layout state');
  assert.equal(picker.value, '#d5a65a');
  metric.value = '';
  metric.dispatch('change');
  assert.equal(nodes['deck-metric-style'].disabled, true);
  // Standalone shortcuts remain supported alongside saved macros.
  grid.children[1].dispatch('click');
  action.value = 'hotkey';
  action.dispatch('change');
  assert.equal(nodes['deck-hotkey-property'].hidden, false);
  assert.equal(nodes['deck-macro-property'].hidden, true);
  const shortcut = nodes['deck-key-shortcut'];
  shortcut.value = 'Ctrl+Alt+T';
  shortcut.dispatch('input');
  await nodes['deck-save'].dispatch('click');
  const shortcutSaved = received.at(-1).pages[1].keys.find(key => key.index === 1);
  assert.equal(shortcutSaved.action, 'hotkey');
  assert.equal(shortcutSaved.target, 'Ctrl+Alt+T');

  // A different key can still run a shared macro in the same layout.
  grid.children[8].dispatch('click');
  action.value = 'macro';
  action.dispatch('change');
  await nodes['deck-save'].dispatch('click');
  const combined = received.at(-1).pages[1].keys;
  assert.equal(combined.find(key => key.index === 1).action, 'hotkey');
  assert.equal(combined.find(key => key.index === 8).action, 'macro');
  assert.equal(combined.find(key => key.index === 8).target, '7');

  // Website and desktop-entry actions use the same Button behavior picker;
  // selecting either one must expose only its own target field.
  grid.children[6].dispatch('click');
  action.value = 'url';
  action.dispatch('change');
  assert.equal(nodes['deck-url-property'].hidden, false);
  assert.equal(nodes['deck-application-property'].hidden, true);
  assert.equal(nodes['deck-key-url'].disabled, false);
  const url = nodes['deck-key-url'];
  url.value = 'https://example.com/launch?q=1';
  url.dispatch('input');
  await nodes['deck-save'].dispatch('click');
  const website = received.at(-1).pages[1].keys.find(key => key.index === 6);
  assert.equal(website.action, 'url');
  assert.equal(website.target, 'https://example.com/launch?q=1');

  action.value = 'application';
  action.dispatch('change');
  assert.equal(nodes['deck-url-property'].hidden, true);
  assert.equal(nodes['deck-application-property'].hidden, false);
  assert.equal(nodes['deck-key-application'].disabled, false);
  assert.equal(nodes['deck-key-application'].value, '',
    'switching from URL never reuses a URL as a desktop app ID');
  nodes['deck-key-application'].value = 'org.gnome.Terminal.desktop';
  nodes['deck-key-application'].dispatch('input');
  await nodes['deck-save'].dispatch('click');
  const application = received.at(-1).pages[1].keys.find(key => key.index === 6);
  assert.equal(application.action, 'application');
  assert.equal(application.target, 'org.gnome.Terminal.desktop');

  action.value = 'media';
  action.dispatch('change');
  assert.equal(nodes['deck-url-property'].hidden, true);
  assert.equal(nodes['deck-application-property'].hidden, true);
  await nodes['deck-save'].dispatch('click');
  assert.equal(received.at(-1).pages[1].keys.find(key => key.index === 6).target, 'play-pause',
    'switching to media must not leave an app ID as the media target');

  // Build two levels of folders from the selected physical key. The editor
  // should create a Back key on physical button 17 (index 16).
  grid.children[7].dispatch('click');
  action.value = 'deck-control';
  action.dispatch('change');
  controls.value = 'folder-open';
  controls.dispatch('change');
  assert.equal(nodes['deck-folder-target-property'].hidden, false);
  assert.equal(nodes['deck-create-folder'].disabled, false);
  nodes['deck-create-folder'].dispatch('click');
  assert.equal(nodes['deck-page'].value, 'folder-1');
  assert.equal(nodes['deck-page-parent'].textContent, 'Folder within Page 2');
  assert.equal(nodes['deck-page-name'].value, 'Folder 1');
  assert.equal(grid.children[16].dataset.hasFolderIcon, 'true');
  assert.equal(grid.children[16].children[1].style.maskImage,
    'url("/static/media-glyphs/folder-back.svg?v=2")');
  await nodes['deck-save'].dispatch('click');
  const firstFolderLayout = received.at(-1);
  const firstFolder = firstFolderLayout.pages.find(page => page.id === 'folder-1');
  assert.equal(firstFolder.parentPage, 'page-2');
  assert.equal(firstFolder.keys[0].index, 16);
  assert.equal(firstFolder.keys[0].action, 'folder-back');
  assert.equal(firstFolderLayout.pages.find(page => page.id === 'page-2').keys
    .find(key => key.index === 7).targetPage, 'folder-1');

  nodes['deck-page-name'].value = 'Games';
  nodes['deck-page-name'].dispatch('input');
  grid.children[0].dispatch('click');
  action.value = 'deck-control';
  action.dispatch('change');
  controls.value = 'folder-open';
  controls.dispatch('change');
  nodes['deck-create-folder'].dispatch('click');
  assert.equal(nodes['deck-page'].value, 'folder-2');
  assert.equal(nodes['deck-page-parent'].textContent, 'Folder within Games');
  await nodes['deck-save'].dispatch('click');
  const nestedLayout = received.at(-1);
  assert.equal(nestedLayout.pages.find(page => page.id === 'folder-2').parentPage, 'folder-1');
  assert.equal(nestedLayout.pages.find(page => page.id === 'folder-1').keys
    .find(key => key.index === 0).targetPage, 'folder-2');
  assert.equal(nestedLayout.pages.find(page => page.id === 'folder-2').keys
    .find(key => key.index === 16).action, 'folder-back');

  // The parent key preview must show the folder glyph and the updated
  // destination name, just like the device's default key caption.
  nodes['deck-page'].value = 'page-2';
  nodes['deck-page'].dispatch('change');
  assert.equal(grid.children[7].dataset.hasFolderIcon, 'true');
  assert.equal(grid.children[7].children[1].style.maskImage,
    'url("/static/media-glyphs/folder-open.svg?v=2")');
  assert.equal(grid.children[7].children[2].textContent, 'Games');
  nodes['deck-page'].value = 'folder-1';
  nodes['deck-page'].dispatch('change');

  // Removing a folder with children should recursively clear the folder pages
  // and the parent key link, without removing the surrounding regular page.
  nodes['deck-page'].value = 'folder-1';
  nodes['deck-page'].dispatch('change');
  nodes['deck-remove-page'].dispatch('click');
  assert.equal(nodes['deck-page'].value, 'page-2');
  await nodes['deck-save'].dispatch('click');
  const cleaned = received.at(-1);
  assert.equal(cleaned.pages.some(page => page.id === 'folder-1' || page.id === 'folder-2'), false);
  assert.equal(cleaned.pages.find(page => page.id === 'page-2').keys.some(key => key.index === 7), false);

  // On an ordinary top-level page, Back must not be configured by mistake.
  grid.children[7].dispatch('click');
  action.value = 'deck-control';
  action.dispatch('change');
  controls.value = 'folder-back';
  controls.dispatch('change');
  assert.match(nodes['deck-message'].textContent, /only on a folder page/);
  assert.notEqual(controls.value, 'folder-back');

  // Mirabox positions 6, 12 and 18 are passive LCDs, not physical keys.
  // They remain selectable for artwork, labels and live metrics but must
  // never accept a physical button action.
  for (const index of [5, 11, 17]) {
    assert.equal(grid.children[index].dataset.displayOnly, 'true',
      `position ${index + 1} must be identified as display-only`);
    grid.children[index].dispatch('click');
    assert.equal(nodes['deck-key-action'].disabled, true);
    assert.equal(nodes['deck-display-only-note'].hidden, false);
    assert.equal(nodes['deck-key-label'].disabled, false);
    assert.equal(nodes['deck-key-metric'].disabled, false);
    action.value = 'deck-control';
    action.dispatch('change');
    assert.equal(action.value, '', 'passive LCD cannot configure an action');
  }
  assert.notEqual(grid.children[16].dataset.displayOnly, 'true',
    'physical button 17 must still support actions');

  // A passive cell can save text and a metric without an action.
  grid.children[17].dispatch('click');
  label.value = 'System';
  label.dispatch('input');
  metric.value = 'cpu.temperature';
  metric.dispatch('change');
  await nodes['deck-save'].dispatch('click');
  const passive = received.at(-1).pages.find(page => page.id === 'page-2')
    .keys.find(key => key.index === 17);
  assert.equal(passive.label, 'System');
  assert.equal(passive.metric, 'cpu.temperature');
  assert.equal(passive.action, undefined);

  // Plugins tab: list, install and remove.
  nodes['deck-tab-plugins'].dispatch('click');
  assert.equal(nodes['deck-panel-plugins'].hidden, false);
  assert.equal(nodes['deck-panel-key'].hidden, true);
  assert.equal(nodes['deck-plugin-host'].dataset.connected, 'true');
  assert.equal(nodes['deck-plugin-count'].textContent, '1 plugin installed');
  const listed = nodes['deck-plugin-list'].children;
  assert.equal(listed.length, 1);
  assert.equal(listed[0].children[0].children[0].textContent, 'Counter 1.0');
  const fileInput = nodes['deck-plugin-file'];
  assert.equal(fileInput.disabled, false);
  assert.equal(nodes['deck-plugin-install'].disabled, true, 'install needs a chosen file');
  fileInput.files = [{ name: 'big.streamDeckPlugin', size: 4096 }];
  fileInput.dispatch('change');
  await nodes['deck-plugin-install'].dispatch('click');
  assert.match(nodes['deck-plugin-install-status'].textContent, /too large/);
  const upload = { name: 'counter.streamDeckPlugin', size: 10 };
  fileInput.files = [upload];
  fileInput.dispatch('change');
  assert.equal(nodes['deck-plugin-install'].disabled, false);
  await nodes['deck-plugin-install'].dispatch('click');
  const post = pluginCalls.find(call => call.method === 'POST');
  assert.ok(post && post.body === upload, 'the chosen file is uploaded as the request body');
  assert.match(nodes['deck-plugin-install-status'].textContent, /Installed com\.example\.counter/);
  const removeButton = nodes['deck-plugin-list'].children[0].children[1];
  await removeButton.dispatch('click');
  const del = pluginCalls.find(call => call.method === 'DELETE');
  assert.ok(del && del.url.endsWith('?plugin=com.example.counter'));
  assert.match(nodes['deck-plugin-install-status'].textContent, /Removed Counter/);

  console.log('Deck Editor interaction test passed');
}

run().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
