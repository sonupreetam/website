# Feature Specification: Projects Carousel

**Feature Branch**: `005-projects-carousel`
**Created**: 2026-03-06
**Status**: Draft
**Phase**: 3 (Visible Features)

## Comparative Analysis

### Current State (`complytime/website`)

- Projects displayed as a static 4-card grid on the homepage
- No JavaScript for project card navigation
- `assets/js/custom.js` exists but is empty
- No carousel component

### Target State (`test-website`)

- `assets/js/carousel.js` — Full carousel implementation:
  - Responsive page sizing (adjusts cards per page based on viewport)
  - Dot indicators for page navigation
  - Previous/Next arrow buttons
  - Smooth transitions between pages
  - Touch/swipe support considerations
- Carousel SCSS styles in `_custom.scss` (covered by Feature 3)
- Carousel HTML structure in `home.html` (covered by Feature 4)

### Delta

| Item | Action | Details |
|------|--------|---------|
| `assets/js/carousel.js` | Add | Carousel logic with responsive pages, dots, prev/next navigation |
| `assets/js/custom.js` | No change | Remains as empty placeholder |

### Conflicts

- None for the JS file itself — it is a net-new addition.
- The carousel requires the corresponding HTML structure in `home.html` (Feature 4) and SCSS styles (Feature 3) to function. Without those, the JS file will load but have no effect.

## Acceptance Criteria

1. `assets/js/carousel.js` exists and is loaded by the site
2. Carousel displays the correct number of cards per page based on viewport width
3. Dot indicators reflect the current page and total pages
4. Clicking prev/next navigates between pages with smooth transitions
5. Carousel handles edge cases: single page (no nav), empty data, window resize
6. No JavaScript errors in the browser console
7. Carousel is keyboard-accessible (arrow keys navigate)

## Migration Steps

1. Add `assets/js/carousel.js` from test-website
2. Verify it is included in the Hugo JS pipeline (check if Doks auto-bundles from `assets/js/`)
3. If not auto-bundled, add a script reference in the footer partial

## Rollback Plan

Delete `assets/js/carousel.js`. The homepage falls back to the static project cards layout.

## Dependencies

- Feature 3 (Custom SCSS) for carousel styles
- Feature 4 (Homepage Layout) for carousel HTML markup
