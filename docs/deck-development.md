# Deck Editor foundation

This feature branch introduces a **separate** editor for programmable LCD
key decks. Device discovery is integrated into OpenLinkHub's normal device
list, but the Corsair Nexus touchscreen editor is unchanged.

## Current development scope

- Mirabox HSV293S (`5548:6670`), HSV293SV3 (`6603:1014` /
  `6603:1005`), and Ajazz AKP153 (`5548:6674`): HID detection,
  press diagnostics, static JPEG image uploads, and brightness codec.
- Elgato Mini, Original, Mk2 and XL variants: **discovery-only**.
  Input and display output are not implemented yet.
- Separate device page with an adaptive LCD key grid, page creation/removal,
  key-label editor, and Save Layout.
- Validated, atomically stored JSON layouts in `database/deck/`.
- Same-origin, size-limited APIs:
  `GET /api/deck/status?serial=...` and
  `GET/PUT /api/deck/layout?serial=...`.
- Mirabox startup and reconnection replay the active saved page to the LCD.
  Saving a layout batches all key images and flushes once; the API reports
  separately whether configuration was saved and hardware display updated.
  Elgato labels save as configuration only.
- HID read errors now close stale handles and trigger a VID/PID/usage-checked
  reconnect attempt about every 1.5 seconds. The same device retains its
  profile and restores its saved images after reconnection.
- **Automatic hot-plug:** a 3-second periodic scan of the supported Deck
  vendor/product IDs discovers a new Mirabox or Ajazz after the service
  started, without restarting it. It uses the same HID interface filter and
  initialization as startup. Already-registered devices are skipped: a
  disconnected Deck keeps its existing reconnect loop and saved layout.
  Elgato devices remain discovery-only. Refresh the browser Devices page to
  show a newly added device; the service itself does not need restarting.
- Editor visuals reuse the shared OpenLinkHub Nexus charcoal/gold gradients,
  icon headers, and file-style Key/Pages tabs.

The Mirabox-family devices use an **18-position, 3x6 layout**, with
15 physical buttons and 3 display-only LCD positions (6, 12 and 18).
The old protocol cannot report true key-release events. There is no support
for long-press detection on old units.

## Enable or disable Deck support in Settings

Use **Settings → Devices & displays → Supported Devices** to find
**Mirabox HSV293S** (or another supported Deck model), change the Enabled
toggle, and click **Save**. New Deck discovery checks the updated
enable/disable list on each scan. If a device is **already connected**, the
service must still be restarted to stop controlling that existing device,
consistent with the existing Corsair device switches.

Deck exclusions are stored by vendor + product ID in `deckExclude` in
`config.json` (for example, `"5548:6670"`). The legacy `exclude`
setting remains PID-only for Corsair and is not modified by Deck toggles.
All supported Deck models appear in the Settings list even when unplugged,
so a disabled model can be re-enabled without connecting it first.

Disabling a model stops OpenLinkHub from opening/controlling that device
on the next startup. **It does not cut USB power or forcibly turn off
its LCD**; behavior of a previously displayed image after disabling is
hardware-dependent.

## Not yet implemented

- Action execution, macros, folders, icon uploads, metrics and plugin hosting.
  These must be wired through a shared restricted action runtime.
- The Elgato HID image/input driver, support for Neo, Plus, pedals and encoders.
- Automatic removal of disconnected devices from the WebUI list is not
  implemented. A previously registered Mirabox remains listed as disconnected
  and its own HID reader waits to reconnect, instead of creating duplicate
  registry entries.
- Hardware validation, cross-platform support and multiple identical legacy
  Mirabox units. Legacy units often share a fake serial number.
- Plugin-process sandboxing and OpenAction/Stream Deck SDK compatibility.

No external plugin code is executed in the system service. When an action
plugin host is added, it must run as the logged-in desktop user and use an
authenticated, permission-restricted channel rather than executing arbitrary
plugin commands as the `openlinkhub` daemon.

## Permissions and testing

The existing `99-openlinkhub.rules` file now grants access to the four
specific Mirabox/Ajazz VID/PID pairs for the OpenLinkHub service user. After
installing updated rules:

```bash
sudo udevadm control --reload-rules
sudo udevadm trigger --subsystem-match=hidraw
sudo systemctl restart OpenLinkHub
```

Other Stream Deck applications should not claim the same HID interface while
testing. Elgato devices are not opened by the initial discovery-only driver.

```bash
CGO_CFLAGS_ALLOW='-fno-strict-overflow' go test ./src/devices ./src/devices/deck ./src/server -count=1
node --check static/js/deck-editor.js
node --test static/js/deck-editor.test.cjs
```

### Manual hot-plug acceptance

1. Start OpenLinkHub with the Mirabox **unplugged** and visit Devices.
2. Plug in a supported Mirabox. After the next 3-second scan, refresh the
   Devices page and confirm the Deck is present; do not restart the service.
3. Open the editor and verify the saved layout, LCD artwork, metrics and
   physical actions work.
4. Unplug and replug the same Deck. It should reconnect without appearing
   twice or overwriting its layout.
5. If available, connect a second **different supported VID/PID model** and
   check that both appear. Identical legacy v1 models may share a fixed ID;
   this was a pre-existing hardware limitation.
6. Stop OpenLinkHub. The discovery loop must terminate before HID teardown.

The background scanner does not initialize new Corsair hardware, open
unsupported HID interfaces, or run third-party plugins. Existing Corsair
discovery remains upstream-owned.

### Reference implementations and licensing

- [Mirajazz](https://github.com/4ndv/mirajazz), MPL-2.0.
- [OpenDeck AKP153 hardware plugin](https://github.com/4ndv/opendeck-akp153),
  GPL-3.0.
- [Elgato Stream Deck Rust library](https://github.com/OpenActionAPI/rust-elgato-streamdeck).
- Original OpenLinkHub, GPL-3.0-or-later.

Mirabox wire behavior was implemented in new Go files with these projects
as protocol references. Retain upstream copyright and license obligations
if source code is copied or modified in future work.

## Upstream strategy

New models, protocol, layout logic and tests live under
`src/devices/deck/`; the editor lives in `web/deck.html` and
`static/{js,css}/deck-editor.*`. Only minimal hooks were added in
`src/devices/devices.go`, `src/common/common.go`,
`src/server/server.go`, and the existing udev rules.
