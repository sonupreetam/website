---
title: "OSCAL Integration"
description: "How ComplyTime uses OSCAL for standardized compliance data."
lead: "Understanding OSCAL and its role in ComplyTime."
date: 2024-01-01T00:00:00+00:00
lastmod: 2024-12-24T00:00:00+00:00
draft: false
images: []
weight: 310
toc: true
---

## What is OSCAL?

**OSCAL (Open Security Controls Assessment Language)** is a set of formats developed by NIST for expressing security controls, control implementations, and assessment results in machine-readable formats (JSON, XML, YAML).

## Why OSCAL?

| Benefit | Description |
|---------|-------------|
| **Standardization** | Common format across tools and organizations |
| **Automation** | Machine-readable enables automation |
| **Interoperability** | Exchange data between compliance tools |
| **Traceability** | Clear links from requirements to implementation |

## OSCAL Models in ComplyTime

### Catalog

A collection of security controls (e.g., NIST 800-53).

```json
{
  "catalog": {
    "uuid": "...",
    "metadata": {
      "title": "NIST SP 800-53 Rev 5"
    },
    "groups": [
      {
        "id": "ac",
        "title": "Access Control",
        "controls": [
          {
            "id": "ac-1",
            "title": "Policy and Procedures",
            "parts": [...]
          }
        ]
      }
    ]
  }
}
```

### Profile

A selection of controls from one or more catalogs.

```json
{
  "profile": {
    "uuid": "...",
    "metadata": {
      "title": "FedRAMP Moderate Baseline"
    },
    "imports": [
      {
        "href": "nist-800-53.json",
        "include-controls": [
          {"with-ids": ["ac-1", "ac-2", "ac-3"]}
        ]
      }
    ]
  }
}
```

### Component Definition

Describes how a component implements controls.

```json
{
  "component-definition": {
    "uuid": "...",
    "components": [
      {
        "uuid": "...",
        "title": "Kubernetes RBAC",
        "control-implementations": [
          {
            "source": "nist-800-53",
            "implemented-requirements": [
              {
                "control-id": "ac-2",
                "description": "Kubernetes RBAC provides account management..."
              }
            ]
          }
        ]
      }
    ]
  }
}
```

### System Security Plan (SSP)

Documents how a system implements security controls.

### Assessment Results

Records the findings from compliance assessments.

## Using OSCAL with ComplyTime

### Import Catalogs

```bash
# Download official NIST catalog
complyctl catalog fetch nist-800-53

# Import custom catalog
complyctl catalog import ./my-catalog.json
```

### Create Profiles

```bash
# Create a profile from catalog
complyctl profile create --catalog nist-800-53 \
  --include ac-1,ac-2,ac-3 \
  --output my-profile.json
```

### Generate SSP

```bash
# Generate SSP from profile and components
complyscribe ssp generate \
  --profile my-profile.json \
  --components ./components/ \
  --output ssp.json
```

### Assessment

```bash
# Run assessment and generate results
complyctl assess \
  --profile my-profile.json \
  --output assessment-results.json
```

## OSCAL SDK for Go

ComplyTime includes `oscal-sdk-go` for programmatic OSCAL manipulation:

```go
package main

import (
    "github.com/complytime/oscal-sdk-go/pkg/oscal"
)

func main() {
    // Load a catalog
    catalog, err := oscal.LoadCatalog("nist-800-53.json")
    if err != nil {
        panic(err)
    }

    // Find a control
    control := catalog.GetControl("ac-2")

    // Create assessment result
    result := oscal.NewAssessmentResult()
    result.AddFinding(oscal.Finding{
        ControlID: "ac-2",
        Status:    oscal.Satisfied,
        Evidence:  "...",
    })

    // Export
    result.Write("results.json")
}
```

## Resources

- [NIST OSCAL Website](https://pages.nist.gov/OSCAL/)
- [OSCAL GitHub](https://github.com/usnistgov/OSCAL)
- [oscal-sdk-go](https://github.com/complytime/oscal-sdk-go)

