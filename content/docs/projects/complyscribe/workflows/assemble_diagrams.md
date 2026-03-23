---
title: "Assemble Diagrams"
description: "A workflow automation tool for compliance content authoring — Assemble Diagrams"
date: 2026-03-23T08:55:23Z
lastmod: 2026-03-23T08:55:23Z
draft: false
weight: 10
params:
  editURL: "https://github.com/complytime/complyscribe/edit/main/docs/workflows/assemble_diagrams.md"
---
<!-- synced from complytime/complyscribe/docs/workflows/assemble_diagrams.md@main (541b4ce555ef) -->

### Context

```kroki {type=mermaid}
graph LR
    User["User"] --> Assemble_Workflow["Assemble Workflow"]
    Assemble_Workflow --> ComplyScribe["ComplyScribe"]
    ComplyScribe --> Branch["User's Git Branch"]
```

### Container

```kroki {type=mermaid}
graph LR
    User["User"] --> GH_Action["GitHub Action"]
    GH_Action --> ComplyScribe["ComplyScribe"]
    ComplyScribe --> Compliance_Trestle["Compliance-Trestle SDK"]
    Compliance_Trestle --> Git_Provider_API["Git Provider API"]
    Git_Provider_API --> Branch["User's Git Branch"]
```