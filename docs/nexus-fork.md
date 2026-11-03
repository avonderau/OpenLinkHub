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

Original project: Nikola Jurkovic and contributors. Fork modifications: contributors to `avonderau/OpenLinkHub`. See repository history and the existing GPL license.
