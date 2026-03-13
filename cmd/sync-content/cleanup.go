// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// cleanStaleContent removes all generated content for repos that are no longer
// active. Uses os.RemoveAll to remove the entire repo directory, including
// _index.md, overview.md, and any doc sub-pages.
func cleanStaleContent(outputDir string, activeRepos map[string]bool) error {
	projectsDir := filepath.Join(outputDir, "content", "docs", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading projects dir: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		repoName := e.Name()
		if activeRepos[repoName] {
			continue
		}
		repoDir := filepath.Join(projectsDir, repoName)
		if !isUnderDir(outputDir, repoDir) {
			slog.Warn("skipping stale repo dir outside output directory", "repo", repoName, "path", repoDir)
			continue
		}
		if err := os.RemoveAll(repoDir); err != nil {
			return fmt.Errorf("removing stale repo dir %s: %w", repoDir, err)
		}
		slog.Info("removed stale repo directory", "repo", repoName)
	}

	return nil
}

// cleanOrphanedFiles removes files present in the old manifest but absent from
// the current sync run. After each removal it prunes empty parent directories
// up to outputDir.
func cleanOrphanedFiles(outputDir string, oldManifest map[string]bool, currentFiles []string) int {
	current := make(map[string]bool, len(currentFiles))
	for _, f := range currentFiles {
		current[f] = true
	}
	removed := 0
	for relPath := range oldManifest {
		if current[relPath] {
			continue
		}
		fullPath := filepath.Join(outputDir, relPath)
		if !isUnderDir(outputDir, fullPath) {
			slog.Warn("skipping orphaned file outside output dir", "path", relPath)
			continue
		}
		if err := os.Remove(fullPath); err != nil {
			if !os.IsNotExist(err) {
				slog.Warn("could not remove orphaned file", "path", fullPath, "error", err)
			}
			continue
		}
		slog.Info("removed orphaned file", "path", relPath)
		removed++
		dir := filepath.Dir(fullPath)
		absOutput := filepath.Clean(outputDir)
		for dir != absOutput && dir != "." && dir != "/" {
			if !isUnderDir(outputDir, dir) {
				break
			}
			if err := os.Remove(dir); err != nil {
				break
			}
			slog.Info("removed empty directory", "path", dir)
			dir = filepath.Dir(dir)
		}
	}
	return removed
}
