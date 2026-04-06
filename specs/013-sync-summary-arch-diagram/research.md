# Research: Architecture Diagram and Getting-Started Content

**Branch**: `013-sync-summary-arch-diagram` | **Date**: 2026-04-06

## R1: CSS-Only Dark/Light Image Switching

**Decision**: Use CSS `display` toggling on two co-present `<img>` elements, driven by Bootstrap's `data-bs-theme` attribute selector and the `prefers-color-scheme` media query.

**Rationale**: The site already uses Bootstrap 5's `data-bs-theme` attribute for theme switching (provided by Doks). Hooking into it for image variants requires zero JavaScript and zero additional dependencies — a CSS-only approach that satisfies Constitution V (No Runtime JavaScript Frameworks) directly.

**Alternatives considered**:
- **JS `matchMedia` listener** — would detect theme changes dynamically but violates Constitution V and adds complexity for a simple use case.
- **CSS `picture` + `prefers-color-scheme` only** — works for OS-level preference but does not respond to Bootstrap's explicit `data-bs-theme` toggle (a user who clicks the dark-mode button would not get the dark image). Rejected.
- **Single image with CSS filter inversion** — only suitable for monochrome or simple graphics; the architecture diagram has specific brand colours that must not be inverted. Rejected.

## R2: Hugo Shortcode vs. Inline HTML

**Decision**: Implement as a Hugo shortcode (`layouts/shortcodes/theme-image.html`) rather than requiring authors to write raw HTML.

**Rationale**: Shortcodes are the Hugo-idiomatic way to encapsulate reusable HTML patterns for content authors. A shortcode call `{{< theme-image light="..." dark="..." alt="..." >}}` is declarative, readable in Markdown, and works in any content file without knowing the underlying CSS class names. Doks supports custom shortcodes via `layouts/shortcodes/` overrides (Constitution I).

**Alternatives considered**:
- **Inline HTML in Markdown** — Hugo's `markup.goldmark.renderer.unsafe` can allow raw HTML in Markdown but it's disabled by default in Doks for security. Enabling it site-wide is a broader change. Rejected in favour of the shortcode.
- **Hugo partial** — Partials are for layout templates, not content. Authors cannot call partials from Markdown. Rejected.

## R3: Image Format (PNG vs. WebP)

**Decision**: Use PNG for v1. WebP conversion deferred.

**Rationale**: The architecture diagrams are high-detail technical illustrations with fine text and thin lines. PNG provides lossless compression that preserves clarity. The images are ~1.2–1.3 MB each — larger than ideal for a web page but acceptable for a documentation site where users are developers on fast connections. The Constitution VIII Lighthouse 90+ target should still be achievable given the rest of the page is a lightweight documentation page with no other large assets.

**Alternatives considered**:
- **WebP** — would reduce file size by ~30–50% with equivalent visual quality. Deferred to a follow-up optimisation pass rather than blocking this feature.
- **SVG** — ideal for line art / diagrams but the architecture diagram was produced as a raster export from a design tool (not vector-native). Converting to SVG would require redrawing. Deferred.

## R4: Getting-Started Guide — complyctl Workflow

**Decision**: Document the complete six-step complyctl workflow: `init` → `get` → `list` → `generate` → `scan` → `doctor/providers`.

**Rationale**: The previous guide was outdated (missing cosign verification, incorrect init behaviour, incomplete scan output formats). The updated guide reflects the current complyctl release: binary download from GitHub releases, Sigstore cosign signature verification using the GitHub Actions OIDC issuer, and all four scan output formats (EvaluationLog, pretty, OSCAL, SARIF).

**Alternatives considered**:
- **Link to upstream complyctl docs only** — avoids maintenance burden but leaves the getting-started page thin. Rejected; the site's purpose is to be the entry point for the ComplyTime ecosystem, so a working quick-start is essential.
- **Video walkthrough** — richer but not maintainable as a static asset and violates Constitution V (would require a video player). Rejected.
