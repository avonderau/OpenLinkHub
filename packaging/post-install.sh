#!/bin/sh
# System-package lifecycle hook shared by Debian and RPM.
# Never erase a user's saved device profiles or Nexus layouts.
set -eu
base=/opt/OpenLinkHub

if ! getent group openlinkhub >/dev/null 2>&1; then
  groupadd --system openlinkhub
fi
if ! id -u openlinkhub >/dev/null 2>&1; then
  useradd --system --gid openlinkhub --no-create-home \
    --home-dir "$base" --shell /usr/sbin/nologin openlinkhub
fi

# Keep the application binary and web assets root-owned. Only the config,
# device data and desktop-bridge runtime folder need service write access.
chown root:root "$base"
chmod 0755 "$base"
install -d -o openlinkhub -g openlinkhub -m 0750 \
  "$base/runtime" "$base/runtime/nexus-desktop"
install -d -o openlinkhub -g openlinkhub -m 0750 "$base/database"
chown -R openlinkhub:openlinkhub "$base/database"

# The app normally generates this file at startup, but a package-installed
# executable tree is read-only to its service account. Seed it once, leaving
# existing user configuration untouched on upgrades.
if [ ! -e "$base/config.json" ]; then
  cat > "$base/config.json" <<'CONFIG'
{
  "debug": false,
  "listenPort": 27003,
  "listenAddress": "127.0.0.1",
  "frontend": true,
  "metrics": false,
  "memory": false,
  "memorySmBus": "i2c-0",
  "memoryType": 5,
  "exclude": [],
  "resumeDelay": 15000,
  "graphProfiles": true,
  "ramTempViaHwmon": true,
  "nvidiaGpuIndex": [0],
  "enableGamepad": true,
  "appMonitor": false
}
CONFIG
fi
chown openlinkhub:openlinkhub "$base/config.json"
chmod 0600 "$base/config.json"

if command -v udevadm >/dev/null 2>&1; then
  udevadm control --reload-rules || true
fi
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
  if [ -d /run/systemd/system ]; then
    if ! systemctl enable --now OpenLinkHub.service; then
      echo "OpenLinkHub Nexus: service did not start; check systemctl status OpenLinkHub.service" >&2
    fi
  fi
fi
