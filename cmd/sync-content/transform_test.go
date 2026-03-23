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
	t.Run("H1 removed", func(t *testing.T) {
		input := "# Plugin Guide\n\nBody text here."
		result := stripLeadingH1(input)
		if strings.Contains(result, "# Plugin Guide") {
			t.Errorf("leading H1 should be removed, got %q", result)
		}
		if !strings.Contains(result, "Body text here.") {
			t.Error("body text should be preserved")
		}
	})

	t.Run("H2 not removed", func(t *testing.T) {
		input := "## Section\n\nBody"
		result := stripLeadingH1(input)
		if result != input {
			t.Errorf("H2 should not be stripped\ngot:  %q\nwant: %q", result, input)
		}
	})

	t.Run("no heading unchanged", func(t *testing.T) {
		input := "Body text without heading"
		result := stripLeadingH1(input)
		if result != input {
			t.Errorf("content without heading should be unchanged\ngot:  %q\nwant: %q", result, input)
		}
	})

	t.Run("H1 only", func(t *testing.T) {
		input := "# Title Only"
		result := stripLeadingH1(input)
		if result != "" {
			t.Errorf("H1-only content should return empty string, got %q", result)
		}
	})

	t.Run("blank lines after H1 trimmed", func(t *testing.T) {
		input := "# Title\n\n\n\nBody"
		result := stripLeadingH1(input)
		if strings.HasPrefix(result, "\n") {
			t.Errorf("leading blank lines after H1 should be trimmed, got %q", result)
		}
		if !strings.Contains(result, "Body") {
			t.Error("body should be preserved")
		}
	})
}

func TestShiftHeadings(t *testing.T) {
	t.Run("H1 becomes H2", func(t *testing.T) {
		input := "# Title\n\nBody text"
		result := shiftHeadings(input)
		if !strings.Contains(result, "## Title") {
			t.Errorf("H1 should become H2, got %q", result)
		}
		if strings.HasPrefix(result, "# Title") {
			t.Error("original H1 should not remain")
		}
	})

	t.Run("H2 becomes H3", func(t *testing.T) {
		input := "## Subtitle\n\nBody"
		result := shiftHeadings(input)
		if !strings.Contains(result, "### Subtitle") {
			t.Errorf("H2 should become H3, got %q", result)
		}
	})

	t.Run("H6 stays H6", func(t *testing.T) {
		input := "###### Deep\n\nBody"
		result := shiftHeadings(input)
		if !strings.Contains(result, "###### Deep") {
			t.Errorf("H6 should stay H6, got %q", result)
		}
	})

	t.Run("multiple headings shifted", func(t *testing.T) {
		input := "# Title\n\n## Section\n\n### Sub\n\nBody"
		result := shiftHeadings(input)
		if !strings.Contains(result, "## Title") {
			t.Errorf("H1 should become H2, got %q", result)
		}
		if !strings.Contains(result, "### Section") {
			t.Errorf("H2 should become H3, got %q", result)
		}
		if !strings.Contains(result, "#### Sub") {
			t.Errorf("H3 should become H4, got %q", result)
		}
	})

	t.Run("no headings unchanged", func(t *testing.T) {
		input := "Body text without any headings"
		result := shiftHeadings(input)
		if result != input {
			t.Errorf("content without headings should be unchanged\ngot:  %q\nwant: %q", result, input)
		}
	})

	t.Run("body text preserved", func(t *testing.T) {
		input := "# Title\n\nParagraph one.\n\nParagraph two."
		result := shiftHeadings(input)
		if !strings.Contains(result, "Paragraph one.") || !strings.Contains(result, "Paragraph two.") {
			t.Error("body text should be preserved")
		}
	})
}

func TestTitleCaseHeadings(t *testing.T) {
	t.Run("lowercase heading becomes Title Case", func(t *testing.T) {
		input := "## getting started\n\nBody text"
		result := titleCaseHeadings(input)
		if !strings.Contains(result, "## Getting Started") {
			t.Errorf("heading should be Title Case, got %q", result)
		}
	})

	t.Run("acronyms are preserved", func(t *testing.T) {
		input := "## api reference\n\nBody"
		result := titleCaseHeadings(input)
		if !strings.Contains(result, "## API Reference") {
			t.Errorf("API should be uppercased, got %q", result)
		}
	})

	t.Run("mixed case normalized", func(t *testing.T) {
		input := "### using the CLI tool\n\nBody"
		result := titleCaseHeadings(input)
		if !strings.Contains(result, "### Using The CLI Tool") {
			t.Errorf("heading should be Title Case with acronyms, got %q", result)
		}
	})

	t.Run("multiple headings normalized", func(t *testing.T) {
		input := "## getting started\n\nParagraph.\n\n## api reference\n\nMore text."
		result := titleCaseHeadings(input)
		if !strings.Contains(result, "## Getting Started") {
			t.Errorf("first heading should be Title Case, got %q", result)
		}
		if !strings.Contains(result, "## API Reference") {
			t.Errorf("second heading should have API uppercase, got %q", result)
		}
	})

	t.Run("already correct casing unchanged", func(t *testing.T) {
		input := "## Quick Start\n\nBody"
		result := titleCaseHeadings(input)
		if !strings.Contains(result, "## Quick Start") {
			t.Errorf("already correct heading should stay the same, got %q", result)
		}
	})

	t.Run("body text not affected", func(t *testing.T) {
		input := "## Title\n\nsome lowercase body text here."
		result := titleCaseHeadings(input)
		if !strings.Contains(result, "some lowercase body text here.") {
			t.Errorf("body text should not be changed, got %q", result)
		}
	})

	t.Run("no headings unchanged", func(t *testing.T) {
		input := "Just some body text without headings."
		result := titleCaseHeadings(input)
		if result != input {
			t.Errorf("content without headings should be unchanged\ngot:  %q\nwant: %q", result, input)
		}
	})

	t.Run("H6 heading normalized", func(t *testing.T) {
		input := "###### deep heading\n\nBody"
		result := titleCaseHeadings(input)
		if !strings.Contains(result, "###### Deep Heading") {
			t.Errorf("H6 heading should be Title Case, got %q", result)
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

func TestRewriteDiagramBlocks(t *testing.T) {
	t.Run("mermaid block rewritten", func(t *testing.T) {
		input := "# Doc\n\n```mermaid\ngraph TD\n  A-->B\n```\n\nMore text."
		result := rewriteDiagramBlocks(input)
		if !strings.Contains(result, "```kroki {type=mermaid}") {
			t.Errorf("mermaid block should be rewritten to kroki format, got %q", result)
		}
		if strings.Contains(result, "```mermaid") {
			t.Error("original ```mermaid fence should not remain")
		}
		if !strings.Contains(result, "graph TD") {
			t.Error("diagram body should be preserved")
		}
	})

	t.Run("plantuml block rewritten", func(t *testing.T) {
		input := "```plantuml\n@startuml\nAlice -> Bob\n@enduml\n```"
		result := rewriteDiagramBlocks(input)
		if !strings.Contains(result, "```kroki {type=plantuml}") {
			t.Errorf("plantuml block should be rewritten, got %q", result)
		}
	})

	t.Run("d2 block rewritten", func(t *testing.T) {
		input := "```d2\nx -> y\n```"
		result := rewriteDiagramBlocks(input)
		if !strings.Contains(result, "```kroki {type=d2}") {
			t.Errorf("d2 block should be rewritten, got %q", result)
		}
	})

	t.Run("dot alias normalised to graphviz", func(t *testing.T) {
		input := "```dot\ndigraph { a -> b }\n```"
		result := rewriteDiagramBlocks(input)
		if !strings.Contains(result, "```kroki {type=graphviz}") {
			t.Errorf("dot should be normalised to graphviz, got %q", result)
		}
	})

	t.Run("graphviz block rewritten", func(t *testing.T) {
		input := "```graphviz\ndigraph { a -> b }\n```"
		result := rewriteDiagramBlocks(input)
		if !strings.Contains(result, "```kroki {type=graphviz}") {
			t.Errorf("graphviz block should be rewritten, got %q", result)
		}
	})

	t.Run("multiple diagram blocks rewritten", func(t *testing.T) {
		input := "# Diagrams\n\n```mermaid\ngraph TD\n```\n\nText.\n\n```plantuml\n@startuml\n@enduml\n```"
		result := rewriteDiagramBlocks(input)
		if !strings.Contains(result, "```kroki {type=mermaid}") {
			t.Error("first diagram block should be rewritten")
		}
		if !strings.Contains(result, "```kroki {type=plantuml}") {
			t.Error("second diagram block should be rewritten")
		}
	})

	t.Run("non-diagram code blocks unchanged", func(t *testing.T) {
		input := "```go\nfunc main() {}\n```\n\n```python\nprint('hi')\n```"
		result := rewriteDiagramBlocks(input)
		if result != input {
			t.Errorf("non-diagram code blocks should be unchanged\ngot:  %q\nwant: %q", result, input)
		}
	})

	t.Run("already kroki block unchanged", func(t *testing.T) {
		input := "```kroki {type=mermaid}\ngraph TD\n```"
		result := rewriteDiagramBlocks(input)
		if result != input {
			t.Errorf("already-kroki block should be unchanged\ngot:  %q\nwant: %q", result, input)
		}
	})

	t.Run("no code blocks unchanged", func(t *testing.T) {
		input := "# Title\n\nPlain text with no code blocks."
		result := rewriteDiagramBlocks(input)
		if result != input {
			t.Errorf("content without code blocks should be unchanged\ngot:  %q\nwant: %q", result, input)
		}
	})

	t.Run("closing fence not touched", func(t *testing.T) {
		input := "```mermaid\ngraph TD\n  A-->B\n```\n"
		result := rewriteDiagramBlocks(input)
		if !strings.HasSuffix(strings.TrimRight(result, "\n"), "```") {
			t.Errorf("closing fence should remain unchanged, got %q", result)
		}
	})

	t.Run("trailing whitespace on fence handled", func(t *testing.T) {
		input := "```mermaid   \ngraph TD\n```"
		result := rewriteDiagramBlocks(input)
		if !strings.Contains(result, "```kroki {type=mermaid}") {
			t.Errorf("trailing whitespace should be handled, got %q", result)
		}
	})

	t.Run("inline mermaid reference not rewritten", func(t *testing.T) {
		input := "Use ```mermaid blocks for diagrams."
		result := rewriteDiagramBlocks(input)
		if result != input {
			t.Errorf("inline reference should not be rewritten\ngot:  %q\nwant: %q", result, input)
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
