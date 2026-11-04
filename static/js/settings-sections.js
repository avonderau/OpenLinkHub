/* OpenLinkHub Nexus Settings: presentation-only category tabs.
 * Original settings.js still owns all controls and API calls.
 */
(function () {
    'use strict';

    function initSettingsSections() {
        const root = document.querySelector('.olh-settings-shell');
        if (!root) return;
        const tabs = Array.from(root.querySelectorAll('[role="tab"]'));
        if (tabs.length === 0) return;

        function adjustDeviceTable() {
            // DataTables can initialize while its tab is hidden. Recalculate
            // column widths only when the Devices panel becomes visible.
            if (!window.jQuery || !window.jQuery.fn ||
                !window.jQuery.fn.DataTable ||
                !window.jQuery.fn.DataTable.isDataTable('#dataTable')) return;
            window.jQuery('#dataTable').DataTable().columns.adjust();
        }

        function selectTab(tab, focus, remember) {
            if (!tabs.includes(tab)) return;
            const selectedId = tab.getAttribute('aria-controls');
            tabs.forEach(function (item) {
                const active = item === tab;
                item.setAttribute('aria-selected', String(active));
                item.tabIndex = active ? 0 : -1;
                const panel = document.getElementById(item.getAttribute('aria-controls'));
                if (panel) panel.hidden = !active;
            });

            if (remember && window.history && window.history.replaceState) {
                window.history.replaceState(null, '', '#' + selectedId);
            }
            if (focus) tab.focus();
            if (selectedId === 'olh-settings-devices') {
                if (typeof window.requestAnimationFrame === 'function') {
                    window.requestAnimationFrame(adjustDeviceTable);
                } else {
                    adjustDeviceTable();
                }
            }
        }

        tabs.forEach(function (tab, index) {
            tab.addEventListener('click', function () {
                selectTab(tab, false, true);
            });
            tab.addEventListener('keydown', function (event) {
                let next = index;
                switch (event.key) {
                    case 'ArrowRight':
                    case 'ArrowDown': next = (index + 1) % tabs.length; break;
                    case 'ArrowLeft':
                    case 'ArrowUp': next = (index + tabs.length - 1) % tabs.length; break;
                    case 'Home': next = 0; break;
                    case 'End': next = tabs.length - 1; break;
                    default: return;
                }
                event.preventDefault();
                selectTab(tabs[next], true, true);
            });
        });

        // Deep-link to a Settings category, including after a settings save
        // refresh, without placing secrets or form values in the URL.
        const hash = decodeURIComponent(window.location.hash.slice(1));
        const initial = tabs.find(function (tab) {
            return tab.getAttribute('aria-controls') === hash;
        }) || tabs[0];
        selectTab(initial, false, false);
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initSettingsSections);
    } else {
        initSettingsSections();
    }
})();
