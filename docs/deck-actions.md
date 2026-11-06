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

## Nested folders and Back navigation

Folders are ordinary Deck pages with an optional saved `parentPage` relationship.
Each folder is created under one parent page and can contain another folder,
up to the existing 16-page limit. Layout version 1 remains compatible.

1. Select a **key** on the parent page.
2. Choose **Deck controls → Open folder**.
3. Click **Create folder**. The editor makes a child page, links the selected
   key to it, and automatically adds a **Back** key at physical button **17**.
4. Rename the folder in the **Pages** tab and assign buttons to its empty keys.
   To create another level, select a key inside that folder and repeat.
5. Click **Save & Apply** before testing the physical Deck.

**Open folder** can also link to an existing *direct child* through the Folder
destination picker. **Deck controls → Back to parent folder** may be assigned to
a different key inside a folder; it is not available on top-level pages. The
Back key returns to the saved parent even after reconnecting or restarting
OpenLinkHub. Folder buttons and Back buttons use built-in LCD icons unless
custom artwork is assigned. A folder button without a custom label displays
the destination folder's current name.

Removing a folder in **Pages** also removes its nested child folders and clears
links to the deleted pages. Removing a top-level page with children does the
same; the last remaining top-level page cannot be removed.

### Folder acceptance checks

- Create **Home → Games → Tools**, add a button to Tools, and save.
- On the physical Deck, open Games, then Tools, then press Back twice. Confirm
  the LCD updates on every transition.
- Unplug/replug or restart while inside Tools; Back must still go to Games.
- Rename a folder and confirm the parent button's default caption updates.
- Verify custom folder artwork, macros, media keys, brightness buttons and
  normal page navigation are unchanged.
- Delete Games in the editor and confirm its nested pages and links are removed.


### Mirabox display-only cells (physical layout)

On the supported **3×6 Mirabox-style hardware**, LCD positions **6, 12 and 18**
(the rightmost LCD in each row) are **not pressable buttons**. They remain
available for images, labels and live metrics, and the Deck Editor marks
them `LCD` with a dashed border. Button actions cannot be assigned to
these three cells.

The automatic folder **Back** action is on physical button **17**
(zero-based key index `16`), not on display-only position 18. The Go
device model and HID event decoder both enforce this distinction; other
families such as Elgato retain their own button geometry.

Previously saved layouts that put Back on display 18 are migrated when
loaded: Back moves to button 17 when free, or the closest free physical
button if it is occupied. Artwork and metric assignments on passive
displays remain intact. A corrected in-process left arrow and matching
preview SVG replace the earlier malformed Back glyph.
