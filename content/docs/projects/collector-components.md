---
title: "Collector Components"
description: "A policy-driven observability toolkit for compliance evidence collection."
lead: "Automated compliance evidence collection with policy-driven observability."
date: 2024-01-01T00:00:00+00:00
lastmod: 2024-12-24T00:00:00+00:00
draft: false
images: []
weight: 230
toc: true
---

## Overview

`complytime-collector-components` is a policy-driven observability toolkit designed for automated compliance evidence collection. It integrates with OpenTelemetry to gather, process, and export compliance-relevant data from your infrastructure.

**Repository**: [github.com/complytime/complytime-collector-components](https://github.com/complytime/complytime-collector-components)

## Features

- **Policy-driven collection** - Define what evidence to collect based on compliance policies
- **OpenTelemetry integration** - Built on the OpenTelemetry Collector framework
- **Multi-source support** - Collect from Kubernetes, cloud APIs, logs, and more
- **Real-time processing** - Process and enrich evidence as it's collected
- **Flexible export** - Export to various destinations for storage and analysis

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                  Collector Components                    │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐ │
│  │  Receivers   │──▶│  Processors  │──▶│  Exporters   │ │
│  └──────────────┘   └──────────────┘   └──────────────┘ │
│        ▲                   ▲                   │        │
│        │                   │                   ▼        │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐ │
│  │ Policy       │   │ Evidence     │   │ Storage/     │ │
│  │ Definitions  │   │ Enrichment   │   │ Analysis     │ │
│  └──────────────┘   └──────────────┘   └──────────────┘ │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

## Components

### Receivers

| Component | Description |
|-----------|-------------|
| `kubernetesreceiver` | Collect compliance data from Kubernetes clusters |
| `cloudapireceiver` | Collect data from cloud provider APIs |
| `filereceiver` | Monitor files for compliance evidence |
| `httpreceiver` | Receive evidence via HTTP endpoints |

### Processors

| Component | Description |
|-----------|-------------|
| `policyprocessor` | Apply policy rules to filter/transform evidence |
| `enrichmentprocessor` | Enrich evidence with additional context |
| `aggregationprocessor` | Aggregate related evidence items |

### Exporters

| Component | Description |
|-----------|-------------|
| `oscalexporter` | Export evidence in OSCAL format |
| `fileexporter` | Export to local files |
| `s3exporter` | Export to S3-compatible storage |

## Installation

### Using Docker

```bash
docker pull ghcr.io/complytime/collector:latest
docker run -v $(pwd)/config.yaml:/etc/collector/config.yaml \
  ghcr.io/complytime/collector:latest
```

### From Source

```bash
git clone https://github.com/complytime/complytime-collector-components.git
cd complytime-collector-components
make build
./bin/collector --config config.yaml
```

## Configuration

```yaml
# config.yaml
receivers:
  kubernetes:
    auth_type: serviceAccount
    collection_interval: 60s

  cloudapi:
    provider: aws
    services:
      - s3
      - iam
      - ec2

processors:
  policy:
    rules_file: ./policies/collection-rules.yaml

  enrichment:
    add_labels:
      environment: production
      compliance_framework: nist-800-53

exporters:
  oscal:
    format: json
    output_dir: ./evidence

  s3:
    bucket: compliance-evidence
    region: us-east-1

service:
  pipelines:
    evidence:
      receivers: [kubernetes, cloudapi]
      processors: [policy, enrichment]
      exporters: [oscal, s3]
```

## Policy Rules

Define collection policies to filter and prioritize evidence:

```yaml
# policies/collection-rules.yaml
rules:
  - name: collect-iam-policies
    description: Collect IAM policy configurations
    source: cloudapi
    conditions:
      - field: service
        operator: equals
        value: iam
    priority: high

  - name: collect-pod-security
    description: Collect pod security configurations
    source: kubernetes
    conditions:
      - field: kind
        operator: in
        values: [Pod, Deployment, StatefulSet]
    enrich:
      - type: security_context
```

## Integration with ComplyTime

The collector integrates with other ComplyTime tools:

```bash
# Use collected evidence with complyctl
complyctl assess --evidence ./evidence/

# Generate reports from collected data
complyctl report --evidence-source collector:8080
```

## Learn More

- [Collector Components Repository](https://github.com/complytime/complytime-collector-components)
- [Collector Distro](/docs/projects/collector-distro/)
- [complyctl Integration](/docs/projects/complyctl/)

