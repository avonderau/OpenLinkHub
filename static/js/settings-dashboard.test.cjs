'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const root = path.join(__dirname, '..', '..');
const script = fs.readFileSync(path.join(__dirname, 'settings.js'), 'utf8');
const template = fs.readFileSync(path.join(root, 'web/settings.html'), 'utf8');
const server = fs.readFileSync(path.join(root, 'src/server/requests/requests.go'), 'utf8');

const oldVisibility = ['showCpu', 'showGpu', 'showDisk', 'showDevices', 'showBattery'];
const oldCheckBoxes = ['checkbox-cpu', 'checkbox-gpu', 'checkbox-storage', 'checkbox-devices', 'checkbox-battery'];
const supported = ['showLabels', 'celsius', 'temperatureBar', 'addDeviceToDashboard', 'rgbOff', 'languageCode', 'theme', 'keyboardLayout'];

test('removed upstream dashboard checkboxes have no stale JS references or synthetic form values', () => {
    for (const id of oldCheckBoxes) {
        assert.doesNotMatch(template, new RegExp('id="' + id + '"'));
        assert.doesNotMatch(script, new RegExp('#' + id + '(?![\\w-])'));
    }
    for (const field of oldVisibility) {
        assert.doesNotMatch(script, new RegExp('response\\.dashboard\\.' + field + '(?![\\w])'));
        assert.doesNotMatch(script, new RegExp('pf\\["' + field + '"\\]'));
    }
});

test('saving the visible Settings controls sends only supported fields and values', () => {
    const start = script.indexOf("    const checkboxDeviceLabels = $('#checkbox-deviceLabels');");
    const end = script.indexOf('    loadDashboardSettings();', start);
    assert.ok(start > 0 && end > start, 'dashboard Settings handler anchors exist');
    const extracted = script.slice(start, end);

    const values = {
        '#checkbox-deviceLabels': false,
        '#checkbox-celsius': true,
        '#checkbox-temperatureBar': true,
        '#checkbox-addDeviceToDashboard': false,
        '#checkbox-rgbOff': true,
        '#userLanguage': 'en_US',
        '#theme': 'default',
        '#keyboardLayout': '1',
    };
    const handlers = new Map();
    const form = [];
    const $ = selector => {
        assert.ok(selector in values || selector === '#btnSaveDashboardSettings' || selector === '#btnSaveAuthSettings',
            'unexpected reference to absent setting: ' + selector);
        return {
            attr(key, value) {
                if (key === 'Checked' && value === 'Checked') values[selector] = true;
                return this;
            },
            is(test) {
                assert.equal(test, ':checked');
                return !!values[selector];
            },
            val() { return values[selector]; },
            on(name, handler) { handlers.set(selector + ':' + name, handler); return this; },
        };
    };
    $.ajax = options => {
        if (options.url === '/api/dashboard') {
            options.success({status: 1, dashboard: {
                showCpu: true, showGpu: true, showDisk: true, showDevices: true, showBattery: true,
                showLabels: false, celsius: true, temperatureBar: true,
                addDeviceToDashboard: false, rgbOff: true,
            }});
            return;
        }
        if (options.url === '/api/dashboard/update') {
            assert.equal(options.type, 'POST');
            form.push(JSON.parse(options.data));
            return;
        }
        assert.fail('unexpected endpoint: ' + options.url);
    };

    vm.runInNewContext('(() => {\n' + extracted + '\nloadDashboardSettings();\n})()', {$, toast:{warning(){}}, location:{reload(){}}});
    assert.ok(handlers.has('#btnSaveDashboardSettings:click'));
    handlers.get('#btnSaveDashboardSettings:click')();
    assert.equal(form.length, 1);
    const payload = form[0];
    assert.deepEqual(Object.keys(payload).sort(), supported.slice().sort());
    assert.equal(payload.showLabels, false);
    assert.equal(payload.celsius, true);
    assert.equal(payload.temperatureBar, true);
    assert.equal(payload.addDeviceToDashboard, false);
    assert.equal(payload.rgbOff, true);
    assert.equal(payload.languageCode, 'en_US');
    assert.equal(payload.theme, 'default');
    assert.equal(payload.keyboardLayout, 1);
    for(const key of oldVisibility) assert.equal(key in payload, false, 'stale field must not be sent: ' + key);
});

test('backend retains legacy visibility flags while processing the current settings payload', () => {
    const begin = server.indexOf('func ProcessDashboardSettingsChange(');
    const end = server.indexOf('func ProcessAudioSettingsChange(', begin);
    assert.ok(begin > 0 && end > begin);
    const handler = server.slice(begin, end);
    assert.match(handler, /dash := dashboard\.GetDashboard\(\)/);
    for (const name of ['Celsius', 'TemperatureBar', 'AddDeviceToDashboard', 'LanguageCode', 'ShowLabels', 'Theme', 'KeyboardLayout', 'RgbOff']) {
        assert.match(handler, new RegExp('dash\\.' + name + ' = req\\.' + name));
    }
    for (const name of ['ShowCpu','ShowGpu','ShowDisk','ShowDevices','ShowBattery']) {
        assert.doesNotMatch(handler, new RegExp('dash\\.' + name + '\\s*='));
    }
    assert.match(handler, /dashboard\.SaveDashboardSettings\(dash, true\)/);
});
