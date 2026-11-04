#!/usr/bin/env bash
# Package the exact binary from the verified GitHub release archive.
set -euo pipefail
version="$1"; archive="$2"; output="$3"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+N$ ]] || exit 2
for tool in dpkg-deb rpmbuild sha256sum tar; do command -v "$tool" >/dev/null; done
mkdir -p "$output"; output="$(cd "$output" && pwd)"
work="$(mktemp -d)"; trap 'rm -rf "$work"' EXIT
bundle="OpenLinkHub-Nexus-$version-linux-amd64"
while IFS= read -r path; do
  case "$path" in "$bundle/"*) ;; *) echo "Invalid archive path: $path" >&2; exit 1;; esac
  case "/$path/" in *"/../"*|*"/./"*) exit 1;; esac
done < <(tar -tzf "$archive")
tar -xzf "$archive" --no-same-owner -C "$work"
source_dir="$work/$bundle"
test -f "$source_dir/OpenLinkHub"
for dir in static web database; do test -d "$source_dir/$dir"; done

stage="$work/stage"; app="$stage/opt/OpenLinkHub"
mkdir -p "$app" "$stage/usr/lib/systemd/system" \
 "$stage/usr/lib/udev/rules.d" "$stage/usr/libexec/openlinkhub-nexus"
install -m 0755 "$source_dir/OpenLinkHub" "$app/OpenLinkHub"
cp -a "$source_dir/static" "$source_dir/web" "$source_dir/database" "$app/"
for file in docs README.md LICENSE; do
 if [ -e "$source_dir/$file" ]; then cp -a "$source_dir/$file" "$app/"; fi
done
install -m 0644 OpenLinkHub.service "$stage/usr/lib/systemd/system/OpenLinkHub.service"
install -m 0644 99-openlinkhub.rules "$stage/usr/lib/udev/rules.d/99-openlinkhub-nexus.rules"
install -m 0755 packaging/post-install.sh "$stage/usr/libexec/openlinkhub-nexus/post-install"

deb_stage="$work/deb"; cp -a "$stage" "$deb_stage"; mkdir -p "$deb_stage/DEBIAN"
cat > "$deb_stage/DEBIAN/control" <<EOF
Package: openlinkhub-nexus
Version: $version
Section: utils
Priority: optional
Architecture: amd64
Maintainer: OpenLinkHub Nexus <https://github.com/avonderau/OpenLinkHub>
Depends: libc6, libudev1, libpipewire-0.3-0, systemd, udev, adduser
Conflicts: openlinkhub
Homepage: https://github.com/avonderau/OpenLinkHub
Description: OpenLinkHub Nexus and programmable Corsair screens
 Independently maintained OpenLinkHub fork with customizable Nexus
 screens, widgets, live metrics, and touch controls.
EOF
cat > "$deb_stage/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
if [ "$1" = configure ]; then /usr/libexec/openlinkhub-nexus/post-install; fi
EOF
cat > "$deb_stage/DEBIAN/prerm" <<'EOF'
#!/bin/sh
set -e
if [ "$1" = remove ] && command -v systemctl >/dev/null 2>&1; then
 systemctl disable --now OpenLinkHub.service || true
fi
EOF
cat > "$deb_stage/DEBIAN/postrm" <<'EOF'
#!/bin/sh
set -e
if command -v systemctl >/dev/null 2>&1; then systemctl daemon-reload || true; fi
if command -v udevadm >/dev/null 2>&1; then udevadm control --reload-rules || true; fi
# Leave untracked user configuration, layouts and backups intact.
EOF
chmod 0755 "$deb_stage/DEBIAN/"{postinst,prerm,postrm}
deb="OpenLinkHub-Nexus_$version"_"amd64.deb"
dpkg-deb --build --root-owner-group "$deb_stage" "$output/$deb"

top="$work/rpm"; mkdir -p "$top"/{BUILD,BUILDROOT,RPMS,SOURCES,SPECS,SRPMS}
cp -a "$stage" "$top/SOURCES/payload"
cat > "$top/SPECS/openlinkhub-nexus.spec" <<EOF
Name:           openlinkhub-nexus
Version:        $version
Release:        1
Summary:        OpenLinkHub Nexus hardware control and customizable screens
License:        GPL-3.0-or-later
URL:            https://github.com/avonderau/OpenLinkHub
BuildArch:      x86_64
Requires:       systemd, shadow-utils
Conflicts:      OpenLinkHub
%global debug_package %{nil}
# Retain the byte-identical, already-reviewed executable from the release.
# Default RPM BRP stripping would change its ELF bytes.
%define __os_install_post %{nil}

%description
Independently maintained OpenLinkHub fork with programmable Corsair
iCUE Nexus screens, live telemetry and touch actions.

%prep

%build

%install
mkdir -p %{buildroot}
cp -a %{_sourcedir}/payload/. %{buildroot}/

%files
%defattr(-,root,root,-)
%dir /opt/OpenLinkHub
/opt/OpenLinkHub/OpenLinkHub
/opt/OpenLinkHub/static
/opt/OpenLinkHub/web
/opt/OpenLinkHub/database
/opt/OpenLinkHub/docs
/opt/OpenLinkHub/README.md
/opt/OpenLinkHub/LICENSE
/usr/lib/systemd/system/OpenLinkHub.service
/usr/lib/udev/rules.d/99-openlinkhub-nexus.rules
/usr/libexec/openlinkhub-nexus/post-install

%post
/usr/libexec/openlinkhub-nexus/post-install

%preun
if [ "\$1" -eq 0 ] && command -v systemctl >/dev/null 2>&1; then
 systemctl disable --now OpenLinkHub.service || true
fi

%postun
if command -v systemctl >/dev/null 2>&1; then systemctl daemon-reload || true; fi
if command -v udevadm >/dev/null 2>&1; then udevadm control --reload-rules || true; fi
EOF
rpmbuild -bb --define "_topdir $top" "$top/SPECS/openlinkhub-nexus.spec"
rpm="OpenLinkHub-Nexus-$version-1.x86_64.rpm"
built="$(find "$top/RPMS" -type f -name '*.rpm' -print -quit)"
test -n "$built"; cp "$built" "$output/$rpm"
(cd "$output" && sha256sum "$deb" > "$deb.sha256" && sha256sum "$rpm" > "$rpm.sha256")
echo "Native packages built using release binary:"
ls -lh "$output/$deb" "$output/$rpm"
