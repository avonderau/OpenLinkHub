# Deck Editor foundation

This feature branch introduces a **separate** editor for programmable LCD
key decks. Device discovery is integrated into OpenLinkHub's normal device
list, but the Corsair Nexus touchscreen editor is unchanged.

## Current development scope

- Mirabox HSV293S (`5548:6670`), HSV293SV3 (`6603:1014` /
  `6603:1005`), and Ajazz AKP153 (`5548:6674`): HID detection,
  press diagnostics, static JPEG image uploads, and brightness codec.
- Elgato Stream Deck Original (`0fd9:0060`), Original V2 (`0fd9:006d`),
  Mini (`0fd9:0063`), Mini Mk.2 (`0fd9:0090`), MK.2 (`0fd9:0080`, scissor
  keys `0fd9:00a5`), XL (`0fd9:006c`) and XL V2 (`0fd9:008f`): key input,
  key images and brightness through the native driver in
  `src/devices/deck/elgato_protocol.go`. **Not yet hardware-tested.**
  See [Elgato Stream Deck driver](#elgato-stream-deck-driver).
- Separate device page with an adaptive LCD key grid, page creation/removal,
  key-label editor, and Save Layout.
- Validated, atomically stored JSON layouts in `database/deck/`.
- Same-origin, size-limited APIs:
  `GET /api/deck/status?serial=...` and
  `GET/PUT /api/deck/layout?serial=...`.
- Mirabox startup and reconnection replay the active saved page to the LCD.
  Saving a layout batches all key images and flushes once; the API reports
  separately whether configuration was saved and hardware display updated.
- HID read errors now close stale handles and trigger a VID/PID/usage-checked
  reconnect attempt about every 1.5 seconds. The same device retains its
  profile and restores its saved images after reconnection.
- **Automatic hot-plug:** OpenLinkHub's existing Linux Netlink USB monitor
  notifies the Deck manager when a supported VID/PID connects. Only then does
  a dedicated worker enumerate that HID device, with up to six short 500 ms
  readiness attempts if udev is still creating its interface. There is **no
  periodic USB polling**. A one-time scan after the Netlink socket binds
  covers devices connected during startup. Already-registered devices are
  skipped: a disconnected Deck keeps its existing reconnect loop and saved
  layout. Refresh the browser Devices
  page to show a newly added device; the service does not need restarting.
- Editor visuals reuse the shared OpenLinkHub Nexus charcoal/gold gradients,
  icon headers, and file-style Key/Pages tabs.

The Mirabox-family devices use an **18-position, 3x6 layout**, with
15 physical buttons and 3 display-only LCD positions (6, 12 and 18).
The old protocol cannot report true key-release events. There is no support
for long-press detection on old units.

## Enable or disable Deck support in Settings

Use **Settings → Devices & displays → Supported Devices** to find
**Mirabox HSV293S** (or another supported Deck model), change the Enabled
toggle, and click **Save**. The next Deck USB connection event consults the
updated enable/disable list. If a device is **already connected**, the
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
- Stream Deck Neo, Plus, Plus XL, Pedal, the module variants, encoders and touch strips.
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

## Elgato Stream Deck driver

The Elgato driver shares everything above the wire format with Mirabox:
layouts, pages and folders, actions, artwork, metrics, brightness keys,
hot-plug and reconnect. `deck_hardware.go` picks the protocol by family.

Three protocol generations are implemented:

| Generation | Models | Key image | Image reports |
| --- | --- | --- | --- |
| Original | Original (`0060`) | 72 px BMP, mirrored on both axes | 8191 bytes, image split in two halves; columns are addressed right-to-left |
| Mini | Mini (`0063`), Mini Mk.2 (`0090`) | 80 px BMP, rotated and mirrored (a transpose) | 1024 bytes, 16-byte header |
| V2 | Original V2 (`006d`), MK.2 (`0080`, `00a5`), XL (`006c`, `008f`) | 72 or 96 px JPEG, mirrored on both axes | 1024 bytes, 8-byte header |

Reset and brightness use HID feature reports. Elgato reports the state of
every key in each input report, so the driver fires an action when a key
changes from released to pressed; holding a key does not repeat it.
Elgato keys have no display-only positions.

The wire format was written in Go from the reference in
[rust-elgato-streamdeck](https://github.com/OpenActionAPI/rust-elgato-streamdeck)
(MPL-2.0), the library OpenDeck uses. No source was copied.

### Elgato hardware acceptance

1. Reload udev rules (above) so the `openlinkhub` user can open
   `0fd9` hidraw devices, and quit any other Stream Deck software.
2. Plug in the Stream Deck and refresh Devices. It should show as connected.
3. Save a layout with labels, artwork and a metric key. Check that every key
   shows the right image the right way up, in the right position.
4. Press each corner key and confirm the editor's last-pressed key matches.
   Hold a key and confirm its action runs once.
5. Test brightness keys, page and folder navigation, then unplug and replug.

## Permissions and testing

The existing `99-openlinkhub.rules` file grants the OpenLinkHub service
user access to the four specific Mirabox/Ajazz VID/PID pairs and the eight
supported Elgato Stream Deck product IDs. After
installing updated rules:

```bash
sudo udevadm control --reload-rules
sudo udevadm trigger --subsystem-match=hidraw
sudo systemctl restart OpenLinkHub
```

Other Stream Deck applications should not claim the same HID interface while
testing. Quit Elgato's own app, OpenDeck or any other Stream Deck software
before OpenLinkHub opens an Elgato device; only one program can own it.

```bash
CGO_CFLAGS_ALLOW='-fno-strict-overflow' go test ./src/devices ./src/devices/deck ./src/server -count=1
node --check static/js/deck-editor.js
node --test static/js/deck-editor.test.cjs
```

### Manual hot-plug acceptance

1. Start OpenLinkHub with the Mirabox **unplugged** and visit Devices.
2. Plug in a supported Mirabox. The Netlink USB-add event triggers discovery;
   allow a few seconds for the HID interface to become ready, then refresh
   Devices and confirm the Deck is present without restarting the service.
3. Open the editor and verify the saved layout, LCD artwork, metrics and
   physical actions work.
4. Unplug and replug the same Deck. It should reconnect without appearing
   twice or overwriting its layout.
5. If available, connect a second **different supported VID/PID model** and
   check that both appear. Identical legacy v1 models may share a fixed ID;
   this was a pre-existing hardware limitation.
6. Stop OpenLinkHub. The Deck event worker must exit and finish any
   in-flight HID enumeration before the HID library is shut down.

The event-driven Deck worker does not initialize new Corsair hardware, open
unsupported HID interfaces, or run third-party plugins. The existing
Corsair/SCUF Netlink monitor retains its established behavior.

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
