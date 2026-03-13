// SPDX-License-Identifier: Apache-2.0
package main

import (
	"strings"
	"testing"
)

func TestInjectFrontmatter(t *testing.T) {
	t.Run("prepend to content without frontmatter", func(t *testing.T) {
		content := []byte("# Hello World\n\nBody text.")
		fm := map[string]any{"title": "Hello", "weight": 10}
		out, err := injectFrontmatter(content, fm)
		if err != nil {
			t.Fatal(err)
		}
		result := string(out)

		if !strings.HasPrefix(result, "---\n") {
			t.Error("result should start with ---")
		}
		if !strings.Contains(result, "title: Hello") {
			t.Error("result should contain title field")
		}
		if !strings.Contains(result, "weight: 10") {
			t.Error("result should contain weight field")
		}
		if !strings.Contains(result, "# Hello World") {
			t.Error("result should preserve original content")
		}
	})

	t.Run("replace existing frontmatter", func(t *testing.T) {
		content := []byte("---\ntitle: Old\n---\n\nBody text.")
		fm := map[string]any{"title": "New"}
		out, err := injectFrontmatter(content, fm)
		if err != nil {
			t.Fatal(err)
		}
		result := string(out)

		if strings.Contains(result, "title: Old") {
			t.Error("old frontmatter should be replaced")
		}
		if !strings.Contains(result, "title: New") {
			t.Error("new frontmatter should be present")
		}
		if !strings.Contains(result, "Body text.") {
			t.Error("body should be preserved")
		}
	})

	t.Run("empty content", func(t *testing.T) {
		out, err := injectFrontmatter([]byte{}, map[string]any{"title": "Test"})
		if err != nil {
			t.Fatal(err)
		}
		result := string(out)
		if !strings.HasPrefix(result, "---\n") {
			t.Error("empty content should still get frontmatter")
		}
		if !strings.Contains(result, "title: Test") {
			t.Error("frontmatter should be present")
		}
	})
}

func TestStripBadges(t *testing.T) {
	t.Run("badge lines removed", func(t *testing.T) {
		input := "[![Build](https://img.shields.io/badge.svg)](https://example.com)\n\n# Title\nBody"
		result := stripBadges(input)
		if strings.Contains(result, "img.shields.io") {
			t.Error("badge line should be removed")
		}
		if !strings.Contains(result, "# Title") {
			t.Error("non-badge content should be preserved")
		}
	})

	t.Run("multiple badges removed", func(t *testing.T) {
		input := "[![A](https://a.svg)](https://a)\n[![B](https://b.svg)](https://b)\n\nContent"
		result := stripBadges(input)
		if strings.Contains(result, "[![A") || strings.Contains(result, "[![B") {
			t.Error("all badge lines should be removed")
		}
		if !strings.Contains(result, "Content") {
			t.Error("content should be preserved")
		}
	})

	t.Run("no badges", func(t *testing.T) {
		input := "# Title\n\nBody text"
		result := stripBadges(input)
		if result != input {
			t.Errorf("content without badges should be unchanged\ngot:  %q\nwant: %q", result, input)
		}
	})

	t.Run("inline badge preserved", func(t *testing.T) {
		input := "See [![badge](https://img.svg)](https://link) for details.\n"
		result := stripBadges(input)
		if !strings.Contains(result, "See") {
			t.Error("inline badge content should not be stripped")
		}
	})
}

func TestStripLeadingH1(t *testing.T) {
	t.Run("matching H1 removed", func(t *testing.T) {
		input := "# my-repo\n\nBody text"
		result := stripLeadingH1(input, "my-repo")
		if strings.Contains(result, "# my-repo") {
			t.Error("matching H1 should be removed")
		}
		if !strings.Contains(result, "Body text") {
			t.Error("body should be preserved")
		}
	})

	t.Run("case insensitive match", func(t *testing.T) {
		input := "# My-Repo\n\nBody"
		result := stripLeadingH1(input, "my-repo")
		if strings.Contains(result, "# My-Repo") {
			t.Error("case-insensitive H1 should be removed")
		}
	})

	t.Run("non-matching H1 preserved", func(t *testing.T) {
		input := "# Different Title\n\nBody"
		result := stripLeadingH1(input, "my-repo")
		if !strings.Contains(result, "# Different Title") {
			t.Error("non-matching H1 should be preserved")
		}
	})

	t.Run("no H1", func(t *testing.T) {
		input := "Body text without heading"
		result := stripLeadingH1(input, "my-repo")
		if result != input {
			t.Error("content without H1 should be unchanged")
		}
	})

	t.Run("H1 only line", func(t *testing.T) {
		input := "# my-repo"
		result := stripLeadingH1(input, "my-repo")
		if result != "" {
			t.Errorf("H1-only content should return empty string, got %q", result)
		}
	})
}

func TestRewriteRelativeLinks(t *testing.T) {
	owner, repo, branch := "org", "repo", "main"

	t.Run("relative link to absolute", func(t *testing.T) {
		input := "See [docs](docs/README.md) for details."
		result := rewriteRelativeLinks(input, owner, repo, branch)
		expected := "https://github.com/org/repo/blob/main/docs/README.md"
		if !strings.Contains(result, expected) {
			t.Errorf("expected link to contain %q, got %q", expected, result)
		}
	})

	t.Run("relative image to raw URL", func(t *testing.T) {
		input := "![logo](assets/logo.png)"
		result := rewriteRelativeLinks(input, owner, repo, branch)
		expected := "https://raw.githubusercontent.com/org/repo/main/assets/logo.png"
		if !strings.Contains(result, expected) {
			t.Errorf("expected image URL to contain %q, got %q", expected, result)
		}
	})

	t.Run("absolute URL unchanged", func(t *testing.T) {
		input := "[link](https://example.com/page)"
		result := rewriteRelativeLinks(input, owner, repo, branch)
		if result != input {
			t.Errorf("absolute URL should be unchanged\ngot:  %q\nwant: %q", result, input)
		}
	})

	t.Run("anchor link unchanged", func(t *testing.T) {
		input := "[section](#my-section)"
		result := rewriteRelativeLinks(input, owner, repo, branch)
		if result != input {
			t.Errorf("anchor link should be unchanged\ngot:  %q\nwant: %q", result, input)
		}
	})

	t.Run("dot-slash prefix stripped", func(t *testing.T) {
		input := "[file](./path/to/file.md)"
		result := rewriteRelativeLinks(input, owner, repo, branch)
		if strings.Contains(result, "./") {
			t.Errorf("dot-slash prefix should be stripped, got %q", result)
		}
		if !strings.Contains(result, "blob/main/path/to/file.md") {
			t.Errorf("path should be correct after stripping ./, got %q", result)
		}
	})
}

func TestInsertAfterFrontmatter(t *testing.T) {
	t.Run("with frontmatter", func(t *testing.T) {
		content := []byte("---\ntitle: Test\n---\n\nBody text")
		insert := []byte("<!-- provenance -->\n")
		result := string(insertAfterFrontmatter(content, insert))

		if !strings.Contains(result, "---\n<!-- provenance -->") {
			t.Errorf("provenance should appear after closing ---, got:\n%s", result)
		}
		if !strings.Contains(result, "Body text") {
			t.Error("body should be preserved")
		}
	})

	t.Run("without frontmatter", func(t *testing.T) {
		content := []byte("# Hello\n\nBody text")
		insert := []byte("<!-- provenance -->\n")
		result := string(insertAfterFrontmatter(content, insert))

		if !strings.HasPrefix(result, "<!-- provenance -->") {
			t.Errorf("provenance should be prepended when no frontmatter, got:\n%s", result)
		}
		if !strings.Contains(result, "# Hello") {
			t.Error("content should be preserved")
		}
	})
}
