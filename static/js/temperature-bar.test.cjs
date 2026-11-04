'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const root = path.join(__dirname, '..', '..');
const source = fs.readFileSync(path.join(__dirname, 'temperature-bar.js'), 'utf8');

test('telemetry bar preserves translated CPU, GPU, and storage identifiers', () => {
    const tpl = fs.readFileSync(path.join(root, 'web/temperature-bar.html'), 'utf8');
    const head = fs.readFileSync(path.join(root, 'web/head.html'), 'utf8');
    const css = fs.readFileSync(path.join(root, 'static/css/openlinkhub-telemetry.css'), 'utf8');
    const login = fs.readFileSync(path.join(root, 'web/login.html'), 'utf8');

    assert.match(tpl, /\{\{ define "temperature-bar" \}\}/);
    assert.match(tpl, /id="cpu_temp"/);
    assert.match(tpl, /id="gpu_temp_\{\{ \.Index \}\}"/);
    assert.match(tpl, /id="storage_temp-\{\{ \.Key \}\}"/);
    for (const key of ['txtCpuTemperature', 'txtGpuTemperature', 'txtStorageTemperature']) {
        assert.ok(tpl.includes('Lang "' + key + '"'), 'missing localized label: ' + key);
    }
    assert.match(tpl, /range \.SystemInfo\.GPU/);
    assert.match(tpl, /range \.SystemInfo\.Storage/);
    assert.match(tpl, /class="olh-telemetry-bar mb-4"/);
    assert.match(tpl, /class="olh-telemetry-segment/);
    assert.doesNotMatch(tpl, /col-md-2|class="card p-3"/, 'old individual temperature cards removed');
    assert.equal((head.match(/openlinkhub-telemetry\.css/g) || []).length, 1);
    assert.match(head, /<title>OpenLinkHub Nexus<\/title>/);
    assert.match(login, /<h1>OpenLinkHub Nexus<\/h1>/);
    assert.match(css, /grid-template-columns:repeat\(auto-fit,/);
    assert.match(css, /@container\(max-width:800px\)/);
    assert.match(css, /@container\(max-width:460px\)/);
    assert.match(css, /--olh-header-line/);
});

test('temperature poll updates all original IDs safely and keeps Fahrenheit support', () => {
    const byId = new Map();
    function element(id, value) {
        const item = {
            id,
            children: [],
            attributes: {},
            textContent: value,
            replaceChildren(...children) {
                this.children = children;
                this.textContent = children.map(child => child.textContent).join('');
            },
            setAttribute(key, value) {
                this.attributes[key] = value;
            }
        };
        byId.set(id, item);
        return item;
    }

    const cpu = element('cpu_temp', '44.6 °C');
    const gpu = element('gpu_temp_0', '58.0 °C');
    const storage = element('storage_temp-ssdA', '39.0 °C');
    const document = {
        getElementById: id => byId.get(id),
        createElement: () => ({ className: '', textContent: '' }),
        querySelectorAll: selector => {
            assert.equal(selector, '.olh-telemetry-temperature');
            return [cpu, gpu, storage];
        }
    };
    const fixture = {
        '/api/cpuTemp': {data: '45.7 °C'},
        '/api/gpuTemps': {data: {'0': '59.1 °C'}},
        '/api/storageTemp': {data: [{Key:'ssdA',TemperatureString:'40.0 °C'}]}
    };
    const requests = [];
    const scheduled = [];
    const $ = arg => {
        assert.equal(arg, document);
        return {ready: callback => callback()};
    };
    $.ajax = opts => {
        requests.push(opts.url);
        assert.equal(opts.type, 'get');
        opts.success(fixture[opts.url]);
    };
    $.each = (items, fn) => {
        for (const [index, item] of Object.entries(items)) fn(index, item);
    };

    vm.runInNewContext(source, {$, document, setInterval: (fn, delay) => scheduled.push({fn,delay})});
    assert.deepEqual(requests, ['/api/cpuTemp','/api/gpuTemps','/api/storageTemp']);
    assert.equal(scheduled.length, 1);
    assert.equal(scheduled[0].delay, 3000);
    assert.deepEqual(cpu.children.map(child => child.textContent), ['45.7','°C']);
    assert.deepEqual(gpu.children.map(child => child.textContent), ['59.1','°C']);
    assert.deepEqual(storage.children.map(child => child.textContent), ['40.0','°C']);
    assert.equal(gpu.children[1].className, 'olh-telemetry-unit');
    assert.equal(cpu.attributes['aria-label'], '45.7 °C');

    fixture['/api/cpuTemp'] = {data:'115.0 °F'};
    fixture['/api/gpuTemps'] = {data:{'0':'N/A'}};
    fixture['/api/storageTemp'] = {data:[{Key:'ssdA',TemperatureString:'<script>alert(1)</script>'}]};
    scheduled[0].fn();
    assert.deepEqual(cpu.children.map(child => child.textContent), ['115.0','°F']);
    assert.equal(cpu.attributes['aria-label'], '115.0 °F');
    assert.equal(gpu.textContent, 'N/A');
    assert.equal(storage.textContent, '<script>alert(1)</script>');
    assert.doesNotMatch(source,/\.html\s*\(/, 'API values must be inserted as text, not parsed as HTML');
});
