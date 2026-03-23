# Plan

Technical architecture and implementation approach for complytime.dev.

## Technology Stack

| Layer | Choice | Rationale |
|-------|--------|-----------|
| Site framework | Hugo extended + Doks theme | Fast static site generator; Doks provides docs-oriented features (search, sidebar, dark mode) out of the box (Constitution I) |
| Content sync | Go CLI (`cmd/sync-content`) | Reads `peribolos.yaml` for repo list, fetches READMEs, metadata, star counts, and config-declared files from the GitHub API. Pure Go, single binary (Constitution II, XIV) |
| Module system | Go Modules (`go.mod`) | Shared between Hugo Modules and the Go sync tool; single dependency file |
| Config parsing | `gopkg.in/yaml.v3` | Only permitted third-party Go dependency (Constitution II) |
| CI/CD | GitHub Actions | Three-workflow model: CI (PR validation), Content Sync Check (weekly lockfile update + PR), Deploy (push to main at locked SHAs) (Constitution XV) |
| Hosting | GitHub Pages | Custom domain at `complytime.dev` (Constitution XVI) |
| Search | FlexSearch (Doks built-in) | Client-side full-text search, no external service needed |
| Styling | SCSS overrides on Doks | Cyan/teal palette, DM Sans, dark-theme-first (Constitution VI) |

## Project Structure

```text
complytime-website/
├── .specify/                        # Spec-kit: constitution, spec, plan, tasks
│   ├── memory/
│   │   └── constitution.md          # Project constitution (17 principles)
│   ├── spec.md                      # Root-level specification (what the site delivers)
│   ├── plan.md                      # This file (how the site is built)
│   └── tasks.md                     # Root-level task backlog
├── specs/
│   └── 006-go-sync-tool/            # Feature spec for the org-scan sync tool port
│       ├── spec.md
│       ├── plan.md
│       ├── tasks.md
│       ├── research.md
│       ├── data-model.md
│       └── quickstart.md
├── cmd/
│   └── sync-content/
│       ├── main.go                  # Entry point and orchestration (~330 lines)
│       ├── config.go                # Config types and loading (incl. Peribolos types)
│       ├── github.go                # GitHub API client and types (incl. peribolos fetch)
│       ├── transform.go             # Markdown transforms
│       ├── hugo.go                  # Hugo page and card generation
│       ├── sync.go                  # Sync logic and repo processing
│       ├── manifest.go              # Manifest I/O and state tracking
│       ├── cleanup.go               # Orphan and stale content removal
│       ├── path.go                  # Path validation utilities
│       ├── lock.go                  # Content lockfile read/write/query
│       └── *_test.go                # Tests mirror source files (~2,060 lines, 10 files, 51 functions)
├── config/
│   └── _default/
│       ├── hugo.toml                # Site title, baseURL (complytime.dev), outputs
│       ├── module.toml              # Hugo module mounts (Doks theme, data, layouts)
│       ├── params.toml              # Doks theme params, colors, SEO, FlexSearch
│       ├── languages.toml           # Language settings
│       ├── markup.toml              # Markup configuration
│       └── menus/
│           └── menus.en.toml        # Navigation: docs sidebar + top nav + social + footer
├── content/
│   ├── _index.md                    # Landing page frontmatter (lead text, hero content)
│   ├── docs/
│   │   ├── _index.md                # Docs section root
│   │   ├── getting-started/
│   │   │   └── _index.md            # Onboarding guide (hand-maintained, committed)
│   │   └── projects/
│   │       ├── _index.md            # Section index with cascade for sidebar collapsing (committed)
│   │       └── {repo}/              # Generated per-repo content (gitignored)
│   │           ├── _index.md        # Section index (frontmatter only, no body)
│   │           ├── overview.md      # README content as child page
│   │           └── {doc}.md         # Doc pages from discovery.scan_paths
├── data/
│   └── projects.json                # Generated landing page card data (gitignored)
├── .sync-manifest.json              # Tracks written files for orphan cleanup (gitignored)
├── .content-lock.json               # Approved upstream SHAs per repo (committed)
├── layouts/
│   ├── home.html                    # Landing page: hero, features grid, projects, CTAs
│   ├── docs/
│   │   └── list.html                # Docs list with sidebar navigation
│   ├── shortcodes/
│   │   └── project-cards.html       # Reusable project cards shortcode
│   ├── _partials/
│   │   ├── footer/
│   │   │   └── script-footer-custom.html
│   │   ├── header/
│   │   │   └── header.html
│   │   └── main/
│   │       └── edit-page.html
│   └── _default/
│       └── _markup/
│           ├── render-heading.html  # Custom heading rendering with anchor links
│           └── render-image.html    # Custom image rendering
├── assets/
│   └── scss/
│       └── common/
│           ├── _custom.scss         # Brand overrides (Constitution VI)
│           └── _variables-custom.scss
├── static/                          # Favicon, cover image, CNAME
├── .github/
│   └── workflows/
│       ├── deploy-gh-pages.yml      # Deploy pipeline (sync at locked SHAs, Hugo build, GitHub Pages)
│       ├── ci.yml                   # PR validation (lint, test, dry-run with --lock, build)
│       └── sync-content-check.yml   # Weekly content check (--update-lock, PR creation)
├── sync-config.yaml                 # Declarative file sync manifest (hybrid mode)
├── go.mod                           # Go module (shared with Hugo)
├── go.sum
└── package.json                     # npm dependencies (Doks, Thulite)
```

## Go Sync Tool (`cmd/sync-content`)

A Go CLI that syncs content the Hugo Module system cannot handle.

### Inputs

- `--org complytime` — GitHub organization (reads `peribolos.yaml` from `{org}/.github` repo)
- `--token` / `GITHUB_TOKEN` env var — API authentication
- `--config sync-config.yaml` — optional hybrid mode config
- `--write` — required for disk I/O (default is dry-run per Constitution XII)
- `--workers 5` — bounded concurrency pool
- `--timeout 3m` — context deadline
- `--repo owner/name` — sync a single repo (validated against peribolos registry)
- `--include` / `--exclude` — comma-separated allow/deny lists for repo names
- `--summary path` — write markdown change summary to file
- `--lock path` — path to `.content-lock.json` for content approval gating (Constitution XV)
- `--update-lock` — write current upstream SHAs to the lockfile (requires `--lock`)

### Process

1. Fetch `peribolos.yaml` from the org's `.github` repo to get the authoritative list of repositories
2. For each eligible repo (peribolos is the governance gate; `--include`/`--exclude` narrow within), fetch metadata from the GitHub API: branch SHA, README content + blob SHA, description, primary language, star count, last push date
3. Two-tier change detection: skip all fetches if branch SHA unchanged (fast path); compare README blob SHA for content-level accuracy (slow path)
4. Generate a section index at `content/docs/projects/{repo}/_index.md` (frontmatter only — metadata, SHAs, SEO fields)
5. Generate an overview page at `content/docs/projects/{repo}/overview.md` (README content with transforms: strip leading H1, strip badges, rewrite relative links)
6. If `discovery.scan_paths` is configured, auto-sync Markdown files from upstream directories (e.g., `docs/`) as additional doc sub-pages with auto-generated frontmatter
7. Write `data/projects.json` with structured metadata for landing page project cards (card URLs point to `/docs/projects/{repo}/`)
8. If `--config` is provided, process config-declared file syncs: fetch declared files, apply transforms (`inject_frontmatter`, `rewrite_links`, `strip_badges`), write to `dest` paths
9. Record all written files in `.sync-manifest.json`; byte-level comparison before each write to avoid unnecessary churn
10. Clean orphaned files from previous runs via manifest diff; clean stale content for repos no longer in the org

### Output Files

- `content/docs/projects/{repo}/_index.md` — section index (frontmatter only) per repo
- `content/docs/projects/{repo}/overview.md` — README content as a child page
- `content/docs/projects/{repo}/{doc}.md` — doc pages from `discovery.scan_paths` directories
- `data/projects.json` — array of `ProjectCard` objects consumed by `layouts/home.html` and `layouts/shortcodes/project-cards.html`
- `.sync-manifest.json` — list of files written for orphan cleanup on subsequent runs
- `.content-lock.json` — updated with current upstream SHAs when `--update-lock` is set
- Config-declared files at arbitrary `dest` paths (when `--config` is used)

### Key Design Decisions

- `net/http` and `encoding/json` only — no third-party GitHub client (Constitution II)
- Single `package main` with domain-organised files in `cmd/sync-content/` (~1,920 lines across 10 source files) — no separate packages, no `internal/` (Constitution XIV)
- `log/slog` for all operational output with structured fields (Constitution XI)
- Exponential backoff on 403/429 with `Retry-After` and `X-RateLimit-Reset` respect

## GitHub Actions Pipeline

Three-workflow model per Constitution XV (v1.3.0). Upstream content changes require human review before reaching production.

### `deploy-gh-pages.yml`

Triggers: `push` to `main`

Steps:
1. Checkout repository
2. Setup Go (from `go.mod`), Node.js 22, Hugo 0.155.1 (extended)
3. `npm ci` — install Doks theme dependencies
4. `go vet ./...` — static analysis
5. `gofmt -l ./cmd/sync-content/` — formatting check
6. `go test -race ./cmd/sync-content/...` — tests with race detector
7. `go run ./cmd/sync-content --org complytime --config sync-config.yaml --lock .content-lock.json --write` — sync content at approved SHAs
8. `hugo --minify --gc --baseURL "https://complytime.dev/"` — build the site
9. Deploy to GitHub Pages via `actions/deploy-pages`

### `ci.yml`

Triggers: `pull_request` targeting `main`

Steps:
1. Checkout, setup Go (from `go.mod`), Node.js 22, Hugo 0.155.1 (extended)
2. `npm ci` — install Doks theme dependencies
3. `go test -race ./cmd/sync-content/...` — tests with race detector
4. `go run ./cmd/sync-content --org complytime --config sync-config.yaml --lock .content-lock.json --write` — sync content with lockfile validation
5. `hugo --minify --gc` — Hugo build

### `sync-content-check.yml`

Triggers: `schedule` (weekly Monday 06:00 UTC), `workflow_dispatch`

Steps:
1. Checkout, setup Go (from `go.mod`)
2. `go run ./cmd/sync-content --org complytime --config sync-config.yaml --lock .content-lock.json --update-lock --summary sync-summary.md` — detect upstream changes and update lockfile
3. Create/update PR via `peter-evans/create-pull-request` with updated `.content-lock.json` and sync summary

## Styling

The site uses the Doks default dark theme as the base. Custom SCSS overrides in `assets/scss/common/_custom.scss` and `_variables-custom.scss` adjust colors, typography, and spacing:

- **Dark theme**: text `#e2e8f0`, accent `#06b6d4` (cyan)
- **Light theme**: text `#0f172a`, accent `#0891b2` (teal)
- No CSS framework beyond what Doks provides (Constitution VI)

## Performance Targets

- Hugo build time: < 2 seconds for current content volume (Constitution VIII)
- Lighthouse Performance: 90+ (Constitution VIII)
- Sync tool: full org scan < 60 seconds with valid token
- WCAG 2.1 AA across all pages (Constitution VII)

## Feature Roadmap

| Feature | Branch | Status | Spec |
|---------|--------|--------|------|
| 006: Go sync tool port | `006-go-sync-tool` | All phases complete (T001–T054) — pending merge | `specs/006-go-sync-tool/spec.md` |
| 006: Hardening | `006-go-sync-tool` | Done (T028–T037) | `specs/006-go-sync-tool/tasks.md` Phase 8 |
| 006: Content approval gate | `006-go-sync-tool` | Done (T038–T047) | `specs/006-go-sync-tool/tasks.md` Phase 11 |
| 006: Governance-driven discovery | `006-go-sync-tool` | Done (T048–T054) | `specs/006-go-sync-tool/tasks.md` Phase 12 |
| CI PR workflow | `006-go-sync-tool` | Done (T014) | `.github/workflows/ci.yml` |
| Specs browser (`/specs/{repo}/`) | — | Deferred | `.specify/spec.md` §Specs Browser |
