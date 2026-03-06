# Feature Specification: Blog + Taxonomies

**Feature Branch**: `010-blog-taxonomies`
**Created**: 2026-03-06
**Status**: Draft
**Phase**: 4 (Content Types)

## Comparative Analysis

### Current State (`complytime/website`)

- Taxonomies defined in `hugo.toml`: `contributor`, `category`, `tag`
- No `content/blog/` directory — taxonomies are configured but unused
- No blog posts or taxonomy term pages
- `content/contributors/`, `content/categories/`, `content/tags/` directories do not exist
- Menus do not include a blog link

### Target State (`test-website`)

- `content/blog/` directory with blog posts
- `content/contributors/` with contributor profile pages
- `content/categories/` and `content/tags/` for taxonomy term pages
- Blog posts use frontmatter: `title`, `date`, `contributors`, `categories`, `tags`
- Blog section is browsable at `/blog/`
- Taxonomy pages at `/contributors/`, `/categories/`, `/tags/`

### Delta

| Item | Action | Details |
|------|--------|---------|
| `content/blog/` | Add | Blog section with `_index.md` and initial posts |
| `content/blog/_index.md` | Add | Blog section index page |
| `content/contributors/` | Add | Contributor taxonomy term pages |
| `content/categories/` | Add | Category taxonomy term pages |
| `content/tags/` | Add | Tag taxonomy term pages |
| `config/_default/hugo.toml` | No change | Taxonomies already defined |
| `config/_default/menus/menus.en.toml` | Modify | Add "Blog" to navigation menu |

### Conflicts

- Taxonomy configuration already exists in `hugo.toml` — no config changes needed for taxonomy definitions.
- Adding "Blog" to the navigation menu modifies `menus.en.toml`. This is a low-risk change that adds a new entry without modifying existing ones.
- Doks theme provides default list/single layouts that should work for blog posts without custom layouts.

## Acceptance Criteria

1. `content/blog/_index.md` exists with appropriate section frontmatter
2. At least one sample blog post exists to verify rendering
3. Blog posts render correctly at `/blog/[slug]/`
4. Blog index page at `/blog/` lists all posts
5. Contributor, category, and tag taxonomy pages render correctly
6. "Blog" appears in the navigation menu
7. Blog posts support `contributors`, `categories`, and `tags` frontmatter
8. `hugo build` completes without errors

## Migration Steps

1. Create `content/blog/_index.md`
2. Add at least one sample blog post
3. Create taxonomy term directories if needed
4. Add "Blog" entry to `menus.en.toml`
5. Verify blog list and single pages render
6. Verify taxonomy pages generate correctly

## Rollback Plan

1. Delete `content/blog/`, `content/contributors/`, `content/categories/`, `content/tags/`
2. Remove "Blog" entry from `menus.en.toml`
