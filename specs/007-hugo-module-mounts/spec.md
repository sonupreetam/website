# Feature Specification: Hugo Module Mounts for Upstream Docs

**Feature Branch**: `007-hugo-module-mounts`
**Created**: 2026-03-06
**Status**: Draft
**Phase**: 2 (Content Infrastructure)

## Comparative Analysis

### Current State (`complytime/website`)

- `config/_default/module.toml` defines mounts for theme assets (Doks, Thulite, Tabler icons) and local content/assets/static directories
- No Hugo Module `[[imports]]` for upstream complytime repos
- Documentation content under `content/docs/projects/` is manually maintained:
  - `content/docs/projects/complyctl/` — 4 pages (index, installation, quick-start, plugin-guide)
  - `content/docs/projects/complyscribe/` — 6 pages (index, troubleshooting, tutorials/*)
  - `content/docs/projects/complytime-collector-components/` — 6 pages (index, design, development, attributes/*, integration/*)
- Some pages contain `<!-- synced from complytime/<repo>@main -->` comments suggesting manual sync
- No `go.mod` exists (required for Hugo Modules imports)

### Target State (`test-website`)

- `config/_default/module.toml` adds `[[imports]]` blocks for 3 upstream repos:
  - `github.com/complytime/complyctl` → `content/docs/projects/complyctl`
  - `github.com/complytime/complyscribe` → `content/docs/projects/complyscribe` (with excludeFiles for contributing.md, troubleshooting.md)
  - `github.com/complytime/complytime-collector-components` → `content/docs/projects/collector-components`
- Docs update via `hugo mod get -u` instead of manual copy
- `go.mod` required for Hugo Module resolution

### Delta

| Item | Action | Details |
|------|--------|---------|
| `config/_default/module.toml` | Modify | Add 3 `[[imports]]` blocks with `[[imports.mounts]]` for upstream repos |
| `go.mod` | Required | Hugo Modules need a Go module (may be added by Feature 6) |
| `content/docs/projects/complyctl/` | Remove local | Replaced by Hugo Module mount from upstream repo's `docs/` |
| `content/docs/projects/complyscribe/` | Remove local | Replaced by Hugo Module mount from upstream repo's `docs/` |
| `content/docs/projects/complytime-collector-components/` | Remove local | Replaced by Hugo Module mount from upstream repo's `docs/` |

### Conflicts

- **Critical**: Removing local `content/docs/projects/` files and replacing with module mounts changes how content is managed. If the upstream repos' `docs/` directories have different structure or frontmatter, the mounted content may not render correctly with the existing `docs/list.html` layout.
- **Dependency**: Requires `go.mod` to exist (Feature 6 adds this). If Feature 6 is not merged first, Hugo Module imports will fail.
- The existing `module.toml` has extensive mount configuration for theme assets. Only the `[[imports]]` blocks need to be added; existing mounts must be preserved.

## Acceptance Criteria

1. `config/_default/module.toml` includes `[[imports]]` for all 3 upstream repos
2. `hugo mod get -u` successfully fetches upstream docs
3. Content at `/docs/projects/complyctl/`, `/docs/projects/complyscribe/`, `/docs/projects/collector-components/` renders from upstream repo `docs/` directories
4. Sidebar navigation generates correctly for mounted content
5. Local `content/docs/projects/` files for mounted repos are removed (no duplicates)
6. `hugo build` completes without errors
7. All existing theme mounts (Doks, Thulite, icons) continue to function

## Migration Steps

1. Ensure `go.mod` exists (from Feature 6 or create minimal one)
2. Add `[[imports]]` blocks to `module.toml` for all 3 repos
3. Run `hugo mod get -u` to fetch upstream content
4. Verify mounted content renders correctly
5. Remove local `content/docs/projects/` files that are now mounted
6. Test sidebar navigation and docs links

## Rollback Plan

1. Remove the `[[imports]]` blocks from `module.toml`
2. Restore local `content/docs/projects/` files from `main` branch
3. Run `hugo mod tidy` to clean up

## Dependencies

- Feature 6 (Go Sync Tool) for `go.mod` — or a minimal `go.mod` must be created independently
