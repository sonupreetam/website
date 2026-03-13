# Feature Specification: Go Content Sync Tool

**Feature Branch**: `006-go-sync-tool`
**Created**: 2026-03-06
**Status**: Draft
**Phase**: 2 (Content Infrastructure)

## Comparative Analysis

### Current State (`complytime/website`)

- No Go tooling — project is Node.js/Hugo only
- No `go.mod` or `go.sum`
- No `cmd/` directory
- Project documentation content is manually synced (some docs include `<!-- synced from complytime/<repo>@main (hash) on date -->` comments indicating a manual or external sync process)
- No `data/projects.json` generation
- No automated README or metadata fetching from GitHub API

### Target State (`test-website`)

- `go.mod` — Go module shared with Hugo Modules
- `cmd/sync-content/main.go` (556 lines) — Go CLI tool that:
  - Lists all repos in the `complytime` GitHub org via REST API
  - Fetches README.md, description, primary language, star count, last push date for each repo
  - Generates `content/projects/[repo].md` with frontmatter and README content
  - Writes `data/projects.json` for the landing page project cards template
  - Checks each repo for `.specify/` directory; fetches `constitution.md`, `spec.md`, `plan.md` into `content/specs/[repo]/`
  - Records source commit SHA in frontmatter for staleness detection
  - Supports flags: `--org`, `--token`, `--output`, `--include`, `--exclude`, `--dry-run`, `--summary`
  - Handles GitHub API rate limiting
  - Uses `net/http` and `encoding/json` only (no third-party GitHub client)

### Delta

| Item | Action | Details |
|------|--------|---------|
| `go.mod` | Add | Go module definition, shared with Hugo Modules |
| `go.sum` | Add | Go dependency checksums |
| `cmd/sync-content/main.go` | Add | 556-line Go CLI for GitHub org content sync |
| `content/projects/` | Generated | Output directory for per-repo Markdown pages |
| `data/projects.json` | Generated | Output file for landing page project cards |
| `content/specs/` | Generated | Output directory for `.specify/` artifacts |

### Conflicts

- Adding `go.mod` introduces Go as a project dependency. Contributors will need Go installed (in addition to Node.js and Hugo).
- The existing `.devcontainer/Dockerfile` does not include Go. It may need updating (separate concern).
- The existing manually-synced docs under `content/docs/projects/` may overlap with the sync tool's `content/projects/` output. These are different paths (`/docs/projects/` vs `/projects/`) so no direct conflict, but the relationship should be documented.

## Acceptance Criteria

1. `go.mod` exists and is valid (`go mod verify` passes)
2. `cmd/sync-content/main.go` compiles without errors (`go build ./cmd/sync-content`)
3. Running `go run ./cmd/sync-content --help` shows usage information
4. Running with `--org complytime --dry-run` lists repos without writing files
5. Running with `--org complytime --token $GITHUB_TOKEN` generates:
   - `content/projects/*.md` files with valid Hugo frontmatter
   - `data/projects.json` with project metadata array
   - `content/specs/*/` directories for repos with `.specify/`
6. Generated frontmatter includes: title, description, language, stars, last_updated, source_sha
7. Rate limiting is handled gracefully (no crashes on 403 responses)
8. `go vet ./...` passes with no issues

## Migration Steps

1. Add `go.mod` with module path matching the repo
2. Add `cmd/sync-content/main.go`
3. Run `go mod tidy` to generate `go.sum`
4. Verify build: `go build ./cmd/sync-content`
5. Test with `--dry-run` flag
6. Document Go requirement in README.md or CONTRIBUTING.md

## Rollback Plan

Delete `go.mod`, `go.sum`, and `cmd/` directory. Remove any generated `content/projects/` and `data/projects.json` files.
