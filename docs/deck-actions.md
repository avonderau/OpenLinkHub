# Native Deck button actions — first increment

OpenLinkHub Nexus provides its own action picker on **Deck Editor → Key**.
The first action types are intentionally limited to internal page navigation:
`page-next`, `page-previous`, and `page-go` (with a valid `targetPage`
ID). Empty action means a display-only label.

## Configure a key

1. Add at least two pages in **Pages**, and save the layout.
2. Select a key on the Home page.
3. Under **Button action**, select **Next page** (or Previous / Go to page).
4. For Go to page, choose a destination from the dropdown.
5. Click **Save & Apply**.
6. Press the physical Mirabox key. The hardware LCD should change to the
   selected page. The browser polls the active hardware page and follows it,
   unless the browser has unsaved edits.

The active page is saved in the same per-device layout so that reconnecting
the hardware restores the selected page. If LCD rendering fails after a key
press, the key event still counts and the saved active page is retained.

The initial navigation action milestone used in-process page transitions.
Later milestones added hotkeys, media controls and an opt-in desktop
launch agent. Arbitrary shell commands and third-party plugins remain
unsupported.

## Verification

- `go test ./src/devices/deck ./src/server -count=1`
- `node --test static/js/deck-editor.test.cjs`
- Mirabox HSV293S: assign key to Next, Previous, and Go to page; confirm
  physical LCD page updates and that reconnecting restores the selected page.
- Verify an unsaved browser edit is not discarded by a hardware page change.

## Keyboard shortcuts and media keys (PR #63)

From **Deck Editor → Key → Button action**, choose either:

- **Keyboard shortcut** and enter a bounded key chord such as `Ctrl+Alt+T`,
  `Super+D` or `Ctrl+Shift+Esc`.
- **Media control** and select Play/Pause, Previous/Next, Stop,
  Volume Up/Down or Mute.

Save & Apply, then press the *physical* Mirabox key. These actions use the
existing Nexus Linux virtual keyboard via uinput. They affect the local active
graphical desktop seat, not the browser session viewing OpenLinkHub remotely.

Only validated key names and media actions are accepted. The Deck API never
runs shell commands or arbitrary executables. A non-graphical/headless host
cannot forward those keys to a separate remote desktop through this feature.

For the initial physical test, assign Play/Pause to a key, then a keyboard
shortcut you know the local desktop handles. Confirm the existing Next Page,
Previous Page and Go to Page actions still work. If no keys are emitted,
check whether the OpenLinkHub virtual keyboard is initialized and visible
with `libinput list-devices` or `sudo evtest`.

## Open URL, Launch Application and Run Saved Macro (PR #64)

The Deck Editor now shares additional **Nexus** button actions:

- **Open URL:** set a complete HTTP/HTTPS URL, such as
  `https://example.com/`. Unsupported schemes, whitespace, credentials,
  and control characters are rejected by the existing Nexus validator.
- **Launch application:** set a desktop-entry ID like
  `org.gnome.Terminal.desktop` or `firefox.desktop`. This uses
  `gtk-launch` and **never** accepts an executable path or shell command.
- **Run saved macro:** choose an existing OpenLinkHub macro in the dropdown.
  Create and edit macros at **Macros** first, then choose **Refresh** in the
  Deck Editor if you just added one. Saved IDs refer to the shared macro
  library, not a duplicate Deck-specific macro editor.

Click **Save & Apply**. These actions are triggered by physical Mirabox
key presses and never by clicking a preview key in the browser.

### Desktop agent requirement

A packaged OpenLinkHub service commonly runs as user `openlinkhub`,
outside the logged-in Mint graphical session. Like the Nexus touchscreen,
the Deck launches URLs and apps through the *opt-in local desktop agent*:

```sh
cd /opt/OpenLinkHub
./OpenLinkHub-deck-test --nexus-desktop-agent \
  --socket /opt/OpenLinkHub/runtime/nexus-desktop/desktop-launch.sock
```

Run the agent **as the signed-in desktop user**, in a separate graphical
terminal; do not use `sudo`. The runtime directory and socket remain owned
by the daemon and accessible only to authorized group members. If the local
account cannot open the socket, check `id -nG` and see the group-access
instructions in `docs/nexus-desktop-launch.md`. The agent is initialized
for Deck devices even if no Corsair Nexus is connected. It forwards only
the validated URL or desktop entry ID, never freeform commands.

If the daemon already runs as a non-root account *inside* the graphical
session, the direct local launcher can be used without the agent. A headless
machine cannot open a browser on the remote WebUI visitor's desktop.

### Macro safety

The existing Nexus macro runner loads an immutable snapshot, rejects
excessive/repeating action configurations, has a 15-second deadline,
and supports cancellation when the Deck stops. Only one macro can execute
per Deck at a time. The HID reader remains responsive and page navigation
continues while macro execution occurs. Macros and hotkeys emit local
virtual input; they do not execute shell processes.

### Manual acceptance test

1. Configure one key as **Open URL** with an HTTPS test URL, and another
   as **Launch application** with a known installed desktop-entry ID.
2. Run the opt-in desktop agent in the logged-in graphical session if
   OpenLinkHub is installed as a system service.
3. Press both physical keys and confirm they launch *locally*.
4. Create or choose a short saved macro (e.g. one harmless keyboard key),
   refresh the list, select it on a key and verify it executes once.
5. Verify that repeated presses don't run overlapping macros; existing
   page navigation, LCD updates, hotkeys and media buttons still work.
6. Unplug/replug, restart the service, and verify the saved assignments
   remain present. Invalid command paths must be rejected at Save & Apply.
