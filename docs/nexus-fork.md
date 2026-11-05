# Nexus customization in this fork

This is an experimental extension of [OpenLinkHub](https://github.com/jurkovic-nikola/OpenLinkHub), not an upstream release.

## Implemented foundations

- 640 × 48 custom screen layout model with validated metric, text and bar widgets.
- Screen persistence and a device mode named `custom-screens`.
- CPU/GPU, memory, network and disk telemetry sources.
- Background-image references in screen configuration, with PNG/JPEG loading and storage.
- Touch-edge navigation zones. Continuous swipe gesture detection is **not** verified.
- Automated Nexus Go tests in GitHub Actions.

## Limitations

The browser-based visual editor, configuration API and background-upload endpoint are still in development. Custom screens are not yet generally configurable through the web interface. Linux network and disk device selection is automatic and may not choose the intended device on multi-device systems.

## Background files

The renderer expects background images in `database/nexus/backgrounds/` named `<background-id>.png`, `<background-id>.jpg`, or `<background-id>.jpeg`. The screen JSON's `background` field is an ID (letters, digits, underscores and hyphens), not a filesystem path. PNG/JPEG files are limited to 4 MiB and 4096 × 4096 pixels by the loader. The display resolution is 640 × 48.

## Development and tests

```bash
go test ./src/devices/nexus -count=1
```

Hardware testing is necessary before treating touchscreen navigation and image output as production-ready.

## Credits

Original project: Nikola Jurkovic and contributors. Fork modifications: contributors to `avonderau/OpenLinkHub-Nexus`. See repository history and the existing GPL license.

## Staying compatible with upstream

Upstream is `jurkovic-nikola/OpenLinkHub` (`main`). The custom Nexus screen engine,
telemetry, rendering, touchscreen actions and configuration/API implementation
live in separate fork-owned Go files. The editor workspace lives in
`web/nexus-custom-workspace.html`; the upstream-owned `web/nexus.html` only
includes the workspace template and assets. Custom HID/refresh logic is in
`src/devices/nexus/custom_integration.go`, with minimal hooks in
`src/devices/nexus/nexus.go`.

**Small upstream-owned integration points remain:**

- `src/devices/nexus/nexus.go` initializes the custom screens, holds state,
  delegates custom HID touch reports and refreshes the custom screen.
- `src/server/server.go` registers the screen/background API endpoints.
- `web/nexus.html` includes the custom workspace and its assets.
- `README.md` contains fork-specific introduction and attribution.

Upstream may modify these files, so merge conflicts are **not guaranteed to
be avoidable**. Resolve conflicts without discarding upstream fixes or fork
integration hooks; compile and test every merged version.

The
[Upstream merge compatibility workflow](../.github/workflows/upstream-compatibility.yml)
runs weekly, on relevant pushes and on demand. It fetches upstream, attempts
a noncommitting merge, tests the Nexus and server packages and builds the app.
**It never pushes, merges or force-updates your fork.** A successful run
confirms that revision passed automated checks; it does not replace hardware
testing.

### Safely merging a new upstream version

Run these commands from a clean local checkout:

```bash
git remote add upstream https://github.com/jurkovic-nikola/OpenLinkHub.git
# If the upstream remote already exists, skip the line above.
git fetch upstream
git switch main
git pull --ff-only origin main
git switch -c upstream-sync-$(date +%Y%m%d)
git merge --no-ff upstream/main
# Resolve any conflicts, then:
CGO_CFLAGS_ALLOW='-fno-strict-overflow' go test ./src/devices/nexus ./src/server -count=1
CGO_CFLAGS_ALLOW='-fno-strict-overflow' go build -o /tmp/OpenLinkHub-upstream-check .
git push -u origin HEAD
```

Open a pull request from the integration branch into `main` after checks
and physical Nexus testing. Do not directly merge untested upstream changes
into `main`. Never force-push either repository. Do not open the pull
request until any merge conflicts are resolved.

## Custom Nexus button icons

In the Nexus editor, select a Button widget (or an older Macro widget), choose
**Custom image** under Icon, select a PNG or JPEG and click **Upload icon**.
A successful upload stores the icon and applies it to the selected widget.
Click **Save Layout** to persist that widget's icon selection.

Uploads must be 512 KiB or smaller with dimensions no greater than 512 × 512.
The server decodes, aspect-fits and normalizes every image to a transparent
32 × 32 PNG under `database/nexus/icons/`. Uploaded artwork is referenced by
a validated ID (`settings.customIcon`) rather than its source filename.
The same icon is shown through `GET /api/nexus/icon?serial=...&id=...`
in the editor and rendered on the physical display. Existing built-in vector
icons remain available, and choosing a built-in icon clears the custom
selection without deleting the uploaded image.

The storage, renderer support, upload API and UI are fork-owned files except
for the minimal API route registration in `src/server/server.go`.

## Stock Nexus screens and cycling settings

Five editable example screens are bundled as **ordinary screen configuration JSON**
files in `static/nexus/templates/`: System Overview, Performance Bars,
Network & Storage, Media Controls and Screen Navigation (animated pulse).
Their generated backgrounds are part of the Nexus renderer. The animated
pulse is procedural rather than a copied iCUE GIF.

On first setup, Nexus loads the five JSON examples as the default screen
collection. Saved user layouts always take precedence and are never
automatically replaced. Existing users can also choose a blank or stock
screen from **New screen** beside the main screen selector and choose
**Add screen**. **Replace selected** requires an explicit confirmation and
retains the selected screen's identifier. Added/replaced screens are ordinary
editable screens and do not change the device until **Save Layout** is used.

The integrated Nexus page places **Save Layout** with the firmware/cycling
controls in the device settings card. The Interval dropdown remains selectable
in all cycling modes and is retained when Manual or Off is selected; it takes
effect only in Automatic mode. Colors use the site's `--text-muted` palette.

## Animated GIF backgrounds

The Background section supports PNG, JPEG and animated GIF files (up to
4 MiB). A GIF must have a logical canvas at most 1280 × 192 pixels, at most
80 frames and at most eight million decoded canvas pixels across its frames.
Frames are decoded and composited with GIF disposal semantics, downscaled
once to the Nexus's 640 × 48 framebuffer, then retained in a two-entry
bounded background cache. Source frame delays are respected with a minimum
50 ms and a maximum 10 seconds per frame. Playback loops.

The Nexus LCD refreshes up to five times per second while displaying an
uploaded GIF and uses its normal one-second refresh for static backgrounds.
GIF animation timing is therefore quantized by the physical display update
rate. Editor previews request the original background through the
`/api/nexus/background` GET endpoint, letting the browser animate GIFs
normally. The app does not copy or bundle third-party animated artwork.

Selecting Automatic cycling enables the interval dropdown. Manual and Off
disable it, while preserving the previously selected interval in the saved
screen configuration.

PNG/JPEG backgrounds continue to work without conversion. Replacing an
existing background ID removes obsolete image extensions to avoid an old
GIF shadowing the replacement image.

## Button actions: keyboard shortcuts and saved macros

Both iCUE NEXUS and the Mirabox Deck support **Keyboard shortcut** and
**Run saved macro** actions. Shortcut actions remain individually configurable
and validated using the existing Nexus key-chord parser. Saved macros are
selected from the shared OpenLinkHub Macros library, allowing reusable
workflows on either device. Existing keyboard shortcut assignments are
preserved; no conversion or migration is required.

Deck's macro execution reuses the bounded Nexus macro runner. Page navigation,
media controls, artwork and live metrics remain available.

### Shared custom icon sets

Manage reusable packs from **Settings → Devices & Displays → Icon Pack
Library**. Import, rename, preview, and remove them there; only select
installed icons inside the Nexus and Deck editors. ZIP imports support
PNG, JPEG and static SVG icons (up to 48 images, 512 KiB each, 8 MiB ZIP).
SVG files are safely rasterized into PNG using `rsvg-convert` (install
`librsvg2-bin` on Linux); SVG markup is never stored for direct display.
Existing single-image uploads remain supported, and saved layouts store
stable icon IDs rather than archive paths.
