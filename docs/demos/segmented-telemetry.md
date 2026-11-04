# Segmented telemetry bar demo

This is a **development preview**, not a new published release or an
upgrade to the `0.9.4N` package. It reproduces the Nexus charcoal/amber
segmented telemetry bar across all pages that show system temperatures.

## Build and download

Find the **Nexus telemetry demo build** workflow under the open PR's
**Checks** tab or the [GitHub Actions page](https://github.com/avonderau/OpenLinkHub-Nexus/actions/workflows/nexus-telemetry-demo.yml).
A successful job uploads a ZIP artifact named
`openlinkhub-nexus-telemetry-demo` containing:
- `OpenLinkHub-Nexus-0.9.4N-telemetry-demo-linux-amd64.tar.gz`: independent
  full Linux amd64 demo bundle compiled from this branch.
- `OpenLinkHub-Nexus-telemetry-demo-overlay.tar.gz`: frontend files that
  can be copied over an existing `0.9.4N` package install for rapid review.
- Matching `.sha256` files.

The Actions ZIP is a CI artifact, **not** a GitHub Release download. GitHub
login may be required to access workflow artifacts. No existing release tag
or published native package is altered.

## Try the frontend overlay on Linux Mint/Ubuntu

Download and extract the Actions artifact ZIP, then from that directory run:

```bash
sha256sum -c OpenLinkHub-Nexus-telemetry-demo-overlay.tar.gz.sha256

TMP="$(mktemp -d)"
tar -xzf OpenLinkHub-Nexus-telemetry-demo-overlay.tar.gz -C "$TMP"

sudo install -m 644 "$TMP/web/head.html" /opt/OpenLinkHub/web/head.html
sudo install -m 644 "$TMP/web/login.html" /opt/OpenLinkHub/web/login.html
sudo install -m 644 "$TMP/web/temperature-bar.html" /opt/OpenLinkHub/web/temperature-bar.html
sudo install -m 644 "$TMP/static/css/openlinkhub-telemetry.css" /opt/OpenLinkHub/static/css/openlinkhub-telemetry.css
sudo install -m 644 "$TMP/static/js/temperature-bar.js" /opt/OpenLinkHub/static/js/temperature-bar.js
rm -rf "$TMP"
sudo systemctl restart OpenLinkHub.service
```

Open `http://127.0.0.1:27003/` and hard-refresh the browser
(`Ctrl+Shift+R`). The system temperature bar is also shared with other
pages when the Temperature Bar setting is enabled.

The overlay modifies package-managed frontend files. A normal package
reinstallation can restore the released UI. This preview does not touch
user profiles or the database; **no backup is automatically created**.

## Quality checks

- The same `cpu_temp`, `gpu_temp_{{ .Index }}` and
  `storage_temp-{{ .Key }}` DOM identifiers are used.
- Existing CPU/GPU/storage polling endpoints continue updating every
  three seconds. Actual live Celsius/Fahrenheit readings are shown;
  no thresholds, sensor history or simulated values are presented in-app.
- Names use template-provided device information, and unit typography is
  safely generated from text.
- The CSS responds to container width, with four segments when there is
  space, two columns on intermediate widths, and compact rows on mobile.
- The browser title and login heading say **OpenLinkHub Nexus**.
- Go backend APIs, device behavior, service unit and configuration are unchanged.

## Review checklist

Compare desktop and mobile widths in a real browser. Verify live sensor
updates for CPU, all GPUs and all disks; switching between Celsius and
Fahrenheit; Settings and hardware pages; login title; and operation with
the sidebar collapsed. Test on a real system before merging this demo PR.
