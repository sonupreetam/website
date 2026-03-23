---
title: "Style Guide"
description: "A command-line tool for streamlining end-to-end compliance workflows on local systems. — Style Guide"
date: 2026-03-23T09:20:36Z
lastmod: 2026-03-23T09:20:36Z
draft: false
weight: 10
params:
  editURL: "https://github.com/complytime/complyctl/edit/main/docs/STYLE_GUIDE.md"
---
<!-- synced from complytime/complyctl/docs/STYLE_GUIDE.md@main (76d51bac5914) -->

This style guide outlines the best practices to ensure consistency and readability across the codebase.

### Complytime Organization Style Guide

Refer to [Style Guide](https://github.com/complytime/community/blob/main/STYLE_GUIDE.md), this is the universal style guide that all projects under the ComplyTime Organization should follow.

### Project Style Guide

#### Code Formatting

- **Braces**: Place opening braces on the same line as the statement (e.g., `if`, `for`, `func`).

#### Additional Guidelines

- Other [Go checks](https://github.com/complytime/complyctl/blob/main/.golangci.yml) are present in CI/CD and therefore it may be useful to also run them locally before submitting a PR.
- The pre-commit and pre-push hooks can be configured by installing [pre-commit](https://pre-commit.com/) and running `make dev-setup`
- Complyctl leverages the [charmbracelet/log](https://github.com/charmbracelet/log) library for logging all command and plugin activity. By default, this output is printed to stdout.
