// SPDX-License-Identifier: Apache-2.0
package main

import (
	"testing"
)

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
