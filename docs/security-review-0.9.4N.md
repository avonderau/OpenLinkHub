# OpenLinkHub Nexus 0.9.4N — security review

**Repository:** [avonderau/OpenLinkHub](https://github.com/avonderau/OpenLinkHub)  
**Baseline:** Upstream OpenLinkHub 0.9.4, synchronized with the fork's Nexus functionality  
**Scope:** Nexus WebUI and API, saved layouts and widgets, image/GIF uploads, macros, synthesized input, URL/app launching, local daemon-to-desktop messaging, service installers, CI, and release packaging.

This is a **source-code audit with automated checks**, not an assertion that the application has been independently penetration-tested or hardware-certified. Hardware-dependent behavior and desktop integration require real-device testing. Upstream device drivers beyond the modified integration paths were not exhaustively reviewed.

## Findings and remediation

| Severity | Area | Finding | Resolution in 0.9.4N |
| --- | --- | --- | --- |
| High | Go standard-library security | Builds pinned to Go 1.26.0 were reported by govulncheck as reaching 31 known standard-library vulnerabilities (HTTP, TLS, X.509, templates, etc.). | Upgrade the source requirement, CI toolchain and Docker builder to patched Go 1.26.10; block release until govulncheck passes. |
| High | Installation permissions | System and remote installers recursively applied mode 0755, exposing local configuration, saved layouts and other application files to all local accounts. | Remove access for the world without indiscriminately changing file execute bits; preserve service account ownership. |
| Medium | Desktop launcher availability / IPC | The launch-agent socket lived within the Nexus private media directory, which can be inaccessible to a separate desktop user. An agent did not check that its peer owned the expected socket. | Place socket in a dedicated daemon-owned 0750 runtime directory, socket mode 0660, reject unexpected ownership, symlinks and unsafe permissions, and verify Linux SO_PEERCRED UID after connection. |
| Medium | Browser write boundary | Nexus POST/PUT writes checked the Origin host but did not validate the entire origin shape or Fetch Metadata; background uploads also accepted media without checking Content-Type. | Require same-origin browser requests, reject cross-site/same-site fetch metadata, parse and allow only PNG/JPEG/GIF upload media types. Preserve origin-less local API clients for compatibility. |
| Low | Sensitive API caching | Screen layout GET had no explicit no-store response headers. | Add private/no-store and nosniff. |
| Low | Installation reliability | User-space installer accidentally removed execute/search permission from the udev rules directory. | Apply permission changes only to the installed rules file. |
| Medium | Fork distribution integrity | The fork's remote installer still downloaded upstream release assets. | Target the fork's own release endpoint; update visible sidebar GitHub link; retain upstream attribution separately. |
| Low | Security analysis coverage | CodeQL scanned Go only, not the JavaScript Nexus editor. | Extend CodeQL to JavaScript and add race-enabled Go tests, go vet, known-vulnerability scanning and installer shell syntax checks. |

Severity describes the potential impact in relevant deployment configurations, not proof of exploitation.

## Defense-in-depth reviewed

- **Authentication and network exposure:** the Nexus endpoints are wrapped in OpenLinkHub's authentication middleware when the operator enables authentication. The default server address is loopback. Do not expose an unauthenticated instance to untrusted networks. When accessing remotely, enable authentication and trusted TLS; avoid direct unauthenticated reverse-proxy publishing.
- **CSRF and browser requests:** write endpoints require same-origin Fetch Metadata and, if supplied, a matching HTTP(S) Origin. Origin-less direct local API clients remain supported, so localhost remains a trust boundary. Do not rely on Origin checks as authentication.
- **Layout validation:** screen/widget count, geometry, IDs, allowed metrics, action types, style and typography values are bounded. Inline arbitrary macros are disallowed; shared macros use bounded IDs, steps and execution time.
- **Image and GIF processing:** image bodies are size-limited, IDs are allowlisted, formats must decode as PNG/JPEG/GIF, dimensions and animation frame/pixel counts are capped, and files use temporary writes with restrictive mode.
- **DOM safety:** the Nexus editor constructs text with DOM APIs rather than concatenating widget-controlled HTML. Browser-side URL testing permits HTTP(S) only and opens new tabs with noopener/noreferrer.
- **Desktop input and launch actions:** URL/app targets are validated; the launcher uses fixed xdg-open/gtk-launch program names and argument arrays, not a shell. Root and headless launches remain disabled; an explicitly connected local graphical desktop agent can receive only validated physical-touch events.
- **Local socket trust:** the socket is local, not network-accessible, and usable only by accounts with appropriate filesystem/group access. The agent verifies the server endpoint owner and Linux peer UID. Membership in the daemon's privileged hardware group is therefore a security-sensitive decision.
- **Supply chain:** Go dependencies remain pinned via go.mod/go.sum; the release workflow rebuilds from the Git tag, runs tests, and attaches a SHA-256 checksum for the published Linux amd64 bundle. Tags/releases are not silently overwritten.

## Remaining risks and operational guidance

1. **Host-local trust:** absent dashboard authentication, local programs reaching the API can read or modify layouts. Bind to 127.0.0.1 or enable authentication; never expose the unauthenticated WebUI to the LAN or internet.
2. **Hardware group privilege:** accounts in the `openlinkhub` group may access device resources and the local desktop launcher socket. Only add trusted interactive accounts.
3. **Input synthesis and macros:** configured widgets can type text or execute bounded key/mouse actions on the host. Treat editable screen layouts as privileged configuration, and avoid passwords or secrets in widget text fields.
4. **Desktop helper opt-in:** launching a URL occurs on the **host's logged-in desktop**, never on the computer viewing a remote WebUI. The helper must run locally as a non-root desktop user. A physically headless host cannot open a remote browser.
5. **Platform and hardware:** no physical iCUE Nexus testing, distribution matrix, independent penetration test, or reproducible-build attestation was completed as part of this review. The first fork release packages Linux amd64; other architectures are not claimed supported by this bundle.
6. **Upstream and third-party code:** the upstream Go and device driver implementation retains its own risks; this audit focused on fork changes and their integration. Continue monitoring Go dependency advisories and upstream security fixes.

## Release gates

- Go build and Nexus Go unit tests
- Nexus JavaScript regression tests and syntax verification
- Upstream merge compatibility
- CodeQL Go and JavaScript/TypeScript
- Go vet, race-enabled Nexus/API/auth tests, known Go vulnerability checks
- Installer script syntax tests
- Fork version, sidebar link, and installer-source tests

The GitHub Actions results for the final release commit are the source of truth for which gates passed. A passing CI run does **not** replace manual testing on real Nexus hardware.
