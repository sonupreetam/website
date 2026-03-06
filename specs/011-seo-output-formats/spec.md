# Feature Specification: SEO + Output Formats

**Feature Branch**: `011-seo-output-formats`
**Created**: 2026-03-06
**Status**: Draft
**Phase**: 4 (Content Types)

## Comparative Analysis

### Current State (`complytime/website`)

- `config/_default/hugo.toml` defines outputs:
  - `home`: `["HTML", "RSS", "searchIndex"]`
  - `section`: `["HTML", "RSS", "SITEMAP"]`
  - `taxonomy` and `term`: `["HTML", "RSS"]`
- Standard SEO via `@thulite/seo` module (meta tags, Open Graph, favicons, schemas)
- No `llms` output format (plain text for LLM consumption)
- No `markdown` output format

### Target State (`test-website`)

- `config/_default/hugo.toml` adds two new output formats:
  - `llms` — Plain text format for LLM consumption (generates `llms.txt`)
  - `markdown` — Markdown output for content reuse
- Updated outputs:
  - `home`: `["HTML", "RSS", "searchIndex", "llms", "markdown"]`
- All existing outputs preserved

### Delta

| Item | Action | Details |
|------|--------|---------|
| `config/_default/hugo.toml` (outputs section) | Modify | Add `llms` and `markdown` to home outputs |
| `config/_default/hugo.toml` (outputFormats) | Modify | Define `llms` and `markdown` output format specs (mediaType, baseName, rel, etc.) |

### Conflicts

- Low risk. Only the `[outputs]` and `[outputFormats]` sections of `hugo.toml` are modified.
- Existing outputs (`HTML`, `RSS`, `searchIndex`, `SITEMAP`) are preserved.
- The `llms` output format requires a corresponding template (`layouts/_default/home.llms.txt` or similar). If no template exists, Hugo will silently skip the output. The template may need to be added as part of this feature.

## Acceptance Criteria

1. `hugo.toml` includes `llms` and `markdown` in the home outputs list
2. `outputFormats` section defines the `llms` format (mediaType `text/plain`, baseName `llms`)
3. `hugo build` generates `public/llms.txt` (or equivalent) when the corresponding template exists
4. All existing outputs (HTML, RSS, searchIndex, SITEMAP) continue to generate correctly
5. SEO metadata (meta tags, Open Graph) is unaffected
6. `hugo build` completes without errors

## Migration Steps

1. Add `llms` and `markdown` output format definitions to `hugo.toml`
2. Add `llms` and `markdown` to the `[outputs.home]` list
3. Create template for `llms` output format if not provided by Doks/Thulite
4. Verify `hugo build` generates all expected output files
5. Verify existing outputs are unaffected

## Rollback Plan

Remove the `llms` and `markdown` entries from `[outputs.home]` and `[outputFormats]` in `hugo.toml`.
