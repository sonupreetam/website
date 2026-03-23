# Design Architecture: Go Content Sync Tool

**Feature**: `006-go-sync-tool`

> **Scope**: This document covers *design rationale* — problem statement, architecture decisions, and what was achieved. For code-level details (type system, function map, data flow), see [sync-tool-architecture.md](sync-tool-architecture.md).

---

## 1. Problem Statement

The ComplyTime website (`complytime.dev`) is a Hugo static site that documents a growing ecosystem of open-source compliance tools hosted across multiple repositories in the `complytime` GitHub organization. Before this feature, project documentation was manually copied into the site — a process that was error-prone, inconsistent, and could not scale as new repos were added to the org. There was no automated mechanism to keep README content, metadata, or project listings in sync with upstream repositories.

The goal: build a tool that automatically discovers every project in the org, fetches its documentation, and generates the Hugo content pages and landing page data — eliminating manual duplication entirely.

---

## 2. Architecture Overview

The system follows a **source → transform → render** pipeline where the GitHub API is the single source of truth, a Go CLI tool handles fetching and transformation, and Hugo consumes the generated artifacts to produce the static site.

```
┌─────────────────────────────────────────────────────────────────┐
│                        GitHub Organization                       │
│                        (complytime)                               │
│                                                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │ complyctl │  │ comply-  │  │collector-│  │  ...N    │        │
│  │          │  │ scribe   │  │components│  │  repos   │        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
└────────────────────────┬────────────────────────────────────────┘
                         │  GitHub REST API
                         │  (peribolos.yaml, repo metadata, README, branches)
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│               Go Sync Tool (cmd/sync-content)                    │
│                                                                   │
│  ┌──────────────────┐    ┌──────────────────────────────┐       │
│  │  Governance Sync  │    │  Config Overlay Engine        │       │
│  │   Engine          │    │                                │       │
│  │ • Fetch peribolos │    │ • Parse sync-config.yaml      │       │
│  │   .yaml registry  │    │ • Fetch declared files        │       │
│  │ • Get repo meta   │    │ • Apply transforms:           │       │
│  │ • Apply include/  │    │   - inject_frontmatter        │       │
│  │   exclude filters │    │   - rewrite_links             │       │
│  │ • Fetch README    │    │   - strip_badges              │       │
│  │ • Fetch branch SA │    │ • Respect skip_org_sync       │       │
│  └────────┬─────────┘    └──────────┬───────────────────┘       │
│           │                          │                            │
│           ▼                          ▼                            │
│  ┌──────────────────────────────────────────────────────┐       │
│  │              Content Generation Layer                  │       │
│  │                                                        │       │
│  │  • Section index (_index.md) — frontmatter only       │       │
│  │  • Overview page (overview.md) — README content       │       │
│  │  • Doc sub-pages ({doc}.md) — from scan_paths         │       │
│  │  • Project cards (projects.json) — landing page data  │       │
│  │  • Sync manifest (.sync-manifest.json) — cleanup      │       │
│  └──────────────────────┬───────────────────────────────┘       │
│                          │                                        │
│  ┌──────────────────────┴───────────────────────────────┐       │
│  │              Safety & Integrity Layer                   │       │
│  │                                                        │       │
│  │  • Dry-run by default (--write required)              │       │
│  │  • SHA-based change detection (branch + README)       │       │
│  │  • Byte-level dedup (skip unchanged files)            │       │
│  │  • Stale content cleanup via manifest diff            │       │
│  │  • Idempotent runs (same input → same output)         │       │
│  └──────────────────────────────────────────────────────┘       │
└────────────────────────┬────────────────────────────────────────┘
                         │  Filesystem (content/, data/)
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Hugo Static Site Generator                     │
│                                                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐      │
│  │ Landing Page  │  │ Docs Sidebar │  │ Project Pages    │      │
│  │               │  │              │  │                    │      │
│  │ Reads         │  │ Hugo section │  │ /docs/projects/   │      │
│  │ data/         │  │ discovery    │  │   {repo}/         │      │
│  │ projects.json │  │ from         │  │     overview.md   │      │
│  │ for card grid │  │ _index.md    │  │     {doc}.md      │      │
│  └──────────────┘  └──────────────┘  └──────────────────┘      │
│                                                                   │
│  Theme: Thulite Doks │ Build: hugo --minify --gc                │
└────────────────────────┬────────────────────────────────────────┘
                         │  public/
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│              GitHub Pages (complytime.dev)                        │
└─────────────────────────────────────────────────────────────────┘
```

---

## 3. Core Design Decisions and Rationale

### 3.1 Governance-Driven Sync + Config Overlay

**Decision**: The tool operates in two complementary layers that compose into a single run.

| Layer | What It Does | When It Applies |
|-------|-------------|-----------------|
| **Governance Sync** (baseline) | Reads the `peribolos.yaml` governance registry to get the list of repos, then fetches metadata via the GitHub API. Generates a project page and landing card for each eligible repo. | Always — every repo registered in peribolos gets coverage. |
| **Config Overlay** (precision) | Reads `sync-config.yaml` to sync specific files with transforms (frontmatter injection, link rewriting, badge stripping). | Only for repos that need precise control over their documentation layout. |

**Why this flow was chosen**:

- **Governance-driven discovery**: When a new repo is added to the `complytime` org's `peribolos.yaml`, it automatically appears on the website after the next sync. The governance registry is the single source of truth for which repos belong to the org.
- **Precision where needed**: Key projects like `complyctl` may need custom frontmatter, specific doc files synced to specific paths, or transforms applied. The config layer handles this without disrupting the baseline.
- **`skip_org_sync` toggle**: For repos with config-driven content, the auto-generated pages can be suppressed while still producing a landing card. This prevents duplicate content.

**Alternative considered**: Direct GitHub org API listing (`GET /orgs/{org}/repos`). Rejected because it bypasses the governance registry and could include repos not yet approved for the website. That approach also demands post-fetch filtering on API metadata (e.g., archived, fork) — every new filter condition requires a code change. Peribolos is the org's authoritative repo list; if a repo is registered there, the tool trusts that decision.

### 3.2 Single-Package, Multi-File Architecture

**Decision**: The sync tool is organized as multiple files within `package main` in `cmd/sync-content/` (~2,100 lines across 10 source files, ~2,300 lines across 10 test files). Files are split by domain: `config.go`, `github.go`, `transform.go`, `hugo.go`, `sync.go`, `manifest.go`, `cleanup.go`, `path.go`, `lock.go`, and `main.go` (entry point). Test files mirror source files 1:1 with a shared `helpers_test.go`.

**Why**: The tool keeps everything in `package main` — no interfaces, no sub-packages, no import cycles. The multi-file split is purely organizational: each file owns a distinct domain (GitHub API, transforms, Hugo pages, etc.), making it easy to navigate, review PRs by concern, and understand test coverage at a glance.

**Evolution**: The tool was originally a single `main.go` file. At ~1880 lines it exceeded comfortable single-file development, and the split was made without changing any APIs or introducing package boundaries.

### 3.3 Dry-Run by Default

**Decision**: The tool performs no disk I/O unless the `--write` flag is explicitly passed.

**Why**: Contributors frequently clone the repo and run the sync tool to preview changes. Dry-run protects their local working tree from accidental overwrites. This is especially important because generated content is gitignored — there's no `git checkout` safety net. The tool logs every file it *would* write, giving full visibility without risk.

### 3.4 SHA-Based Change Detection (Two-Tier)

**Decision**: Change detection uses two SHA values stored in the `_index.md` frontmatter of each project.

```
Tier 1: Branch SHA (params.source_sha)
  └─ Fast pre-filter: if the branch HEAD hasn't moved, skip all API calls for this repo.

Tier 2: README SHA (params.readme_sha)  
  └─ Content-level: if the branch moved but README content is identical, report as "unchanged".
```

**Why**: The GitHub API has rate limits (60 req/hr unauthenticated, 5000/hr with token). The branch SHA check avoids fetching README content for repos that haven't changed since the last sync. For an org with 10+ repos, this cuts API calls dramatically on repeat runs. The two-tier approach means the tool correctly reports "unchanged" even when non-documentation commits move the branch HEAD.

### 3.5 Content Separation: Section Index vs Overview Page

**Decision**: Each repo generates two files:
- `_index.md` — Hugo section index with metadata frontmatter only (no body)
- `overview.md` — README content as a child page

**Why**: The Doks theme renders `_index.md` body content inline at the section level, which prevents the sidebar from showing collapsible sections with child pages. By keeping `_index.md` as frontmatter-only, Hugo treats the repo as a section heading. The README content lives in `overview.md` (weight: 1), which becomes the first navigable child. This enables the sidebar to show:

```
▸ complyctl          ← _index.md (section, collapsed)
    Overview         ← overview.md (README content)
    Quick Start      ← doc sub-page from docs/
    Man Pages        ← doc sub-page from docs/
```

### 3.6 Hugo Cascade for Sidebar Behavior

**Decision**: Sidebar collapse state is controlled by a `cascade` block in the hand-maintained `content/docs/projects/_index.md`, not by the sync tool.

**Why**: Sidebar behavior is a UI concern, not a content concern. Using Hugo's cascade mechanism means:
- The sync tool stays focused on content generation
- New repos automatically inherit collapse behavior
- Changing sidebar behavior requires editing one file, not the sync tool
- Sub-folder sections (e.g., `complyctl/man`) remain expanded because the cascade target only matches repo-level sections

### 3.7 Concurrent Processing with Bounded Workers

**Decision**: Repos are processed concurrently using a worker pool (default: 5 goroutines) with a shared mutex-protected `syncResult` for counters.

**Why**: With 10+ repos and multiple API calls per repo (branch SHA, README, docs directory listing), serial processing would take 2-3 minutes. Concurrent processing with 5 workers completes in under 60 seconds. The `--workers` flag allows tuning for different environments (CI with higher limits, local dev with conservative defaults).

### 3.8 Generated Content Is Gitignored

**Decision**: All sync tool output is excluded from version control via `.gitignore`:
- `content/docs/projects/*/` (generated repo pages)
- `data/projects.json` (landing page cards)
- `.sync-manifest.json` (cleanup tracking)

**Why**: Derived content is not committed — the repository tracks only source files. CI generates all content from scratch on every build, guaranteeing freshness. The gitignore pattern `content/docs/projects/*/` uses a directory glob that preserves the hand-maintained `content/docs/projects/_index.md` while excluding all repo subdirectories.

### 3.9 Content Approval Gate (Lockfile + PR Workflow)

**Decision**: Upstream content changes require human review before reaching production. A committed `.content-lock.json` file pins each repo to an approved branch SHA. The deploy workflow fetches content at those locked SHAs — not HEAD. A weekly check workflow detects upstream changes and opens a PR to update the lockfile.

**Why**: The original daily cron deploy model would silently propagate broken or undesirable upstream content to production. With an active org and frequent upstream changes, gating deployments on reviewed PRs prevents content regressions. The lockfile is lightweight (a JSON map of repo name → SHA), and the weekly cadence matches the team's review capacity.

**How it works**:
- `--lock .content-lock.json` gates the sync tool to fetch content only at approved SHAs
- `--update-lock` writes current upstream SHAs to the lockfile (used by the check workflow)
- `sync-content-check.yml` runs weekly, compares upstream SHAs to the lockfile, and creates a PR if they differ
- Merging the PR updates the lockfile, and the deploy workflow picks up the new SHAs on the next push to `main`

`.content-lock.json` is a committed control file, not derived content. The three-workflow PR-gated model ensures all content changes are reviewed.

---

## 4. Data Flow

### 4.1 Sync Pipeline (per repo)

```
GitHub API: GET /repos/{org}/.github/contents/peribolos.yaml  (governance registry)
    │   + GET /repos/{owner}/{repo}  per repo  (metadata enrichment)
    │
    ▼
Filter: apply --include/--exclude lists
    │
    ▼
For each eligible repo (concurrent, bounded by --workers):
    │
    ├─ GET /repos/{owner}/{repo}/branches/{branch}
    │   └─ Compare branch SHA with stored params.source_sha
    │       ├─ Unchanged → skip (no API calls, report "unchanged")
    │       └─ Changed → continue
    │
    ├─ GET /repos/{owner}/{repo}/readme
    │   └─ Compare README SHA with stored params.readme_sha
    │       ├─ Unchanged → report "unchanged" (branch moved, content same)
    │       └─ Changed → continue
    │
    ├─ Transform README content:
    │   ├─ Strip leading H1 (title already in frontmatter)
    │   ├─ Shift headings down one level (H1→H2, H2→H3, …)
    │   ├─ Title Case headings (acronym-aware, normalises ALL CAPS)
    │   ├─ Strip CI badge lines
    │   └─ Rewrite relative links → absolute GitHub URLs
    │
    ├─ Generate files (if --write):
    │   ├─ _index.md  (section index, frontmatter only)
    │   ├─ overview.md (README body with transforms)
    │   └─ {doc}.md   (from discovery.scan_paths)
    │
    └─ Build ProjectCard for data/projects.json
```

### 4.2 Config Overlay Pipeline (per source)

```
Parse sync-config.yaml
    │
    ▼
For each source in config.sources:
    │
    ├─ If skip_org_sync: true → suppress _index.md and overview.md
    │
    └─ For each file in source.files:
        │
        ├─ GET /repos/{owner}/{repo}/contents/{src}?ref={branch}
        │
        ├─ Apply transforms:
        │   ├─ inject_frontmatter → prepend YAML frontmatter block
        │   ├─ rewrite_links → convert ./relative to absolute GitHub URLs
        │   └─ strip_badges → remove CI badge lines
        │
        └─ Write to config dest path (if --write)
```

### 4.3 Post-Sync Pipeline

```
All repos processed
    │
    ├─ Sort ProjectCards alphabetically
    ├─ Write data/projects.json (if --write)
    │
    ├─ Diff current manifest vs previous .sync-manifest.json
    │   └─ Remove orphaned files (repos removed from org)
    │
    ├─ Write .sync-manifest.json (if --write)
    │
    └─ Report summary:
        ├─ added / updated / unchanged / removed counts
        ├─ GITHUB_OUTPUT variables (for CI)
        └─ GITHUB_STEP_SUMMARY (for PR reviews)
```

---

## 5. What Was Achieved

### 5.1 Governance-Driven Project Discovery

Every repository registered in the `complytime` org's `peribolos.yaml` governance registry automatically receives:
- A project page in the docs sidebar (`/docs/projects/{repo}/`)
- A landing page card on the homepage
- README content transformed and rendered as a child page

New repos appear on the next sync run once added to `peribolos.yaml` — no sync tool configuration changes required.

### 5.2 Single Source of Truth

All project documentation now traces back to a canonical source — the GitHub API and upstream repository files. The manual copy-paste workflow that previously maintained 16 documentation files has been eliminated.

### 5.3 Safe Developer Experience

- **Dry-run by default**: Contributors can safely preview sync output without modifying their local tree
- **Idempotent runs**: Running the tool twice produces identical output
- **Change detection**: Only changed repos trigger API fetches and file writes
- **Stale cleanup**: Repos removed from the org have their generated content automatically deleted

### 5.4 CI/CD Integration

Three GitHub Actions workflows automate the entire pipeline:

1. **CI** (`ci.yml`) — validates PRs with dry-run sync (with `--lock`), Go checks, and Hugo build.
2. **Content Sync Check** (`sync-content-check.yml`) — runs weekly to detect upstream changes, updates `.content-lock.json`, and opens a PR for human review.
3. **Deploy** (`deploy-gh-pages.yml`) — on push to `main` (or manual dispatch), syncs content at the approved SHAs in `.content-lock.json`, builds Hugo, and publishes to GitHub Pages.

Upstream content changes require a reviewed PR before reaching production — no unreviewed content is deployed.

### 5.5 Performance

| Metric | Target | Achieved |
|--------|--------|----------|
| Full org sync (with token) | < 60s | Yes — concurrent processing with 5 workers |
| Hugo build time | < 2s | Yes — generated Markdown is lightweight |
| Repeat sync (no changes) | Minimal | Yes — SHA-based skip avoids API calls |

### 5.6 Comprehensive Testing

- **57 test functions** across 10 `*_test.go` files covering pure transformations, integration with mock API, lockfile operations, `ref` parameter threading, hardening (path traversal, context cancellation, stale cleanup), and heading/casing transforms
- **Integration tests** using `net/http/httptest` to mock the GitHub API and verify end-to-end processing
- **Race detector** validation passed — `go test -race` with zero warnings

---

## 6. Technology Stack

| Component | Technology | Role |
|-----------|-----------|------|
| Sync Tool | Go 1.25, `gopkg.in/yaml.v3` | Content fetching, transformation, generation |
| Site Generator | Hugo 0.155.1 extended | Static site build from Markdown + templates |
| Theme | Thulite Doks | Documentation UI, sidebar, search |
| Build System | Node.js 22, npm | Theme dependency management, PostCSS |
| CI/CD | GitHub Actions | Automated sync, build, and deploy |
| Hosting | GitHub Pages | Production site at `complytime.dev` |
| API | GitHub REST API v3 | Source data for all project content |

---

## 7. Deferred Items

The following items were identified during development and code audit but deferred as they are not bugs at the current scale of ~10 repos:

| Item | Reason for Deferral |
|------|-------------------|
| Serial recursive API calls | Meets 60s target; parallel recursion adds complexity |
| HTTP connection pooling tuning | 10 repos — default pool is sufficient |
| Redundant README fetch during doc page scan | N/A — discovery mode removed in T054; doc pages now scanned via `syncRepoDocPages` using config `scan_paths` |
| Hardcoded GitHub API URL | Only one API to target today |
| Public-only repo listing | Org has no private repos requiring sync |
| No log level control (`--verbose` / `--quiet`) | Structured logging is sufficient for debugging |
| No config schema version | Single config format, no migration needed yet |

These can be addressed in follow-up work if the tool's usage grows.

---

## 8. Summary

The Go Content Sync Tool transforms the ComplyTime website from a manually-maintained documentation site into an automatically-synchronized platform that scales with the organization. By combining governance-driven repo discovery (via `peribolos.yaml`) with a precision config overlay, the tool provides both convenience and control. The dry-run-first safety model, SHA-based change detection, and idempotent operation ensure that the tool is safe to run in any environment — from a contributor's laptop to a CI pipeline.

The result is a focused, single-package Go tool with comprehensive test coverage, minimal dependencies, and full CI/CD integration including a PR-gated content approval workflow.
