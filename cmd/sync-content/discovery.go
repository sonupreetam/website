// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
)

// DiscoveryResult holds the output of an org-wide discovery scan.
type DiscoveryResult struct {
	NewRepos     []string
	NewFiles     map[string][]string // repo → untracked .md files
	ScannedOrg   string
	TotalRepos   int
	TrackedRepos int
}

// runDiscovery scans the org for repos and doc files not yet in the config.
// Reuses the existing apiClient and listOrgRepos/listDirMD methods.
func runDiscovery(ctx context.Context, gh *apiClient, org string, cfg *SyncConfig, excludeSet map[string]bool) (*DiscoveryResult, error) {
	result := &DiscoveryResult{
		NewFiles:   make(map[string][]string),
		ScannedOrg: org,
	}

	ignoreRepoSet := make(map[string]bool)
	ignoreFileSet := make(map[string]bool)
	var scanPaths []string
	if cfg != nil {
		for _, r := range cfg.Discovery.IgnoreRepos {
			ignoreRepoSet[r] = true
		}
		for _, f := range cfg.Discovery.IgnoreFiles {
			ignoreFileSet[f] = true
		}
		scanPaths = cfg.Discovery.ScanPaths
	}
	for name := range excludeSet {
		ignoreRepoSet[name] = true
	}

	trackedRepoSet := make(map[string]bool)
	trackedFileSet := make(map[string]bool)
	if cfg != nil {
		for _, src := range cfg.Sources {
			trackedRepoSet[src.Repo] = true
			for _, f := range src.Files {
				trackedFileSet[src.Repo+"::"+f.Src] = true
			}
		}
	}

	slog.Info("discovering repos", "org", org)
	repos, err := gh.listOrgRepos(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("list org repos: %w", err)
	}

	result.TotalRepos = len(repos)
	result.TrackedRepos = len(trackedRepoSet)

	for _, repo := range repos {
		if repo.Archived || repo.Fork {
			continue
		}
		fullName := repo.FullName
		if ignoreRepoSet[repo.Name] || ignoreRepoSet[fullName] {
			continue
		}
		if !trackedRepoSet[fullName] {
			result.NewRepos = append(result.NewRepos, fullName)
			continue
		}
		if !trackedFileSet[fullName+"::README.md"] && !ignoreFileSet["README.md"] {
			if _, _, err := gh.getREADME(ctx, org, repo.Name, ""); err == nil {
				result.NewFiles[fullName] = append(result.NewFiles[fullName], "README.md")
			}
		}
		if len(scanPaths) == 0 {
			continue
		}
		for _, scanPath := range scanPaths {
			files, err := gh.listDirMD(ctx, org, repo.Name, scanPath, "")
			if err != nil {
				slog.Warn("discovery scan failed", "repo", fullName, "path", scanPath, "error", err)
				continue
			}
			for _, filePath := range files {
				baseName := filepath.Base(filePath)
				if ignoreFileSet[baseName] || trackedFileSet[fullName+"::"+filePath] {
					continue
				}
				result.NewFiles[fullName] = append(result.NewFiles[fullName], filePath)
			}
		}
	}

	sort.Strings(result.NewRepos)
	return result, nil
}

// printDiscoveryReport outputs the discovery results to stderr and optionally
// writes a GITHUB_STEP_SUMMARY.
func printDiscoveryReport(result *DiscoveryResult) {
	fmt.Fprintf(os.Stderr, "\n== Discovery Report (%s) ==\n", result.ScannedOrg)
	fmt.Fprintf(os.Stderr, "  org repos:     %d\n", result.TotalRepos)
	fmt.Fprintf(os.Stderr, "  tracked repos: %d\n", result.TrackedRepos)

	if len(result.NewRepos) > 0 {
		fmt.Fprintf(os.Stderr, "\nNew repos not in config (%d):\n", len(result.NewRepos))
		for _, repo := range result.NewRepos {
			fmt.Fprintf(os.Stderr, "  - %s\n", repo)
		}
	} else {
		fmt.Fprintln(os.Stderr, "\nNo new repos found.")
	}

	totalNewFiles := 0
	for _, files := range result.NewFiles {
		totalNewFiles += len(files)
	}
	if totalNewFiles > 0 {
		fmt.Fprintf(os.Stderr, "\nNew doc files not in config (%d):\n", totalNewFiles)
		sortedRepos := make([]string, 0, len(result.NewFiles))
		for repo := range result.NewFiles {
			sortedRepos = append(sortedRepos, repo)
		}
		sort.Strings(sortedRepos)
		for _, repo := range sortedRepos {
			for _, f := range result.NewFiles[repo] {
				fmt.Fprintf(os.Stderr, "  - %s: %s\n", repo, f)
			}
		}
	} else {
		fmt.Fprintln(os.Stderr, "No new doc files found in tracked repos.")
	}

	if summaryPath := os.Getenv("GITHUB_STEP_SUMMARY"); summaryPath != "" {
		f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
		if err == nil {
			defer f.Close()
			fmt.Fprintln(f, "## Content Discovery Report")
			fmt.Fprintf(f, "| Metric | Count |\n|---|---|\n")
			fmt.Fprintf(f, "| Org repos | %d |\n", result.TotalRepos)
			fmt.Fprintf(f, "| Tracked repos | %d |\n", result.TrackedRepos)
			fmt.Fprintf(f, "| New repos found | %d |\n", len(result.NewRepos))
			fmt.Fprintf(f, "| New doc files found | %d |\n\n", totalNewFiles)
			if len(result.NewRepos) > 0 {
				fmt.Fprintln(f, "### New Repositories")
				for _, repo := range result.NewRepos {
					fmt.Fprintf(f, "- [`%s`](https://github.com/%s)\n", repo, repo)
				}
				fmt.Fprintln(f)
			}
			if totalNewFiles > 0 {
				fmt.Fprintln(f, "### New Documentation Files")
				sortedRepos := make([]string, 0, len(result.NewFiles))
				for repo := range result.NewFiles {
					sortedRepos = append(sortedRepos, repo)
				}
				sort.Strings(sortedRepos)
				for _, repo := range sortedRepos {
					for _, file := range result.NewFiles[repo] {
						fmt.Fprintf(f, "- `%s`: [`%s`](https://github.com/%s/blob/main/%s)\n", repo, file, repo, file)
					}
				}
				fmt.Fprintln(f)
			}
			if len(result.NewRepos) == 0 && totalNewFiles == 0 {
				fmt.Fprintln(f, "All repos and doc files are tracked. Nothing new to add.")
			}
		}
	}

	if ghOutput := os.Getenv("GITHUB_OUTPUT"); ghOutput != "" {
		f, err := os.OpenFile(ghOutput, os.O_APPEND|os.O_WRONLY, 0o644)
		if err == nil {
			defer f.Close()
			fmt.Fprintf(f, "new_repos=%d\n", len(result.NewRepos))
			fmt.Fprintf(f, "new_files=%d\n", totalNewFiles)
			hasDiscoveries := "false"
			if len(result.NewRepos) > 0 || totalNewFiles > 0 {
				hasDiscoveries = "true"
			}
			fmt.Fprintf(f, "has_discoveries=%s\n", hasDiscoveries)
		}
	}
}
