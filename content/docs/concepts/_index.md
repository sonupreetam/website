---
title: "Concepts"
description: "Understand the core concepts, architecture, and standards behind ComplyTime."
lead: "Learn the foundational concepts that power ComplyTime's compliance automation."
date: 2024-01-01T00:00:00+00:00
lastmod: 2024-12-24T00:00:00+00:00
draft: false
images: []
weight: 300
toc: true
---

## Overview

ComplyTime is designed as a modular, composable system where each component can be used independently or together to create comprehensive compliance automation workflows.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           ComplyTime Architecture                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                         User Interface Layer                            ││
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    ││
│  │  │  complyctl  │  │ complyscribe│  │   Web UI    │  │    APIs     │    ││
│  │  │    (CLI)    │  │  (Authoring)│  │  (Future)   │  │   (REST)    │    ││
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘    ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                      │                                       │
│                                      ▼                                       │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                         Core Framework Layer                            ││
│  │  ┌─────────────────────────────┐  ┌─────────────────────────────────┐  ││
│  │  │  Compliance-to-Policy (C2P) │  │        OSCAL SDK (Go)           │  ││
│  │  │  - Control Mapping          │  │  - Catalog/Profile Parsing      │  ││
│  │  │  - Policy Generation        │  │  - SSP/SAR Generation           │  ││
│  │  │  - Result Aggregation       │  │  - Validation                   │  ││
│  │  └─────────────────────────────┘  └─────────────────────────────────┘  ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                      │                                       │
│                                      ▼                                       │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                      Data Collection Layer                              ││
│  │  ┌─────────────────────────────┐  ┌─────────────────────────────────┐  ││
│  │  │   Collector Components      │  │      Policy Engines             │  ││
│  │  │  - Kubernetes Receiver      │  │  - OPA/Rego                     │  ││
│  │  │  - Cloud API Receiver       │  │  - Kyverno                      │  ││
│  │  │  - Log Receivers            │  │  - Gatekeeper                   │  ││
│  │  └─────────────────────────────┘  └─────────────────────────────────┘  ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                      │                                       │
│                                      ▼                                       │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                        Infrastructure Layer                             ││
│  │  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────────────┐    ││
│  │  │Kubernetes │  │  Cloud    │  │  On-Prem  │  │  Container        │    ││
│  │  │  Clusters │  │ Providers │  │  Systems  │  │  Registries       │    ││
│  │  └───────────┘  └───────────┘  └───────────┘  └───────────────────┘    ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Core Concepts

### OSCAL Foundation

ComplyTime is built on [OSCAL (Open Security Controls Assessment Language)](https://pages.nist.gov/OSCAL/), a NIST standard for expressing security controls and assessment results in machine-readable formats.

Key OSCAL models used:

| Model | Purpose |
|-------|---------|
| **Catalog** | Defines security controls (e.g., NIST 800-53) |
| **Profile** | Selects and customizes controls for specific use |
| **Component Definition** | Describes how components implement controls |
| **System Security Plan** | Documents system security implementation |
| **Assessment Results** | Records assessment findings |

### Compliance-to-Policy (C2P)

C2P bridges high-level compliance requirements with enforceable policies:

1. **Parse** - Read OSCAL catalogs and profiles
2. **Map** - Connect controls to policy templates
3. **Generate** - Create policies for target engines
4. **Assess** - Run policies against infrastructure
5. **Aggregate** - Collect results back to OSCAL format

### Evidence Collection

The collector components gather compliance evidence from:

- **Kubernetes** - Cluster configurations, workload specs
- **Cloud APIs** - IAM policies, network configs, storage settings
- **Logs** - Audit logs, access logs
- **Files** - Configuration files, certificates

## Data Flow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Catalog    │────▶│   Profile    │────▶│  C2P Mapping │
│  (Controls)  │     │  (Selection) │     │  (Policies)  │
└──────────────┘     └──────────────┘     └──────┬───────┘
                                                  │
                                                  ▼
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Assessment  │◀────│   Policy     │◀────│   Policy     │
│   Results    │     │   Results    │     │   Engine     │
└──────┬───────┘     └──────────────┘     └──────────────┘
       │
       ▼
┌──────────────┐     ┌──────────────┐
│     SSP      │────▶│   Reports    │
│   (Plan)     │     │   (Output)   │
└──────────────┘     └──────────────┘
```

## Integration Patterns

### GitOps Workflow

```yaml
# Example: ArgoCD + ComplyTime
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: compliance-policies
spec:
  source:
    repoURL: https://github.com/org/compliance-policies
    path: generated-policies/
  destination:
    server: https://kubernetes.default.svc
```

### CI/CD Integration

```yaml
# GitHub Actions Example
name: Compliance Check
on: [pull_request]
jobs:
  assess:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run Compliance Assessment
        run: |
          complyctl assess --profile nist-800-53
          complyctl report --format sarif > results.sarif
      - name: Upload Results
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: results.sarif
```

## Extensibility

ComplyTime is designed to be extended:

- **Custom Receivers** - Add new data sources
- **Custom Processors** - Transform evidence
- **Custom Exporters** - Output to new destinations
- **C2P Plugins** - Support new policy engines

## Learn More

- [OSCAL Deep Dive](/docs/concepts/oscal/)
- [C2P Framework](/docs/projects/compliance-to-policy/)
- [Collector Architecture](/docs/projects/collector-components/)

