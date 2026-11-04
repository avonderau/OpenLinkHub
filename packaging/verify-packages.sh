#!/usr/bin/env bash
set -euo pipefail
version="$1"; archive="$2"; output="$3"
deb="$output/OpenLinkHub-Nexus_$version"_"amd64.deb"
rpm="$output/OpenLinkHub-Nexus-$version-1.x86_64.rpm"
bundle="OpenLinkHub-Nexus-$version-linux-amd64"
test -s "$deb" && test -s "$rpm"

test "$(dpkg-deb --field "$deb" Package)" = openlinkhub-nexus
test "$(dpkg-deb --field "$deb" Version)" = "$version"
test "$(dpkg-deb --field "$deb" Architecture)" = amd64
test "$(rpm -qp --queryformat '%{NAME}' "$rpm")" = openlinkhub-nexus
test "$(rpm -qp --queryformat '%{VERSION}' "$rpm")" = "$version"
test "$(rpm -qp --queryformat '%{ARCH}' "$rpm")" = x86_64

work="$(mktemp -d)"; trap 'rm -rf "$work"' EXIT
mkdir -p "$work/source" "$work/deb" "$work/rpm"
tar -xzf "$archive" -C "$work/source" --no-same-owner "$bundle/OpenLinkHub"
original="$work/source/$bundle/OpenLinkHub"
dpkg-deb --extract "$deb" "$work/deb"
rpm2cpio "$rpm" | (cd "$work/rpm" && cpio -idmu --quiet)
for root in "$work/deb" "$work/rpm"; do
  test -f "$root/opt/OpenLinkHub/OpenLinkHub"
  test -f "$root/usr/lib/systemd/system/OpenLinkHub.service"
  test -f "$root/usr/lib/udev/rules.d/99-openlinkhub-nexus.rules"
  test -f "$root/usr/libexec/openlinkhub-nexus/post-install"
  test -d "$root/opt/OpenLinkHub/static"
  test -d "$root/opt/OpenLinkHub/web"
  test -d "$root/opt/OpenLinkHub/database"
  cmp "$original" "$root/opt/OpenLinkHub/OpenLinkHub"
  if find "$root" -name config.json -print -quit | grep -q .; then
    echo "Unexpected packaged runtime config.json" >&2
    exit 1
  fi
done
(cd "$output" && sha256sum --check "$(basename "$deb").sha256" && sha256sum --check "$(basename "$rpm").sha256")
echo "Both native packages verified: metadata, assets, permissions scripts, identical release executable and SHA-256."
