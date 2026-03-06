# Feature Specification: Head/Footer Partials

**Feature Branch**: `002-head-footer-partials`
**Created**: 2026-03-06
**Status**: Draft
**Phase**: 1 (Foundation)

## Comparative Analysis

### Current State (`complytime/website`)

- `layouts/_partials/header/header.html` — Custom navbar with logo, FlexSearch, section nav, main nav, social menu, Get Started button
- `layouts/_partials/footer/script-footer-custom.html` — Empty placeholder
- `layouts/_default/_markup/render-image.html` — Custom image render hook (remote passthrough, local Hugo pipeline, SVG skip)
- No `head/` partials directory exists
- Theme resolver and FOUC prevention are handled by the Doks theme defaults

### Target State (`test-website`)

- `layouts/_partials/head/script-header-priority.html` — Inline theme resolver for FOUC prevention, color mode handling, dismissable alert logic, math support (KaTeX/MathJax)
- `layouts/_partials/head/script-header.html` — Placeholder for non-critical header scripts
- `layouts/_partials/head/custom-head.html` — Placeholder for custom head elements
- `layouts/_partials/footer/script-footer-custom.html` — Placeholder for custom footer scripts
- Existing `header/header.html` and `_markup/render-image.html` are retained

### Delta

| Item | Action | Details |
|------|--------|---------|
| `layouts/_partials/head/script-header-priority.html` | Add | FOUC prevention, theme resolver, color mode, dismissable alert, math support |
| `layouts/_partials/head/script-header.html` | Add | Placeholder for non-critical scripts |
| `layouts/_partials/head/custom-head.html` | Add | Placeholder for custom head elements |
| `layouts/_partials/footer/script-footer-custom.html` | No change | Already exists as empty placeholder |
| `layouts/_partials/header/header.html` | No change | Retained as-is |
| `layouts/_default/_markup/render-image.html` | No change | Retained as-is |

### Conflicts

- The Doks theme already provides its own `script-header-priority.html`. The local override will take precedence due to Hugo's lookup order. Verify the override does not break Doks' built-in dark mode toggle or search functionality.
- If the existing site relies on Doks' default theme resolver, replacing it with the custom one must produce identical behavior.

## Acceptance Criteria

1. `layouts/_partials/head/script-header-priority.html` exists with inline theme resolver script
2. `layouts/_partials/head/script-header.html` exists as a placeholder
3. `layouts/_partials/head/custom-head.html` exists as a placeholder
4. No FOUC (Flash of Unstyled Content) on page load — theme is applied before first paint
5. Dark/light mode toggle continues to work correctly
6. FlexSearch continues to function
7. Existing `header.html` and `render-image.html` are untouched
8. `hugo build` completes without errors

## Migration Steps

1. Create `layouts/_partials/head/` directory
2. Add `script-header-priority.html` with theme resolver and FOUC prevention
3. Add `script-header.html` placeholder
4. Add `custom-head.html` placeholder
5. Test dark mode toggle and search still work
6. Verify no FOUC on initial page load

## Rollback Plan

Delete `layouts/_partials/head/` directory. The Doks theme defaults will resume control.
