'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const source = fs.readFileSync(path.join(__dirname, 'overview.js'), 'utf8');

test('device readings and profile labels are inserted as text rather than HTML', () => {
  const selectors = [
    'elementTemperatureId', 'elementSpeedId', 'elementGpuSpeedId',
    'elementGpuTemperatureId', 'elementPowerOut', 'elementVolts',
    'elementAmps', 'elementWatts', 'elementRailVoltageId',
  ];
  for (const selector of selectors) {
    assert.doesNotMatch(source, new RegExp('\\$\\(' + selector + '\\)\\.html\\('), selector);
    assert.match(source, new RegExp('\\$\\(' + selector + '\\)\\.text\\('), selector);
  }
  assert.doesNotMatch(source, /\$\("#selected(?:Profile|Rgb)_"\s*\+[^\n]*\.html\(/);
  assert.match(source, /\$\("#selectedProfile_"\s*\+[^\n]*\.text\(/);
  assert.match(source, /\$\("#selectedRgb_"\s*\+[^\n]*\.text\(/);
});

test('keyboard labels preserve a line break without parsing device-provided HTML', () => {
  assert.doesNotMatch(source, /label\.html\(\(key\.KeyName/);
  assert.match(source, /label\.text\(key\.KeyName\s*\|\|\s*""\)/);
  assert.match(source, /document\.createTextNode\(key\.SubKeyName\)/);
});

test('RGB modal treats device channel names as text and appends LED DOM nodes', () => {
  assert.doesNotMatch(source, /\$\{channelName\}/);
  assert.doesNotMatch(source, /\$\{containerHtml\}/);
  assert.match(source, /modal\.find\('\.modal-title'\)\.text\(channelName\)/);
  assert.match(source, /modal\.find\('\.modal-body'\)\.append\(\$\(ledContainer\)\.contents\(\)\)/);
  assert.match(source, /function generateLedDataPerDevice[\s\S]*?return wrapperDiv/);
  assert.doesNotMatch(source, /result\s*=\s*wrapperDiv\.innerHTML/);
});
