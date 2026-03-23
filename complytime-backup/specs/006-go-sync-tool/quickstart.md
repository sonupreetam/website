# Quickstart: Go Content Sync Tool

## Prerequisites

- Go 1.25+ installed
- Node.js 22+ installed (for Hugo/Doks theme build)
- Hugo extended installed
- (Optional) `GITHUB_TOKEN` env var for higher API rate limits

## Local Development

### 1. Clone and install dependencies

```bash
git clone https://github.com/complytime/website.git
cd website
npm ci
```

### 2. Run the sync tool (dry-run)

Preview what the sync tool would generate without writing anything:

```bash
go run ./cmd/sync-content --org complytime --config sync-config.yaml
```

### 3. Run the sync tool (write)

Generate all content from the GitHub org:

```bash
go run ./cmd/sync-content --org complytime --config sync-config.yaml --write
```

This produces:
- `content/docs/projects/{repo}/_index.md` — section index (frontmatter only) per org repo
- `content/docs/projects/{repo}/overview.md` — README content as a child page
- `content/docs/projects/{repo}/{doc}.md` — doc pages from `discovery.scan_paths` directories
- `data/projects.json` — landing page project cards
- `.sync-manifest.json` — manifest for orphan cleanup on subsequent runs
- `.content-lock.json` — approved upstream SHAs per repo (only updated with `--update-lock`)

### 4. Start Hugo dev server

```bash
npm run dev
```

Navigate to `http://localhost:1313/`. Project pages appear at `/docs/projects/`.

## CLI Flags

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
| `--exclude` | (see config) | Comma-separated repo names to skip; merged with `discovery.ignore_repos` in `sync-config.yaml` |
| `--repo` | (none) | Sync only this repo (e.g., `complytime/complyctl`); validated against peribolos. Overrides `--include`. |
| `--summary` | (none) | Write markdown change summary to this file |
| `--lock` | (none) | Path to `.content-lock.json` for content approval gating |
| `--update-lock` | `false` | Write current upstream SHAs to the lockfile (requires `--lock`) |

## Common Tasks

### Sync a single repo

```bash
go run ./cmd/sync-content --repo complytime/complyctl --config sync-config.yaml --write
```

### Run with increased concurrency

```bash
go run ./cmd/sync-content --org complytime --workers 10 --write
```

### Generate a change summary for PR review

```bash
go run ./cmd/sync-content --org complytime --config sync-config.yaml --write --summary sync-report.md
```

### Build the production site

```bash
go run ./cmd/sync-content --org complytime --config sync-config.yaml --lock .content-lock.json --write
hugo --minify --gc
```

Output is in `public/`. The `--lock` flag ensures content is fetched at the approved SHAs in `.content-lock.json` (see US7 in spec.md). Omit `--lock` for local development to fetch latest HEAD content.

## Adding a New Config Source

Edit `sync-config.yaml`:

```yaml
sources:
  - repo: complytime/new-project
    skip_org_sync: true   # suppress auto-generated section index and overview page
    files:
      - src: README.md
        dest: content/docs/projects/new-project/_index.md
        transform:
          inject_frontmatter:
            title: "New Project"
            description: "Description here."
            weight: 10
          rewrite_links: true
          strip_badges: true
```

If `skip_org_sync` is omitted or `false`, the org scan generates section index + overview page AND the config files are synced as additional content.

## Verification

```bash
# Build compiles
go build ./cmd/sync-content/

# Code quality
go vet ./...

# Run tests
go test -race ./cmd/sync-content/...

# Full site build (local dev — no --lock, fetches HEAD)
go run ./cmd/sync-content --org complytime --config sync-config.yaml --write
hugo --minify --gc

# Full site build (production — locked SHAs)
go run ./cmd/sync-content --org complytime --config sync-config.yaml --lock .content-lock.json --write
hugo --minify --gc
```
