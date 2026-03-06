# Feature Specification: Custom SCSS Theming

**Feature Branch**: `003-custom-scss-theming`
**Created**: 2026-03-06
**Status**: Draft
**Phase**: 1 (Foundation)

## Comparative Analysis

### Current State (`complytime/website`)

- `assets/scss/common/_variables-custom.scss` — Fonts (DM Sans, JetBrains Mono), color palettes (cyan-50–950, slate-50–950), primary color (`$cyan-600`), component classes (`.feature-card`, `.project-card`, `.section-*`, `.bg-accent-subtle`, `.btn-primary`, `.btn-outline-primary`)
- `assets/scss/common/_custom.scss` — Google Fonts import, hero styles, smooth scroll, links, feature grid (3-column), code blocks, tables, breadcrumb, alerts, badges, TOC, responsive tweaks
- No hero badge styles
- No carousel-specific styles
- No dark/light mode variant overrides beyond Doks defaults

### Target State (`test-website`)

- `_variables-custom.scss` — Same font and color foundation
- `_custom.scss` — Extended with:
  - Hero badge component (`.hero-badge`)
  - Project cards carousel styles (`.project-carousel`, `.carousel-track`, `.carousel-dots`, `.carousel-nav`)
  - Enhanced responsive layout for carousel
  - Dark/light mode variant overrides for cards and backgrounds
  - Updated feature card hover effects

### Delta

| Item | Action | Details |
|------|--------|---------|
| `assets/scss/common/_custom.scss` | Modify | Add hero badge, carousel, dark/light variants, enhanced hover effects |
| `assets/scss/common/_variables-custom.scss` | No change | Same font and color definitions |

### Conflicts

- The existing `_custom.scss` has hero styles, feature grid, and project card styles. The test-website version modifies these. A careful merge is needed — not a full replacement — to preserve any existing styles that are working correctly.
- Carousel styles depend on Feature 5 (Projects Carousel) for the HTML structure. These styles will have no visual effect until the carousel markup is added but will not cause errors.

## Acceptance Criteria

1. Hero badge (`.hero-badge`) renders correctly above the hero title
2. Feature card hover effects work on desktop
3. All existing styles (hero, feature grid, code blocks, tables, breadcrumb, alerts, badges, TOC) continue to render correctly
4. Dark mode and light mode both render correctly with no visual regressions
5. Responsive layout works on mobile, tablet, and desktop
6. Lighthouse performance score remains 90+
7. `hugo build` completes without SCSS compilation errors

## Migration Steps

1. Diff existing `_custom.scss` against test-website version to identify additions
2. Merge new styles (hero badge, carousel, dark/light variants) into existing file
3. Preserve existing styles that are not present in test-website
4. Test both dark and light modes
5. Test responsive breakpoints (mobile, tablet, desktop)

## Rollback Plan

Restore `assets/scss/common/_custom.scss` from the `main` branch. The `_variables-custom.scss` file is not modified.
