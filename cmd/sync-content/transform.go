// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// mdLinkRe matches Markdown links and images: [text](url) and ![alt](url).
var mdLinkRe = regexp.MustCompile(`(!?\[[^\]]*\])\(([^)]+)\)`)

// badgeRe matches Markdown badge patterns at the start of a line:
// [![alt](img-url)](link-url) followed by optional whitespace/newline.
var badgeRe = regexp.MustCompile(`(?m)^\[!\[[^\]]*\]\([^\)]*\)\]\([^\)]*\)\s*\n?`)

// rewriteRelativeLinks converts relative Markdown links and images to absolute
// GitHub URLs so they work when the content is rendered on the Hugo site.
// Regular links point to /blob/ (rendered view), images point to raw.githubusercontent (raw content).
// The optional basePath resolves relative URLs from a subdirectory (e.g. "docs/")
// rather than the repo root. Pass "" for repo-root files like README.
func rewriteRelativeLinks(markdown, owner, repo, branch string, basePath ...string) string {
	prefix := ""
	if len(basePath) > 0 && basePath[0] != "" {
		prefix = strings.TrimSuffix(basePath[0], "/") + "/"
	}
	baseBlob := fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", owner, repo, branch, prefix)
	baseRaw := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, branch, prefix)

	return mdLinkRe.ReplaceAllStringFunc(markdown, func(match string) string {
		subs := mdLinkRe.FindStringSubmatch(match)
		if len(subs) < 3 {
			return match
		}
		bracket := subs[1]
		href := subs[2]

		if isAbsoluteURL(href) {
			return match
		}

		href = strings.TrimPrefix(href, "./")

		if strings.HasPrefix(bracket, "!") {
			return bracket + "(" + baseRaw + href + ")"
		}
		return bracket + "(" + baseBlob + href + ")"
	})
}

func isAbsoluteURL(href string) bool {
	return strings.HasPrefix(href, "http://") ||
		strings.HasPrefix(href, "https://") ||
		strings.HasPrefix(href, "//") ||
		strings.HasPrefix(href, "#") ||
		strings.HasPrefix(href, "mailto:")
}

// stripLeadingH1 removes the first H1 heading from the content. The title is
// already captured in frontmatter, so the leading H1 in the body is redundant.
// Only strips a line starting with exactly "# " (not "## " or deeper).
func stripLeadingH1(content string) string {
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) == 0 {
		return content
	}
	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, "# ") && !strings.HasPrefix(first, "## ") {
		if len(lines) > 1 {
			return strings.TrimLeft(lines[1], "\n")
		}
		return ""
	}
	return content
}

// headingRe matches Markdown ATX headings (# through ######) at the start of a line.
var headingRe = regexp.MustCompile(`(?m)^(#{1,6})\s`)

// headingFullRe captures the hashes and text of a Markdown ATX heading.
var headingFullRe = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)

// shiftHeadings bumps every Markdown heading down one level (H1->H2, H2->H3, ...)
// so that Hugo's page title remains the only H1. Headings already at H6 stay at H6.
func shiftHeadings(content string) string {
	return headingRe.ReplaceAllStringFunc(content, func(match string) string {
		hashes := strings.TrimRight(match, " \t")
		if len(hashes) >= 6 {
			return match
		}
		return "#" + match
	})
}

// titleCaseHeadings applies smartTitle to every Markdown heading's text,
// normalising casing for both the rendered page and Hugo's TableOfContents.
func titleCaseHeadings(content string) string {
	return headingFullRe.ReplaceAllStringFunc(content, func(match string) string {
		subs := headingFullRe.FindStringSubmatch(match)
		if len(subs) < 3 {
			return match
		}
		hashes := subs[1]
		text := strings.TrimSpace(subs[2])
		words := strings.Fields(text)
		return hashes + " " + smartTitle(words)
	})
}

// stripBadges removes Markdown badge lines from the start of content.
func stripBadges(content string) string {
	return strings.TrimLeft(badgeRe.ReplaceAllString(content, ""), "\n")
}

// injectFrontmatter prepends YAML frontmatter to the content. If the content
// already has frontmatter (starts with "---"), it is replaced.
func injectFrontmatter(content []byte, fm map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("---\n")

	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return nil, fmt.Errorf("marshaling frontmatter: %w", err)
	}
	buf.Write(fmBytes)
	buf.WriteString("---\n\n")

	body := content
	if bytes.HasPrefix(body, []byte("---\n")) {
		rest := body[4:]
		end := bytes.Index(rest, []byte("\n---\n"))
		if end != -1 {
			body = bytes.TrimLeft(rest[end+4+1:], "\n")
		} else if end2 := bytes.Index(rest, []byte("\n---")); end2 != -1 && end2+4 == len(rest) {
			body = nil
		}
	}

	buf.Write(body)
	return buf.Bytes(), nil
}

// insertAfterFrontmatter inserts extra bytes right after the closing "---"
// of YAML frontmatter. If there is no frontmatter, content is prepended.
func insertAfterFrontmatter(content, insert []byte) []byte {
	if !bytes.HasPrefix(content, []byte("---\n")) {
		return append(append(insert, '\n'), content...)
	}
	idx := bytes.Index(content[4:], []byte("\n---\n"))
	if idx == -1 {
		return append(append(insert, '\n'), content...)
	}
	insertPoint := 4 + idx + len("\n---\n")
	var buf bytes.Buffer
	buf.Write(content[:insertPoint])
	buf.Write(insert)
	buf.Write(content[insertPoint:])
	return buf.Bytes()
}
