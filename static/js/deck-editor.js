(() => {
  'use strict';

  const id = document.getElementById('deck-device-id')?.value;
  const grid = document.getElementById('deck-key-grid');
  if (!id || !grid) return;

  const keyCount = Number.parseInt(grid.dataset.keys || '0', 10);
  const pageSelect = document.getElementById('deck-page');
  const labelInput = document.getElementById('deck-key-label');
  const artworkInput = document.getElementById('deck-key-image');
  const uploadArtworkButton = document.getElementById('deck-upload-image');
  const removeArtworkButton = document.getElementById('deck-remove-image');
  const artworkStatus = document.getElementById('deck-icon-status');
  const artworkPreview = document.getElementById('deck-artwork-preview');
  const inspectorFields = document.getElementById('deck-key-settings');
  const emptySelection = document.getElementById('deck-empty-state');
  const selectedSummary = document.getElementById('deck-selected-summary');
  const selectedHero = document.getElementById('deck-selected-hero');
  const iconSetSelect = document.getElementById('deck-icon-set');
  const iconSetItem = document.getElementById('deck-icon-set-item');
  const iconSetApply = document.getElementById('deck-apply-icon-set');
  let iconSets = [];
  const metricInput = document.getElementById('deck-key-metric');
  const metricSourceSelect = document.getElementById('deck-metric-source');
  const metricSourceProperty = document.getElementById('deck-metric-source-property');
  let metricSources = {};
  const metricInterval = document.getElementById('deck-metric-interval');
  const metricStyle = document.getElementById('deck-metric-style');
  const metricColor = document.getElementById('deck-metric-color');
  const metricColorReset = document.getElementById('deck-metric-color-reset');
  const defaultMetricColor = '#d5a65a';
  const safeMetricColor = value => /^#[0-9a-fA-F]{6}$/.test(value || '') ? value : '';
  const metricStatus = document.getElementById('deck-metric-status');
  const metricDetails = document.getElementById('deck-metric-details');
  const metricIndicator = document.getElementById('deck-metric-indicator');
  const actionIndicator = document.getElementById('deck-action-indicator');
  const actionInput = document.getElementById('deck-key-action');
  const deckControlInput = document.getElementById('deck-key-control');
  const deckControlIndicator = document.getElementById('deck-control-indicator');
  const brightnessProperty = document.getElementById('deck-brightness-property');
  const brightnessInput = document.getElementById('deck-key-brightness');
  const deckControls = new Set(['page-next','page-previous','page-go','brightness-up','brightness-down','brightness-set']);
  const shortcutInput = document.getElementById('deck-key-shortcut');
  const shortcutProperty = document.getElementById('deck-hotkey-property');
  const macroProperty = document.getElementById('deck-macro-property');
  const macroInput = document.getElementById('deck-key-macro');
  const macroStatus = document.getElementById('deck-macro-status');
  const macroRefreshButton = document.getElementById('deck-refresh-macros');
  const mediaInput = document.getElementById('deck-key-media');
  const mediaProperty = document.getElementById('deck-media-property');
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
  let metricLive = {};
  let savedMacros = {};
  let macroLoadMessage = '';

  const endpoint = (resource) => `/api/deck/${resource}?serial=${encodeURIComponent(id)}`;
  const currentPage = () => layout?.pages?.find(page => page.id === layout.activePage);
  const validIconID = id => typeof id === 'string' && /^[a-f0-9]{32}$/.test(id);
  const iconURL = id => `${endpoint('icon')}&id=${encodeURIComponent(id)}`;

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

  function formatMetricPreview(metric) {
    const value = metricLive[metric];
    if (!Number.isFinite(value)) return '—';
    if (metric.endsWith('.temperature')) return `${value.toFixed(0)} C`;
    if (metric.endsWith('.utilization')) return `${value.toFixed(0)}%`;
    if (metric.endsWith('.rpm')) return `${value.toFixed(0)} rpm`;
    if (metric === 'memory.used') return `${(value / (1024 ** 3)).toFixed(1)} GiB`;
    if (metric.endsWith('_rate')) return Math.abs(value) >= 1e6
      ? `${(value / 1e6).toFixed(1)} MB/s` : `${(value / 1e3).toFixed(0)} KB/s`;
    if (metric.endsWith('_mb')) return `${value.toFixed(0)} MB`;
    if (metric.endsWith('_mhz')) return `${value.toFixed(0)} MHz`;
    if (metric.endsWith('_w')) return `${value.toFixed(0)} W`;
    if (metric === 'disk.free_gb') return `${value.toFixed(0)} GB`;
    return value.toFixed(1);
  }

  function updateMetricPreview() {
    for (let i = 0; i < keyCount; i += 1) {
      const metric = assignment(i)?.metric;
      if (!metric) continue;
      const valueNode = grid.children[i]?.querySelector?.('.deck-live-value');
      if (valueNode) valueNode.textContent = formatMetricPreview(metric);
    }
  }

  // These preview masks use the physical device's pixel-level icon shapes.
  const mediaGlyphs = Object.freeze({
    'play-pause': true, 'previous-track': true, 'next-track': true,
    stop: true, 'volume-up': true, 'volume-down': true, 'volume-mute': true,
  });
  const mediaKeyLabels = Object.freeze({
    'play-pause': 'Play/Pause',
    'previous-track': 'Previous',
    'next-track': 'Next Track',
    stop: 'Stop',
    'volume-up': 'Volume +',
    'volume-down': 'Volume -',
    'volume-mute': 'Mute',
  });

  function actionName(action) {
    return {
      'page-next': 'Next Page',
      'page-previous': 'Previous Page',
      'page-go': 'Go to Page',
      'brightness-up': 'Brightness +',
      'brightness-down': 'Brightness -',
      'brightness-set': 'Set Brightness',
      hotkey: 'Shortcut',
      macro: 'Run Macro',
      media: 'Media',
    }[action] || '';
  }

  function setKeyProperties(patch) {
    const page = currentPage();
    if (!page || selected < 0) return;
    const key = { index: selected, ...assignment(selected), ...patch };
    if (!key.action || key.action === 'none') delete key.action;
    if (key.action !== 'page-go') delete key.targetPage;
    if (key.action !== 'macro' && key.action !== 'media' && key.action !== 'hotkey' && key.action !== 'brightness-set') delete key.target;
    // Selecting an input action does not invent a valid target; the save
    // button remains available, and the server validates configured commands.
    page.keys = page.keys.filter(item => item.index !== selected);
    if (!key.metric) {
      delete key.metric;
      delete key.refreshSeconds;
      delete key.showGraph;
      delete key.metricStyle;
      delete key.metricColor;
      delete key.metricSource;
    }
    if (!key.metricColor) delete key.metricColor;
    if (!key.metricSource) delete key.metricSource;
    if (key.label || key.action || key.icon || key.metric) page.keys.push(key);
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
      const mediaLabel = configured?.action === 'media' ? mediaKeyLabels[configured?.target] : '';
      const brightnessLabel = configured?.action === 'brightness-set' ? 'Brightness '+configured.target+'%' : '';
      const displayName = configured?.label || mediaLabel || brightnessLabel || actionName(configured?.action) || '—';
      button.setAttribute('aria-label', `Key ${i + 1}: ${displayName}`);

      const number = document.createElement('small');
      number.textContent = String(i + 1).padStart(2, '0');
      const label = document.createElement('span');
      label.textContent = displayName;
      button.append(number);
      if (configured?.action === 'media' && !configured?.metric &&
          !validIconID(configured?.icon) && mediaGlyphs[configured?.target]) {
        const glyph = document.createElement('span');
        glyph.className = 'deck-media-glyph';
        glyph.style.maskImage = `url("/static/media-glyphs/${configured.target}.svg")`;
        glyph.style.webkitMaskImage = glyph.style.maskImage;
        glyph.setAttribute('aria-hidden', 'true');
        button.appendChild(glyph);
        button.dataset.hasMediaIcon = 'true';
      }
      button.append(label);
      if (configured?.metric) {
        const live = document.createElement('span');
        live.className = 'deck-live-value';
        live.textContent = formatMetricPreview(
          configured.metricSource ? configured.metric + '|' + configured.metricSource : configured.metric);
        live.style.color = safeMetricColor(configured.metricColor);
        button.appendChild(live);
        button.dataset.hasMetric = 'true';
      }
      if (validIconID(configured?.icon)) {
        button.style.backgroundImage = `linear-gradient(180deg, transparent 45%, rgba(0,0,0,.65)), url("${iconURL(configured.icon)}")`;
        button.dataset.hasImage = 'true';
      }
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

  function updateSelectedSummary(key) {
    selectedSummary.textContent = [
      key?.label || 'Unlabeled key',
      key?.metric ? 'Live data' : '',
      key?.action ? 'Button action' : '',
    ].filter(Boolean).join(' · ');
  }

  function renderInspector() {
    const valid = selected >= 0 && selected < keyCount;
    const key = valid ? assignment(selected) : null;
    inspectorFields.hidden = !valid;
    emptySelection.hidden = valid;
    selectedHero.hidden = !valid;
    if (valid) updateSelectedSummary(key);
    artworkPreview.dataset.hasArtwork = String(validIconID(key?.icon));
    artworkPreview.style.backgroundImage = validIconID(key?.icon) ? `url("${iconURL(key.icon)}")` : '';
    labelInput.disabled = !valid;
    labelInput.value = key?.label || '';
    labelInput.placeholder = key?.action === 'media' ? (mediaKeyLabels[key.target] || '') : '';
    artworkInput.disabled = !valid;
    uploadArtworkButton.disabled = !valid;
    removeArtworkButton.disabled = !valid || !validIconID(key?.icon);
    removeArtworkButton.hidden = !validIconID(key?.icon);
    iconSetApply.disabled = !valid || !validIconID(iconSetItem.value);
    artworkStatus.textContent = validIconID(key?.icon)
      ? 'Artwork assigned. Save & Apply to send it to the physical LCD.'
      : 'PNG/JPEG · up to 512 KiB · maximum 512 × 512 pixels.';
    const hasMetric = valid && !!key?.metric;
    metricDetails.hidden = !hasMetric;
    metricIndicator.textContent = hasMetric ? 'Active' : 'Off';
    metricIndicator.dataset.active = String(hasMetric);
    metricInput.disabled = !valid;
    metricInput.value = key?.metric || '';
    const sourceChoices = metricSources[key?.metric] || [];
    const selectedSource = key?.metricSource || '';
    metricSourceProperty.hidden = !hasMetric || (sourceChoices.length === 0 && !selectedSource);
    metricSourceSelect.disabled = !hasMetric;
    metricSourceSelect.replaceChildren();
    const autoSource = document.createElement('option');
    autoSource.value = '';
    autoSource.textContent = 'Automatic / default';
    metricSourceSelect.appendChild(autoSource);
    for (const source of sourceChoices) {
      const option = document.createElement('option');
      option.value = source.id;
      option.textContent = source.label;
      metricSourceSelect.appendChild(option);
    }
    if (selectedSource && !sourceChoices.some(source => source.id === selectedSource)) {
      const unavailable = document.createElement('option');
      unavailable.value = selectedSource;
      unavailable.textContent = selectedSource + ' (unavailable)';
      metricSourceSelect.appendChild(unavailable);
    }
    metricSourceSelect.value = selectedSource;
    metricInterval.disabled = !hasMetric;
    metricInterval.value = String(key?.refreshSeconds || 5);
    metricStyle.disabled = !hasMetric;
    metricStyle.value = key?.metricStyle || (key?.showGraph ? 'line' : 'value');
    metricColor.disabled = !hasMetric;
    metricColor.value = safeMetricColor(key?.metricColor) || defaultMetricColor;
    metricColorReset.disabled = !hasMetric || !safeMetricColor(key?.metricColor);
    metricStatus.textContent = hasMetric
      ? (Number.isFinite(metricLive[key.metricSource ? key.metric + '|' + key.metricSource : key.metric])
        ? 'Live reading available from this computer.'
        : 'No current reading. Unavailable sensors display —.')
      : '';
    deckControlInput.disabled = !valid;
    deckControlInput.value = deckControls.has(key?.action) ? key.action : '';
    deckControlIndicator.textContent = deckControlInput.value ? 'Assigned' : 'None';
    deckControlIndicator.dataset.active = String(!!deckControlInput.value);
    brightnessProperty.hidden = !valid || deckControlInput.value !== 'brightness-set';
    brightnessInput.disabled = brightnessProperty.hidden;
    brightnessInput.value = deckControlInput.value === 'brightness-set' ? key.target || '50' : '50';
    actionInput.disabled = !valid;
    actionInput.value = deckControls.has(key?.action) ? '' : key?.action || '';
    actionIndicator.textContent = actionInput.value ? 'Assigned' : 'None';
    actionIndicator.dataset.active = String(!!actionInput.value);
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
    const hasTarget = valid && deckControlInput.value === 'page-go';
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
      (macroIDs.length ? 'Select an existing macro. It runs on this computer when pressed.' :
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
    updateSelectedSummary(assignment(selected));
  });
  metricInput.addEventListener('change', () => {
    const metric = metricInput.value;
    setKeyProperties(metric ? { metric, metricSource: '', refreshSeconds: 5, metricStyle: 'value', showGraph: false } :
      { metric: '', metricSource: '', refreshSeconds: 0, metricStyle: '', showGraph: false });
    renderInspector();
  });
  metricSourceSelect.addEventListener('change', () => {
    if (!metricInput.value) return;
    setKeyProperties({ metricSource: metricSourceSelect.value });
  });
  metricInterval.addEventListener('change', () => {
    if (!metricInput.value) return;
    setKeyProperties({ refreshSeconds: Number(metricInterval.value) });
  });
  metricStyle.addEventListener('change', () => {
    if (!metricInput.value) return;
    setKeyProperties({ metricStyle: metricStyle.value, showGraph: false });
  });
  metricColor.addEventListener('input', () => {
    if (!metricInput.value) return;
    const value = safeMetricColor(metricColor.value);
    if (!value) return;
    setKeyProperties({ metricColor: value.toLowerCase() === defaultMetricColor ? '' : value });
    renderInspector();
  });
  metricColorReset.addEventListener('click', () => {
    if (!metricInput.value) return;
    setKeyProperties({ metricColor: '' });
    renderInspector();
  });

  uploadArtworkButton.addEventListener('click', async () => {
    const file = artworkInput.files?.[0];
    if (!layout || selected < 0 || !file) {
      setMessage('Choose a PNG or JPEG file and select a key.', 'error');
      return;
    }
    if (!['image/png', 'image/jpeg'].includes(file.type) || file.size > 512 * 1024 || file.size < 1) {
      setMessage('Artwork must be PNG/JPEG and no larger than 512 KiB.', 'error');
      return;
    }
    const pageID = layout.activePage;
    const keyIndex = selected;
    uploadArtworkButton.disabled = true;
    artworkStatus.textContent = 'Uploading image...';
    try {
      const result = await api('icon', {
        method: 'POST',
        headers: { 'Content-Type': file.type },
        body: file,
      });
      if (!validIconID(result?.id)) throw new Error('Invalid image identifier returned');
      if (layout.activePage !== pageID || selected !== keyIndex) {
        setMessage('Artwork uploaded, but selection changed. Select the key and upload again.', 'error');
        return;
      }
      setKeyProperties({ icon: result.id });
      renderInspector();
      setMessage('Artwork uploaded. Save & Apply to update the Mirabox LCD.', 'success');
    } catch (error) {
      artworkStatus.textContent = 'Upload failed: ' + error.message;
      setMessage('Unable to upload artwork: ' + error.message, 'error');
    } finally {
      if (layout && selected >= 0) uploadArtworkButton.disabled = false;
    }
  });

  function refreshIconSetPicker() {
    const chosen = iconSetSelect.value;
    iconSetSelect.replaceChildren();
    const none = document.createElement('option');
    none.value = ''; none.textContent = 'Select icon set';
    iconSetSelect.appendChild(none);
    for (const pack of iconSets) {
      const option = document.createElement('option');
      option.value = pack.id;
      option.textContent = pack.name;
      iconSetSelect.appendChild(option);
    }
    iconSetSelect.value = iconSets.some(set => set.id === chosen) ? chosen : '';
    const pack = iconSets.find(set => set.id === iconSetSelect.value);
    const selectedIcon = iconSetItem.value;
    iconSetItem.replaceChildren();
    const placeholder = document.createElement('option');
    placeholder.value = ''; placeholder.textContent = 'Select icon';
    iconSetItem.appendChild(placeholder);
    for (const icon of pack?.icons || []) {
      const option = document.createElement('option');
      option.value = icon.deckId;
      option.textContent = icon.name;
      iconSetItem.appendChild(option);
    }
    iconSetItem.value = pack?.icons.some(icon => icon.deckId === selectedIcon) ? selectedIcon : '';
    iconSetItem.disabled = !pack;
    iconSetApply.disabled = selected < 0 || !validIconID(iconSetItem.value);
  }
  async function loadIconSets() {
    try {
      const response = await fetch('/api/icon-sets', { cache: 'no-store' });
      if (!response.ok) return;
      const payload = await response.json();
      iconSets = Array.isArray(payload?.sets) ? payload.sets : [];
      refreshIconSetPicker();
    } catch (_error) {
      setMessage('Icon packs are unavailable. Check Settings.', 'error');
    }
  }
  iconSetSelect.addEventListener('change', refreshIconSetPicker);
  iconSetItem.addEventListener('change', refreshIconSetPicker);
  iconSetApply.addEventListener('click', () => {
    if (!validIconID(iconSetItem.value) || selected < 0) return;
    setKeyProperties({ icon: iconSetItem.value });
    renderInspector();
    setMessage('Icon set artwork assigned. Save & Apply to update the LCD.', 'success');
  });
  removeArtworkButton.addEventListener('click', () => {
    setKeyProperties({ icon: '' });
    artworkInput.value = '';
    renderInspector();
  });

  actionInput.addEventListener('change', () => {
    setKeyProperties({
      action: actionInput.value,
      ...(actionInput.value === 'media'
        ? { target: assignment(selected)?.action === 'media' ? assignment(selected)?.target || 'play-pause' : 'play-pause' }
        : {}),
      ...(actionInput.value === 'macro'
        ? { target: assignment(selected)?.action === 'macro' ? assignment(selected)?.target || '' :
            Object.keys(savedMacros).sort((a, b) => Number(a) - Number(b))[0] || '' }
        : {}),
      ...(actionInput.value === 'hotkey'
        ? { target: assignment(selected)?.action === 'hotkey' ? assignment(selected)?.target || '' : '' }
        : {}),
    });
    renderInspector();
  });

  deckControlInput.addEventListener('change', () => {
    const value = deckControlInput.value;
    setKeyProperties({
      action: value,
      targetPage: value === 'page-go' ? assignment(selected)?.targetPage || layout.activePage : '',
      target: value === 'brightness-set' ? '50' : '',
    });
    renderInspector();
  });
  brightnessInput.addEventListener('change', () => {
    if (deckControlInput.value !== 'brightness-set') return;
    setKeyProperties({ target: brightnessInput.value });
  });
  targetPageInput.addEventListener('change', () => {
    if (deckControlInput.value !== 'page-go') return;
    setKeyProperties({ targetPage: targetPageInput.value });
  });
  shortcutInput.addEventListener('input', () => {
    if (actionInput.value !== 'hotkey') return;
    setKeyProperties({ target: shortcutInput.value });
  });
  macroInput.addEventListener('change', () => {
    if (actionInput.value !== 'macro') return;
    setKeyProperties({ target: macroInput.value });
  });
  mediaInput.addEventListener('change', () => {
    if (actionInput.value !== 'media') return;
    setKeyProperties({ target: mediaInput.value });
    renderInspector(); // Update the default label placeholder for this command.
  });

  async function loadSavedMacros() {
    macroRefreshButton.disabled = true;
    try {
      const response = await fetch('/api/macro/', { cache: 'no-store' });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const payload = await response.json();
      if (payload?.status !== 1 || !payload.data || typeof payload.data !== 'object' ||
          Array.isArray(payload.data)) throw new Error('Invalid macro list');
      savedMacros = Object.fromEntries(Object.entries(payload.data)
        .filter(([id, value]) => /^[1-9][0-9]{0,8}$/.test(id) && value && typeof value === 'object'));
      macroLoadMessage = '';
    } catch (error) {
      macroLoadMessage = `Could not load macros: ${error.message}`;
    } finally {
      macroRefreshButton.disabled = false;
      if (layout) renderInspector();
    }
  }
  macroRefreshButton.addEventListener('click', loadSavedMacros);

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
      page.keys = page.keys.filter(key => key.label || key.action || key.icon || key.metric);
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

  async function loadMetricSources() {
    try {
      const response = await fetch('/api/telemetry/sources', { cache: 'no-store' });
      if (response.ok) {
        const result = await response.json();
        metricSources = result?.sources || {};
        renderInspector();
      }
    } catch (_error) { /* Previously saved sources stay visible as unavailable. */ }
  }
  async function refreshStatus() {
    try {
      const status = await api('status');
      statusText.textContent = status.status;
      statusText.dataset.connected = status.connected ? 'true' : 'false';
      const nextMetrics = status.metrics || {};
      if (JSON.stringify(nextMetrics) !== JSON.stringify(metricLive)) {
        metricLive = nextMetrics;
        // Update value labels in place: keep keyboard focus, and avoid
        // reloading uploaded artwork from the local server on each poll.
        updateMetricPreview();
        const selectedMetric = assignment(selected)?.metric;
        if (selectedMetric) {
          metricStatus.textContent = Number.isFinite(metricLive[selectedMetric])
            ? 'Live sensor reading available.'
            : 'No current reading for this metric.';
        }
      }
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
      await loadIconSets();
      renderPages();
      renderGrid();
      renderInspector();
      saveButton.disabled = false;
      setMessage('Ready. Choose a key, then Save & Apply.');
    } catch (error) {
      setMessage(`Cannot load Deck layout: ${error.message}`, 'error');
    }
    refreshStatus();
    loadMetricSources();
    setInterval(refreshStatus, 3000);
  }

  load();
})();
