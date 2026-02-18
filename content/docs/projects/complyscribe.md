---
title: "complyscribe"
description: "A CLI tool for leveraging Compliance-Trestle in CI/CD workflows for OSCAL formatted compliance content management."
lead: "Automate OSCAL compliance content management in CI/CD workflows with complyscribe."
date: 2024-01-01T00:00:00+00:00
lastmod: 2024-12-24T00:00:00+00:00
draft: false
images: []
weight: 220
toc: true
---

## Overview

`complyscribe` is a CLI tool that assists users in leveraging [Compliance-Trestle](https://ibm.github.io/compliance-trestle/) in CI/CD workflows for OSCAL formatted compliance content management.

**Repository**: [github.com/complytime/complyscribe](https://github.com/complytime/complyscribe)

> **WARNING**: This project is currently under initial development. APIs may be changed incompatibly from one commit to another.

## Available Commands

### autosync

The `autosync` command syncs trestle-generated Markdown files to OSCAL JSON files in a trestle workspace. All content under the provided markdown directory will be transformed when the action is run. This action supports all top-level models supported by compliance-trestle for authoring.

### rules-transform

The `rules-transform` command can be used when managing OSCAL Component Definitions in a trestle workspace. The action transforms rules defined in the rules YAML view to an OSCAL Component Definition JSON file.

### create compdef

The `create compdef` command creates a new OSCAL Component Definition in a trestle workspace. It generates a new Component Definition JSON file and corresponding directories that contain rules YAML files and trestle-generated Markdown files. This prepares the workspace for use with the `rules-transform` and `autosync` actions.

### sync-upstreams

The `sync-upstreams` command syncs and validates upstream OSCAL content stored in a git repository to a local trestle workspace. The inputs `include_models` and `exclude_models` determine which content is synced.

### create ssp

The `create ssp` command creates a new OSCAL System Security Plan (SSP) in a trestle workspace. It generates a new SSP JSON file and corresponding directories that contain trestle-generated Markdown files. This prepares the workspace for use with the `autosync` action by creating or updating the `ssp-index.json` file, which tracks the relationships between the SSP and other OSCAL content in the workspace.

### sync-cac-content

The `sync-cac-content` command supports transforming CaC (Compliance as Code) content to OSCAL models in a trestle workspace.

### sync-oscal-content

The `sync-oscal-content` command supports syncing OSCAL models to CaC content in a trestle workspace.

## Command Availability

| Command                                 | Available as a GitHub Action |
| --------------------------------------- | :--------------------------: |
| autosync                                | ✓                            |
| rules-transform                         | ✓                            |
| create compdef                          | ✓                            |
| sync-upstreams                          | ✓                            |
| create ssp                              |                              |
| sync-cac-content component-definition   |                              |
| sync-cac-content profile                |                              |
| sync-cac-content catalog                |                              |
| sync-oscal-content component-definition |                              |
| sync-oscal-content profile              |                              |
| sync-oscal-content catalog              |                              |

## Supported Git Providers

> **Note**: Only applicable if using `complyscribe` to create pull requests. Automatically detecting the git provider information is supported for GitHub Actions (GitHub) and GitLab CI (GitLab).

- GitHub
- GitLab

## Run as a Container

> **Note**: When running the commands in a container, all are prefixed with `complyscribe` (e.g. `complyscribe autosync`). The default entrypoint for the container is the autosync command.

### Build and Run Locally

```bash
podman build -f Dockerfile -t complyscribe .
podman run -v $(pwd):/data -w /data complyscribe
```

### Using Container Images from Quay.io

```bash
podman run -v $(pwd):/data -w /data quay.io/continuouscompliance/complyscribe:<tag>
```

## Learn More

- [complyscribe GitHub Repository](https://github.com/complytime/complyscribe)
- [complyscribe Documentation](https://complytime.github.io/complyscribe/)
- [complyctl Integration](/docs/projects/complyctl/)
- [OSCAL Overview](/docs/concepts/oscal/)
