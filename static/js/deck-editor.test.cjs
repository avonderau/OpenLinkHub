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
    'deck-key-metric', 'deck-metric-source', 'deck-metric-source-property',
    'deck-metric-interval', 'deck-metric-style',
    'deck-metric-color', 'deck-metric-color-reset', 'deck-metric-status',
    'deck-key-action', 'deck-key-target-page', 'deck-target-property',
    'deck-key-shortcut', 'deck-hotkey-property',
    'deck-macro-property', 'deck-key-macro', 'deck-macro-status', 'deck-refresh-macros',
    'deck-key-media', 'deck-media-property',
    'deck-page-name', 'deck-selected-key', 'deck-save', 'deck-add-page',
    'deck-remove-page', 'deck-message', 'deck-key-count', 'deck-page-count',
    'deck-connection-state', 'deck-tab-key', 'deck-tab-pages',
    'deck-panel-key', 'deck-panel-pages',
  ];
  const nodes = Object.fromEntries(ids.map(id => [id, new FakeElement()]));
  nodes['deck-device-id'].value = 'deck-test';
  nodes['deck-key-grid'].dataset.keys = '18';
  const received = [];
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
    if (type === '/api/macro/') {
      return { ok: true, status: 200, json: async () => ({
        status: 1,
        data: { '7': { id: 7, name: 'Window Manager' }, '12': { id: 12, name: 'Editor Macro' } },
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

  assert.match(html, /<select class="form-select" id="deck-metric-style"/);
  assert.match(html, /class="deck-metric-options">\s*<div class="deck-metric-option">/);
  assert.equal((html.match(/class="deck-metric-option"/g) || []).length, 2,
    'Refresh and Visualization use two aligned columns');
  const css = fs.readFileSync(path.join(__dirname, '..', 'css', 'deck-editor.css'), 'utf8');
  assert.match(css, /\.deck-metric-options\s*\{[^}]*grid-template-columns:\s*minmax\(0,1fr\) minmax\(0,1\.15fr\)/,
    'metric options remain on the same row in the narrow inspector');
  assert.ok(!html.includes("Mini graph"), "metric UI must use visualization names");
  assert.match(html, /id="deck-metric-color" type="color"/);

  // load() fetches data asynchronously and builds the preview.
  await new Promise(resolve => setImmediate(resolve));
  const grid = nodes['deck-key-grid'];
  assert.equal(grid.children.length, 18, 'all LCD keys should be previewed');
  assert.equal(nodes['deck-save'].disabled, false, 'Save & Apply must work even without edits');
  assert.equal(nodes['deck-connection-state'].dataset.connected, 'true');
  assert.equal(nodes['deck-metric-status'].textContent, '', 'empty metric selection has no helper sentence');

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

  // Saved macros are shared with the existing OpenLinkHub macro library.
  grid.children[1].dispatch('click');
  action.value = 'macro';
  action.dispatch('change');
  assert.equal(nodes['deck-macro-property'].hidden, false);
  assert.equal(nodes['deck-media-property'].hidden, true);
  const macros = nodes['deck-key-macro'];
  assert.equal(macros.disabled, false);
  assert.equal(macros.children.length, 2);
  assert.equal(macros.value, '7');
  macros.value = '12';
  macros.dispatch('change');
  await nodes['deck-save'].dispatch('click');
  assert.equal(received[2].pages[1].keys.find(key => key.index === 1).action, 'macro');
  assert.equal(received[2].pages[1].keys.find(key => key.index === 1).target, '12');

  // Switching to media must not reuse the old macro identifier.
  action.value = 'media';
  action.dispatch('change');
  assert.equal(nodes['deck-macro-property'].hidden, true);
  assert.equal(nodes['deck-media-property'].hidden, false);
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
  assert.equal(received[3].pages[1].keys.find(key => key.index === 1).action, 'media');
  assert.equal(received[3].pages[1].keys.find(key => key.index === 1).target, 'volume-mute');

  // Image-only keys must survive the editor's assignment filtering and
  // show their artwork in the preview without interpreting a file as HTML.
  grid.children[3].dispatch('click');
  const image = nodes['deck-key-image'];
  image.files = [{ type: 'image/png', size: 150, name: 'icon.png' }];
  await nodes['deck-upload-image'].dispatch('click');
  assert.equal(nodes['deck-remove-image'].disabled, false);
  assert.equal(grid.children[3].dataset.hasImage, 'true');
  assert.ok(grid.children[3].style.backgroundImage.includes('/api/deck/icon'));
  await nodes['deck-save'].dispatch('click');
  assert.equal(received[4].pages[1].keys.find(key => key.index === 3).icon, 'a'.repeat(32));

  nodes['deck-remove-image'].dispatch('click');
  assert.equal(nodes['deck-remove-image'].disabled, true);
  assert.equal(grid.children[3].dataset.hasImage, undefined);
  await nodes['deck-save'].dispatch('click');
  assert.equal(received[5].pages[1].keys.some(key => key.index === 3), false);
  // Metric-only keys retain configuration, update the browser preview from
  // the device's live metric snapshot, and save refresh + visualization options.
  grid.children[4].dispatch('click');
  const metric = nodes['deck-key-metric'];
  metric.value = 'cpu.temperature';
  metric.dispatch('change');
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
  const metricKey = received[6].pages[1].keys.find(key => key.index === 4);
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
  assert.equal(nodes['deck-metric-status'].textContent, '', 'clearing metric hides generic copy');
  await nodes['deck-save'].dispatch('click');
  assert.equal(received[7].pages[1].keys.some(key => key.index === 4), false);
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
  grid.children[5].dispatch('click');
  action.value = 'macro';
  action.dispatch('change');
  await nodes['deck-save'].dispatch('click');
  const combined = received.at(-1).pages[1].keys;
  assert.equal(combined.find(key => key.index === 1).action, 'hotkey');
  assert.equal(combined.find(key => key.index === 5).action, 'macro');
  assert.equal(combined.find(key => key.index === 5).target, '7');

  console.log('Deck Editor interaction test passed');
}

run().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
