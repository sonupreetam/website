# Feature Specification: CI/CD Pipeline

**Feature Branch**: `012-cicd-pipeline`
**Created**: 2026-03-06
**Status**: Draft
**Phase**: 5 (Automation)

## Comparative Analysis

### Current State (`complytime/website`)

- `.github/workflows/deploy-gh-pages.yml` — Simple pipeline:
  - Triggers: `push` to `main`, `workflow_dispatch`
  - Build job: checkout, Node 22, Hugo 0.155.1 extended, `npm ci`, `hugo --minify --gc`, upload artifact
  - Deploy job: deploy to GitHub Pages
  - No Go tooling step
  - No content sync step
  - No `repository_dispatch` trigger
  - No cron schedule

### Target State (`test-website`)

- `.github/workflows/sync-and-deploy.yml` — Enhanced pipeline:
  - Triggers: `repository_dispatch` (docs-update), `schedule` (cron `0 6 * * *` daily), `workflow_dispatch`
  - Build job: checkout, Go setup, Node setup, Hugo setup, `npm ci`, `hugo mod get -u`, `go run ./cmd/sync-content --org complytime`, `hugo --minify`, upload artifact
  - Deploy job: deploy to GitHub Pages
- `.github/workflows/notify-website.yml` — Reusable workflow:
  - For upstream repos to trigger website rebuilds
  - Fires `repository_dispatch` event when `docs/**` or `README.md` changes on push to default branch

### Delta

| Item | Action | Details |
|------|--------|---------|
| `.github/workflows/deploy-gh-pages.yml` | Replace or rename | Current simple pipeline replaced by enhanced version |
| `.github/workflows/sync-and-deploy.yml` | Add | Enhanced pipeline with Go sync, Hugo Module updates, multi-trigger |
| `.github/workflows/notify-website.yml` | Add | Reusable workflow for upstream repos to trigger rebuilds |

### Conflicts

- **Critical**: The existing `deploy-gh-pages.yml` is the active deployment pipeline. Replacing it requires careful coordination to avoid breaking the live site.
- The enhanced pipeline requires Go to be installed in CI, which increases build time.
- The `repository_dispatch` trigger requires a GitHub token with `repo` scope to be configured as a secret in upstream repos.
- The cron schedule (`0 6 * * *`) will run daily even if no changes occurred, consuming Actions minutes.
- The `notify-website.yml` is designed for upstream repos, not this repo — it needs to be documented for adoption by other complytime repos.

## Acceptance Criteria

1. `.github/workflows/sync-and-deploy.yml` exists with all 3 triggers (repository_dispatch, schedule, workflow_dispatch)
2. Build job installs Go, Node.js, and Hugo
3. Build job runs `hugo mod get -u` to update module imports
4. Build job runs the Go sync tool (`go run ./cmd/sync-content --org complytime`)
5. Build job runs `hugo --minify` to build the site
6. Deploy job publishes to GitHub Pages
7. `workflow_dispatch` trigger can be run manually from the Actions tab
8. `.github/workflows/notify-website.yml` is a valid reusable workflow
9. Existing deployment to complytime.dev continues to work after the transition
10. Old `deploy-gh-pages.yml` is removed or disabled to avoid duplicate deployments

## Migration Steps

1. Add `.github/workflows/sync-and-deploy.yml`
2. Add `.github/workflows/notify-website.yml`
3. Test `sync-and-deploy.yml` via `workflow_dispatch` (manual trigger)
4. Verify the site builds and deploys correctly
5. Remove or rename `.github/workflows/deploy-gh-pages.yml`
6. Configure `GITHUB_TOKEN` or PAT for `repository_dispatch` in upstream repos
7. Document the `notify-website.yml` usage for upstream repo maintainers

## Rollback Plan

1. Restore `.github/workflows/deploy-gh-pages.yml` from `main` branch
2. Delete `.github/workflows/sync-and-deploy.yml` and `notify-website.yml`

## Dependencies

- Feature 6 (Go Sync Tool) — the pipeline runs the sync tool
- Feature 7 (Hugo Module Mounts) — the pipeline runs `hugo mod get -u`
