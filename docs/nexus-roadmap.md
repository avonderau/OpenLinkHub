# OpenLinkHub Nexus feature roadmap

This roadmap covers gaps between the Linux Nexus custom-screen implementation
and major iCUE Nexus capabilities. It is a planning document, not a claim of
feature parity or hardware certification.

## Existing foundation

- Ordinary editable JSON screens, five stock layouts and themes
- Add/replace/rename/duplicate screens and widgets
- 640 × 48 screen preview with 8-pixel snapping
- Text, metric, bar and touch-button widgets
- Widget colors, border/flat styles, custom PNG/JPEG button icons
- Custom PNG/JPEG/animated GIF backgrounds, GIF playback capped at 5 FPS
- CPU/GPU temperature and utilization, memory and network/disk rates
- Manual/automatic/fixed cycling, media actions and basic macros
- Upstream merge compatibility workflow and editor/Go test coverage

## User acceptance verification — November 3, 2026

The user has confirmed that the latest Nexus feature set passed hands-on user
testing. This acceptance covers the recently delivered animated GIF background
support, keyboard shortcut actions, mouse click/scroll actions, and clock/date
widgets, alongside the current Nexus editor workflow.

**Status: user-tested / passed (user-reported).** This is a user acceptance
result, not an independently reproduced automated test or certification for
every Linux desktop, HID device, or compositor.

The corresponding features can be treated as completed for the current
release. Additional functionality listed below remains planned and should
still use feature branches, automated checks, and manual acceptance testing.

## Implementation principles

Every feature is developed in a separate branch/PR, with Go, Nexus, browser
editor, upstream-compatibility and security checks passing before merging.
No modification of upstream-owned files unless absolutely necessary; isolate
integration points in fork-owned Nexus files. Every action uses typed,
validated configuration. Never interpret widget text as shell commands.
Existing saved JSON layouts remain valid across updates.

## Phase 1 — Button actions and reusable execution

1. **Keyboard shortcuts [implemented; user-tested]**: structured key combinations for buttons
   and macro steps, implemented through the existing Linux uinput keyboard.
   Validate key/modifier names, hold/release ordering and error handling.
   Document permissions and Wayland/compositor limitations.
2. **Shared action engine**: consolidate dispatch, debouncing and macros.
   Create explicit boundaries for input, media and device actions, bounds on
   execution time, cancellation and safe concurrent input handling.
3. **Mouse actions [implemented; user-tested: clicks and scrolling]**: left/right/middle click, double-click and scroll via
   the existing input manager; clamp repetition and release on error.
4. **Application and URL actions [first Linux implementation]**: launch only explicitly approved desktop
   entries or executables; validate http/https links. Never use a shell for
   arbitrary user-provided command lines. Require user control and safe errors.
5. **Visual macro builder**: step list rather than raw JSON, including
   navigation, media, hotkeys, mouse, approved launch and bounded waits.
   Validate total duration and ensure no stuck keys on cancel.

## Phase 2 — Better system and hardware information

6. **Expanded telemetry provider [Linux sysfs metrics in progress]**: discover available sensors and expose
   supported CPU/GPU clocks, VRAM, power, fan/pump RPM, coolant and drive
   utilization without repeated external process spawning. Missing sensors
   display N/A and do not break screens. Prefer OpenLinkHub's existing
   device/system data.
7. **Clock/date widgets [implemented; user-tested: basic formats]**: local date/time formatting, update cadence,
   timezone support and font fitting in 640 × 48.
8. **History graphs [implemented: bounded 60-second sparklines]**: bounded rolling sensor data, sparkline/area/line
   widgets with explicit sampling intervals, memory caps and clipping.
9. **Hardware actions**: only expose RGB/profile, fan/pump, headset and mouse
   DPI controls where the detected OpenLinkHub device safely supports them;
   require range validation and clear authorization.

## Phase 3 — Editor, assets and screen management

10. **Import/export**: versioned JSON for full layouts and individual screens,
    collision handling, previews before overwrite and safe content limits.
11. **Reusable widget library [browser-local presets implemented]**: save, rename, delete and reuse button/widget
    definitions with independent IDs and safe references.
12. **Typography/layout [font sizing and horizontal alignment implemented]**: font sizes, alignment, vertical centering, shadows
    and adaptive text clipping; keep editor and physical Nexus consistent.
13. **Animated widgets/icons**: GIF artwork on buttons/widgets, with independent
    animation timers, disposal handling, memory and size caps.
14. **Screen navigation [screen reordering implemented]**: true left/right swipe gestures, reorder screens,
    overview and configurable behavior around button hit targets.

## Phase 4 — Context, hardware controls and finish quality

15. **Context-aware profiles**: optional app-specific screen sets, fallback
    screen, profile selection and privacy-preserving Linux foreground-app
    detection where desktop APIs permit. No privileged process enumeration
    requirement.
16. **Brightness and display controls**: verify current Nexus brightness
    support, expose settings and persist a device-safe configured range.
17. **Performance and resilience**: cache static assets and metrics, avoid
    redundant HID transfers, cap animation FPS, add reduced-motion modes and
    graceful fallback when sensors, input permissions or devices disappear.
18. **Accessibility and usability**: keyboard navigation in the browser,
    clear error/status notifications, no unnecessary colored text, mobile
    responsiveness and user testing on the physical Nexus.

## Definition of done for each feature

- Supported use cases and security boundaries documented
- Unit tests for data validation, error handling and boundary cases
- Browser-facing tests for configuration and UI workflows
- Builds and Nexus/upstream/CodeQL checks pass on the PR
- No regression in existing saved screen JSON compatibility
- Hardware checks identified, with results recorded when a device is available
- No unreviewed direct-to-main commits

GitHub Issues are currently disabled in this repository; this file is the
version-controlled feature backlog until Issues are enabled.

### Current feature notes

Keyboard shortcuts and basic mouse actions have been implemented with
allowlisted names and shared Linux input-manager functions. Clock/date
widgets display the host's local time with 24-hour, 12-hour, date, or
date-and-time formatting. Additional rich typography, locale controls and
additional cross-device compatibility tests remain on the roadmap. The existing keyboard/mouse actions
do not imply that application launch or all iCUE macros are complete.

### History graph implementation

Graph widgets use real CPU/GPU, memory, network or disk telemetry from the
same metrics as number/bar widgets. The per-device sampler retains at most
60 one-second snapshots per metric. Missing data produces gaps rather than
fabricated values. Utilization graphs use a fixed 0–100% axis; other metrics
use their available sample range. The editor shows an indicative graph label
because it does not stream live telemetry over the configuration API.

### Screen editing enhancements

The screen selector offers Move Up and Move Down operations for user-defined
screen order. Widget Copy/Paste transfers a deep copy between screens in the
current loaded layout, assigning a fresh widget ID and keeping all widget
properties and actions. These are editor actions only until Save Layout.
A persistent cross-session reusable widget library is still planned.

### Browser-local widget presets

The editor can save selected widgets as named presets in this browser, then
add them to any screen in the current Nexus layout. Presets are stored in
localStorage with 50-item and 256 KiB bounds. New widget IDs are assigned
when placed, and navigation references to unknown screens fall back to the
selected screen. Presets are not synchronized across browsers or hosts, and
clearing browser data removes them. Server-side synchronized libraries remain
future work.

### Desktop app and web URL actions

Nexus buttons and macro steps can request an HTTP(S) URL through `xdg-open`
or a desktop entry ID (such as `firefox.desktop`) through `gtk-launch`.
The implementation never invokes a shell. IDs and URLs are validated before
saving. Runtime launch is refused when OpenLinkHub runs as root or without a
graphical desktop environment (DISPLAY / WAYLAND_DISPLAY). A single bounded
launcher worker prevents rapid touches spawning unlimited processes; operations
have a four-second timeout. Desktop sessions can impose additional D-Bus or
compositor restrictions. No arbitrary executable path or shell command is
accepted. Application auto-discovery and curated allowlists remain planned.

### Nexus typography

Widgets accept optional `fontSize` values normal, small, or large and
`textAlign` values left, center, or right. The device renderer uses
cached embedded Open Font fonts for the nondefault sizes with clipping to
the widget rectangle. Widgets without typography settings retain the
legacy fixed-width font and alignment. Vertical alignment, text shadows,
and more font families remain future work.

### Optional Linux sysfs telemetry

The Nexus collector can read the following additional metrics without launching
other processes: `cpu.clock_mhz` (CPU0 current cpufreq),
`disk.utilization` (root filesystem usage percentage),
`disk.free_gb` (root filesystem available GB), `fan.rpm` (first valid
unlabelled/normal hwmon fan), `pump.rpm` (first hwmon fan labelled as a pump),
`gpu.vram.used_mb` and `gpu.vram.utilization` (AMDGPU DRM sysfs, when
present). Data are sampled at most once per second even during animated
background playback. Missing or implausible sensors remain unavailable instead
of showing invented readings. NVIDIA VRAM, GPU clocks and per-device Corsair
coolant/fan sensors still need dedicated providers; the generic fan reading
must not be labelled as a specific Corsair device.
