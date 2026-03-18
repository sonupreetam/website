// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ProjectCard is the structure written to data/projects.json for landing page templates.
type ProjectCard struct {
	Name        string `json:"name"`
	Language    string `json:"language"`
	Type        string `json:"type"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Repo        string `json:"repo"`
	Stars       int    `json:"stars"`
}

// deriveProjectType infers a human-readable type label from repo topics and description.
func deriveProjectType(r Repo) string {
	topics := make(map[string]bool, len(r.Topics))
	for _, t := range r.Topics {
		topics[strings.ToLower(t)] = true
	}
	desc := strings.ToLower(r.Description)

	switch {
	case topics["cli"] || strings.Contains(desc, "command-line") || strings.Contains(desc, " cli"):
		return "CLI Tool"
	case topics["automation"] || strings.Contains(desc, "automation") || strings.Contains(desc, "automat"):
		return "Automation"
	case topics["observability"] || strings.Contains(desc, "observability") || strings.Contains(desc, "collector"):
		return "Observability"
	case topics["framework"] || strings.Contains(desc, "framework") || strings.Contains(desc, "bridging"):
		return "Framework"
	default:
		return "Library"
	}
}

// buildSectionIndex generates a lightweight Hugo section index (_index.md) for a
// project. Contains only frontmatter metadata so the Doks sidebar renders the
// section heading as a collapsible toggle with child pages listed underneath.
func buildSectionIndex(repo Repo, sha, readmeSHA string) string {
	lang := languageOrDefault(repo.Language)
	title := formatRepoTitle(repo.Name)

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %q\n", title)
	fmt.Fprintf(&b, "linkTitle: %q\n", repo.Name)
	fmt.Fprintf(&b, "description: %q\n", repo.Description)
	fmt.Fprintf(&b, "date: %s\n", repo.PushedAt)
	fmt.Fprintf(&b, "lastmod: %s\n", repo.PushedAt)
	b.WriteString("draft: false\n")
	b.WriteString("toc: false\n")
	b.WriteString("params:\n")
	fmt.Fprintf(&b, "  language: %q\n", lang)
	fmt.Fprintf(&b, "  stars: %d\n", repo.StargazersCount)
	fmt.Fprintf(&b, "  repo: %q\n", repo.HTMLURL)
	fmt.Fprintf(&b, "  source_sha: %q\n", sha)
	fmt.Fprintf(&b, "  readme_sha: %q\n", readmeSHA)
	b.WriteString("  seo:\n")
	fmt.Fprintf(&b, "    title: %q\n", title+" | ComplyTime")
	fmt.Fprintf(&b, "    description: %q\n", repo.Description)
	b.WriteString("---\n")

	return b.String()
}

// buildOverviewPage generates the README content as a child page (overview.md)
// so it appears as a navigable sidebar link in the Doks theme.
func buildOverviewPage(repo Repo, readme string) string {
	editURL := fmt.Sprintf("https://github.com/%s/edit/%s/README.md", repo.FullName, repo.DefaultBranch)

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %q\n", "Overview")
	fmt.Fprintf(&b, "description: %q\n", repo.Description)
	fmt.Fprintf(&b, "date: %s\n", repo.PushedAt)
	fmt.Fprintf(&b, "lastmod: %s\n", repo.PushedAt)
	b.WriteString("draft: false\n")
	b.WriteString("toc: true\n")
	fmt.Fprintf(&b, "weight: %d\n", 1)
	b.WriteString("params:\n")
	fmt.Fprintf(&b, "  editURL: %q\n", editURL)
	b.WriteString("---\n\n")
	b.WriteString(readme)

	return b.String()
}

// knownAcronyms maps lowercase tokens to their canonical uppercase form.
// Used by smartTitle to preserve intended casing for common technical terms.
var knownAcronyms = map[string]string{
	"api":    "API",
	"apis":   "APIs",
	"cac":    "CAC",
	"ci":     "CI",
	"cd":     "CD",
	"cli":    "CLI",
	"cpu":    "CPU",
	"css":    "CSS",
	"dns":    "DNS",
	"faq":    "FAQ",
	"grpc":   "gRPC",
	"html":   "HTML",
	"http":   "HTTP",
	"https":  "HTTPS",
	"id":     "ID",
	"io":     "I/O",
	"ip":     "IP",
	"json":   "JSON",
	"jwt":    "JWT",
	"k8s":    "K8s",
	"oauth":  "OAuth",
	"openid": "OpenID",
	"oscal":  "OSCAL",
	"rbac":   "RBAC",
	"rest":   "REST",
	"sdk":    "SDK",
	"sql":    "SQL",
	"ssh":    "SSH",
	"sso":    "SSO",
	"tcp":    "TCP",
	"tls":    "TLS",
	"toml":   "TOML",
	"ui":     "UI",
	"uri":    "URI",
	"url":    "URL",
	"uuid":   "UUID",
	"vm":     "VM",
	"xml":    "XML",
	"yaml":   "YAML",
}

// smartTitle capitalises the first letter of each word, but preserves
// canonical casing for known acronyms (e.g. "api" → "API", "cac" → "CAC").
func smartTitle(words []string) string {
	for i, w := range words {
		if canonical, ok := knownAcronyms[strings.ToLower(w)]; ok {
			words[i] = canonical
			continue
		}
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// formatRepoTitle converts a GitHub repo name (typically lowercase/kebab-case)
// into a human-readable title for Hugo frontmatter.
// E.g. "complyctl" → "Complyctl", "oscal-sdk" → "OSCAL SDK".
func formatRepoTitle(repoName string) string {
	words := strings.FieldsFunc(repoName, func(r rune) bool {
		return r == '-' || r == '_'
	})
	return smartTitle(words)
}

// titleFromFilename converts a Markdown filename stem to a human-readable title.
// E.g. "quick-start" → "Quick Start", "sync_cac_content" → "Sync CAC Content".
func titleFromFilename(name string) string {
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.NewReplacer("-", " ", "_", " ").Replace(name)
	words := strings.Fields(name)
	return smartTitle(words)
}

// buildDocPage generates a Hugo doc page with auto-generated frontmatter
// derived from the file path. The title comes from the filename, the
// description combines the repo description with the title, and a provenance
// comment is inserted after the frontmatter closing delimiter.
func buildDocPage(filePath, repoFullName, repoDescription, pushedAt, branch, sha, content string) string {
	title := titleFromFilename(filepath.Base(filePath))

	shortSHA := sha
	if len(shortSHA) > 12 {
		shortSHA = shortSHA[:12]
	}

	editURL := fmt.Sprintf("https://github.com/%s/edit/%s/%s", repoFullName, branch, filePath)

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %q\n", title)
	fmt.Fprintf(&b, "description: %q\n", repoDescription+" — "+title)
	fmt.Fprintf(&b, "date: %s\n", pushedAt)
	fmt.Fprintf(&b, "lastmod: %s\n", pushedAt)
	b.WriteString("draft: false\n")
	fmt.Fprintf(&b, "weight: %d\n", 10)
	b.WriteString("params:\n")
	fmt.Fprintf(&b, "  editURL: %q\n", editURL)
	b.WriteString("---\n")
	fmt.Fprintf(&b, "<!-- synced from %s/%s@%s (%s) -->\n\n", repoFullName, filePath, branch, shortSHA)
	b.WriteString(content)

	return b.String()
}

// buildProjectCard constructs a ProjectCard from repo metadata.
func buildProjectCard(repo Repo) ProjectCard {
	return ProjectCard{
		Name:        repo.Name,
		Language:    languageOrDefault(repo.Language),
		Type:        deriveProjectType(repo),
		Description: repo.Description,
		URL:         fmt.Sprintf("/docs/projects/%s/", repo.Name),
		Repo:        repo.HTMLURL,
		Stars:       repo.StargazersCount,
	}
}
