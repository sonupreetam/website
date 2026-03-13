// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanOrphanedFiles(t *testing.T) {
	dir := t.TempDir()

	staleFile := filepath.Join(dir, "content", "docs", "projects", "complyctl", "quick-start.md")
	keptFile := filepath.Join(dir, "content", "docs", "projects", "complyctl", "_index.md")
	otherFile := filepath.Join(dir, "content", "docs", "projects", "complyscribe", "_index.md")

	for _, f := range []string{staleFile, keptFile, otherFile} {
		os.MkdirAll(filepath.Dir(f), 0o755)
		os.WriteFile(f, []byte("test"), 0o644)
	}

	oldManifest := map[string]bool{
		"content/docs/projects/complyctl/_index.md":      true,
		"content/docs/projects/complyctl/quick-start.md": true,
		"content/docs/projects/complyscribe/_index.md":   true,
	}

	currentFiles := []string{
		"content/docs/projects/complyctl/_index.md",
		"content/docs/projects/complyscribe/_index.md",
	}

	removed := cleanOrphanedFiles(dir, oldManifest, currentFiles)

	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Error("stale file quick-start.md should have been removed")
	}
	if _, err := os.Stat(keptFile); err != nil {
		t.Error("kept file _index.md should still exist")
	}
	if _, err := os.Stat(otherFile); err != nil {
		t.Error("other repo file should still exist")
	}
}

func TestCleanOrphanedFiles_PrunesEmptyDirs(t *testing.T) {
	dir := t.TempDir()

	staleDir := filepath.Join(dir, "content", "docs", "projects", "removed-repo")
	staleFile := filepath.Join(staleDir, "_index.md")
	os.MkdirAll(staleDir, 0o755)
	os.WriteFile(staleFile, []byte("test"), 0o644)

	oldManifest := map[string]bool{
		"content/docs/projects/removed-repo/_index.md": true,
	}

	removed := cleanOrphanedFiles(dir, oldManifest, nil)

	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if _, err := os.Stat(staleDir); !os.IsNotExist(err) {
		t.Error("empty directory should have been pruned")
	}
}

func TestCleanOrphanedFiles_TraversalBlocked(t *testing.T) {
	dir := t.TempDir()

	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "should-survive.txt")
	os.WriteFile(outsideFile, []byte("protected"), 0o644)

	relTraversal, err := filepath.Rel(dir, outsideFile)
	if err != nil {
		t.Fatalf("could not compute relative path: %v", err)
	}

	oldManifest := map[string]bool{
		relTraversal: true,
	}

	removed := cleanOrphanedFiles(dir, oldManifest, nil)

	if removed != 0 {
		t.Errorf("removed = %d, want 0 (traversal should be blocked)", removed)
	}
	if _, err := os.Stat(outsideFile); err != nil {
		t.Errorf("file outside output dir was deleted: %v", err)
	}
}

func TestCleanOrphanedFiles_LegitimateRemoval(t *testing.T) {
	dir := t.TempDir()

	legitFile := filepath.Join(dir, "content", "docs", "projects", "old-repo", "_index.md")
	os.MkdirAll(filepath.Dir(legitFile), 0o755)
	os.WriteFile(legitFile, []byte("stale"), 0o644)

	oldManifest := map[string]bool{
		"content/docs/projects/old-repo/_index.md": true,
	}

	removed := cleanOrphanedFiles(dir, oldManifest, nil)

	if removed != 1 {
		t.Errorf("removed = %d, want 1 (legitimate orphan should be cleaned)", removed)
	}
	if _, err := os.Stat(legitFile); !os.IsNotExist(err) {
		t.Error("legitimate orphan should have been removed")
	}
}

func TestCleanStaleContent_RemovesAllFiles(t *testing.T) {
	output := t.TempDir()
	projectsDir := filepath.Join(output, "content", "docs", "projects")

	staleDir := filepath.Join(projectsDir, "removed-repo")
	os.MkdirAll(staleDir, 0o755)
	os.WriteFile(filepath.Join(staleDir, "_index.md"), []byte("---\ntitle: stale\n---\n"), 0o644)
	os.WriteFile(filepath.Join(staleDir, "overview.md"), []byte("stale overview"), 0o644)
	subDir := filepath.Join(staleDir, "docs")
	os.MkdirAll(subDir, 0o755)
	os.WriteFile(filepath.Join(subDir, "guide.md"), []byte("stale guide"), 0o644)

	activeDir := filepath.Join(projectsDir, "active-repo")
	os.MkdirAll(activeDir, 0o755)
	os.WriteFile(filepath.Join(activeDir, "_index.md"), []byte("---\ntitle: active\n---\n"), 0o644)

	activeRepos := map[string]bool{"active-repo": true}
	err := cleanStaleContent(output, activeRepos)
	if err != nil {
		t.Fatalf("cleanStaleContent failed: %v", err)
	}

	if _, err := os.Stat(staleDir); !os.IsNotExist(err) {
		t.Error("stale repo directory should have been completely removed")
	}
	if _, err := os.Stat(filepath.Join(staleDir, "overview.md")); !os.IsNotExist(err) {
		t.Error("stale overview.md should have been removed")
	}
	if _, err := os.Stat(filepath.Join(subDir, "guide.md")); !os.IsNotExist(err) {
		t.Error("stale doc sub-page should have been removed")
	}

	if _, err := os.Stat(filepath.Join(activeDir, "_index.md")); err != nil {
		t.Error("active repo _index.md should be preserved")
	}
}
