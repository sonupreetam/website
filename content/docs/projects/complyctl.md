---
title: "complyctl"
description: "A command-line tool for streamlining end-to-end compliance workflows on local systems."
lead: "Streamline your compliance workflows with the complyctl CLI."
date: 2024-01-01T00:00:00+00:00
lastmod: 2024-12-24T00:00:00+00:00
draft: false
images: []
weight: 210
toc: true
---

## Overview

`complyctl` is a command-line tool designed to streamline end-to-end compliance workflows on local systems. It provides a unified interface for managing compliance assessments, generating reports, and integrating with other ComplyTime tools.

**Repository**: [github.com/complytime/complyctl](https://github.com/complytime/complyctl)

## Features

- **End-to-end workflows** - Manage complete compliance lifecycle from assessment to reporting
- **OSCAL integration** - Native support for OSCAL-based compliance documents
- **Policy validation** - Validate configurations against compliance policies
- **Evidence collection** - Gather and organize compliance evidence
- **Report generation** - Generate compliance reports in multiple formats

## Installation

### Using Go

```bash
go install github.com/complytime/complyctl@latest
```

### From Binary Releases

Download the latest release from [GitHub Releases](https://github.com/complytime/complyctl/releases):

```bash
# Linux (amd64)
curl -LO https://github.com/complytime/complyctl/releases/latest/download/complyctl-linux-amd64
chmod +x complyctl-linux-amd64
sudo mv complyctl-linux-amd64 /usr/local/bin/complyctl

# macOS (arm64)
curl -LO https://github.com/complytime/complyctl/releases/latest/download/complyctl-darwin-arm64
chmod +x complyctl-darwin-arm64
sudo mv complyctl-darwin-arm64 /usr/local/bin/complyctl
```

## Quick Start

### Initialize a Project

```bash
# Create a new compliance project
complyctl init my-compliance-project
cd my-compliance-project
```

### Run an Assessment

```bash
# Run compliance assessment against current configuration
complyctl assess

# Assess against a specific profile
complyctl assess --profile nist-800-53
```

### Generate Reports

```bash
# Generate a compliance report
complyctl report --format html --output compliance-report.html
```

## Commands

| Command | Description |
|---------|-------------|
| `init` | Initialize a new compliance project |
| `assess` | Run compliance assessment |
| `validate` | Validate compliance configurations |
| `report` | Generate compliance reports |
| `evidence` | Manage compliance evidence |
| `version` | Display version information |

## Configuration

`complyctl` uses a configuration file (`.complyctl.yaml`) in your project root:

```yaml
# .complyctl.yaml
version: "1.0"
project:
  name: my-project
  description: My compliance project

profiles:
  - nist-800-53
  - fedramp-moderate

evidence:
  sources:
    - type: file
      path: ./evidence
    - type: collector
      endpoint: http://localhost:8080
```

## Integration with C2P

`complyctl` integrates seamlessly with the Compliance-to-Policy (C2P) framework:

```bash
# Convert OSCAL to policy
complyctl c2p convert --input catalog.json --output policies/

# Validate C2P mappings
complyctl c2p validate --mapping c2p-mapping.yaml
```

## Learn More

- [complyctl GitHub Repository](https://github.com/complytime/complyctl)
- [C2P Framework](/docs/projects/compliance-to-policy/)
- [OSCAL Integration](/docs/concepts/oscal/)

