---
title: "Compliance to Policy (C2P)"
description: "A framework bridging the gap between compliance and policy administration."
lead: "Bridge compliance requirements with policy engines using C2P."
date: 2024-01-01T00:00:00+00:00
lastmod: 2024-12-24T00:00:00+00:00
draft: false
images: []
weight: 240
toc: true
---

## Overview

Compliance-to-Policy (C2P) provides a framework to bridge the gap between compliance requirements and policy administration. It enables organizations to translate high-level compliance controls into enforceable policies for various policy engines.

**Repository**: [github.com/complytime/compliance-to-policy-go](https://github.com/complytime/compliance-to-policy-go)

## The Problem

Organizations face challenges connecting:
- **Compliance Requirements** (NIST 800-53, FedRAMP, ISO 27001)
- **Policy Engines** (OPA, Kyverno, Gatekeeper)

C2P solves this by providing a standardized mapping layer.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Compliance-to-Policy (C2P)                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────┐         ┌─────────────────┐                │
│  │ OSCAL Catalog   │         │ Policy Engine   │                │
│  │ (Controls)      │         │ (OPA, Kyverno)  │                │
│  └────────┬────────┘         └────────▲────────┘                │
│           │                           │                         │
│           ▼                           │                         │
│  ┌─────────────────────────────────────────────────┐            │
│  │              C2P Mapping Layer                  │            │
│  │                                                 │            │
│  │  ┌───────────┐   ┌───────────┐   ┌───────────┐ │            │
│  │  │ Control   │──▶│ Mapping   │──▶│ Policy    │ │            │
│  │  │ Parser    │   │ Engine    │   │ Generator │ │            │
│  │  └───────────┘   └───────────┘   └───────────┘ │            │
│  │                                                 │            │
│  └─────────────────────────────────────────────────┘            │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Features

- **OSCAL Integration** - Native support for OSCAL catalogs and profiles
- **Multi-engine support** - Generate policies for OPA, Kyverno, and more
- **Bidirectional mapping** - Convert policies back to compliance status
- **Validation** - Validate mappings and generated policies
- **Extensible** - Plugin architecture for custom policy engines

## Installation

```bash
go install github.com/complytime/compliance-to-policy-go/cmd/c2p@latest
```

## Quick Start

### Define a Mapping

```yaml
# c2p-mapping.yaml
apiVersion: c2p.complytime.dev/v1
kind: ControlMapping
metadata:
  name: nist-to-kyverno
spec:
  catalog: nist-800-53
  policyEngine: kyverno

  mappings:
    - controlId: AC-2
      description: Account Management
      policies:
        - name: require-labels
          template: require-labels
          parameters:
            labels:
              - owner
              - environment

    - controlId: AC-6
      description: Least Privilege
      policies:
        - name: restrict-privileged
          template: disallow-privileged-containers
```

### Generate Policies

```bash
# Generate policies from mapping
c2p generate --mapping c2p-mapping.yaml --output policies/

# Validate the mapping
c2p validate --mapping c2p-mapping.yaml
```

### Check Compliance Status

```bash
# Convert policy results to compliance status
c2p status --policy-results results.json --mapping c2p-mapping.yaml
```

## Supported Policy Engines

| Engine | Status | Description |
|--------|--------|-------------|
| OPA | ✅ Supported | Open Policy Agent |
| Kyverno | ✅ Supported | Kubernetes Native Policy Engine |
| Gatekeeper | ✅ Supported | OPA Gatekeeper |
| Checkov | 🚧 Planned | Infrastructure as Code scanner |

## Plugin Architecture

Create custom plugins for additional policy engines:

```go
package myplugin

import (
    "github.com/complytime/compliance-to-policy-go/pkg/plugin"
)

type MyPlugin struct{}

func (p *MyPlugin) Name() string {
    return "my-policy-engine"
}

func (p *MyPlugin) GeneratePolicy(control Control, mapping Mapping) ([]byte, error) {
    // Generate policy for your engine
    return policy, nil
}

func (p *MyPlugin) ParseResults(results []byte) ([]PolicyResult, error) {
    // Parse results from your engine
    return parsedResults, nil
}

func init() {
    plugin.Register(&MyPlugin{})
}
```

## Integration Examples

### With complyctl

```bash
# Assess using C2P
complyctl assess --c2p-mapping c2p-mapping.yaml --profile nist-800-53
```

### With CI/CD

```yaml
# .github/workflows/compliance.yml
- name: Generate Policies
  run: c2p generate --mapping c2p-mapping.yaml --output policies/

- name: Apply Policies
  run: kubectl apply -f policies/

- name: Check Compliance
  run: |
    c2p status --policy-results <(kyverno test .) \
               --mapping c2p-mapping.yaml \
               --output compliance-status.json
```

## Related Projects

- [compliance-to-policy-plugins](https://github.com/complytime/compliance-to-policy-plugins) - Additional C2P plugins
- [baseline-demo](https://github.com/complytime/baseline-demo) - OSPS Baseline demonstration

## Learn More

- [C2P Go Repository](https://github.com/complytime/compliance-to-policy-go)
- [OSCAL Documentation](/docs/concepts/oscal/)
- [complyctl Documentation](/docs/projects/complyctl/)

