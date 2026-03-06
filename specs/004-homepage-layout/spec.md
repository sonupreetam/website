# Feature Specification: Homepage Layout

**Feature Branch**: `004-homepage-layout`
**Created**: 2026-03-06
**Status**: Draft
**Phase**: 3 (Visible Features)

## Comparative Analysis

### Current State (`complytime/website`)

- `layouts/home.html` contains:
  - Hero section: badge, title, lead, "Get Started" + "View on GitHub" buttons
  - Features section: 6 hardcoded cards (Intelligent Automation, Policy as Code, Cloud Native, Evidence Collection, OSCAL Native, Open Source) with inline SVG icons
  - Projects section: 4 hardcoded project cards (complyctl, complyscribe, collector-components, compliance-to-policy-go) with static descriptions
  - CTA section: "Ready to Automate Your Compliance?" with buttons
  - Sidebar footer: "Start building with ComplyTime today"
- All content is hardcoded in the template

### Target State (`test-website`)

- `layouts/home.html` contains:
  - Hero section: badge with link, same title/lead, same CTAs
  - Features section: same 6 cards with inline SVG icons
  - Projects section: **dynamic carousel** driven by `data/projects.json` instead of hardcoded cards, with prev/next navigation and dot indicators
  - CTA section: same structure
  - Sidebar footer: same structure
- Project cards are rendered via `range` over `site.Data.projects`

### Delta

| Item | Action | Details |
|------|--------|---------|
| `layouts/home.html` | Modify | Replace hardcoded projects section with `range`-based carousel driven by `data/projects.json` |
| Hero section | Minor | Add badge link wrapper (currently static badge) |
| Features section | No change | Same 6 cards with inline SVG |
| CTA section | No change | Same structure |
| Sidebar footer | No change | Same structure |

### Conflicts

- The existing `home.html` has a working projects section with 4 hardcoded cards. Replacing it with the data-driven carousel requires Feature 5 (carousel JS) and Feature 8 (projects.json data) to be present for full functionality.
- This feature can be implemented incrementally: first update to `range` over data, then add carousel behavior.
- If `data/projects.json` does not exist, the projects section will be empty. A fallback or the data file must be created first.

## Acceptance Criteria

1. Hero section renders with badge, title, lead, and both CTA buttons
2. Features section renders all 6 cards with icons
3. Projects section renders cards from `data/projects.json` (when present)
4. Projects section gracefully handles missing `data/projects.json` (empty or fallback)
5. CTA and sidebar footer sections render correctly
6. Layout is responsive across mobile, tablet, and desktop
7. `hugo build` completes without template errors

## Migration Steps

1. Diff existing `home.html` against test-website version
2. Update projects section from hardcoded to data-driven (`range site.Data.projects`)
3. Add hero badge link wrapper
4. Preserve all other sections (features, CTA, footer) as-is
5. Create a minimal `data/projects.json` with the existing 4 projects for backwards compatibility
6. Test with and without `data/projects.json`

## Rollback Plan

Restore `layouts/home.html` from the `main` branch.

## Dependencies

- Feature 8 (`data/projects.json`) for full functionality
- Feature 5 (Projects Carousel) for carousel behavior
- Feature 3 (SCSS) for carousel and badge styles
