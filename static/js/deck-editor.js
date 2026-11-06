(() => {
  'use strict';

  const id = document.getElementById('deck-device-id')?.value;
  const grid = document.getElementById('deck-key-grid');
  if (!id || !grid) return;

  const keyCount = Number.parseInt(grid.dataset.keys || '0', 10);
  const columns = Number.parseInt(grid.dataset.columns || '0', 10);
  const isDisplayOnly = index => grid.dataset.family === 'mirabox' &&
    index >= 0 && index < keyCount && columns > 0 && (index + 1) % columns === 0;
  const physicalButtonCount = Array.from({ length: keyCount }, (_, i) => i)
    .filter(index => !isDisplayOnly(index)).length;
  const keyCountLabel = physicalButtonCount === keyCount
    ? `${keyCount} LCD keys`
    : `${physicalButtonCount} buttons · ${keyCount - physicalButtonCount} display-only LCDs`;
  const defaultBackKeyIndex = () => {
    for (let i = keyCount - 1; i >= 0; i -= 1) if (!isDisplayOnly(i)) return i;
    return -1;
  };
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
  const iconSetSearch = document.getElementById('deck-icon-set-search');
  const iconSetApply = document.getElementById('deck-apply-icon-set');
  // A library selection is preview-only until the user assigns it.
  // Reset that preview when changing keys to avoid showing another key's icon.
  let iconPreviewCandidate = '';
  function updateArtworkPreview() {
    const assigned = selected >= 0 ? assignment(selected)?.icon : '';
    const icon = validIconID(iconPreviewCandidate) ? iconPreviewCandidate : assigned;
    const hasIcon = validIconID(icon);
    artworkPreview.dataset.hasArtwork = String(hasIcon);
    artworkPreview.style.backgroundImage = hasIcon ? `url("${iconURL(icon)}")` : '';
    artworkPreview.setAttribute('aria-label', validIconID(iconPreviewCandidate)
      ? 'Selected library icon preview (not applied yet)'
      : 'Selected key artwork preview');
  }
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
  const displayOnlyNote = document.getElementById('deck-display-only-note');
  const deckControlInput = document.getElementById('deck-key-control');
  const deckControlProperty = document.getElementById('deck-control-property');
  const brightnessProperty = document.getElementById('deck-brightness-property');
  const brightnessInput = document.getElementById('deck-key-brightness');
  const deckControls = new Set(['page-next','page-previous','page-go','folder-open','folder-back','brightness-up','brightness-down','brightness-set']);
  const folderTargetProperty = document.getElementById('deck-folder-target-property');
  const folderTargetInput = document.getElementById('deck-key-folder');
  const createFolderButton = document.getElementById('deck-create-folder');
  const pageParentNote = document.getElementById('deck-page-parent');
  const shortcutInput = document.getElementById('deck-key-shortcut');
  const shortcutProperty = document.getElementById('deck-hotkey-property');
  const macroProperty = document.getElementById('deck-macro-property');
  const macroInput = document.getElementById('deck-key-macro');
  const macroStatus = document.getElementById('deck-macro-status');
  const macroRefreshButton = document.getElementById('deck-refresh-macros');
  const mediaInput = document.getElementById('deck-key-media');
  const mediaProperty = document.getElementById('deck-media-property');
  const urlProperty = document.getElementById('deck-url-property');
  const urlInput = document.getElementById('deck-key-url');
  const applicationProperty = document.getElementById('deck-application-property');
  const applicationInput = document.getElementById('deck-key-application');
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
  const childFolders = () => layout?.pages?.filter(page => page.parentPage === layout.activePage) || [];
  const pageDepth = page => {
    let depth = 0;
    while (page?.parentPage && depth < (layout?.pages?.length || 0)) {
      page = layout.pages.find(candidate => candidate.id === page.parentPage);
      depth += 1;
    }
    return depth;
  };
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
      'folder-open': 'Folder',
      'folder-back': 'Back',
      'brightness-up': 'Brightness +',
      'brightness-down': 'Brightness -',
      'brightness-set': 'Set Brightness',
      hotkey: 'Shortcut',
      macro: 'Run Macro',
      media: 'Media',
      url: 'Open Website',
      application: 'Launch App',
    }[action] || '';
  }

  function setKeyProperties(patch) {
    const page = currentPage();
    if (!page || selected < 0) return;
    const key = { index: selected, ...assignment(selected), ...patch };
    if (isDisplayOnly(selected)) {
      // Passive LCDs may display artwork and metrics, but pressing them
      // is impossible. Never persist an action or its stale targets.
      delete key.action;
      delete key.target;
      delete key.targetPage;
    }
    if (!key.action || key.action === 'none') delete key.action;
    if (!['page-go','folder-open'].includes(key.action)) delete key.targetPage;
    if (!['macro','media','hotkey','brightness-set','url','application'].includes(key.action)) delete key.target;
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
      const folderLabel = configured?.action === 'folder-open'
        ? layout.pages.find(page => page.id === configured.targetPage)?.name || 'Folder' : '';
      const displayName = configured?.label || mediaLabel || brightnessLabel || folderLabel || actionName(configured?.action) || '—';
      button.setAttribute('aria-label', `${isDisplayOnly(i) ? 'Display' : 'Key'} ${i + 1}: ${displayName}`);
      if (isDisplayOnly(i)) {
        button.dataset.displayOnly = 'true';
        button.title = 'Display-only LCD: labels, artwork and metrics; no button actions';
      }

      const number = document.createElement('small');
      number.textContent = String(i + 1).padStart(2, '0');
      const label = document.createElement('span');
      label.className = 'deck-key-caption';
      const hasArtwork = validIconID(configured?.icon);
      // Mirabox renders a caption band only when a real label exists.
      // A standalone image has no placeholder text covering the artwork.
      const caption = displayName === '—' ? '' : displayName;
      label.textContent = hasArtwork ? caption : displayName;
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
      if (['folder-open','folder-back'].includes(configured?.action) && !configured?.metric &&
          !validIconID(configured?.icon)) {
        const glyph = document.createElement('span');
        glyph.className = 'deck-media-glyph';
        glyph.style.maskImage = `url("/static/media-glyphs/${configured.action}.svg?v=2")`;
        glyph.style.webkitMaskImage = glyph.style.maskImage;
        glyph.setAttribute('aria-hidden', 'true');
        button.appendChild(glyph);
        button.dataset.hasFolderIcon = 'true';
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
      if (hasArtwork) {
        button.style.backgroundImage = `url("${iconURL(configured.icon)}")`;
        button.dataset.hasImage = 'true';
        if (caption) button.dataset.hasCaption = 'true';
      }
      button.addEventListener('click', () => {
        selected = i;
        iconPreviewCandidate = '';
        renderGrid();
        renderInspector();
        selectTab('key');
        labelInput.focus();
      });
      grid.appendChild(button);
    }
    keyCounter.textContent = `${keyCountLabel} · ${page.name}`;
  }

  function renderPages() {
    if (!layout) return;
    pageSelect.replaceChildren();
    for (const page of layout.pages) {
      const option = document.createElement('option');
      option.value = page.id;
      option.textContent = (page.parentPage ? '  '.repeat(pageDepth(page)) + '↳ ' : '') + page.name;
      pageSelect.appendChild(option);
    }
    pageSelect.value = layout.activePage;
    pageNameInput.value = currentPage()?.name || '';
    const parent = layout.pages.find(page => page.id === currentPage()?.parentPage);
    pageParentNote.textContent = parent ? `Folder within ${parent.name}` : 'Top-level page';
    pageCounter.textContent = `${layout.pages.length} of 16 pages`;
    addButton.disabled = layout.pages.length >= 16;
    removeButton.disabled = layout.pages.length <= 1 ||
      (!currentPage()?.parentPage && layout.pages.filter(page => !page.parentPage).length <= 1);
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
    updateArtworkPreview();
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
    const passiveLCD = valid && isDisplayOnly(selected);
    displayOnlyNote.hidden = !passiveLCD;
    actionInput.disabled = !valid || passiveLCD;
    actionInput.value = passiveLCD ? '' : (deckControls.has(key?.action) ? 'deck-control' : key?.action || '');
    actionIndicator.textContent = passiveLCD ? 'Display only' : (actionInput.value ? 'Assigned' : 'None');
    actionIndicator.dataset.active = String(!passiveLCD && !!actionInput.value);
    const isDeckControl = valid && !passiveLCD && actionInput.value === 'deck-control';
    deckControlProperty.hidden = !isDeckControl;
    deckControlInput.disabled = !isDeckControl;
    deckControlInput.value = deckControls.has(key?.action) ? key.action : 'page-next';
    brightnessProperty.hidden = !isDeckControl || deckControlInput.value !== 'brightness-set';
    brightnessInput.disabled = brightnessProperty.hidden;
    brightnessInput.value = deckControlInput.value === 'brightness-set' ? key.target || '50' : '50';
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
    const hasTarget = isDeckControl && deckControlInput.value === 'page-go';
    targetProperty.hidden = !hasTarget;
    targetPageInput.disabled = !hasTarget;
    const hasFolder = isDeckControl && deckControlInput.value === 'folder-open';
    folderTargetProperty.hidden = !hasFolder;
    const folders = childFolders();
    folderTargetInput.replaceChildren();
    if (!folders.length) {
      const empty = document.createElement('option');
      empty.value = '';
      empty.textContent = 'Create a folder to begin';
      folderTargetInput.appendChild(empty);
    }
    for (const folder of folders) {
      const option = document.createElement('option');
      option.value = folder.id;
      option.textContent = folder.name;
      folderTargetInput.appendChild(option);
    }
    folderTargetInput.value = hasFolder ? key?.targetPage || folders[0]?.id || '' : '';
    folderTargetInput.disabled = !hasFolder || folders.length === 0;
    createFolderButton.disabled = !hasFolder || !valid || passiveLCD || layout.pages.length >= 16;
    const hasShortcut = valid && actionInput.value === 'hotkey';
    shortcutProperty.hidden = !hasShortcut;
    shortcutInput.disabled = !hasShortcut;
    shortcutInput.value = hasShortcut ? key?.target || '' : '';
    const hasMedia = valid && actionInput.value === 'media';
    mediaProperty.hidden = !hasMedia;
    mediaInput.disabled = !hasMedia;
    mediaInput.value = hasMedia ? key?.target || 'play-pause' : 'play-pause';
    const hasURL = valid && actionInput.value === 'url';
    urlProperty.hidden = !hasURL;
    urlInput.disabled = !hasURL;
    urlInput.value = hasURL ? key?.target || '' : '';
    const hasApplication = valid && actionInput.value === 'application';
    applicationProperty.hidden = !hasApplication;
    applicationInput.disabled = !hasApplication;
    applicationInput.value = hasApplication ? key?.target || '' : '';
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
    selectedText.textContent = valid ? `${passiveLCD ? 'Display' : 'Key'} ${selected + 1}` : 'Select a key';
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
      iconPreviewCandidate = '';
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
    const query=(iconSetSearch?.value || '').trim().toLocaleLowerCase();
    const matches=(pack?.icons || []).filter(icon=>!query || icon.name.toLocaleLowerCase().includes(query) || (icon.tags||[]).some(tag=>tag.toLocaleLowerCase().includes(query)));
    for (const icon of matches.slice(0,150)) {
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
  iconSetSelect.addEventListener('change', () => {
    iconPreviewCandidate = '';
    if(iconSetSearch) iconSetSearch.value='';
    refreshIconSetPicker();
    updateArtworkPreview();
  });
  iconSetSearch?.addEventListener('input', () => {
    iconPreviewCandidate = '';
    refreshIconSetPicker();
    updateArtworkPreview();
  });
  iconSetItem.addEventListener('change', () => {
    iconPreviewCandidate = validIconID(iconSetItem.value) ? iconSetItem.value : '';
    iconSetApply.disabled = selected < 0 || !validIconID(iconSetItem.value);
    updateArtworkPreview();
  });
  iconSetApply.addEventListener('click', () => {
    if (!validIconID(iconSetItem.value) || selected < 0) return;
    setKeyProperties({ icon: iconSetItem.value });
    iconPreviewCandidate = '';
    renderInspector();
    setMessage('Icon set artwork assigned. Save & Apply to update the LCD.', 'success');
  });
  removeArtworkButton.addEventListener('click', () => {
    setKeyProperties({ icon: '' });
    iconPreviewCandidate = '';
    artworkInput.value = '';
    renderInspector();
  });

  actionInput.addEventListener('change', () => {
    if (isDisplayOnly(selected)) {
      setMessage('This Mirabox position is display-only; select a physical button for actions.', 'error');
      renderInspector();
      return;
    }
    const selectedAction = actionInput.value;
    // The UI exposes one "Deck controls" action family; layouts retain
    // the existing concrete page/brightness action IDs for compatibility.
    const resolvedAction = selectedAction === 'deck-control'
      ? (deckControls.has(assignment(selected)?.action) ? assignment(selected).action : 'page-next')
      : selectedAction;
    setKeyProperties({
      action: resolvedAction,
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
      ...(['url', 'application'].includes(actionInput.value)
        ? { target: assignment(selected)?.action === actionInput.value ? assignment(selected)?.target || '' : '' }
        : {}),
    });
    renderInspector();
  });

  deckControlInput.addEventListener('change', () => {
    if (actionInput.value !== 'deck-control') return;
    const value = deckControlInput.value;
    if (value === 'folder-back' && !currentPage()?.parentPage) {
      deckControlInput.value = assignment(selected)?.action || 'page-next';
      setMessage('Back is available only on a folder page.', 'error');
      return;
    }
    const existing = assignment(selected)?.targetPage;
    const children = childFolders();
    setKeyProperties({
      action: value,
      targetPage: value === 'page-go' ? existing || layout.activePage :
        value === 'folder-open' ? (children.some(page => page.id === existing) ? existing : children[0]?.id || '') : '',
      target: value === 'brightness-set' ? '50' : '',
    });
    renderInspector();
    if (value === 'folder-open' && !children.length) {
      setMessage('Create a folder to complete this button action.');
    }
  });
  brightnessInput.addEventListener('change', () => {
    if (actionInput.value !== 'deck-control' || deckControlInput.value !== 'brightness-set') return;
    setKeyProperties({ target: brightnessInput.value });
  });
  targetPageInput.addEventListener('change', () => {
    if (actionInput.value !== 'deck-control' || deckControlInput.value !== 'page-go') return;
    setKeyProperties({ targetPage: targetPageInput.value });
  });
  folderTargetInput.addEventListener('change', () => {
    if (actionInput.value !== 'deck-control' || deckControlInput.value !== 'folder-open') return;
    setKeyProperties({ targetPage: folderTargetInput.value });
  });

  createFolderButton.addEventListener('click', () => {
    if (!layout || selected < 0 || isDisplayOnly(selected) || layout.pages.length >= 16 ||
        actionInput.value !== 'deck-control' || deckControlInput.value !== 'folder-open') return;
    const parent = currentPage();
    if (!parent) return;
    let index = 1;
    while (layout.pages.some(page => page.id === `folder-${index}`)) index += 1;
    const backIndex = defaultBackKeyIndex();
    if (backIndex < 0) return;
    const folder = {
      id: `folder-${index}`, name: `Folder ${index}`, parentPage: parent.id,
      keys: [{ index: backIndex, label: '', action: 'folder-back' }],
    };
    layout.pages.push(folder);
    // The selected key becomes a folder button; all previous metric/button
    // targets are cleared instead of being carried into the new folder.
    setKeyProperties({
      action: 'folder-open', targetPage: folder.id, target: '',
      metric: '', label: '', icon: '',
    });
    layout.activePage = folder.id;
    selected = -1;
    renderPages();
    renderGrid();
    renderInspector();
    selectTab('pages');
    pageNameInput.focus();
    setDirty();
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

  urlInput.addEventListener('input', () => {
    if (actionInput.value === 'url') setKeyProperties({ target: urlInput.value });
  });
  applicationInput.addEventListener('input', () => {
    if (actionInput.value === 'application') setKeyProperties({ target: applicationInput.value });
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
    if (option) option.textContent =
      (page.parentPage ? '  '.repeat(pageDepth(page)) + '↳ ' : '') + page.name;
    setDirty();
    keyCounter.textContent = `${keyCountLabel} · ${page.name}`;
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
    if (!layout || removeButton.disabled) return;
    const deletedPage = layout.activePage;
    const fallbackParent = currentPage()?.parentPage;
    const removed = new Set([deletedPage]);
    // Removing a parent also removes its descendant folders to avoid
    // dangling parent pointers and Back navigation to deleted pages.
    for (let changed = true; changed;) {
      changed = false;
      for (const page of layout.pages) {
        if (page.parentPage && removed.has(page.parentPage) && !removed.has(page.id)) {
          removed.add(page.id);
          changed = true;
        }
      }
    }
    const description = removed.size > 1
      ? `Delete this page and its ${removed.size - 1} nested folder(s)?`
      : 'Delete the selected page and all its keys?';
    if (!window.confirm(description)) return;
    layout.pages = layout.pages.filter(page => !removed.has(page.id));
    for (const page of layout.pages) {
      for (const key of page.keys) {
        if (['page-go', 'folder-open'].includes(key.action) && removed.has(key.targetPage)) {
          delete key.action;
          delete key.targetPage;
        }
      }
      page.keys = page.keys.filter(key => key.label || key.action || key.icon || key.metric);
    }
    layout.activePage = layout.pages.some(page => page.id === fallbackParent)
      ? fallbackParent : layout.pages.find(page => !page.parentPage).id;
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
