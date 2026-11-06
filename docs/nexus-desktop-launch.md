# Nexus Open URL and Launch Application

A physical Nexus button runs on the **OpenLinkHub host**, not in whichever
remote browser happens to have the WebUI open. The on-screen preview is an
editor: clicking it selects and moves widgets, but does not execute actions.
For URL widgets, use **Test URL in this browser** to check the link manually.

## Why a desktop helper may be needed

When the daemon is running as the desktop user with `DISPLAY` or
`WAYLAND_DISPLAY`, **Open URL** uses `xdg-open` and **Launch Application**
uses `gtk-launch`. OpenLinkHub's packaged system service instead runs as
the separate `openlinkhub` user, which cannot access your graphical session.
Background user services started without the session environment have the
same issue.

The optional Nexus desktop agent solves this without launching a browser as
root, copying X11 credentials, or adding a remote HTTP command endpoint.
It only accepts validated HTTP(S) URLs and safe desktop-entry IDs from the
local Nexus daemon. It never accepts shell commands.

## Packaged Linux desktop startup

The Debian and RPM packages install an XDG autostart entry at
`/etc/xdg/autostart/openlinkhub-nexus-agent.desktop`. It starts the agent in
an interactive graphical session using a non-root launcher, not in the
system daemon. The launcher uses the fixed installation/socket paths and
checks that the session user belongs to the `openlinkhub` group.

**For security, package installation never automatically authorizes all
local users.** An administrator must explicitly opt in each desktop account.
The installer prints this command, with the installing user's name filled in,
when that account is not yet authorized:

```sh
sudo usermod -aG openlinkhub "$(id -un)"
```

**Log out of the graphical desktop completely and log back in.** Changing
account groups does not alter the existing session or previously launched
agents. Check `id -nG` (without a username) in the new session and confirm
`openlinkhub` appears. The system-wide desktop entry will take effect on the
next login for authorized users.

The service accepts one desktop agent at a time. The first agent to connect
receives every launch; any other agent (a second logged-in authorized user,
or a leftover manually configured per-user autostart entry) is refused and
keeps retrying, taking over only after the first agent disconnects. Remove
old per-user autostart entries so they do not compete with the packaged one.

To turn the packaged autostart off for one account, create
`~/.config/autostart/openlinkhub-nexus-agent.desktop` containing
`[Desktop Entry]` and `Hidden=true`. Edits an administrator makes to the
file in `/etc/xdg/autostart` are kept across package upgrades.

If the agent appears to run but launch buttons still fail, read its log.
An autostarted agent writes to the user journal on systemd desktops
(`journalctl --user -b | grep "Nexus desktop agent"`) or to
`~/.xsession-errors` on some X11 sessions. You can also run
`/usr/libexec/openlinkhub-nexus/desktop-agent` from a desktop terminal to
see its output directly. Failed socket checks, connection attempts, peer
validation and refusals because another agent is connected are reported
when they change, and a successful connection prints a confirmation. Verify
permissions with
`namei -l /opt/OpenLinkHub/runtime/nexus-desktop/desktop-launch.sock`.

## Starting the agent from your desktop

Start the daemon normally, then **from a terminal in the locally logged-in
graphical desktop** run the agent using the same rebuilt OpenLinkHub binary.
For a system installation:

```sh
cd /opt/OpenLinkHub
./OpenLinkHub --nexus-desktop-agent
```

For a user installation, `cd` to its actual installation directory first,
then run `./OpenLinkHub --nexus-desktop-agent`. The agent stays running in
that terminal and reconnects if the daemon is restarted.

The default socket is
`<OpenLinkHub working directory>/runtime/nexus-desktop/desktop-launch.sock`.
When your agent has a different working directory:

```sh
./OpenLinkHub --nexus-desktop-agent --socket /opt/OpenLinkHub/runtime/nexus-desktop/desktop-launch.sock
```

The Unix socket is local-only, mode **0660**, inside a daemon-owned mode **0750** runtime folder. The agent checks the socket owner and Linux peer credentials. It is not exposed to
the network. The desktop account must have access to the socket's group.
For the packaged service check membership with `id -nG`, and if needed:

```sh
sudo usermod -aG openlinkhub "$USER"
```

Log out and log back in for the group change to take effect. If the device
or server is physically headless, the host has no desktop on which to launch
a browser; Open URL cannot open a tab on a different computer through the
remote WebUI.

## Troubleshooting

1. Create a **Button** widget with **Open URL** and a full `https://` URL.
   Click **Save Layout**; simply editing the preview does not persist it.
2. Use **Test URL in this browser** for an immediate browser-side check.
   This is separate from the physical Nexus action.
3. Ensure the daemon's socket exists, the graphical desktop agent is
   running, and its account can connect.
4. Press the button on the **physical Nexus**. Launch failures appear in
   `journalctl -u OpenLinkHub.service` for system services or
   `journalctl --user -u OpenLinkHub.service` for user services, and in the
   desktop agent's terminal. URLs themselves are not written into log entries.

If your existing user service already runs with a valid graphical session
environment, the helper is unnecessary: the action uses `xdg-open` directly.
