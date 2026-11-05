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

No desktop programs, commands, hotkeys, or plugins run in this increment.
Future desktop actions must be delivered via the logged-in user session,
not executed as the system service.

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
