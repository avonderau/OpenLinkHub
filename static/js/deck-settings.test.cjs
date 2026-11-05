'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

const root = path.join(__dirname, '..', '..');
const script = fs.readFileSync(path.join(__dirname, 'settings.js'), 'utf8');
const template = fs.readFileSync(path.join(root, 'web/settings.html'), 'utf8');

test('Supported Devices settings retain Corsair switches and send vendor-specific Deck switches', () => {
  const initStart = script.indexOf("    const dt = $('#dataTable').DataTable(");
  const initEnd = script.indexOf("    $('.allDevicesRgb').on(", initStart);
  const loadStart = script.indexOf("    $.ajax({\n        url: '/api/getSupportedDevices'", initEnd);
  const loadEnd = script.indexOf("    $('.enableVirtualAudio').on(", loadStart);
  assert.ok(initStart >= 0 && initEnd > initStart && loadStart > initEnd && loadEnd > loadStart);

  const rowsData = [];
  const handlers = new Map();
  let sent = null;
  let columns;
  const rows = () => ({
    every(fn) {
      rowsData.forEach((item) => fn.call({ data: () => item }));
    },
    add: data => rowsData.push(...data),
  });
  rows.add = data => rowsData.push(...data);
  const table = {
    clear() { rowsData.length = 0; return this; },
    rows,
    draw() {},
    on(event, selector, fn) { handlers.set('table:' + event, fn); return this; },
    row() { throw Error('not needed to test save payload'); },
  };
  const $ = selector => {
    if (selector === '#dataTable') return { DataTable(options) { columns = options.columns; return table; } };
    if (selector === '#btnSaveSupportedDevices') return { on(event, fn) { handlers.set('save:' + event, fn); } };
    throw Error('unexpected selector ' + String(selector));
  };
  $.ajax = options => {
    if (options.url === '/api/getSupportedDevices') {
      options.success({ code: 200, data: [
        { ProductId: 7054, VendorId: 6940, Family: 'corsair', DeviceKey: '1b1c:1b8e', Name: 'Corsair Nexus', Enabled: true },
        { ProductId: 26224, VendorId: 21832, Family: 'deck', DeviceKey: '5548:6670', Name: 'Mirabox HSV293S', Enabled: true },
        { ProductId: 96, VendorId: 4057, Family: 'deck', DeviceKey: '0fd9:0060', Name: 'Elgato Stream Deck', Enabled: true },
      ]});
      return;
    }
    if (options.url === '/api/setSupportedDevices') {
      sent = JSON.parse(options.data);
      options.success({ status: 1, message: 'saved' });
      return;
    }
    throw Error('unexpected request: ' + options.url);
  };
  const extracted = script.slice(initStart, initEnd) + '\n' + script.slice(loadStart, loadEnd);
  vm.runInNewContext('(() => {\n' + extracted + '\n})()', { $, toast: {success(){},warning(){}}, console });
  assert.equal(columns.length, 5, 'table must show vendor and product identifiers');
  assert.match(columns[4].render(true, null, rowsData[1]), /data-id="5548:6670"/);

  rowsData[1].Enabled = false;
  rowsData[2].Enabled = false;
  handlers.get('save:click')();

  assert.deepEqual(JSON.parse(JSON.stringify(sent.supportedDevices)), { '7054': true });
  assert.deepEqual(JSON.parse(JSON.stringify(sent.deckSupportedDevices)), {
    '5548:6670': false,
    '0fd9:0060': false,
  });
});

test('Settings table header matches the DataTables column count', () => {
  const open = template.indexOf('<table class="dataTable compact" id="dataTable"');
  assert.ok(open >= 0);
  const table = template.slice(open,template.indexOf('</table>',open));
  assert.equal((table.match(/<th>/g)||[]).length,5);
});
