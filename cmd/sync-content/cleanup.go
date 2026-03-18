// SPDX-License-Identifier: Apache-2.0

package main

import (
	"log/slog"
	"os"
	"path/filepath"
)

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
