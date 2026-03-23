---
title: "Plugin Guide"
description: "A command-line tool for streamlining end-to-end compliance workflows on local systems. — Plugin Guide"
date: 2026-03-23T09:20:36Z
lastmod: 2026-03-23T09:20:36Z
draft: false
weight: 10
params:
  editURL: "https://github.com/complytime/complyctl/edit/main/docs/PLUGIN_GUIDE.md"
---
<!-- synced from complytime/complyctl/docs/PLUGIN_GUIDE.md@main (a1e7a8b83329) -->

complyctl extends to arbitrary policy engines through plugins. Each plugin is a standalone executable that communicates with the CLI via gRPC using the [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) framework.

### Discovery

Scanning providers are discovered by scanning `~/.complytime/providers/` for executables matching the naming convention:

```
complyctl-provider-<evaluator-id>
```

The CLI strips the `complyctl-provider-` prefix to derive the **evaluator ID** used for routing Generate and Scan requests.

| Example Binary | Evaluator ID |
|:---|:---|
| `complyctl-provider-openscap` | `openscap` |
| `complyctl-provider-kubernetes` | `kubernetes` |
| `complyctl-provider-test` | `test` |

No manifest files, no configuration files. The executable must be in the plugin directory and have execute permission.

### gRPC Interface

Plugins implement the `Plugin` interface (defined in `pkg/plugin/manager.go`):

```go
type Plugin interface {
    Describe(ctx context.Context, req *DescribeRequest) (*DescribeResponse, error)
    Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
    Scan(ctx context.Context, req *ScanRequest) (*ScanResponse, error)
}
```

#### Describe

Called during plugin discovery and `complyctl doctor` diagnostics. Returns plugin health, version, and declared variable requirements (`RequiredGlobalVariables`, `RequiredTargetVariables`). Plugins that return `Healthy: false` or fail the RPC are skipped during loading.

#### Generate

Called by `complyctl generate`. Receives a three-tier variable model (R48):

| Tier | Field | Source |
|:---|:---|:---|
| 1 — Global | `GlobalVariables` | `complytime.yaml` top-level `variables` |
| 2 — Target | `TargetVariables` | `complytime.yaml` `targets[].variables` (one target per call) |
| 3 — Test | `Configuration[].Parameters` | Per-requirement parameters from the assessment plan |

The plugin prepares declarative policies in whatever format the underlying engine expects.

#### Scan

Called by `complyctl scan`. Receives targets only — no requirement IDs are sent. The provider evaluates all requirements from Generate-time state (R47). Returns `AssessmentLog` entries — one per requirement evaluated — each containing steps with pass/fail/skip/error results and a `ConfidenceLevel` enum.

### Protobuf Contract

The canonical protobuf definition lives at `api/plugin/plugin.proto`. Key types:

| Type | Purpose |
|:---|:---|
| `GenerateRequest` | Global variables, target variables, assessment configurations |
| `AssessmentConfiguration` | Plan ID, requirement ID, parameters map |
| `Target` | Target ID + plugin-defined variables |
| `AssessmentLog` | Requirement ID, steps, message, confidence level |
| `Step` | Name, result, message |
| `DescribeResponse` | Health, version, required global/target variable names |
| `ConfidenceLevel` | Enum: NOT_SET, UNDETERMINED, LOW, MEDIUM, HIGH |
| `Result` | Enum: UNSPECIFIED, PASSED, FAILED, SKIPPED, ERROR |

### Authoring A Plugin (go)

Use `plugin.Serve()` to register and start the gRPC server. The handshake is handled automatically.

```go
package main

import (
    "context"

    "github.com/complytime/complyctl/pkg/plugin"
)

var _ plugin.Plugin = (*myPlugin)(nil)

type myPlugin struct{}

func (p *myPlugin) Describe(_ context.Context, _ *plugin.DescribeRequest) (*plugin.DescribeResponse, error) {
    return &plugin.DescribeResponse{
        Healthy: true,
        Version: "1.0.0",
        RequiredGlobalVariables: []string{"output_dir"},
        RequiredTargetVariables: []string{"kubeconfig"},
    }, nil
}

func (p *myPlugin) Generate(_ context.Context, req *plugin.GenerateRequest) (*plugin.GenerateResponse, error) {
    _ = req.GlobalVariables
    _ = req.TargetVariables
    for _, cfg := range req.Configuration {
        _ = cfg.RequirementID
        _ = cfg.Parameters
    }
    return &plugin.GenerateResponse{Success: true}, nil
}

func (p *myPlugin) Scan(_ context.Context, req *plugin.ScanRequest) (*plugin.ScanResponse, error) {
    var assessments []plugin.AssessmentLog
    for _, target := range req.Targets {
        assessments = append(assessments, plugin.AssessmentLog{
            RequirementID: target.TargetID + "-check",
            Steps: []plugin.Step{{
                Name:    "my-check",
                Result:  plugin.ResultPassed,
                Message: "check passed",
            }},
            Message:    "evaluation complete",
            Confidence: plugin.ConfidenceLevelHigh,
        })
    }
    return &plugin.ScanResponse{Assessments: assessments}, nil
}

func main() {
    plugin.Serve(&myPlugin{})
}
```

Build and install:

```bash
go build -o complyctl-provider-myplugin ./cmd/myplugin
cp complyctl-provider-myplugin ~/.complytime/providers/
```

### Routing

The CLI routes requests based on **evaluator ID** extracted from the Gemara policy graph:

1. Policy assessment configs include an `evaluator_id` field
2. CLI groups configs by evaluator ID
3. Each group is dispatched to the matching plugin
4. If no match is found, the request is broadcast to all loaded plugins

### Variables

Plugins receive variables through a three-tier model (R48):

| Tier | Config Location | Delivered Via | Scope |
|:---|:---|:---|:---|
| Global | `complytime.yaml` `variables` | `GenerateRequest.GlobalVariables` | Workspace-wide |
| Target | `complytime.yaml` `targets[].variables` | `GenerateRequest.TargetVariables` (Generate) / `Target.Variables` (Scan) | Per-target |
| Test | Assessment plan parameters | `AssessmentConfiguration.Parameters` | Per-requirement |

Plugins declare their required variable names via the `Describe` RPC (`RequiredGlobalVariables`, `RequiredTargetVariables`). The `complyctl doctor` command validates these exist in the workspace config (R51).

```yaml
variables:
  output_dir: /tmp/scan-results

targets:
  - id: production-cluster
    policies:
      - nist-800-53-r5
    variables:
      kubeconfig: /path/to/kubeconfig
      namespace: default
```

### Reference Implementation

See `cmd/test-plugin/main.go` for a complete working example. Build with:

```bash
make build-test-plugin
```
