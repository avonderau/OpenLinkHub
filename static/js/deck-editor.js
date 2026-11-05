(() => {
  'use strict';

  const id = document.getElementById('deck-device-id')?.value;
  const grid = document.getElementById('deck-key-grid');
  if (!id || !grid) return;

  const keyCount = Number.parseInt(grid.dataset.keys || '0', 10);
  const pageSelect = document.getElementById('deck-page');
  const labelInput = document.getElementById('deck-key-label');
  const actionInput = document.getElementById('deck-key-action');
  const shortcutInput = document.getElementById('deck-key-shortcut');
  const shortcutProperty = document.getElementById('deck-hotkey-property');
  const mediaInput = document.getElementById('deck-key-media');
  const mediaProperty = document.getElementById('deck-media-property');
  const launchProperty = document.getElementById('deck-launch-property');
  const launchLabel = document.getElementById('deck-launch-label');
  const launchInput = document.getElementById('deck-key-launch-target');
  const launchHelp = document.getElementById('deck-launch-help');
  const macroProperty = document.getElementById('deck-macro-property');
  const macroInput = document.getElementById('deck-key-macro');
  const macroStatus = document.getElementById('deck-macro-status');
  const macroRefreshButton = document.getElementById('deck-refresh-macros');
  const targetPageInput = document.getElementById('deck-key-target-page');
  const targetProperty = document.getElementById('deck-target-property');
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
  let savedMacros = {};
  let macroLoadMessage = '';

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

  function actionName(action) {
    return {
      'page-next': 'Next Page',
      'page-previous': 'Previous Page',
      'page-go': 'Go to Page',
      hotkey: 'Shortcut',
      media: 'Media',
      url: 'Open URL',
      application: 'Launch App',
      macro: 'Run Macro',
    }[action] || '';
  }

  function setKeyProperties(patch) {
    const page = currentPage();
    if (!page || selected < 0) return;
    const key = { index: selected, ...assignment(selected), ...patch };
    if (!key.action || key.action === 'none') delete key.action;
    if (key.action !== 'page-go') delete key.targetPage;
    if (!['hotkey', 'media', 'url', 'application', 'macro'].includes(key.action)) delete key.target;
    // Selecting an input action does not invent a valid target; the save
    // button remains available, and the server validates configured commands.
    page.keys = page.keys.filter(item => item.index !== selected);
    if (key.label || key.action) page.keys.push(key);
    page.keys.sort((a, b) => a.index - b.index);
    setDirty();
    renderGrid();
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
      button.setAttribute('aria-label', `Key ${i + 1}: ${configured?.label || actionName(configured?.action) || 'Unassigned'}`);

      const number = document.createElement('small');
      number.textContent = String(i + 1).padStart(2, '0');
      const label = document.createElement('span');
      label.textContent = configured?.label || actionName(configured?.action) || '—';
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
    const key = valid ? assignment(selected) : null;
    labelInput.disabled = !valid;
    labelInput.value = key?.label || '';
    actionInput.disabled = !valid;
    actionInput.value = key?.action || '';
    targetPageInput.replaceChildren();
    if (layout) {
      for (const page of layout.pages) {
        const option = document.createElement('option');
        option.value = page.id;
        option.textContent = page.name;
        targetPageInput.appendChild(option);
      }
    }
    targetPageInput.value = key?.targetPage || layout?.activePage || '';
    const hasTarget = valid && actionInput.value === 'page-go';
    targetProperty.hidden = !hasTarget;
    targetPageInput.disabled = !hasTarget;
    const hasShortcut = valid && actionInput.value === 'hotkey';
    shortcutProperty.hidden = !hasShortcut;
    shortcutInput.disabled = !hasShortcut;
    shortcutInput.value = hasShortcut ? key?.target || '' : '';
    const hasMedia = valid && actionInput.value === 'media';
    mediaProperty.hidden = !hasMedia;
    mediaInput.disabled = !hasMedia;
    mediaInput.value = hasMedia ? key?.target || 'play-pause' : 'play-pause';
    const isURL = valid && actionInput.value === 'url';
    const isApplication = valid && actionInput.value === 'application';
    launchProperty.hidden = !(isURL || isApplication);
    launchInput.disabled = !(isURL || isApplication);
    launchLabel.textContent = isURL ? 'Web URL' : 'Desktop application ID';
    launchHelp.textContent = isURL
      ? 'Enter an HTTP or HTTPS address. Opens on this computer, not a remote browser.'
      : 'Enter an installed desktop-entry ID such as org.gnome.Terminal.desktop. No shell commands.';
    launchInput.placeholder = isURL ? 'https://example.com' : 'org.gnome.Terminal.desktop';
    launchInput.maxLength = isURL ? 256 : 128;
    launchInput.value = isURL || isApplication ? key?.target || '' : '';
    const hasMacro = valid && actionInput.value === 'macro';
    macroProperty.hidden = !hasMacro;
    macroInput.disabled = !hasMacro || !Object.keys(savedMacros).length;
    macroInput.replaceChildren();
    const macroIDs = Object.keys(savedMacros).sort((a, b) => Number(a) - Number(b));
    for (const macroID of macroIDs) {
      const option = document.createElement('option');
      option.value = macroID;
      option.textContent = savedMacros[macroID]?.name || `Macro #${macroID}`;
      macroInput.appendChild(option);
    }
    const selectedMacro = hasMacro ? key?.target || '' : '';
    if (selectedMacro && !Object.hasOwn(savedMacros, selectedMacro)) {
      const missing = document.createElement('option');
      missing.value = selectedMacro;
      missing.textContent = `Missing macro #${selectedMacro}`;
      macroInput.appendChild(missing);
    }
    macroInput.value = selectedMacro || macroIDs[0] || '';
    macroStatus.textContent = macroLoadMessage ||
      (macroIDs.length ? 'Choose an existing macro. It runs on this computer when pressed.' :
        'No saved macros found. Create one in OpenLinkHub Macros, then Refresh.');
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
    setKeyProperties({ label: labelInput.value });
  });

  actionInput.addEventListener('change', () => {
    setKeyProperties({
      action: actionInput.value,
      ...(actionInput.value === 'page-go'
        ? { targetPage: assignment(selected)?.targetPage || layout.activePage }
        : {}),
      ...(actionInput.value === 'media'
        ? { target: assignment(selected)?.action === 'media' ? assignment(selected)?.target || 'play-pause' : 'play-pause' }
        : {}),
      ...(actionInput.value === 'hotkey'
        ? { target: assignment(selected)?.action === 'hotkey' ? assignment(selected)?.target || '' : '' }
        : {}),
      ...(['url', 'application'].includes(actionInput.value)
        ? { target: assignment(selected)?.action === actionInput.value ? assignment(selected)?.target || '' : '' }
        : {}),
      ...(actionInput.value === 'macro'
        ? { target: assignment(selected)?.action === 'macro' ? assignment(selected)?.target || '' :
          Object.keys(savedMacros).sort((a, b) => Number(a) - Number(b))[0] || '' }
        : {}),
    });
    renderInspector();
  });

  targetPageInput.addEventListener('change', () => {
    if (actionInput.value !== 'page-go') return;
    setKeyProperties({ targetPage: targetPageInput.value });
  });
  shortcutInput.addEventListener('input', () => {
    if (actionInput.value !== 'hotkey') return;
    setKeyProperties({ target: shortcutInput.value });
  });
  mediaInput.addEventListener('change', () => {
    if (actionInput.value !== 'media') return;
    setKeyProperties({ target: mediaInput.value });
  });
  launchInput.addEventListener('input', () => {
    if (actionInput.value !== 'url' && actionInput.value !== 'application') return;
    setKeyProperties({ target: launchInput.value });
  });
  macroInput.addEventListener('change', () => {
    if (actionInput.value !== 'macro') return;
    setKeyProperties({ target: macroInput.value });
  });

  async function loadSavedMacros() {
    macroRefreshButton.disabled = true;
    try {
      const response = await fetch('/api/macro/', { cache: 'no-store' });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const payload = await response.json();
      if (payload?.status !== 1 || !payload.data || typeof payload.data !== 'object') {
        throw new Error('Invalid macro list');
      }
      savedMacros = Object.fromEntries(Object.entries(payload.data)
        .filter(([id, value]) => /^[1-9][0-9]{0,8}$/.test(id) && value && typeof value === 'object'));
      macroLoadMessage = '';
    } catch (error) {
      macroLoadMessage = `Could not load macros: ${error.message}`;
      // Preserve previously loaded macros on transient network failures.
    } finally {
      macroRefreshButton.disabled = false;
      if (layout) renderInspector();
    }
  }
  macroRefreshButton.addEventListener('click', () => { loadSavedMacros(); });

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
    const deletedPage = layout.activePage;
    layout.pages = layout.pages.filter(page => page.id !== deletedPage);
    for (const page of layout.pages) {
      for (const key of page.keys) {
        if (key.action === 'page-go' && key.targetPage === deletedPage) {
          delete key.action;
          delete key.targetPage;
        }
      }
      page.keys = page.keys.filter(key => key.label || key.action);
    }
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
      // Physical key presses may change pages without involving the editor.
      // Only adopt server changes when there are no unsaved browser edits.
      if (layout && !dirty && !saving && status.activePage && layout.activePage !== status.activePage) {
        layout = normalizeLayout(await api('layout'));
        selected = -1;
        renderPages();
        renderGrid();
        renderInspector();
        setMessage(`Device switched to ${currentPage()?.name || 'another page'}.`, 'success');
      }
    } catch {
      statusText.textContent = 'Connection status unavailable';
      statusText.dataset.connected = 'false';
    }
  }

  async function load() {
    try {
      layout = normalizeLayout(await api('layout'));
      await loadSavedMacros();
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
