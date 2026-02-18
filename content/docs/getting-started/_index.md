---
title: "Getting Started"
description: "Get started with ComplyTime in minutes."
lead: "Get up and running with ComplyTime compliance automation tools."
date: 2024-01-01T00:00:00+00:00
lastmod: 2024-12-24T00:00:00+00:00
draft: false
images: []
weight: 100
toc: true
---

## Introduction

ComplyTime is a suite of open source tools designed to automate compliance workflows in cloud native environments. Our engineering-first approach brings compliance into your existing DevSecOps pipeline.

## Prerequisites

Before you begin, ensure you have:

- **Go 1.21+** (for Go-based tools like `complyctl`)
- **Python 3.10+** (for Python-based tools like `complyscribe`)
- **Git** for cloning repositories

## Quick Start with complyctl

The fastest way to get started is with `complyctl`, our command-line tool for compliance workflows.

### Installation

```bash
# Using Go
go install github.com/complytime/complyctl@latest

# Or download from releases
curl -LO https://github.com/complytime/complyctl/releases/latest/download/complyctl-linux-amd64
chmod +x complyctl-linux-amd64
sudo mv complyctl-linux-amd64 /usr/local/bin/complyctl
```

### Verify Installation

```bash
complyctl version
```

### Your First Compliance Check

```bash
# Initialize a new compliance project
complyctl init my-project

# Run compliance assessment
cd my-project
complyctl assess
```

## Next Steps

- Explore [all ComplyTime projects](/docs/projects/)
- Learn about the [core concepts](/docs/concepts/)
- Read about [OSCAL integration](/docs/concepts/oscal/)
- [Contribute](/docs/contributing/) to ComplyTime

