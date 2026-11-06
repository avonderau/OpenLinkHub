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
saved macros use the shared bounded macro engine. Website and desktop-application launch actions are also available, using
the same validated local launcher as the Nexus.

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


## Open website and launch desktop application

Under **Deck Editor → Key → Button behavior → Button action**:

- Choose **Open website** and enter a full `https://` or `http://` URL (for example `https://example.com`).
- Choose **Launch desktop application** and enter a desktop-entry ID installed on this Linux host (for example `firefox.desktop` or `org.gnome.Terminal.desktop`). Do not enter a filesystem path, shell command, or arbitrary executable.
- Click **Save & Apply**, then press the physical Mirabox key. The action runs on the **OpenLinkHub host**, never in the remote browser.

The shared Nexus launcher enforces strict URL and desktop ID validation, a maximum four-second local command timeout, and a single simultaneous launch across Nexus and Deck. Deck executes launch actions asynchronously so the HID input loop remains responsive. A launch failure appears in the Deck connection/status message.

### System service / headless behavior

If OpenLinkHub runs as the packaged `openlinkhub` system service, it cannot launch into your desktop session by itself. Start the optional **Nexus desktop agent** on the *same machine's graphical desktop*, as described in [Nexus desktop launch](nexus-desktop-launch.md). The same agent handles both Nexus and Deck buttons; no second agent or API endpoint is needed. Without a local desktop and a running agent, a launch action will fail safely. Do not add other users to the privileged `openlinkhub` group unless you trust them.

### Regression tests

`go test ./src/devices/deck ./src/devices/nexus -count=1` and `node --test static/js/deck-editor.test.cjs`. On physical hardware, verify that launching a trusted website and an installed desktop entry works, bad IDs are refused when saving, and holding/pressing other keys while a slow launcher runs does not freeze display navigation.
