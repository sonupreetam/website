---
title: "Create Diagrams"
description: "A workflow automation tool for compliance content authoring — Create Diagrams"
date: 2026-03-23T08:55:23Z
lastmod: 2026-03-23T08:55:23Z
draft: false
weight: 10
params:
  editURL: "https://github.com/complytime/complyscribe/edit/main/docs/workflows/create_diagrams.md"
---
<!-- synced from complytime/complyscribe/docs/workflows/create_diagrams.md@main (4cbca99defdf) -->

### Context

```kroki {type=mermaid}
graph LR
    User["User"] --> Workflow_Dispatch["Workflow Dispatch"]
    Workflow_Dispatch --> ComplyScribe["ComplyScribe"]
    ComplyScribe --> New_Branch["New Branch"]
    New_Branch --> PR["Draft Pull Request"]

```

### Container

```kroki {type=mermaid}
graph LR
    User["User"] --> GH_Action["GitHub Action"]
    GH_Action --> ComplyScribe["ComplyScribe"]
    ComplyScribe --> Compliance_Trestle["Compliance-Trestle SDK"]
    Compliance_Trestle --> Git_Provider_API["Git Provider API"]
    Git_Provider_API --> Branch["New Branch"]
    Branch --> PR["Draft Pull Request"]
```