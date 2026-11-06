#!/bin/sh
# System-package lifecycle hook shared by Debian and RPM.
# Never erase a user's saved device profiles or Nexus layouts.
set -eu
base=/opt/OpenLinkHub
# Never follow a pre-existing installation symlink during privileged package
# setup, even when migrating from a writable upstream/manual installation.
for entry in "$base" "$base/database" "$base/runtime" \
  "$base/runtime/nexus-desktop" "$base/config.json"; do
  if [ -L "$entry" ]; then
    echo "Refusing a symlink in package installation path: $entry" >&2
    exit 1
  fi
done

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

# The Nexus desktop agent runs in each user's graphical session and needs
# that account in the openlinkhub group. Installation never grants this by
# itself, so tell the administrator exactly how to opt an account in.
# SUDO_USER is only used to word the hint; no account is changed here.
agent_user=${SUDO_USER:-}
if [ -n "$agent_user" ] && [ "$agent_user" != root ] &&
  id -nG "$agent_user" 2>/dev/null | tr ' ' '\n' | grep -qx openlinkhub; then
  cat <<NOTE

OpenLinkHub Nexus: $agent_user is already in the openlinkhub group. The
desktop agent starts automatically when you log in to the desktop.
NOTE
else
  who=${agent_user:-<desktop user>}
  [ "$who" = root ] && who='<desktop user>'
  cat <<NOTE

OpenLinkHub Nexus: to use launch actions (URLs and apps on Nexus/Deck keys),
authorize your desktop account for the desktop agent:

  sudo usermod -aG openlinkhub $who

Then log out of the desktop completely and log back in. The agent starts
automatically at login. Check with: id -nG   (openlinkhub should appear)
See /opt/OpenLinkHub/docs/nexus-desktop-launch.md or the project docs.
NOTE
fi
