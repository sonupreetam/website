# Sync Tool Architecture

**Source**: `cmd/sync-content/` (~2,100 lines across 10 files in `package main`) | **Language**: Go 1.25 | **Dependency**: `gopkg.in/yaml.v3`

> **Scope**: This document is the *implementation reference* — type system, function map, data flow, and code-level behavior. For design rationale and high-level decisions, see [design-architecture.md](design-architecture.md).

---

## 1. System Overview

The sync tool is a single-binary Go CLI that connects the `complytime` GitHub organization to a Hugo static site. It reads from the GitHub REST API, transforms Markdown content, and writes Hugo-compatible files to the local filesystem.

```
                      ┌─────────────────────┐
                      │    GitHub REST API   │
                      │  api.github.com/v3  │
                      └──────────┬──────────┘
                                 │
                    Authenticated HTTP (Bearer token)
                    Paginated, rate-limit aware
                    Retries with exponential backoff
                                 │
               ┌─────────────────▼─────────────────┐
               │                                     │
               │       apiClient (HTTP layer)        │
               │                                     │
               │  do()         → raw HTTP request    │
               │  getJSON()    → decode + retry      │
               │  fetchPeribolosRepos → governance  │
               │  getRepoMetadata → per-repo meta   │
               │  getREADME   → README + SHA         │
               │  getFileContent → file + SHA        │
               │  getBranchSHA → HEAD commit SHA     │
               │  listDir     → directory entries    │
               │  listDirMD   → recursive .md scan   │
               │                                     │
               └───────┬────────────────┬────────────┘
                       │                │
          ┌────────────▼──┐    ┌────────▼────────────┐
          │               │    │                      │
          │   Org Scan    │    │   Config Overlay     │
          │   Engine      │    │   Engine             │
          │               │    │                      │
          │ processRepo() │    │ syncConfigSource()   │
          │ per repo:     │    │ per source:          │
          │  • SHA check  │    │  • fetch file        │
          │  • README     │    │  • stripBadges       │
          │  • doc pages  │    │  • rewriteLinks      │
          │  • card build │    │  • injectFrontmatter │
          │               │    │  • provenance stamp  │
          └───────┬───────┘    └────────┬─────────────┘
                  │                     │
                  └──────────┬──────────┘
                             │
               ┌─────────────▼─────────────────┐
               │     Content Generation         │
               │                                 │
               │  buildSectionIndex()  → _index │
               │  buildOverviewPage()  → readme │
               │  buildDocPage()       → docs   │
               │  ProjectCard struct   → JSON   │
               │                                 │
               │  writeFileSafe()               │
               │  (byte-level dedup)            │
               └─────────────┬─────────────────┘
                             │
               ┌─────────────▼─────────────────┐
               │     Post-Sync Operations       │
               │                                 │
               │  cleanOrphanedFiles()          │
               │  (stale cleanup via manifest)  │
               │  writeManifest()               │
               │  writeGitHubOutputs()          │
               │  result.toMarkdown()           │
               └────────────────────────────────┘
```

---

## 2. Type System

### 2.1 GitHub API Types (input)

These structs map directly to GitHub REST API JSON responses.

```
Repo                    FileResponse            DirEntry              BranchResponse
├─ Name        string   ├─ Content  string      ├─ Name  string      └─ Commit
├─ FullName    string   ├─ Encoding string      ├─ Path  string          └─ SHA string
├─ Description string   └─ SHA      string      └─ Type  string
├─ Language    string
├─ StargazersCount int
├─ HTMLURL     string
├─ DefaultBranch string
├─ PushedAt    string
└─ Topics      []string
```

### 2.2 Configuration Types (input)

Parsed from `sync-config.yaml` via `gopkg.in/yaml.v3`.

```
SyncConfig
├─ Defaults
│   └─ Branch     string          (fallback branch, default: "main")
├─ Discovery
│   ├─ IgnoreRepos  []string      (repos excluded from discovery reports)
│   ├─ IgnoreFiles  []string      (filenames to skip, e.g. CHANGELOG.md)
│   └─ ScanPaths    []string      (directories to scan, e.g. "docs")
└─ Sources []Source
    ├─ Repo           string      (owner/name, e.g. "complytime/complyctl")
    ├─ Branch         string      (override, inherits Defaults.Branch)
    ├─ SkipOrgSync    bool        (suppress auto-generated pages)
    └─ Files []FileSpec
        ├─ Src        string      (path in repo, e.g. "docs/QUICK_START.md")
        ├─ Dest       string      (path in site, e.g. "content/docs/...")
        └─ Transform
            ├─ InjectFrontmatter  map[string]any
            ├─ RewriteLinks       bool
            └─ StripBadges        bool
```

### 2.3 Output Types

```
ProjectCard                     repoWork (internal)
├─ Name        string           ├─ repo      Repo
├─ Language    string           ├─ sha       string
├─ Type        string           ├─ card      ProjectCard
├─ Description string           └─ unchanged bool
├─ URL         string
├─ Repo        string
└─ Stars       int
```

### 2.4 Internal State Types

```
syncResult (mutex-protected, shared across goroutines)
├─ mu           sync.Mutex
├─ synced       int          (repos successfully processed)
├─ skipped      int          (filtered out by include/exclude)
├─ warnings     int          (non-fatal: missing README, SHA failure)
├─ errors       int          (fatal: write failures, API errors)
├─ added        []string     (new repos since last sync)
├─ updated      []string     (content changed)
├─ removed      []string     (no longer in org)
├─ unchanged    []string     (SHA identical)
└─ writtenFiles []string     (manifest of written paths)

repoState (per-repo change detection, read from existing _index.md)
├─ branchSHA    string       (fast pre-filter: Tier 1)
└─ readmeSHA    string       (content comparison: Tier 2)

ContentLock (committed lockfile for content approval gating)
└─ Repos        map[string]string   (repo name → approved branch SHA)
```

---

## 3. Function Map

### 3.1 Entry Point

| Function | Lines | Responsibility |
|----------|-------|----------------|
| `main()` | ~420 lines | CLI flag parsing, lockfile loading, orchestration, worker pool dispatch, post-sync operations |

### 3.2 API Client Layer

| Function | Responsibility |
|----------|----------------|
| `apiClient.do()` | Raw authenticated HTTP GET with `Accept: application/vnd.github.v3+json` |
| `apiClient.getJSON()` | JSON decode with retry on 403/429; exponential backoff respecting `Retry-After` and `X-RateLimit-Reset` |
| `apiClient.fetchPeribolosRepos()` | Fetch `peribolos.yaml` from `{org}/.github` repo, parse governance registry, return sorted repo names |
| `apiClient.getRepoMetadata()` | Fetch per-repo metadata via `GET /repos/{owner}/{name}` and decode into `Repo` struct |
| `apiClient.getREADME()` | Fetch README content + blob SHA via `/repos/{owner}/{repo}/readme`; accepts `ref` to pin to a specific SHA |
| `apiClient.getFileContent()` | Fetch arbitrary file + SHA via `/repos/{owner}/{repo}/contents/{path}`; accepts `ref` |
| `apiClient.getBranchSHA()` | Fetch HEAD commit SHA via `/repos/{owner}/{repo}/branches/{branch}` |
| `apiClient.listDir()` | List directory entries via Contents API; accepts `ref` |
| `apiClient.listDirMD()` | Recursive `.md` file discovery under a directory; accepts `ref` |
| `isRateLimited()` | Detect 429 or 403-with-zero-remaining |
| `retryWait()` | Compute backoff from `Retry-After`, `X-RateLimit-Reset`, or `2^attempt` seconds |
| `decodeContent()` | Base64 decode file content from API response |

### 3.3 Content Transformation

| Function | Input | Output |
|----------|-------|--------|
| `stripLeadingH1()` | Markdown text | Text with first H1 heading removed (title already in frontmatter) |
| `shiftHeadings()` | Markdown text | Headings shifted down one level (H1→H2, H2→H3, …) |
| `titleCaseHeadings()` | Markdown text | All in-page headings normalised to acronym-aware Title Case |
| `stripBadges()` | Markdown text | Text with badge lines removed |
| `rewriteRelativeLinks()` | Markdown, owner, repo, branch | Relative links → absolute GitHub URLs |
| `injectFrontmatter()` | Content bytes, frontmatter map | Content with YAML frontmatter prepended/replaced |
| `insertAfterFrontmatter()` | Content bytes, insert bytes | Content with bytes inserted after `---` |
| `deriveProjectType()` | Repo struct | Type label: CLI Tool, Automation, Observability, Framework, Library |
| `smartTitle()` | []string words | Acronym-aware Title Case; normalises ALL CAPS; preserves `knownAcronyms` |
| `formatRepoTitle()` | Repo name string | Repo name → display title (e.g. `oscal-sdk` → `OSCAL SDK`) |
| `titleFromFilename()` | Filename string | Human-readable title (e.g., "quick-start" → "Quick Start", "CONTRIBUTING.md" → "Contributing") |

### 3.4 Page Builders

| Function | Generates |
|----------|-----------|
| `buildSectionIndex()` | `_index.md` — frontmatter-only section page with metadata + SHAs |
| `buildOverviewPage()` | `overview.md` — README content as child page (weight: 1) |
| `buildDocPage()` | `{doc}.md` — auto-synced doc page with provenance comment |

### 3.5 Sync Engines

| Function | Responsibility |
|----------|----------------|
| `processRepo()` | Full per-repo pipeline: SHA check → README fetch → page generation → card build |
| `syncConfigSource()` | Per-source config pipeline: file fetch → transforms → write |
| `syncRepoDocPages()` | Auto-sync `.md` files from `discovery.scan_paths` directories |

### 3.6 Change Detection and Cleanup

| Function | Responsibility |
|----------|----------------|
| `readExistingState()` | Scan existing `_index.md` files for `source_sha` and `readme_sha` |
| `readFrontmatterField()` | Extract a single field value from YAML frontmatter |
| `readManifest()` | Load `.sync-manifest.json` from previous run |
| `writeManifest()` | Persist sorted file list for next run's orphan detection |
| `cleanOrphanedFiles()` | Remove files in old manifest but not in current run; prune empty dirs |
| (stale cleanup) | Handled by `cleanOrphanedFiles()` — manifest diff removes orphaned files and prunes empty directories for repos no longer in org |
| `carryForwardManifest()` | Preserve manifest entries for unchanged repos (fast-path) |
| `hasDocPagesInManifest()` | Check if doc pages exist for a repo (first-run detection) |
| `writeFileSafe()` | Write with byte-level dedup — skip if file exists with identical content |

### 3.7 Reporting and CI Integration

| Function | Responsibility |
|----------|----------------|
| `syncResult.printSummary()` | Structured log output of sync stats |
| `syncResult.toMarkdown()` | Markdown-formatted change summary |
| `syncResult.hasChanges()` | Boolean: any adds, updates, or removes? |
| `syncResult.recordFile()` | Thread-safe manifest append |
| `writeGitHubOutputs()` | Write `GITHUB_OUTPUT` vars and `GITHUB_STEP_SUMMARY` |

### 3.8 Content Lockfile (`lock.go`)

| Function | Responsibility |
|----------|----------------|
| `readLock()` | Load `.content-lock.json` from disk; return empty lock if file missing |
| `writeLock()` | Write lockfile with sorted keys and indented JSON for deterministic output |
| `ContentLock.sha()` | Look up approved SHA for a repo name; return empty string if absent |
| `appendRef()` | Conditionally append `?ref=<sha>` to GitHub API URLs (in `github.go`) |

### 3.9 Utilities

| Function | Responsibility |
|----------|----------------|
| `isValidRepoName()` | Reject empty, `.`, `..`, names with `/`, `\`, or `..` |
| `languageOrDefault()` | Return "Unknown" for empty language strings |
| `isAbsoluteURL()` | Check for `http://`, `https://`, `//`, `#`, `mailto:` prefixes |
| `parseNameList()` | Split comma-separated flag value into a `map[string]bool` |

---

## 4. Concurrency Model

```
main()
  │
  ├─ Creates semaphore channel: sem = make(chan struct{}, workers)
  ├─ Creates sync.WaitGroup for goroutine tracking
  ├─ Creates sync.Mutex-protected cards slice
  │
  └─ For each eligible repo:
       │
       ├─ sem <- struct{}{}  (acquire worker slot, blocks if pool full)
       ├─ go func(repo) {
       │     defer wg.Done()
       │     defer func() { <-sem }()  (release slot)
       │     
       │     processRepo()    ← reads/writes via result.mu
       │     syncRepoDocPages()  ← reads/writes via result.mu
       │     syncConfigSource()  ← reads/writes via result.mu
       │     
       │     cardsMu.Lock()   ← append card
       │     cardsMu.Unlock()
       │  }(repo)
       │
  wg.Wait()  ← block until all workers finish
  │
  └─ Sequential post-sync: cleanup, JSON write, summary
```

**Thread-safety guarantees**:
- `syncResult.mu` protects all counter increments and slice appends on the shared result
- `syncResult.recordFile()` uses the same mutex for manifest tracking
- `cardsMu` is a separate mutex for the `cards` slice to avoid holding `result.mu` during card appends
- `processedConfigMu` protects the set tracking which config sources have been processed
- Each goroutine operates on its own `Repo` value (passed by value, not reference)
- The `apiClient` is stateless — `http.Client` is safe for concurrent use

---

## 5. CLI Interface

```
go run ./cmd/sync-content [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--org` | string | `complytime` | GitHub organization to scan |
| `--token` | string | `$GITHUB_TOKEN` | API token for authenticated requests |
| `--output` | string | `.` | Hugo site root directory |
| `--config` | string | — | Path to `sync-config.yaml` |
| `--write` | bool | `false` | Required for disk I/O (default: dry-run) |
| `--workers` | int | `5` | Max concurrent goroutines |
| `--timeout` | duration | `3m` | Overall context timeout |
| `--include` | string | — | Comma-separated repo allowlist |
| `--exclude` | string | (see config `discovery.ignore_repos`) | Comma-separated repo denylist |
| `--repo` | string | — | Single-repo mode (e.g., `complytime/complyctl`); validated against peribolos |
| `--summary` | string | — | Write markdown change summary to file |
| `--lock` | string | — | Path to `.content-lock.json` for content approval gating |
| `--update-lock` | bool | `false` | Write current upstream SHAs to the lockfile (requires `--lock`) |

**Operating modes** (mutually exclusive paths in `main()`):

```
(default)               → peribolos discovery + config overlay → post-sync → exit
--repo owner/name       → validate against peribolos, process single repo
--lock path             → gate repos to approved SHAs in lockfile; skip unapproved
--lock path --update-lock → run normal pipeline, then write upstream SHAs to lockfile
```

---

## 6. Change Detection (Two-Tier)

Change detection is the key optimization that minimizes API calls on repeat runs.

```
Previous _index.md frontmatter:
  params:
    source_sha: "abc123..."     ← Tier 1 (branch HEAD)
    readme_sha: "def456..."     ← Tier 2 (README blob)

Current sync run:
  ┌─────────────────────────────────────────────────────┐
  │ GET /repos/{owner}/{repo}/branches/{branch}         │
  │   → current branch SHA                              │
  └────────────────────────┬────────────────────────────┘
                           │
              ┌────────────▼────────────┐
              │ source_sha == current?  │
              └────────┬────────┬───────┘
                  YES  │        │  NO
                       ▼        ▼
            ┌──────────────┐  ┌──────────────────────────┐
            │ SKIP         │  │ GET /repos/.../readme     │
            │ (unchanged)  │  │   → current readme SHA    │
            │ No API calls │  └────────────┬──────────────┘
            │ Carry forward│               │
            │ manifest     │  ┌────────────▼────────────┐
            └──────────────┘  │ readme_sha == current?  │
                              └────────┬────────┬───────┘
                                  YES  │        │  NO
                                       ▼        ▼
                            ┌──────────────┐  ┌──────────────┐
                            │ Branch moved │  │ Content       │
                            │ but README   │  │ changed       │
                            │ unchanged    │  │ → regenerate  │
                            │ (report as   │  │   all pages   │
                            │  unchanged)  │  └──────────────┘
                            └──────────────┘
```

---

## 7. Output File Structure

For each eligible repo, the tool generates the following under the Hugo site root:

```
{output}/
├── content/docs/projects/
│   └── {repo}/
│       ├── _index.md         ← buildSectionIndex()
│       │   Frontmatter only: title, description, date, lastmod,
│       │   params.language, params.stars, params.repo,
│       │   params.source_sha, params.readme_sha, params.seo
│       │   Body: none
│       │
│       ├── overview.md       ← buildOverviewPage()
│       │   Frontmatter: title="Overview", weight=1, toc=true,
│       │   params.editURL (points to GitHub edit URL)
│       │   Body: README content (after transforms)
│       │
│       └── {subpath}/        ← syncRepoDocPages() (from scan_paths)
│           ├── _index.md     ← auto-generated section index for subdirs
│           └── {doc}.md      ← buildDocPage()
│               Frontmatter: title (from filename), weight=10,
│               params.editURL, provenance comment
│               Body: file content (after transforms)
│
├── data/
│   └── projects.json         ← JSON array of ProjectCard structs
│       Sorted alphabetically by name
│
├── .sync-manifest.json       ← sorted list of all written file paths
│   Used by cleanOrphanedFiles() on next run
│
└── .content-lock.json        ← approved upstream SHAs per repo (committed)
    Written by --update-lock; read by --lock
```

---

## 8. Content Transform Pipeline

Every piece of Markdown content passes through an ordered transform pipeline before being written.

### README content (org scan path):

```
Raw README from GitHub API
    │
    ▼
stripLeadingH1(content)
    Removes the first H1 heading — title is already in frontmatter
    │
    ▼
shiftHeadings(content)
    Bumps every heading down one level (H1→H2, H2→H3, …)
    Hugo's page title is the sole H1
    │
    ▼
titleCaseHeadings(content)
    Applies smartTitle() to all in-page heading text
    Normalises ALL CAPS and preserves known acronyms
    Ensures Hugo's TableOfContents matches rendered headings
    │
    ▼
stripBadges(content)
    Regex: ^\[!\[...\](...)\](...)\s*\n?
    Removes CI badge lines (e.g., [![Build](...)][...]) from the start
    │
    ▼
rewriteRelativeLinks(markdown, owner, repo, branch)
    Regex: (!?\[[^\]]*\])\(([^)]+)\)
    For each relative link:
      Images (![]()) → raw.githubusercontent.com/{owner}/{repo}/{branch}/...
      Links  ([]())  → github.com/{owner}/{repo}/blob/{branch}/...
    Absolute URLs, anchors (#), and mailto: are left unchanged
    │
    ▼
buildOverviewPage(repo, readme)
    Wraps in Hugo frontmatter with title="Overview", weight=1
```

### Config source file (config overlay path):

```
Raw file from GitHub Contents API
    │
    ▼
stripLeadingH1(content)
    │
    ▼
shiftHeadings(content)
    │
    ▼
titleCaseHeadings(content)
    │
    ▼ (if transform.strip_badges: true)
stripBadges(content)
    │
    ▼ (if transform.rewrite_links: true)
rewriteRelativeLinks(content, owner, repo, branch)
    │
    ▼ (if transform.inject_frontmatter has entries)
injectFrontmatter(content, frontmatterMap)
    If content starts with "---", existing frontmatter is replaced
    Otherwise, new YAML frontmatter block is prepended
    │
    ▼
insertAfterFrontmatter(content, provenanceComment)
    Inserts "<!-- synced from {repo}/{src}@{branch} ({sha}) -->"
    after the closing "---" delimiter
```

### Doc page (discovery scan path):

```
Raw file from GitHub Contents API
    │
    ▼
stripLeadingH1(content)
    Removes the first H1 heading — title already in frontmatter
    │
    ▼
shiftHeadings(content)
    Bumps every heading down one level (H1→H2, H2→H3, …)
    │
    ▼
titleCaseHeadings(content)
    Applies smartTitle() — normalises ALL CAPS, preserves acronyms
    │
    ▼
stripBadges(content)
    │
    ▼
rewriteRelativeLinks(content, owner, repo, branch, fileDir)
    basePath = directory of the source file
    Relative links resolve from the file's location, not repo root
    │
    ▼
buildDocPage(filePath, repoFullName, description, pushedAt, branch, sha, content)
    Title derived from filename: "quick-start.md" → "Quick Start"
    (CONTRIBUTING.md → "Contributing" via smartTitle ALL CAPS normalisation)
    Provenance comment embedded
    editURL points to GitHub edit page for the source file
```

---

## 9. Rate Limiting and Retry Strategy

```
apiClient.getJSON(ctx, url, &dst)
    │
    ├─ HTTP 200 → decode JSON → return
    │
    ├─ HTTP 429 (Too Many Requests) → rate limited
    │
    ├─ HTTP 403 + X-RateLimit-Remaining: 0 → rate limited
    │
    └─ Any other error → return immediately (no retry)

When rate limited (up to 3 retries):
    │
    ├─ Check Retry-After header → sleep that many seconds
    │
    ├─ Check X-RateLimit-Reset header → sleep until reset timestamp + 1s
    │   (capped at 5 minutes; falls back to exponential if longer)
    │
    └─ Fallback → exponential backoff: 2^attempt seconds (1s, 2s, 4s)
```

---

## 10. Manifest-Based Orphan Cleanup

The tool uses a file manifest (`.sync-manifest.json`) for precise cleanup across runs.

```
Run N writes files:
  content/docs/projects/complyctl/_index.md
  content/docs/projects/complyctl/overview.md
  content/docs/projects/complyscribe/_index.md
  content/docs/projects/complyscribe/overview.md
  → saved to .sync-manifest.json

Run N+1 (complyscribe removed from org):
  content/docs/projects/complyctl/_index.md       ← in current run
  content/docs/projects/complyctl/overview.md      ← in current run

  Diff: old manifest − current manifest =
    content/docs/projects/complyscribe/_index.md   ← orphan, deleted
    content/docs/projects/complyscribe/overview.md ← orphan, deleted
    (empty dir complyscribe/ pruned upward)
```

**First-run fallback**: When no manifest exists (first sync), the tool writes a fresh manifest. On subsequent runs, `cleanOrphanedFiles()` uses the manifest diff to remove files for repos no longer in the active set and prunes empty parent directories.

---

## 11. Execution Flow (main)

```
1. Parse CLI flags
2. Resolve GitHub token (flag → env var → warn)
3. Build include/exclude sets from flags
4. Create context with timeout
5. Initialize apiClient with token and 30s HTTP timeout
6. Load sync-config.yaml (if --config provided)
6a. Load .content-lock.json (if --lock provided)
6b. Determine lock-gate mode: active when --lock without --update-lock

7. Read existing state (source_sha, readme_sha from _index.md files)
8. Read old manifest (.sync-manifest.json)
9. Fetch repo names from peribolos.yaml (governance registry)
10. Fetch per-repo metadata from GitHub API
11. Sort repos alphabetically
12. Filter: valid name, passes include/exclude

13. Worker pool dispatch:
    For each eligible repo (bounded by --workers semaphore):
      a. If lock-gate active and repo not in lockfile → skip (unapproved)
      b. Derive lockedSHA from lockfile (if --lock)
      c. processRepo(lockedSHA)  → SHA check, README at ref, pages, card
      d. syncRepoDocPages(ref)   → auto-sync from scan_paths at ref
      e. syncConfigSource(ref)   → config-declared file syncs at ref
      f. Store upstream SHA in sync.Map (for --update-lock)

14. wg.Wait() (block until all workers complete)
14a. If --update-lock: write collected upstream SHAs to lockfile

15. Process config-only sources (repos in config but not in org)
16. Sort cards alphabetically
17. Detect removed repos (in old state but not in current)

18. If --write:
    a. Clean orphaned files (manifest diff) or stale content (first run)
    b. Write .sync-manifest.json
    c. Write data/projects.json

19. Print structured summary
20. Write GITHUB_OUTPUT and GITHUB_STEP_SUMMARY (if in CI)
21. Write --summary file (if specified)
22. Exit with code 1 if any errors occurred
```

---

## 12. Project Type Derivation

The `deriveProjectType()` function classifies repos for landing page cards using a priority chain:

```
Topics and description keywords → Type label

1. Topic "cli" OR description contains "command-line" or " cli"
   → "CLI Tool"

2. Topic "automation" OR description contains "automation" or "automat"
   → "Automation"

3. Topic "observability" OR description contains "observability" or "collector"
   → "Observability"

4. Topic "framework" OR description contains "framework" or "bridging"
   → "Framework"

5. Default (no match)
   → "Library"
```

Priority is first-match — a repo with both `cli` and `automation` topics is classified as "CLI Tool".

---

## 13. Error Handling Strategy

| Severity | Behavior | Exit Code |
|----------|----------|-----------|
| Fatal config error | `slog.Error` + `os.Exit(1)` | 1 |
| API error (per repo) | Log warning, increment `result.warnings`, continue | 0 |
| Write error (per file) | Log error, increment `result.errors`, continue | 1 |
| Missing README | Log warning, use placeholder text, continue | 0 |
| Rate limit | Retry up to 3 times with backoff, then fail | depends |
| Invalid repo name | Skip repo, increment `result.skipped` | 0 |

The tool processes all repos even when individual ones fail. The exit code is 1 only if `result.errors > 0`, ensuring CI pipelines detect partial failures.

---

## 14. Logging

All logging uses `log/slog` with structured fields. No `fmt.Println` or `log.Printf`.

| Level | Usage |
|-------|-------|
| `slog.Info` | Progress: "processing repo", "wrote section index", "sync complete" |
| `slog.Warn` | Non-fatal: "no README found", "rate limited, retrying", "no GitHub token" |
| `slog.Error` | Fatal or per-file: "error writing section index", "error loading config" |
| `slog.Debug` | Verbose: "scan path not found" (only in doc page scanning) |

Structured fields used consistently: `repo`, `path`, `sha`, `error`, `src`, `dest`, `attempt`, `wait`, `count`.
