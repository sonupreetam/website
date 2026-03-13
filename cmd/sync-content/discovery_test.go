// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunDiscovery(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/orgs/testorg/repos", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]Repo{
			{Name: "tracked-repo", FullName: "testorg/tracked-repo", DefaultBranch: "main"},
			{Name: "new-repo", FullName: "testorg/new-repo", DefaultBranch: "main"},
			{Name: "archived-repo", FullName: "testorg/archived-repo", Archived: true},
		})
	})

	mux.HandleFunc("/repos/testorg/tracked-repo/readme", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# tracked-repo\n\nA repo."),
			Encoding: "base64",
		})
	})

	mux.HandleFunc("/repos/testorg/tracked-repo/contents/docs", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]DirEntry{
			{Name: "guide.md", Path: "docs/guide.md", Type: "file"},
			{Name: "tracked.md", Path: "docs/tracked.md", Type: "file"},
			{Name: "CHANGELOG.md", Path: "docs/CHANGELOG.md", Type: "file"},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	ctx := context.Background()

	cfg := &SyncConfig{
		Defaults: Defaults{Branch: "main"},
		Sources: []Source{
			{
				Repo:   "testorg/tracked-repo",
				Branch: "main",
				Files: []FileSpec{
					{Src: "docs/tracked.md", Dest: "content/tracked.md"},
				},
			},
		},
		Discovery: Discovery{
			ScanPaths:   []string{"docs"},
			IgnoreFiles: []string{"CHANGELOG.md"},
		},
	}

	result, err := runDiscovery(ctx, gh, "testorg", cfg, nil)
	if err != nil {
		t.Fatalf("runDiscovery: %v", err)
	}

	if len(result.NewRepos) != 1 || result.NewRepos[0] != "testorg/new-repo" {
		t.Errorf("NewRepos = %v, want [testorg/new-repo]", result.NewRepos)
	}

	newFiles := result.NewFiles["testorg/tracked-repo"]
	wantFiles := map[string]bool{"README.md": true, "docs/guide.md": true}
	gotFiles := make(map[string]bool)
	for _, f := range newFiles {
		gotFiles[f] = true
	}
	if len(gotFiles) != len(wantFiles) {
		t.Errorf("NewFiles[tracked-repo] = %v, want %v", newFiles, wantFiles)
	}
	for w := range wantFiles {
		if !gotFiles[w] {
			t.Errorf("missing expected new file %q in tracked-repo", w)
		}
	}

	if result.TotalRepos != 3 {
		t.Errorf("TotalRepos = %d, want 3", result.TotalRepos)
	}
	if result.TrackedRepos != 1 {
		t.Errorf("TrackedRepos = %d, want 1", result.TrackedRepos)
	}
}
