(() => {
  'use strict';

  const id = document.getElementById('deck-device-id')?.value;
  const grid = document.getElementById('deck-key-grid');
  if (!id || !grid) return;

  const keyCount = Number.parseInt(grid.dataset.keys || '0', 10);
  const pageSelect = document.getElementById('deck-page');
  const labelInput = document.getElementById('deck-key-label');
  const pageNameInput = document.getElementById('deck-page-name');
  const selectedText = document.getElementById('deck-selected-key');
  const saveButton = document.getElementById('deck-save');
  const addButton = document.getElementById('deck-add-page');
  const removeButton = document.getElementById('deck-remove-page');
  const message = document.getElementById('deck-message');
  const keyCounter = document.getElementById('deck-key-count');
  const pageCounter = document.getElementById('deck-page-count');
  const statusText = document.getElementById('deck-connection-state');
  const tabs = {
    key: document.getElementById('deck-tab-key'),
    pages: document.getElementById('deck-tab-pages'),
  };
  const panels = {
    key: document.getElementById('deck-panel-key'),
    pages: document.getElementById('deck-panel-pages'),
  };

  let layout = null;
  let selected = -1;
  let dirty = false;
  let saving = false;

  const endpoint = (resource) => `/api/deck/${resource}?serial=${encodeURIComponent(id)}`;
  const currentPage = () => layout?.pages?.find(page => page.id === layout.activePage);

  async function api(resource, options) {
    const response = await fetch(endpoint(resource), { cache: 'no-store', ...options });
    if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
    return response.status === 204 ? null : response.json();
  }

  function setMessage(text, state = '') {
    message.textContent = text;
    delete message.dataset.error;
    delete message.dataset.success;
    if (state === 'error') message.dataset.error = 'true';
    if (state === 'success') message.dataset.success = 'true';
  }

  function setDirty() {
    dirty = true;
    setMessage('Unsaved changes. Save & Apply to update the device.');
  }

  function normalizeLayout(data) {
    if (!data || !Array.isArray(data.pages) || !data.pages.length) {
      throw new Error('No pages returned by the device');
    }
    // Older or missing JSON lists may deserialize as null. Do not silently
    // fail the key editor when the page has never had an assigned key.
    data.pages.forEach(page => {
      if (!Array.isArray(page.keys)) page.keys = [];
    });
    return data;
  }

  function assignment(index) {
    return currentPage()?.keys.find(key => key.index === index);
  }

  function renderGrid() {
    grid.replaceChildren();
    const page = currentPage();
    if (!page) return;

    for (let i = 0; i < keyCount; i += 1) {
      const configured = assignment(i);
      const button = document.createElement('button');
      button.type = 'button';
      button.className = 'deck-key';
      button.setAttribute('aria-pressed', i === selected ? 'true' : 'false');
      button.setAttribute('aria-label', `Key ${i + 1}: ${configured?.label || 'Unassigned'}`);

      const number = document.createElement('small');
      number.textContent = String(i + 1).padStart(2, '0');
      const label = document.createElement('span');
      label.textContent = configured?.label || '—';
      button.append(number, label);
      button.addEventListener('click', () => {
        selected = i;
        renderGrid();
        renderInspector();
        selectTab('key');
        labelInput.focus();
      });
      grid.appendChild(button);
    }
    keyCounter.textContent = `${keyCount} LCD keys · ${page.name}`;
  }

  function renderPages() {
    if (!layout) return;
    pageSelect.replaceChildren();
    for (const page of layout.pages) {
      const option = document.createElement('option');
      option.value = page.id;
      option.textContent = page.name;
      pageSelect.appendChild(option);
    }
    pageSelect.value = layout.activePage;
    pageNameInput.value = currentPage()?.name || '';
    pageCounter.textContent = `${layout.pages.length} of 16 pages`;
    addButton.disabled = layout.pages.length >= 16;
    removeButton.disabled = layout.pages.length <= 1;
  }

  function renderInspector() {
    const valid = selected >= 0 && selected < keyCount;
    labelInput.disabled = !valid;
    labelInput.value = valid ? assignment(selected)?.label || '' : '';
    selectedText.textContent = valid ? `Key ${selected + 1}` : 'Select a key';
  }

  function selectTab(which) {
    Object.keys(tabs).forEach(name => {
      const active = which === name;
      tabs[name].setAttribute('aria-selected', String(active));
      tabs[name].tabIndex = active ? 0 : -1;
      panels[name].hidden = !active;
    });
  }

  for (const name of Object.keys(tabs)) {
    tabs[name].addEventListener('click', () => selectTab(name));
    tabs[name].addEventListener('keydown', event => {
      if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
      event.preventDefault();
      const order = Object.keys(tabs);
      const next = order[(order.indexOf(name) + (event.key === 'ArrowRight' ? 1 : -1) + order.length) % order.length];
      selectTab(next);
      tabs[next].focus();
    });
  }

  labelInput.addEventListener('input', () => {
    const page = currentPage();
    if (!page || selected < 0) return;
    if (!Array.isArray(page.keys)) page.keys = [];
    const value = labelInput.value;
    page.keys = page.keys.filter(key => key.index !== selected);
    if (value) page.keys.push({ index: selected, label: value });
    page.keys.sort((a, b) => a.index - b.index);
    setDirty();
    renderGrid();
  });

  pageNameInput.addEventListener('input', () => {
    const page = currentPage();
    if (!page) return;
    page.name = pageNameInput.value;
    const option = pageSelect.selectedOptions[0];
    if (option) option.textContent = page.name;
    setDirty();
    keyCounter.textContent = `${keyCount} LCD keys · ${page.name}`;
  });

  pageSelect.addEventListener('change', () => {
    layout.activePage = pageSelect.value;
    selected = -1;
    renderPages();
    renderGrid();
    renderInspector();
    setDirty();
  });

  addButton.addEventListener('click', () => {
    if (!layout || layout.pages.length >= 16) return;
    let index = layout.pages.length + 1;
    while (layout.pages.some(page => page.id === `page-${index}`)) index++;
    const page = { id: `page-${index}`, name: `Page ${index}`, keys: [] };
    layout.pages.push(page);
    layout.activePage = page.id;
    selected = -1;
    renderPages();
    renderGrid();
    renderInspector();
    setDirty();
  });

  removeButton.addEventListener('click', () => {
    if (!layout || layout.pages.length <= 1) return;
    if (!window.confirm('Delete the selected page and all its key labels?')) return;
    layout.pages = layout.pages.filter(page => page.id !== layout.activePage);
    layout.activePage = layout.pages[0].id;
    selected = -1;
    renderPages();
    renderGrid();
    renderInspector();
    setDirty();
  });

  async function saveAndApply() {
    if (!layout || saving) return;
    const page = currentPage();
    if (!page?.name.trim()) {
      selectTab('pages');
      pageNameInput.focus();
      setMessage('Page name cannot be empty.', 'error');
      return;
    }

    const snapshot = JSON.stringify(layout);
    saving = true;
    saveButton.disabled = true;
    setMessage('Saving layout and updating device...');
    try {
      const result = await api('layout', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: snapshot,
      });
      dirty = JSON.stringify(layout) !== snapshot;
      if (!result?.saved) throw new Error('Server did not confirm the save');
      setMessage(result.message || 'Saved.', result.applied ? 'success' : '');
      if (dirty) setMessage('Saved previous changes; newer edits still need applying.');
      await refreshStatus();
    } catch (error) {
      dirty = true;
      setMessage(`Unable to save or apply: ${error.message}`, 'error');
    } finally {
      saving = false;
      saveButton.disabled = false; // Allow reapplying even when there are no new edits.
    }
  }
  saveButton.addEventListener('click', saveAndApply);
  document.addEventListener('keydown', event => {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
      event.preventDefault();
      saveAndApply();
    }
  });

  async function refreshStatus() {
    try {
      const status = await api('status');
      statusText.textContent = status.status;
      statusText.dataset.connected = status.connected ? 'true' : 'false';
    } catch {
      statusText.textContent = 'Connection status unavailable';
      statusText.dataset.connected = 'false';
    }
  }

  async function load() {
    try {
      layout = normalizeLayout(await api('layout'));
      renderPages();
      renderGrid();
      renderInspector();
      saveButton.disabled = false;
      setMessage('Ready. Choose a key, then Save & Apply.');
    } catch (error) {
      setMessage(`Cannot load Deck layout: ${error.message}`, 'error');
    }
    refreshStatus();
    setInterval(refreshStatus, 3000);
  }

  load();
})();
