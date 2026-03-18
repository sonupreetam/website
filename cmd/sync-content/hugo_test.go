// SPDX-License-Identifier: Apache-2.0
package main

import (
	"strings"
	"testing"
)

func TestFormatRepoTitle(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"complyctl", "Complyctl"},
		{"oscal-sdk", "OSCAL SDK"},
		{"cac-content-sync", "CAC Content Sync"},
		{"my-cli-tool", "My CLI Tool"},
		{"rest-api-server", "REST API Server"},
		{"simple", "Simple"},
		{"json-yaml-converter", "JSON YAML Converter"},
		{"k8s-operator", "K8s Operator"},
		{"oauth-grpc-bridge", "OAuth gRPC Bridge"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := formatRepoTitle(tc.input)
			if got != tc.want {
				t.Errorf("formatRepoTitle(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestTitleFromFilename(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"quick-start.md", "Quick Start"},
		{"sync_cac_content.md", "Sync CAC Content"},
		{"api-reference.md", "API Reference"},
		{"installation.md", "Installation"},
		{"cli-usage.md", "CLI Usage"},
		{"rest-api.md", "REST API"},
		{"getting-started", "Getting Started"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := titleFromFilename(tc.input)
			if got != tc.want {
				t.Errorf("titleFromFilename(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestSmartTitle(t *testing.T) {
	cases := []struct {
		name  string
		input []string
		want  string
	}{
		{"plain words", []string{"hello", "world"}, "Hello World"},
		{"acronym api", []string{"my", "api"}, "My API"},
		{"mixed case preserved", []string{"OAuth", "setup"}, "OAuth Setup"},
		{"already uppercase acronym", []string{"CLI"}, "CLI"},
		{"h6 cap", []string{"some", "uuid", "generator"}, "Some UUID Generator"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := make([]string, len(tc.input))
			copy(input, tc.input)
			got := smartTitle(input)
			if got != tc.want {
				t.Errorf("smartTitle(%v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestBuildSectionIndex(t *testing.T) {
	repo := Repo{
		Name:            "oscal-sdk",
		FullName:        "complytime/oscal-sdk",
		Description:     "OSCAL SDK for Go",
		Language:        "Go",
		StargazersCount: 10,
		HTMLURL:         "https://github.com/complytime/oscal-sdk",
		PushedAt:        "2025-06-01T00:00:00Z",
	}

	result := buildSectionIndex(repo, "sha-branch", "sha-readme")

	if !strings.Contains(result, `title: "OSCAL SDK"`) {
		t.Error("section index title should use formatRepoTitle (OSCAL SDK)")
	}
	if !strings.Contains(result, `linkTitle: "oscal-sdk"`) {
		t.Error("section index should have linkTitle with raw repo name for sidebar")
	}
	if !strings.Contains(result, `seo:`) {
		t.Error("section index should have seo params")
	}
	if !strings.Contains(result, `title: "OSCAL SDK | ComplyTime"`) {
		t.Error("SEO title should use formatted repo title")
	}
	if !strings.Contains(result, "readme_sha:") {
		t.Error("section index should contain readme_sha")
	}
}

func TestBuildDocPage(t *testing.T) {
	content := "## Getting Started\n\nSome content here."
	result := buildDocPage(
		"docs/api-reference.md",
		"complytime/complyctl",
		"A CLI tool",
		"2025-06-01T00:00:00Z",
		"main",
		"abc123def456789",
		content,
	)

	if !strings.Contains(result, `title: "API Reference"`) {
		t.Error("doc page title should use titleFromFilename with acronym handling")
	}
	if !strings.Contains(result, `description: "A CLI tool — API Reference"`) {
		t.Error("description should combine repo description with title")
	}
	if !strings.Contains(result, "<!-- synced from complytime/complyctl/docs/api-reference.md@main") {
		t.Error("provenance comment should be present")
	}
	if !strings.Contains(result, "## Getting Started") {
		t.Error("content body should be preserved")
	}
}

func TestBuildProjectCard(t *testing.T) {
	repo := Repo{
		Name:            "complyctl",
		FullName:        "complytime/complyctl",
		Description:     "A CLI tool",
		Language:        "Go",
		StargazersCount: 42,
		HTMLURL:         "https://github.com/complytime/complyctl",
		Topics:          []string{"cli"},
	}

	card := buildProjectCard(repo)
	if card.Name != "complyctl" {
		t.Errorf("Name = %q, want %q", card.Name, "complyctl")
	}
	if card.URL != "/docs/projects/complyctl/" {
		t.Errorf("URL = %q, want %q", card.URL, "/docs/projects/complyctl/")
	}
	if card.Type != "CLI Tool" {
		t.Errorf("Type = %q, want %q", card.Type, "CLI Tool")
	}
	if card.Stars != 42 {
		t.Errorf("Stars = %d, want 42", card.Stars)
	}
}
