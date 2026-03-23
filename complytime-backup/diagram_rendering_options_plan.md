---
name: Diagram Rendering Options
overview: Analysis of options for rendering diagrams (Mermaid, D2, PlantUML, etc.) in synced Hugo content and AI-driven diagram generation for repo/project architecture visualization.
todos:
  - id: phase1-transform
    content: Add rewriteDiagramBlocks transform to cmd/sync-content/transform.go that converts standard diagram code blocks (```mermaid, ```plantuml, etc.) to Kroki format (```kroki {type=...})
    status: pending
  - id: phase1-tests
    content: Add unit tests for the diagram block transform in transform_test.go
    status: pending
  - id: phase1-wire
    content: Wire rewriteDiagramBlocks into the sync pipeline and add rewrite_diagrams config option
    status: pending
  - id: phase1-verify
    content: Verify end-to-end Kroki rendering with a test diagram in synced content
    status: pending
  - id: phase2-spec
    content: "Draft a new feature spec (007) for AI-driven diagram generation covering: repo analysis inputs, LLM prompt templates, diagram output format, CLI interface, determinism strategy, and CI integration"
    status: pending
isProject: false
---

# Diagram Rendering and AI-Driven Diagram Generation

## Current State

The site already has **partial diagram infrastructure** in place:

- **Kroki configured**: `krokiURL = "https://kroki.io"` in `[config/_default/params.toml](config/_default/params.toml)` line 78
- **Kroki render hook**: Doks theme ships `render-codeblock-kroki.html` which calls the Kroki API at build time, receives SVG, and embeds it as an `<img>` tag -- supports 25 diagram types (Mermaid, D2, PlantUML, Graphviz, etc.)
- **Mermaid render hook**: Doks theme ships `render-codeblock-mermaid.html` which outputs a `<div class="mermaid">` but **mermaid.js is not loaded**, so client-side Mermaid doesn't work
- **GoAT**: Hugo natively renders ASCII art diagrams via `render-codeblock-goat.html`
- **Sync tool**: Has no diagram awareness -- standard ````mermaid` blocks from GitHub repos pass through without conversion to the ````kroki {type=mermaid}` format Hugo needs

## The Two Capabilities

There are two distinct concerns here, and they should be treated as **separate features**:

### Capability 1: Diagram Rendering in Synced Content

**Problem**: Source repos use standard GitHub-flavored Markdown diagram blocks (````mermaid`, ````plantuml`, etc.). GitHub renders these natively. But when synced to Hugo, they need to be in Kroki format (````kroki {type=mermaid}`) for build-time SVG rendering.

### Capability 2: AI-Driven Diagram Generation

**Problem**: Understanding a repo's architecture, workflows, or data flows requires manually creating diagrams. AI could analyze repos and generate deterministic, reproducible diagram source code.

---

## Options for Capability 1: Diagram Rendering

### Option A: Sync Tool Transform (Recommended)

Add a new transform to `[cmd/sync-content/transform.go](cmd/sync-content/transform.go)` that converts standard diagram code blocks to Kroki format during sync.

**How it works**:

- Regex matches ````mermaid`, ````plantuml`, ````d2`, ````graphviz`, etc.
- Rewrites to ````kroki {type=mermaid}`, ````kroki {type=plantuml}`, etc.
- Runs as part of the existing transform pipeline (alongside `shiftHeadings`, `rewriteRelativeLinks`, etc.)
- Add a `rewrite_diagrams` transform option in `sync-config.yaml`

**Pros**: Deterministic, server-side SVG at build time (no client JS), works with all 25 Kroki types, fits the existing sync architecture, diagrams are cached as Hugo resources

**Cons**: Requires kroki.io availability at build time (or self-hosted Kroki)

```go
// Example transform signature in transform.go
var diagramBlockRe = regexp.MustCompile("(?m)^```(mermaid|plantuml|d2|graphviz|...)\\s*$")

func rewriteDiagramBlocks(content string) string {
    return diagramBlockRe.ReplaceAllStringFunc(content, func(match string) string {
        lang := strings.TrimPrefix(strings.TrimSpace(match), "```")
        return fmt.Sprintf("```kroki {type=%s}", lang)
    })
}
```

### Option B: Client-Side Mermaid.js

Load the Mermaid.js library to render standard ````mermaid` blocks client-side.

**How it works**:

- Add mermaid.js to `[layouts/_partials/footer/script-footer-custom.html](layouts/_partials/footer/script-footer-custom.html)` (currently empty)
- The existing `render-codeblock-mermaid.html` already sets `hasMermaid` -- conditionally load the script

**Pros**: Zero sync tool changes, standard GitHub Markdown works as-is, live/interactive diagrams

**Cons**: Only Mermaid (not PlantUML, D2, etc.), client-side rendering (flash of unstyled content), larger JS bundle, harder to get consistent styling

### Option C: Hybrid (Kroki + Client Mermaid)

Use Kroki for non-Mermaid types (PlantUML, D2, etc.) and client-side Mermaid for ````mermaid` blocks.

**Pros**: Standard Mermaid blocks work without transform, other types get server-side SVG

**Cons**: Two rendering paths to maintain, inconsistent behavior between diagram types

### Option D: Self-Hosted Kroki via Docker

Run Kroki locally or in CI instead of using kroki.io.

**Pros**: No external dependency, faster builds, private diagrams

**Cons**: Infrastructure overhead, Docker in CI adds complexity

**Recommendation**: **Option A** (sync tool transform) is the cleanest fit. It aligns with the existing architecture, requires no client-side JS, supports all diagram types through a single mechanism, and keeps the rendering deterministic. Option D can be layered on later if kroki.io becomes a bottleneck.

---

## Options for Capability 2: AI-Driven Diagram Generation

### Option E: CLI Command with AI Backend

A new subcommand or standalone tool that:

1. Analyzes a repo (file tree, Go packages, imports, CI workflows, README)
2. Sends structured context to an LLM API
3. Receives Mermaid/D2/PlantUML source code
4. Writes `.md` files with diagram code blocks into the content tree

**Trigger options**:

- `go run ./cmd/sync-content --generate-diagrams` (flag on existing tool)
- Separate `cmd/generate-diagrams/` tool
- CI job that runs periodically or on-demand

**Determinism approach**: Pin the LLM prompt template and repo analysis inputs, use temperature=0, hash the input to detect changes and only regenerate when the repo structure changes.

### Option F: Pre-Computed Diagram Files in Source Repos

Instead of AI at build time, generate diagrams as checked-in `.md` files in each source repo (e.g. `docs/architecture.md` with Mermaid blocks). The sync tool would pick these up naturally.

**Pros**: Simplest, no AI dependency at build time, diagrams are human-reviewable in PRs

**Cons**: Manual maintenance, diagrams drift from code

### Option G: Cursor/IDE-Assisted Diagram Authoring

Use AI in the development workflow (e.g. Cursor agent skills) to generate diagrams that get committed to source repos, then synced naturally.

**Pros**: Human-in-the-loop, diagrams are reviewed before commit, no runtime AI dependency

**Cons**: Not automated, relies on contributor discipline

**Recommendation**: Start with **Option F** (diagrams in source repos) as the immediate path -- the sync tool already handles this. Then build **Option E** (AI CLI) as a future feature when the diagram patterns and needs are better understood. Option G is a good practice regardless.

---

## Recommended Implementation Path

### Phase 1: Diagram Rendering in Synced Content (Option A)

This is a **small enhancement to the existing sync tool** (spec 006), not a separate feature. Scope:

1. Add `rewriteDiagramBlocks` transform to `transform.go`
2. Add corresponding tests in `transform_test.go`
3. Wire it into the sync pipeline (apply after `rewriteRelativeLinks`)
4. Add `rewrite_diagrams: true` as a config-level default in `sync-config.yaml`
5. Verify Kroki rendering works end-to-end with a test diagram

### Phase 2: AI-Driven Diagram Generation (New Feature -- Spec 007 or similar)

This **should be a new feature spec** because it introduces:

- AI/LLM dependency (API keys, prompt engineering, cost management)
- New content type (generated diagram pages)
- New CLI command or flags
- Determinism and caching strategy
- Potentially a new CI workflow

Scope of the spec:

- Define what diagrams to generate (architecture, data flow, component, sequence)
- Choose diagram language (D2 or Mermaid -- D2 is more expressive for architecture)
- Define the AI prompt template and input analysis
- Define the output format and where diagrams live in the content tree
- Define the regeneration trigger and caching strategy

---

## Summary


| Concern                             | New Feature?                              | Approach                                                    | Effort                                                   |
| ----------------------------------- | ----------------------------------------- | ----------------------------------------------------------- | -------------------------------------------------------- |
| Diagram rendering in synced content | No -- enhancement to sync tool (spec 006) | Sync transform: rewrite diagram blocks to Kroki format      | Small (1-2 files, ~50 lines Go + tests)                  |
| AI-driven diagram generation        | Yes -- new feature spec                   | CLI tool + LLM API for repo analysis and diagram generation | Medium-Large (new spec, new command, prompt engineering) |


