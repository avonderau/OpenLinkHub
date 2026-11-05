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
- Saving a Mirabox layout renders the active page's labels to static
  key images. Elgato labels save as configuration only.

The Mirabox-family reference plugin uses an **18-position, 3x6 layout**.
The physical key placement, key orientation and any special positions must
still be verified on real hardware. The old protocol cannot report true
key-release events. There is no support for long-press detection on old units.

## Not yet implemented

- Action execution, macros, folders, icon uploads, metrics and plugin hosting.
  These must be wired through a shared restricted action runtime.
- The Elgato HID image/input driver, support for Neo, Plus, pedals and encoders.
- USB hotplug/reconnection after service startup and device disconnection
  removal from the existing device registry.
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
CGO_CFLAGS_ALLOW='-fno-strict-overflow' go test ./src/devices/deck ./src/server -count=1
node --check static/js/deck-editor.js
```

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
