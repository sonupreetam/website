# Data Model: Go Content Sync Tool

**Date**: 2026-03-12
**Source**: Extracted from `cmd/sync-content/` (`package main`, 10 source files) and `sync-config.yaml`

## Input Entities

### PeribolosConfig (governance registry — `peribolos.yaml` in `{org}/.github`)

| Field | Type | Source | Description |
|-------|------|--------|-------------|
| Orgs | map[string]PeribolosOrg | `yaml:"orgs"` | Org name → org definition |

### PeribolosOrg

| Field | Type | Source | Description |
|-------|------|--------|-------------|
| Repos | map[string]PeribolosRepo | `yaml:"repos"` | Repo name → repo config |

### PeribolosRepo

| Field | Type | Source | Description |
|-------|------|--------|-------------|
| Description | string | `yaml:"description"` | Repo description (may differ from GitHub API description) |
| DefaultBranch | string | `yaml:"default_branch"` | Default branch name |

**Data flow**: `peribolos.yaml` provides the authoritative list of repo names. For each repo name, the tool fetches full metadata from the GitHub API (`GET /repos/{owner}/{name}`) to populate the `Repo` struct below. Peribolos descriptions are not used — API metadata takes precedence for consistency with stars, language, and topics.

### Repo (GitHub API response — per-repo metadata)

| Field | Type | Source | Description |
|-------|------|--------|-------------|
| Name | string | `json:"name"` | Repository short name (e.g., `complyctl`) |
| FullName | string | `json:"full_name"` | Owner/name format (e.g., `complytime/complyctl`) |
| Description | string | `json:"description"` | GitHub repo description |
| Language | string | `json:"language"` | Primary language (e.g., `Go`) |
| StargazersCount | int | `json:"stargazers_count"` | GitHub star count |
| HTMLURL | string | `json:"html_url"` | GitHub web URL |
| DefaultBranch | string | `json:"default_branch"` | Default branch name (e.g., `main`) |
| PushedAt | string | `json:"pushed_at"` | ISO 8601 timestamp of last push |
| Topics | []string | `json:"topics"` | GitHub topics (used for type derivation) |

**Relationships**: One Repo produces one ProjectCard, one section index (`_index.md`), and one overview page (`overview.md`). If `skip_org_sync` is true, the section index and overview are suppressed.
**Validation**: `isValidRepoName` rejects empty, `.`, `..`, and names containing `/`, `\`, or `..`.
**Filtering**: Peribolos is the governance gate — only repos declared in `peribolos.yaml` are considered. `--include` and `--exclude` flags narrow within that set. No API metadata filtering (e.g., archived, fork) is applied; governance decisions are trusted.

### FileResponse (GitHub Contents API response)

| Field | Type | Source | Description |
|-------|------|--------|-------------|
| Content | string | `json:"content"` | Base64-encoded file content |
| Encoding | string | `json:"encoding"` | Encoding type (typically `base64`) |
| SHA | string | `json:"sha"` | Git blob SHA for change detection |

### DirEntry (GitHub Contents API directory listing)

| Field | Type | Source | Description |
|-------|------|--------|-------------|
| Name | string | `json:"name"` | File or directory name |
| Path | string | `json:"path"` | Path relative to repo root |
| Type | string | `json:"type"` | `file` or `dir` |

### BranchResponse (GitHub Branches API response)

| Field | Type | Source | Description |
|-------|------|--------|-------------|
| Commit.SHA | string | `json:"sha"` | HEAD commit SHA for fast change detection |

### SyncConfig (YAML config file)

| Field | Type | Description |
|-------|------|-------------|
| Defaults | Defaults | Fallback values applied to every source |
| Sources | []Source | List of config-declared repositories |
| Discovery | Discovery | Auto-detection of new repos and doc files |

### Discovery (config section for auto-detection)

| Field | Type | Description |
|-------|------|-------------|
| IgnoreRepos | []string | Repo names to exclude from discovery reports |
| IgnoreFiles | []string | Filenames to skip during doc page auto-sync (e.g., `CHANGELOG.md`) |
| ScanPaths | []string | Directories to scan for Markdown files in each repo (e.g., `docs`) |

### Defaults (fallback values for sources)

| Field | Type | Description |
|-------|------|-------------|
| Branch | string | Fallback branch for all sources (default: `main`) |

### Source (per-repo config entry)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| Repo | string | (required) | Owner/name format (e.g., `complytime/complyctl`) |
| Branch | string | Defaults.Branch | Branch override for this source |
| SkipOrgSync | bool | `false` | When `true`, suppress auto-generated section index and overview page |
| Files | []FileSpec | (required) | List of files to sync |

### FileSpec (per-file sync declaration)

| Field | Type | Description |
|-------|------|-------------|
| Src | string | Path relative to repo root (e.g., `docs/QUICK_START.md`) |
| Dest | string | Path relative to site root (e.g., `content/docs/projects/complyctl/quick-start.md`) |
| Transform.InjectFrontmatter | map[string]any | YAML frontmatter key-value pairs to prepend |
| Transform.RewriteLinks | bool | Convert relative links to absolute GitHub URLs |
| Transform.StripBadges | bool | Remove CI badge lines from start of content |

## Output Entities

### ProjectCard (data/projects.json entry)

| Field | Type | Source | Description |
|-------|------|--------|-------------|
| name | string | Repo.Name | Repository short name |
| language | string | Repo.Language or "Unknown" | Primary language |
| type | string | deriveProjectType(Repo) | Human-readable type (CLI Tool, Automation, Observability, Framework, Library) |
| description | string | Repo.Description | GitHub repo description |
| url | string | `/docs/projects/{name}/` | Local docs URL |
| repo | string | Repo.HTMLURL | GitHub web URL |
| stars | int | Repo.StargazersCount | Star count |

**Derivation**: `type` is inferred from `Topics` and `Description` using keyword matching:
- `cli` topic or "command-line"/"cli" in description → "CLI Tool"
- `automation` topic or "automation"/"automat" in description → "Automation"
- `observability` topic or "observability"/"collector" in description → "Observability"
- `framework` topic or "framework"/"bridging" in description → "Framework"
- Default → "Library"

### Section Index (content/docs/projects/{repo}/_index.md)

A lightweight Hugo section page with frontmatter only and no body content. Enables the Doks sidebar to render the repo as a collapsible section heading with child pages underneath.

Hugo frontmatter schema:

| Field | Type | Source | Description |
|-------|------|--------|-------------|
| title | string | `formatRepoTitle(Repo.Name)` (quoted) | Page title — acronym-aware Title Case (e.g. `oscal-sdk` → `OSCAL SDK`) |
| linkTitle | string | Repo.Name (quoted) | Sidebar label — raw repo name for compact display |
| description | string | Repo.Description (quoted) | Page description |
| date | string | Repo.PushedAt | Creation date |
| lastmod | string | Repo.PushedAt | Last modified (deterministic) |
| draft | bool | `false` | Always published |
| toc | bool | `false` | No table of contents (section index only) |
| params.language | string | Repo.Language | Programming language |
| params.stars | int | Repo.StargazersCount | Star count |
| params.repo | string | Repo.HTMLURL | GitHub URL |
| params.source_sha | string | Branch SHA | For branch-level change detection |
| params.readme_sha | string | README blob SHA | For content-level change detection |
| params.seo.title | string | `{name} \| ComplyTime` | SEO title |
| params.seo.description | string | Repo.Description | SEO description |

**Body**: None. The section index is frontmatter-only; README content lives in `overview.md`.

### Overview Page (content/docs/projects/{repo}/overview.md)

A child page containing the README content, rendered as the first navigable item in the sidebar.

Hugo frontmatter schema:

| Field | Type | Source | Description |
|-------|------|--------|-------------|
| title | string | `"Overview"` | Fixed page title |
| description | string | Repo.Description | Page description |
| date | string | Repo.PushedAt | Creation date |
| lastmod | string | Repo.PushedAt | Last modified |
| draft | bool | `false` | Always published |
| toc | bool | `true` | Table of contents enabled |
| weight | int | `1` | First in sort order |
| params.editURL | string | GitHub edit URL for README.md | "Edit this page" link |

**Body**: README content after `stripLeadingH1` → `shiftHeadings` → `titleCaseHeadings` → `stripBadges` → `rewriteRelativeLinks` transforms.

### Doc Page (content/docs/projects/{repo}/{path}.md)

Auto-synced Markdown files from `discovery.scan_paths` directories. Generated by `syncRepoDocPages`.

Hugo frontmatter schema:

| Field | Type | Source | Description |
|-------|------|--------|-------------|
| title | string | Derived from filename | Human-readable title (e.g., "Quick Start") |
| description | string | Repo.Description + title | Combined description |
| date | string | Repo.PushedAt | Creation date |
| lastmod | string | Repo.PushedAt | Last modified |
| draft | bool | `false` | Always published |
| weight | int | `10` | Default sort weight |
| params.editURL | string | GitHub edit URL for source file | "Edit this page" link |

**Body**: File content after `stripLeadingH1` → `shiftHeadings` → `titleCaseHeadings` → `stripBadges` → `rewriteRelativeLinks` transforms, preceded by a provenance comment (`<!-- synced from ... -->`).

**Section indexes**: Intermediate directories under `scan_paths` get auto-generated `_index.md` files with title derived from the directory name.

## Internal State Entities

### syncResult (shared, mutex-protected)

| Field | Type | Description |
|-------|------|-------------|
| synced | int | Repos successfully processed |
| skipped | int | Repos filtered out (by include/exclude or lockfile) |
| warnings | int | Non-fatal issues (missing README, SHA fetch failure) |
| errors | int | Fatal issues (write failures, API errors) |
| added | []string | New repos (not in previous state) |
| updated | []string | Changed repos (SHA differs from previous) |
| removed | []string | Stale repos (in old state, not in new) |
| unchanged | []string | Repos with identical SHA |
| writtenFiles | []string | Manifest of files written during this sync run |

### repoState (change detection per repo)

| Field | Type | Source | Description |
|-------|------|--------|-------------|
| branchSHA | string | `params.source_sha` from existing `_index.md` | Fast pre-filter: skip all fetches if unchanged |
| readmeSHA | string | `params.readme_sha` from existing `_index.md` | Content-level: accurate change reporting when branch moved |

### repoWork (per-repo processing output)

| Field | Type | Description |
|-------|------|-------------|
| repo | Repo | The processed repository |
| sha | string | Branch SHA at time of processing |
| card | ProjectCard | Generated landing page card |
| unchanged | bool | Whether content was unchanged (fast path) |

### ContentLock (committed lockfile for content approval gating)

| Field | Type | Description |
|-------|------|-------------|
| Repos | map[string]string | Repo name → approved branch SHA |

**Source**: `.content-lock.json` at project root. Read by `readLock()`, written by `writeLock()`. When `--lock` is active, `sha(repo)` returns the approved SHA for a given repo (empty string if absent, causing the repo to be skipped).

**Relationships**: One `ContentLock` gates all repos. Each entry maps to a `Repo.Name`. The deploy workflow uses locked SHAs via the `ref` parameter in API calls. The check workflow updates this file and opens a PR.

## Entity Relationships

```
Peribolos (governance registry)
  └─ Repo names (1:N, from peribolos.yaml)
       └─ Repo metadata (1:1, from GitHub API per repo)
       ├─ ProjectCard (1:1, always produced for eligible repos)
       ├─ Section Index (1:0..1, _index.md, skipped when skip_org_sync=true)
       ├─ Overview Page (1:0..1, overview.md, skipped when skip_org_sync=true)
       ├─ Doc Pages (1:0..N, from discovery.scan_paths directories)
       └─ Config Source overlay (0..1, if declared in sync-config.yaml)
            └─ FileSpec (1:N, each produces one output file)

SyncConfig
  ├─ Defaults (1:1)
  ├─ Discovery (1:1, scan_paths, ignore_files, ignore_repos)
  └─ Source (1:N)
       └─ FileSpec (1:N)
            └─ Transform (1:1, optional mutations)
```
