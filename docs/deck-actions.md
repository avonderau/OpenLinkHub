# Native Deck button actions

OpenLinkHub Nexus provides its own action picker on **Deck Editor → Key**.
Available action types are internal page navigation, validated keyboard shortcuts,
media controls and saved OpenLinkHub macros. Empty action means a display-only
label.

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

Keyboard shortcuts and media controls use the existing Linux virtual keyboard;
saved macros use the shared bounded macro engine. Standalone launch-app and
open-website actions are not part of this Deck increment.

## Verification

- `go test ./src/devices/deck ./src/server -count=1`
- `node --test static/js/deck-editor.test.cjs`
- Mirabox HSV293S: assign key to Next, Previous, and Go to page; confirm
  physical LCD page updates and that reconnecting restores the selected page.
- Verify an unsaved browser edit is not discarded by a hardware page change.

## Keyboard shortcuts, media controls and saved macros

**Keyboard shortcut** remains available for a validated combination such as
`Ctrl+Alt+T` or `Super+D`. **Media control** provides Play/Pause, Previous/Next,
Stop and Volume Up/Down/Mute. Media keys display built-in playback and
volume icons on their physical LCDs and in the Deck preview; uploaded custom
key artwork takes precedence over those built-in icons. **Run saved macro** assigns a reusable macro from
the existing OpenLinkHub Macros page. The macro picker includes a Refresh
button and a link to Manage macros.

Choose the action, configure its target, then click **Save & Apply**. Press
the physical Mirabox key. Keyboard shortcuts and media controls use Nexus's
existing validated Linux virtual keyboard backend; saved macros use the
shared Nexus macro execution engine.

All three action types can be assigned to different keys in one layout.
Existing keyboard shortcut assignments remain supported.

For a combined regression check, assign a **Keyboard shortcut** to one key and
**Run saved macro** to another, then Save & Apply. Verify both actions on the
physical device and confirm their assignments persist after restarting the
installed OpenLinkHub service. Use the service account so Corsair HID devices
have the expected access permissions; a root-only test may behave differently
from the production installation.
