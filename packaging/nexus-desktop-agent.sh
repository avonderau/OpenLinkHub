#!/bin/sh
# Run only inside an authorized, logged-in graphical desktop session.
set -eu
if [ "$(id -u)" -eq 0 ]; then
  echo "Nexus desktop agent must not run as root" >&2
  exit 1
fi
if [ -z "${DISPLAY:-}" ] && [ -z "${WAYLAND_DISPLAY:-}" ]; then
  echo "Nexus desktop agent requires a graphical desktop session" >&2
  exit 1
fi
case " $(id -nG) " in
  *" openlinkhub "*) ;;
  *)
    echo "Nexus desktop agent: user $(id -un) is not in the openlinkhub group. An administrator must run: sudo usermod -aG openlinkhub $(id -un); then log out and back in." >&2
    exit 1
    ;;
esac
cd /opt/OpenLinkHub
exec /opt/OpenLinkHub/OpenLinkHub --nexus-desktop-agent --socket /opt/OpenLinkHub/runtime/nexus-desktop/desktop-launch.sock
