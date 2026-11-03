# Site-wide visual theme overlay

This fork gives the OpenLinkHub dashboard a cohesive dark charcoal / muted
amber design while intentionally keeping upstream source changes small.

## Implementation

- `static/css/openlinkhub-fork-theme.css` owns the presentation overrides.
- `web/head.html` has **one added stylesheet link** immediately after the
  existing theme stylesheet. This is the only shared upstream template edited.
- Existing HTML markup, IDs, JavaScript, API contracts, hardware controls and
  `static/css/themes/default.css` remain unchanged.
- Pages rendering the shared Go `head` template inherit the overlay; standalone
  utilities that use a custom document head do not automatically inherit it.
- Accent and surface colors use `--olh-*` CSS custom properties. Keep new
  changes in this file rather than adding global rules to upstream CSS.

The design uses the **real** application sidebar and controls, not the
conceptual mockup's invented navigation links. This preserves existing routing.

## Upstream update workflow

1. Fetch `upstream/main` into a feature branch, not directly into `main`.
2. Merge upstream and resolve conflicts without dropping the single theme link.
3. Test the Go server, Nexus, browser editor, and affected device pages.
4. Verify the overlay still loads **after** upstream's theme CSS.
5. Open a PR; run upstream compatibility/CodeQL tests before merging.
6. If upstream changes shared class names, update only the fork CSS selectors.

The overlay is deliberately presentation-only. Never restyle color picker
surfaces, device RGB visualizations, keyboard keycaps, gauge values or semantic
success/error colors without a separate device/UI review.

## Browser acceptance checklist

Check the dashboard, Apps, Macros, Settings, RGB/temperature pages, and Nexus
in a Chromium and Firefox browser at both desktop and narrow screen widths.
Confirm menu navigation, collapsed sidebar, form labels, contrasts, dropdowns,
modal buttons, disabled controls, scrolling, Nexus preview and live buttons.
Validate both standard and custom hardware pages; CI cannot replace this
manual workflow.

**Note:** This is a first pass for the site's shared components, not a
pixel-by-pixel rebuild of every legacy hardware-specific view.
