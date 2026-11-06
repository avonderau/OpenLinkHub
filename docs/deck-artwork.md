# Custom Deck key artwork

Select a key in the Deck Editor, choose a PNG/JPEG image (up to 512 KiB and
512 × 512 pixels), then press **Upload artwork**. Uploading stores a private,
normalized 128 × 128 PNG and inserts a content-addressed icon ID into that
key's layout. Press **Save & Apply** to send it to the Mirabox LCD; upload
alone does not apply an unsaved layout.

- Existing labels/actions are preserved. The optional label is drawn over
  a dark band at the bottom of the image.
- Artwork-only keys (no label or action) are supported.
- **Remove artwork** removes the icon reference from the selected key; the
  saved file remains available to another layout using the same image.
- The image preview is loaded through a same-origin API that validates the
  device ID and icon ID, not an arbitrary filesystem path.
- Files are private to `database/deck/icons/` with restrictive permissions,
  deduplicated by SHA-256, and limited to 8192 stored images even when
  multiple uploads arrive simultaneously; repeated images do not consume slots.
- Images are bounded and decoded once before normalization. SVG, GIF and
  animated output are not included in this first milestone.
- Disconnected devices can save layouts and restore icon assignments on
  reconnect. Elgato devices remain discovery-only and cannot display
  artwork yet.

## Regression coverage

```sh
go test ./src/devices/deck ./src/server -count=1
node --test static/js/deck-editor.test.cjs
```

Manual smoke test on HSV293S: upload a PNG and JPEG, verify previews and
labels, Save & Apply, confirm the physical key shows the image, reconnect
the device, and verify the saved page and image restore correctly.

### Reusable icon sets

Manage packs under **Settings → Devices & Displays → Icon Pack Library**.
Import a ZIP containing up to 2,048 PNG, JPEG or static SVG icons (up to 512 KiB
per image, 32 MiB ZIP size and 64 MiB extracted-data limit). Settings supports renaming and removing
packs and viewing each icon. Deleting a pack keeps previously used artwork
in saved layouts. In the Deck editor, choose a pack and icon in
**Reusable icon sets**, then Save & Apply.
The same set appears in the Nexus editor and can be reused on multiple keys
or widgets. Uploaded icons are normalized and saved privately, not served from
the original ZIP or extracted as arbitrary filesystem paths. Deleting a set
manifest does not remove icons referenced by saved layouts.

To enable SVG icon import on Debian/Ubuntu/Linux Mint, install:

```sh
sudo apt install librsvg2-bin
```

The backend validates imported SVG markup and renders it into a bounded PNG
before device storage. SVG with scripts, DTDs, embedded images or external
resources is rejected. PNG/JPEG archives do not require SVG utilities.
