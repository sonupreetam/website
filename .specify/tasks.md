# Tasks

Root-level task backlog for the complytime.dev website.

## Current Status

Feature 006 (Go sync tool port) is the active branch (`006-go-sync-tool`). All 12 phases (T001–T054) are implementation-complete. Two tasks (T008, T009) are deferred pending `sync-config.yaml` source declarations. T026b (sidebar visual verification) pending. The branch is pending merge to `main`.

## Pending Work

### Feature 006: Merge to Main

- [ ] Merge `006-go-sync-tool` branch to `main`
- [ ] Run `sync-content-check.yml` via `workflow_dispatch` to bootstrap `.content-lock.json` with real SHAs (SC-016)
- [ ] Verify production deploy at `complytime.dev` renders dynamic project cards and docs pages

### Post-Merge

- [ ] T008 / T009 (deferred): Declare `sources` entries in `sync-config.yaml` for repos needing custom documentation layouts (e.g., `complyctl` with `skip_org_sync: true`). Verify `skip_org_sync` behaviour and config file transforms end-to-end.
- [ ] Update `.specify/memory/constitution.md` transitional provisions if needed post-merge (T027 done — v1.5.0 synced)

## Deferred Features

| Feature | Description | Blocked On |
|---------|-------------|------------|
| Specs browser (`/specs/{repo}/`) | Publish `.specify/` artifacts as browsable pages at `/specs/{repo}/` | Future feature decision |
| Config-driven source declarations | Declare `sources` in `sync-config.yaml` for per-repo customisation | Repos needing custom layouts |
| Log level control | `--verbose` / `--quiet` flags for sync tool | Future need |
| Config schema versioning | Version the `sync-config.yaml` format | Future need |

## Completed Features

| Feature | Branch | Tasks | Status |
|---------|--------|-------|--------|
| 006: Go sync tool port | `006-go-sync-tool` | T001–T054 (52 done, 2 deferred, 1 pending verification) | Pending merge |
