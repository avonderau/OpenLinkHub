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

## Implementation principles

Every feature is developed in a separate branch/PR, with Go, Nexus, browser
editor, upstream-compatibility and security checks passing before merging.
No modification of upstream-owned files unless absolutely necessary; isolate
integration points in fork-owned Nexus files. Every action uses typed,
validated configuration. Never interpret widget text as shell commands.
Existing saved JSON layouts remain valid across updates.

## Phase 1 — Button actions and reusable execution

1. **Keyboard shortcuts [in progress]**: structured key combinations for buttons
   and macro steps, implemented through the existing Linux uinput keyboard.
   Validate key/modifier names, hold/release ordering and error handling.
   Document permissions and Wayland/compositor limitations.
2. **Shared action engine**: consolidate dispatch, debouncing and macros.
   Create explicit boundaries for input, media and device actions, bounds on
   execution time, cancellation and safe concurrent input handling.
3. **Mouse actions**: left/right/middle click, double-click and scroll via
   the existing input manager; clamp repetition and release on error.
4. **Application and URL actions**: launch only explicitly approved desktop
   entries or executables; validate http/https links. Never use a shell for
   arbitrary user-provided command lines. Require user control and safe errors.
5. **Visual macro builder**: step list rather than raw JSON, including
   navigation, media, hotkeys, mouse, approved launch and bounded waits.
   Validate total duration and ensure no stuck keys on cancel.

## Phase 2 — Better system and hardware information

6. **Expanded telemetry provider**: discover available sensors and expose
   supported CPU/GPU clocks, VRAM, power, fan/pump RPM, coolant and drive
   utilization without repeated external process spawning. Missing sensors
   display N/A and do not break screens. Prefer OpenLinkHub's existing
   device/system data.
7. **Clock/date widgets**: local date/time formatting, update cadence,
   timezone support and font fitting in 640 × 48.
8. **History graphs**: bounded rolling sensor data, sparkline/area/line
   widgets with explicit sampling intervals, memory caps and clipping.
9. **Hardware actions**: only expose RGB/profile, fan/pump, headset and mouse
   DPI controls where the detected OpenLinkHub device safely supports them;
   require range validation and clear authorization.

## Phase 3 — Editor, assets and screen management

10. **Import/export**: versioned JSON for full layouts and individual screens,
    collision handling, previews before overwrite and safe content limits.
11. **Reusable widget library**: save, rename, delete and reuse button/widget
    definitions with independent IDs and safe references.
12. **Typography/layout**: font sizes, alignment, vertical centering, shadows
    and adaptive text clipping; keep editor and physical Nexus consistent.
13. **Animated widgets/icons**: GIF artwork on buttons/widgets, with independent
    animation timers, disposal handling, memory and size caps.
14. **Screen navigation**: true left/right swipe gestures, reorder screens,
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
