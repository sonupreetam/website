# Feature Specification: Architecture Diagram and Getting-Started Content

**Feature Branch**: `013-sync-summary-arch-diagram`  
**Created**: 2026-04-06  
**Status**: Done (retroactive spec)  
**Input**: PR #7 `feat/add-architecture-diagram` (CPLYTM-1314)

## Overview

The ComplyTime getting-started documentation described the toolchain in prose only. New users had no visual entry point to understand how the Definition, Measurement, and Enforcement domains fit together before running their first command.

This feature adds:

1. A **theme-aware architecture diagram** on the getting-started page — a static image that automatically switches between a light-background and dark-background variant based on the active site theme, with no JavaScript required.
2. A **reusable `theme-image` shortcode** so the same dark/light switching behaviour can be applied to any image on any page without per-page CSS.
3. A **complete complyctl quick-start guide** replacing the outdated CLI instructions — covering binary installation with release signature verification, scanning provider setup, workspace init, policy fetch, assessment generation, and scan execution with all supported output formats.

## User Scenarios & Testing

### User Story 1 — New User Understands the Architecture at a Glance (Priority: P1)

A developer who has just discovered ComplyTime opens the getting-started page and wants to understand how the tools relate before diving into commands.

**Why this priority**: A diagram communicates the system topology in seconds; without it, the relationship between policy authoring, scanning, and enforcement is buried in prose that most users skip.

**Independent Test**: Open the getting-started page. Confirm an "Architecture Overview" section exists above the prerequisites, containing an image and three labelled bullet points describing Definition, Measurement, and Preventative Enforcement.

**Acceptance Scenarios**:

1. **Given** a visitor opens the getting-started page in light mode, **When** the page renders, **Then** the light-background architecture diagram is visible and the dark variant is hidden.
2. **Given** a visitor opens the getting-started page in dark mode, **When** the page renders, **Then** the dark-background architecture diagram is visible and the light variant is hidden.
3. **Given** a visitor's OS is set to `prefers-color-scheme: dark` with no explicit site theme override, **When** the page loads, **Then** the dark diagram is shown.
4. **Given** JavaScript is disabled, **When** the page loads, **Then** the light diagram is shown and the page is fully readable.

---

### User Story 2 — Content Author Reuses the Theme-Image Shortcode (Priority: P2)

A contributor wants to add a diagram to another docs page with automatic dark/light switching.

**Why this priority**: If the shortcode is well-defined, contributors never need to write inline CSS or duplicate the display-switching logic.

**Independent Test**: Add `{{< theme-image light="/images/example-light.png" dark="/images/example-dark.png" alt="Example" >}}` to any content page, build with Hugo, and confirm the correct variant renders per theme.

**Acceptance Scenarios**:

1. **Given** a content page uses `{{< theme-image light="..." dark="..." alt="..." >}}`, **When** Hugo builds, **Then** two `<img>` elements are rendered — one with class `theme-image-light`, one with `theme-image-dark`.
2. **Given** the active theme is light, **When** the page displays, **Then** only the light image is visible (dark image has `display: none`).
3. **Given** the active theme is dark, **When** the page displays, **Then** only the dark image is visible (light image has `display: none`).

---

### User Story 3 — New User Runs Their First Compliance Scan (Priority: P2)

A developer follows the getting-started guide end-to-end: installs `complyctl`, fetches policies, and runs a scan.

**Why this priority**: The previous guide contained outdated instructions that did not match the current complyctl release, causing first-run failures that eroded trust.

**Independent Test**: Follow the guide verbatim on a clean machine. Confirm each command exits without error and produces the described output.

**Acceptance Scenarios**:

1. **Given** a user downloads a complyctl binary release and runs the `cosign verify-blob` command from the guide, **When** it completes, **Then** it exits 0 (signature is valid).
2. **Given** a user runs `complyctl init` in an empty directory, **When** the command completes, **Then** a `complytime.yaml` workspace config exists.
3. **Given** a user runs `complyctl get`, **When** it completes, **Then** policies are present in `~/.complytime/policies/`.
4. **Given** a user runs `complyctl scan --policy-id <id>`, **When** the scan completes, **Then** output is written to `./.complytime/scan/`.
5. **Given** a user passes `--format pretty`, `--format oscal`, or `--format sarif`, **When** the scan runs, **Then** output is in the requested format.

---

### Edge Cases

| Case | Expected Behaviour |
|------|--------------------|
| `data-bs-theme="light"` is set explicitly while OS is dark | Explicit attribute takes precedence; light image is shown |
| `alt` parameter omitted from `theme-image` shortcode | Falls back to `"Architecture Diagram"` default |
| Hugo build runs without the shortcode partial present | Build fails with a missing partial error — shortcode file is a committed layout override |

## Requirements

### Functional Requirements

- **FR-001**: The getting-started page MUST display an "Architecture Overview" section containing a theme-aware architecture diagram and a three-point description of the ComplyTime domains (Definition, Measurement, Preventative Enforcement).
- **FR-002**: The site MUST provide a `{{< theme-image >}}` shortcode accepting `light`, `dark`, and `alt` parameters that renders two image variants and shows only the correct one based on the active theme.
- **FR-003**: Theme switching MUST be driven by CSS only — no JavaScript required. The mechanism MUST respond to both the Bootstrap `data-bs-theme` attribute and the `prefers-color-scheme` CSS media query.
- **FR-004**: The light-background diagram image MUST be the default fallback when no theme preference is detectable.
- **FR-005**: The getting-started guide MUST include: binary installation from the GitHub releases page, `cosign verify-blob` signature verification, scanning provider installation (`~/.complytime/providers/`), `complyctl init`, `complyctl get`, `complyctl list`, `complyctl generate`, and `complyctl scan` with EvaluationLog, `pretty`, `oscal`, and `sarif` output format examples.
- **FR-006**: The guide MUST document `complyctl doctor` and `complyctl providers` as optional health-check commands.
- **FR-007**: Hugo MUST build with zero errors after this feature is applied.

### Key Entities

- **Theme Image**: A reusable site component that renders a light and a dark image variant and shows only the appropriate one. Implemented as a Hugo shortcode (`layouts/shortcodes/theme-image.html`) plus SCSS rules targeting `.theme-image-light` and `.theme-image-dark` classes.
- **Architecture Diagram**: A visual representation of the ComplyTime system (Definition → SDLC → Measurement → Enforcement). Exists as two committed static image files: `static/images/complytime-architecture.png` (light) and `static/images/complytime-architecture-dark.png` (dark). These are not generated by the sync tool and are not gitignored.

## Success Criteria

| ID | Criterion | Verification |
|----|----------|--------------|
| SC-001 | Architecture diagram renders in light mode (light image visible, dark hidden) | Visual check; SCSS class inspection |
| SC-002 | Architecture diagram renders in dark mode (dark image visible, light hidden) | Visual check with `data-bs-theme="dark"` |
| SC-003 | `prefers-color-scheme: dark` media query activates dark image when no explicit theme is set | Browser devtools media query emulation |
| SC-004 | `{{< theme-image >}}` shortcode produces valid HTML with two `<img>` elements | `hugo --minify --gc` zero errors; HTML inspection |
| SC-005 | Page is fully readable with JavaScript disabled | NoScript browser test |
| SC-006 | complyctl quick-start commands are accurate for the current release | Manual walkthrough on a clean machine |
| SC-007 | Hugo build completes with zero errors | `hugo --minify --gc` |

## Assumptions

- The site uses Bootstrap's `data-bs-theme` attribute for theme switching (provided by the Doks theme) — no additional JS-based theme detection library is needed.
- Architecture diagram images are maintained manually as committed static assets; they are not synced from upstream repos by the Go sync tool.
- The getting-started guide targets Linux and macOS users; Windows-specific paths are out of scope.
- Release signature verification uses `cosign` from the Sigstore project and reflects the GitHub Actions OIDC issuer in the complyctl release workflow.
