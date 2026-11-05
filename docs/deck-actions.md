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
