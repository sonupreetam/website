# Tasks: Architecture Diagram and Getting-Started Content

**Input**: Design documents from `specs/013-sync-summary-arch-diagram/`  
**Branch**: `013-sync-summary-arch-diagram`  
**Status**: Retroactive — feature delivered in PR #7. Tasks reflect implementation order.  
**Prerequisites**: plan.md ✅ spec.md ✅ research.md ✅

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to ([US1], [US2], [US3])
- No test tasks generated — spec calls for manual/visual verification only

## Path Conventions

Hugo site root. All paths relative to `website/`.

---

## Phase 1: Setup (Static Assets)

**Purpose**: Commit the architecture diagram image files that all subsequent work depends on.

- [ ] T001 Add light-theme architecture diagram PNG to `static/images/complytime-architecture.png`
- [ ] T002 [P] Add dark-theme architecture diagram PNG to `static/images/complytime-architecture-dark.png`

**Checkpoint**: Both PNG assets present in `static/images/` — theme-image shortcode can now reference them

---

## Phase 2: Foundational (Theme-Image Component)

**Purpose**: Build the CSS-only theme switching component. Blocks US1 and US2 — must be complete before any content page can use the shortcode.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T003 Create `layouts/shortcodes/theme-image.html` accepting `light`, `dark`, and `alt` params; render two `<img>` elements with classes `theme-image-light` and `theme-image-dark`; default `alt` to `"Architecture Diagram"` when omitted
- [ ] T004 [P] Add `.theme-image-light` / `.theme-image-dark` CSS display rules to `assets/scss/common/_custom.scss` covering three cases: light default, `[data-bs-theme="dark"]` attribute, and `@media (prefers-color-scheme: dark)` with `:not([data-bs-theme="light"])` guard

**Checkpoint**: Foundation ready — `{{< theme-image >}}` shortcode usable from any content page

---

## Phase 3: User Story 1 — Architecture Diagram on Getting-Started Page (Priority: P1) 🎯 MVP

**Goal**: New users opening the getting-started page see an "Architecture Overview" section with a theme-aware diagram before they reach any prerequisites or commands.

**Independent Test**: Open `http://localhost:1313/docs/getting-started/`. Confirm an "Architecture Overview" section appears above Prerequisites, containing the architecture diagram image and three labelled domain descriptions.

- [ ] T005 [US1] Add an "Architecture Overview" section heading above the Prerequisites section in `content/docs/getting-started/_index.md`
- [ ] T006 [US1] Insert `{{< theme-image light="/images/complytime-architecture.png" dark="/images/complytime-architecture-dark.png" alt="ComplyTime Architecture Diagram" >}}` shortcode call within the Architecture Overview section in `content/docs/getting-started/_index.md`
- [ ] T007 [US1] Add three-domain description bullet points (Definition, Measurement, Preventative Enforcement) below the diagram in `content/docs/getting-started/_index.md`

**Checkpoint**: User Story 1 independently testable — architecture section visible with correct theme switching

---

## Phase 4: User Story 2 — Reusable Theme-Image Shortcode (Priority: P2)

**Goal**: The theme-image shortcode is general-purpose; any contributor can use it on any docs page without per-page CSS or raw HTML.

**Independent Test**: Add `{{< theme-image light="/images/example-light.png" dark="/images/example-dark.png" alt="Example" >}}` to any content page, run `hugo --minify --gc`, inspect HTML output for two `<img>` elements with correct classes.

- [ ] T008 [P] [US2] Confirm `layouts/shortcodes/theme-image.html` renders exactly two `<img>` elements — one with `class="theme-image-light"`, one with `class="theme-image-dark"` — for any valid `light`/`dark` parameter values
- [ ] T009 [P] [US2] Confirm CSS in `assets/scss/common/_custom.scss` shows only the light image when `data-bs-theme` is absent or `"light"` (`.theme-image-dark { display: none }` is the default)
- [ ] T010 [P] [US2] Confirm CSS in `assets/scss/common/_custom.scss` shows only the dark image when `[data-bs-theme="dark"]` is set on a parent element (explicit Bootstrap dark mode toggle)
- [ ] T011 [P] [US2] Confirm CSS in `assets/scss/common/_custom.scss` shows dark image under `@media (prefers-color-scheme: dark)` when no explicit `data-bs-theme` override is present (`:not([data-bs-theme="light"])` guard)
- [ ] T012 [P] [US2] Confirm `alt` parameter defaults to `"Architecture Diagram"` in `layouts/shortcodes/theme-image.html` when the parameter is omitted by the caller

**Checkpoint**: User Stories 1 and 2 both pass — shortcode works on getting-started and any other page

---

## Phase 5: User Story 3 — complyctl Quick-Start Guide (Priority: P2)

**Goal**: Replace the outdated getting-started CLI section with accurate, end-to-end complyctl instructions that a new user on Linux or macOS can follow verbatim on a clean machine.

**Independent Test**: Follow the guide verbatim on a clean machine with no prior complyctl install. Each numbered command must exit 0 and produce the described output.

- [ ] T013 [US3] Replace the outdated CLI installation section with current binary download instructions (GitHub releases page download of `complyctl` binary) in `content/docs/getting-started/_index.md`
- [ ] T014 [US3] Add `cosign verify-blob` signature verification step with the correct GitHub Actions OIDC issuer and certificate identity in `content/docs/getting-started/_index.md`
- [ ] T015 [US3] Add scanning provider installation step (download provider binary to `~/.complytime/providers/`, set executable) in `content/docs/getting-started/_index.md`
- [ ] T016 [US3] Add `complyctl init` step showing workspace init in an empty directory producing `complytime.yaml` in `content/docs/getting-started/_index.md`
- [ ] T017 [US3] Add `complyctl get` step showing policy fetch to `~/.complytime/policies/` in `content/docs/getting-started/_index.md`
- [ ] T018 [US3] Add `complyctl list` and `complyctl generate` steps in `content/docs/getting-started/_index.md`
- [ ] T019 [US3] Add `complyctl scan --policy-id <id>` step with all four output format examples (`EvaluationLog` default, `--format pretty`, `--format oscal`, `--format sarif`) in `content/docs/getting-started/_index.md`
- [ ] T020 [US3] Add `complyctl doctor` and `complyctl providers` as optional health-check steps in `content/docs/getting-started/_index.md`

**Checkpoint**: All three user stories independently functional — architecture diagram, shortcode, and guide all complete

---

## Phase 6: Polish & Cross-Cutting Validation

**Purpose**: Verify constitution compliance and cross-cutting quality gates across all delivered stories.

- [ ] T021 Run `hugo --minify --gc` from repo root and confirm zero errors and zero warnings (SC-007, FR-007)
- [ ] T022 [P] Visual check in browser light mode: light architecture diagram is visible, dark variant is hidden (SC-001)
- [ ] T023 [P] Visual check in browser dark mode (`data-bs-theme="dark"` on `<html>`): dark diagram visible, light hidden (SC-002)
- [ ] T024 [P] Browser DevTools media query emulation: enable `prefers-color-scheme: dark` with no explicit `data-bs-theme` set — dark diagram must appear (SC-003)
- [ ] T025 [P] NoScript test: disable JavaScript in browser, reload getting-started page — confirm fully readable with light diagram shown (SC-005, Constitution V)
- [ ] T026 [P] HTML inspection: confirm both `<img>` elements have non-empty `alt` attributes in the rendered page source (Constitution VII)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS US1 and US2 (shortcode references the images)
- **US1 (Phase 3)**: Depends on Phase 2 completion — shortcode must exist before use
- **US2 (Phase 4)**: Depends on Phase 2 completion — verifies shortcode contract; runs in parallel with US1 if desired
- **US3 (Phase 5)**: Independent of Phases 3 and 4 — guide rewrite touches only `content/docs/getting-started/_index.md` sections separate from the diagram section; can begin after Phase 1
- **Polish (Phase 6)**: Depends on all prior phases — requires complete page for full visual validation

### User Story Dependencies

- **US1 (P1)**: Depends on Phases 1 and 2 only
- **US2 (P2)**: Depends on Phase 2 only — no dependency on US1
- **US3 (P2)**: Depends on Phase 1 only (no shortcode needed) — can be worked in parallel with Phases 2–4

### Parallel Opportunities

- T001 and T002 (image assets) can be committed together
- T003 and T004 (shortcode + CSS) can be authored in parallel — different files
- T008–T012 (US2 verifications) are all independent — run together
- T022–T026 (Phase 6 visual checks) are all independent — run together in browser
- US3 (T013–T020) can be drafted entirely in parallel with Phases 2–4 since it only edits different sections of the same file

---

## Parallel Example: Phase 2

```
# Author in parallel (different files, no dependency):
Task T003: "Create theme-image shortcode in layouts/shortcodes/theme-image.html"
Task T004: "Add CSS rules in assets/scss/common/_custom.scss"
```

## Parallel Example: US3

```
# US3 guide sections can be drafted in any order (all in same file, different sections):
Task T013: binary download section
Task T014: cosign verification section
Task T015: provider installation section
# ... T016-T020 similarly independent of each other
```

---

## Implementation Strategy

### MVP (User Story 1 Only)

1. Complete Phase 1: Commit PNG assets
2. Complete Phase 2: Create shortcode + CSS
3. Complete Phase 3 (US1): Add Architecture Overview section to getting-started
4. **STOP and VALIDATE**: `hugo --minify --gc`; visual check light/dark; NoScript test
5. If passing — US1 is shippable independently

### Incremental Delivery

1. Phase 1 + Phase 2 → Theme-image infrastructure ready
2. Phase 3 (US1) → Architecture diagram live → Demo/validate
3. Phase 4 (US2) → Shortcode reusability confirmed for other contributors
4. Phase 5 (US3) → Full complyctl guide → Demo/validate  
5. Phase 6 → Full polish and constitution compliance sign-off

---

## Notes

- No new Go code, no Hugo module changes, no new npm dependencies — all tasks touch only content, layouts, SCSS, and static assets
- [P] tasks operate on different files or independent file sections — safe to run concurrently
- Commit after each phase checkpoint for clean rollback points
- US3 tasks (guide rewrite) are the most content-intensive but lowest technical risk — good candidate for parallel authoring alongside Phase 2/3 work
- Image files (~1.2–1.3 MB each) should be optimised to WebP in a follow-up (see research.md R3) but PNG is acceptable for v1
