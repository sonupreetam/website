# PR Command for 006-go-sync-tool

## Pre-requisites

1. All commits are pushed to the remote branch
2. Specs and documentation changes are excluded from this PR (separate PR)
3. Branch is `006-go-sync-tool` on fork `sonupreetam/website`

## Commits (in order)

| # | Scope | Message |
|---|-------|---------|
| 1 | Config, path, lock + tests | `feat(sync): add config parsing, path validation, and lock file management` |
| 2 | GitHub API, manifest, discovery + tests | `feat(sync): add GitHub API client, manifest tracking, and content discovery` |
| 3 | Hugo output, transforms, cleanup + layouts | `feat(sync): add Hugo frontmatter generation, content transforms, and cleanup` |
| 4 | Sync orchestration + main refactor | `refactor(sync): extract sync orchestration from monolithic main.go` |
| 5 | CI workflows, sync-config, lockfile | `ci: add Go test workflow, sync-content check, and deploy pipeline integration` |

## Command

````bash
gh pr create \
  --repo complytime/website \
  --head sonupreetam:006-go-sync-tool \
  --base main \
  --title "feat(sync): add Go content sync tool for org-wide documentation" \
  --body "$(cat <<'EOF'
## Summary

Adds a Go CLI tool (`cmd/sync-content/`) that syncs all repos registered in the `complytime` governance registry (`peribolos.yaml`), fetches README content and metadata via the GitHub REST API, applies Markdown transforms, and generates Hugo-compatible project pages and landing page card data. A declarative config overlay (`sync-config.yaml`) provides precision control for repos needing custom documentation layouts.

### Key capabilities

- **Governance-driven repo listing**: repos are sourced from `peribolos.yaml` — new repos appear on the site when added to the governance registry
- **Config-driven precision sync**: `sync-config.yaml` controls per-repo file destinations, frontmatter injection, and transforms (`strip_badges`, `rewrite_links`, `inject_frontmatter`)
- **Content approval gate**: `.content-lock.json` pins each repo to an approved branch SHA; production deploys fetch content at locked SHAs only
- **Two-tier SHA change detection**: branch SHA for fast pre-filtering, README SHA for content-level accuracy
- **Dry-run by default**: `--write` flag required for any disk I/O
- **Concurrent processing**: bounded worker pool (`--workers`) with race-safe implementation
- **Stale content cleanup**: manifest-based orphan tracking removes all generated files when repos are removed
- **CI integration**: `GITHUB_OUTPUT` variables and `GITHUB_STEP_SUMMARY` for GitHub Actions

### CLI interface

| Flag | Default | Description |
|------|---------|-------------|
| `--org` | `complytime` | GitHub organization to scan |
| `--token` | `$GITHUB_TOKEN` | GitHub API token (or set env var) |
| `--config` | (none) | Path to `sync-config.yaml` for config-driven file syncs |
| `--write` | `false` | Required to write files to disk (default: dry-run) |
| `--output` | `.` | Hugo site root directory |
| `--workers` | `5` | Max concurrent repo processing goroutines |
| `--timeout` | `3m` | Overall timeout for all API operations |
| `--include` | (all) | Comma-separated repo allowlist |
| `--exclude` | (see config) | Comma-separated repo names to skip |
| `--repo` | (none) | Sync only this repo (e.g., `complytime/complyctl`) |
| `--summary` | (none) | Write markdown change summary to file |
| `--lock` | (none) | Path to `.content-lock.json` for content approval gating |
| `--update-lock` | `false` | Write current upstream SHAs to lockfile (requires `--lock`) |

### Output structure

```text
content/docs/projects/
├── _index.md                 # Hand-maintained section index (committed)
└── {repo}/                   # Generated per-repo content (gitignored)
    ├── _index.md             # Section index — frontmatter only, no body
    ├── overview.md           # README content as child page (weight: 1)
    └── {doc}.md              # Doc pages from discovery.scan_paths

data/
└── projects.json             # Landing page project cards (gitignored)

.sync-manifest.json           # Written file manifest for orphan cleanup (gitignored)
.content-lock.json            # Approved upstream SHAs per repo (committed)
```

### Files added/changed

- `cmd/sync-content/` — Go CLI tool (~2,100 lines across 10 source files, ~2,300 lines across 10 test files)
- `sync-config.yaml` — declarative sync configuration (added `ignore_repos`)
- `.content-lock.json` — SHA-pinned content approval lockfile (bootstrap)
- `.github/workflows/ci.yml` — PR validation (vet, fmt, test, dry-run, Hugo build)
- `.github/workflows/sync-content-check.yml` — weekly upstream change detection with auto-PR
- `.github/workflows/deploy-gh-pages.yml` — updated deploy pipeline with Go setup + sync step
- `layouts/_partials/main/edit-page.html` — Hugo partial for upstream "Edit on GitHub" links
- `layouts/shortcodes/project-cards.html` — Hugo shortcode for landing page project card grid

## Reviewer guide

**Pre-reading** (recommended before reviewing code):
1. [`specs/006-go-sync-tool/spec.md`](specs/006-go-sync-tool/spec.md) — full scope, user stories, acceptance criteria
2. `sync-config.yaml` — the actual config file (44 lines, in the diff)
3.  Reference document: [sync-content](https://docs.google.com/document/d/1xm4xIJb97CieErHzrgxVTi80R12fFoJcstOiMhPg34o/edit?tab=t.ipr0gpek6bcw)

**Suggested review order** (commits are layered bottom-up by dependency):

| Commit | Scope | What to look for |
|--------|-------|------------------|
| 1 | Config, path, lock + tests | Types, YAML parsing, path traversal guards |
| 2 | GitHub API, manifest, discovery + tests | REST client, httptest stubs, rate limiting |
| 3 | Hugo output, transforms, cleanup + layouts | Frontmatter gen, link rewriting, Hugo templates |
| 4 | Sync orchestration + main refactor | Worker pool, end-to-end flow, main.go slimdown |
| 5 | CI workflows, sync-config, lockfile | GitHub Actions, deploy pipeline integration |

**Workflow runs** (manual dispatch on fork — all green):

| Workflow | Run |
|----------|-----|
| CI | [#23267705888](https://github.com/sonupreetam/website/actions/runs/23267705888) |
| Deploy Hugo to GitHub Pages | [#23267713720](https://github.com/sonupreetam/website/actions/runs/23267713720) |
| Content Sync Check | [#23267718811](https://github.com/sonupreetam/website/actions/runs/23267718811) |

## Test plan

- [ ] `go vet ./...` passes
- [ ] `gofmt -l ./cmd/sync-content/` reports no unformatted files
- [ ] `go test -race ./cmd/sync-content/...` passes (57 test functions, 10 test files)
- [ ] Dry-run produces zero files: `go run ./cmd/sync-content --org complytime --config sync-config.yaml`
- [ ] Write mode generates correct output: `go run ./cmd/sync-content --org complytime --config sync-config.yaml --write`
- [ ] Hugo builds with zero errors after sync: `hugo --minify --gc`
- [ ] CI workflow (`ci.yml`) passes on this PR
- [ ] `--lock` skips repos not in `.content-lock.json`
EOF
)"
````
