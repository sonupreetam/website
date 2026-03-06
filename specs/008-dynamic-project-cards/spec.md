# Feature Specification: Dynamic Project Cards (`data/projects.json`)

**Feature Branch**: `008-dynamic-project-cards`
**Created**: 2026-03-06
**Status**: Draft
**Phase**: 3 (Visible Features)

## Comparative Analysis

### Current State (`complytime/website`)

- `layouts/home.html` has 4 project cards hardcoded directly in the template:
  - complyctl ("Go CLI Tool")
  - complyscribe ("Python Automation")
  - collector-components ("Go Observability")
  - compliance-to-policy-go ("Go Framework")
- Each card has: language badge, project name, description, link
- No `data/` directory or `projects.json` file
- Adding a new project requires editing `home.html` directly

### Target State (`test-website`)

- `data/projects.json` — Array of project objects with: name, description, language, stars, last_updated, html_url, docs_url, has_specify
- `layouts/home.html` — Projects section uses `range site.Data.projects` to render cards dynamically
- New repos added to the complytime GitHub org appear automatically after running the sync tool (Feature 6)
- Cards display: language badge, project name, description, GitHub link

### Delta

| Item | Action | Details |
|------|--------|---------|
| `data/projects.json` | Add | Structured project metadata array (initially seeded with existing 4 projects) |
| `layouts/home.html` (projects section) | Modify | Replace hardcoded cards with `range site.Data.projects` loop |

### Conflicts

- Modifying the projects section of `home.html` overlaps with Feature 4 (Homepage Layout). These two features should be coordinated — either merged together or Feature 4 should be merged first.
- The `data/projects.json` schema must match the template expectations. If the sync tool (Feature 6) generates a different schema, the template must be updated accordingly.

## Acceptance Criteria

1. `data/projects.json` exists with at least the 4 existing projects
2. Each project object includes: `name`, `description`, `language`, `html_url`
3. Homepage projects section renders cards from `data/projects.json` instead of hardcoded HTML
4. Adding a new entry to `projects.json` results in a new card on the homepage
5. Removing an entry removes the corresponding card
6. Empty `projects.json` (empty array) does not cause template errors
7. Cards display language badge, project name, description, and link
8. `hugo build` completes without errors

## Migration Steps

1. Create `data/projects.json` with the existing 4 projects in the expected schema
2. Update the projects section of `home.html` to use `range site.Data.projects`
3. Verify the rendered output matches the current hardcoded version
4. Test with additional entries added to `projects.json`
5. Test with empty `projects.json`

## Rollback Plan

1. Restore `layouts/home.html` projects section from `main` branch
2. Delete `data/projects.json`

## Dependencies

- Feature 6 (Go Sync Tool) for automated `projects.json` generation
- Feature 4 (Homepage Layout) — coordinated `home.html` changes
