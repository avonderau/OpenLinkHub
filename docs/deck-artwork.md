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
  deduplicated by SHA-256, and limited to 256 stored images even when
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
