(() => {
  'use strict';

  const id = document.getElementById('deck-device-id')?.value;
  const grid = document.getElementById('deck-key-grid');
  if (!id || !grid) return;

  const keyCount = Number.parseInt(grid.dataset.keys || '0', 10);
  const pageSelect = document.getElementById('deck-page');
  const labelInput = document.getElementById('deck-key-label');
  const selectedText = document.getElementById('deck-selected-key');
  const saveButton = document.getElementById('deck-save');
  const addButton = document.getElementById('deck-add-page');
  const removeButton = document.getElementById('deck-remove-page');
  const message = document.getElementById('deck-message');
  const keyCounter = document.getElementById('deck-key-count');
  const statusText = document.getElementById('deck-connection-state');
  let layout;
  let selected = -1;
  let dirty = false;

  const endpoint = (resource) => `/api/deck/${resource}?serial=${encodeURIComponent(id)}`;
  const currentPage = () => layout?.pages?.find((page) => page.id === layout.activePage);

  async function api(resource, options) {
    const response = await fetch(endpoint(resource), { cache: 'no-store', ...options });
    if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
    return response.status === 204 ? null : response.json();
  }

  function setMessage(text, error = false) {
    message.textContent = text;
    message.style.color = error ? '#f2bdbd' : '#abd9ee';
  }

  function touch() {
    dirty = true;
    saveButton.disabled = false;
    setMessage('Unsaved changes');
  }

  function assignment(index) {
    const page = currentPage();
    return page?.keys?.find((key) => key.index === index);
  }

  function renderGrid() {
    grid.replaceChildren();
    const page = currentPage();
    if (!page) return;
    for (let i = 0; i < keyCount; i += 1) {
      const assigned = assignment(i);
      const button = document.createElement('button');
      button.type = 'button';
      button.className = 'deck-key';
      button.setAttribute('aria-pressed', i === selected ? 'true' : 'false');
      button.setAttribute('aria-label', `Key ${i + 1}: ${assigned?.label || 'Unassigned'}`);
      const icon = document.createElement('small');
      icon.textContent = String(i + 1).padStart(2, '0');
      const text = document.createElement('span');
      text.textContent = assigned?.label || '—';
      button.append(icon, text);
      button.addEventListener('click', () => {
        selected = i;
        renderGrid();
        renderInspector();
      });
      grid.appendChild(button);
    }
    keyCounter.textContent = `${keyCount} LCD keys · ${page.name}`;
  }

  function renderPages() {
    pageSelect.replaceChildren();
    for (const page of layout.pages) {
      const option = document.createElement('option');
      option.value = page.id;
      option.textContent = page.name;
      pageSelect.appendChild(option);
    }
    pageSelect.value = layout.activePage;
    removeButton.disabled = layout.pages.length < 2;
    addButton.disabled = layout.pages.length >= 16;
  }

  function renderInspector() {
    const valid = selected >= 0 && selected < keyCount;
    labelInput.disabled = !valid;
    labelInput.value = valid ? assignment(selected)?.label || '' : '';
    selectedText.textContent = valid ? `Key ${selected + 1}` : 'Select a key to edit';
  }

  labelInput.addEventListener('input', () => {
    if (selected < 0) return;
    const page = currentPage();
    if (!page) return;
    page.keys = page.keys.filter((key) => key.index !== selected);
    if (labelInput.value) page.keys.push({ index: selected, label: labelInput.value });
    page.keys.sort((a, b) => a.index - b.index);
    touch();
    renderGrid();
  });

  pageSelect.addEventListener('change', () => {
    layout.activePage = pageSelect.value;
    selected = -1;
    renderGrid();
    renderInspector();
    touch();
  });

  addButton.addEventListener('click', () => {
    if (!layout || layout.pages.length >= 16) return;
    const requested = window.prompt('Name the new deck page:', `Page ${layout.pages.length + 1}`);
    if (requested === null) return;
    const name = requested.trim();
    if (!name || name.length > 64) return setMessage('Page name must be 1–64 characters', true);
    const uniqueID = `page-${Date.now().toString(36)}`;
    layout.pages.push({ id: uniqueID, name, keys: [] });
    layout.activePage = uniqueID;
    selected = -1;
    renderPages();
    renderGrid();
    renderInspector();
    touch();
  });

  removeButton.addEventListener('click', () => {
    if (!layout || layout.pages.length < 2) return;
    if (!window.confirm('Remove this page? This will discard its keys.')) return;
    layout.pages = layout.pages.filter((page) => page.id !== layout.activePage);
    layout.activePage = layout.pages[0].id;
    selected = -1;
    renderPages();
    renderGrid();
    renderInspector();
    touch();
  });

  saveButton.addEventListener('click', async () => {
    if (!layout || !dirty) return;
    saveButton.disabled = true;
    setMessage('Saving...');
    try {
      await api('layout', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(layout)
      });
      dirty = false;
      setMessage('Layout saved.');
    } catch (error) {
      saveButton.disabled = false;
      setMessage(`Unable to save: ${error.message}`, true);
    }
  });

  async function refreshStatus() {
    try {
      const status = await api('status');
      statusText.textContent = status.status;
      statusText.dataset.connected = status.connected ? 'true' : 'false';
    } catch {
      statusText.textContent = 'Connection status unavailable';
    }
  }

  async function load() {
    try {
      layout = await api('layout');
      if (!Array.isArray(layout.pages) || !layout.pages.length) throw new Error('No pages returned');
      renderPages();
      renderGrid();
      renderInspector();
      setMessage('Select a key to begin editing.');
    } catch (error) {
      setMessage(`Cannot load layout: ${error.message}`, true);
    }
    refreshStatus();
    setInterval(refreshStatus, 3000);
  }

  load();
})();
