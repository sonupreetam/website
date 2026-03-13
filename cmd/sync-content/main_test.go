// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// T015: Unit tests for pure functions
// ---------------------------------------------------------------------------

func TestLoadConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "sync-config.yaml")
		os.WriteFile(path, []byte(`
defaults:
  branch: main
sources:
  - repo: org/repo1
    files:
      - src: README.md
        dest: content/docs/projects/repo1/_index.md
`), 0o644)

		cfg, err := loadConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Defaults.Branch != "main" {
			t.Errorf("branch = %q, want %q", cfg.Defaults.Branch, "main")
		}
		if len(cfg.Sources) != 1 {
			t.Fatalf("sources count = %d, want 1", len(cfg.Sources))
		}
		if cfg.Sources[0].Repo != "org/repo1" {
			t.Errorf("repo = %q, want %q", cfg.Sources[0].Repo, "org/repo1")
		}
		if cfg.Sources[0].Branch != "main" {
			t.Errorf("source branch = %q, want %q (inherited from defaults)", cfg.Sources[0].Branch, "main")
		}
	})

	t.Run("default branch applied", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "cfg.yaml")
		os.WriteFile(path, []byte(`
sources:
  - repo: org/repo1
    files:
      - src: README.md
        dest: out/README.md
`), 0o644)

		cfg, err := loadConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Defaults.Branch != "main" {
			t.Errorf("default branch = %q, want %q", cfg.Defaults.Branch, "main")
		}
	})

	t.Run("malformed YAML", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.yaml")
		os.WriteFile(path, []byte(`{{{not yaml`), 0o644)

		_, err := loadConfig(path)
		if err == nil {
			t.Fatal("expected error for malformed YAML")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := loadConfig("/nonexistent/path.yaml")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("missing repo field", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "cfg.yaml")
		os.WriteFile(path, []byte(`
sources:
  - files:
      - src: README.md
        dest: out/README.md
`), 0o644)

		_, err := loadConfig(path)
		if err == nil {
			t.Fatal("expected error for missing repo")
		}
		if !strings.Contains(err.Error(), "missing required field 'repo'") {
			t.Errorf("error = %q, want it to mention missing repo", err)
		}
	})

	t.Run("missing src field", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "cfg.yaml")
		os.WriteFile(path, []byte(`
sources:
  - repo: org/repo1
    files:
      - dest: out/README.md
`), 0o644)

		_, err := loadConfig(path)
		if err == nil {
			t.Fatal("expected error for missing src")
		}
		if !strings.Contains(err.Error(), "missing 'src'") {
			t.Errorf("error = %q, want it to mention missing src", err)
		}
	})
}

func TestInjectFrontmatter(t *testing.T) {
	t.Run("prepend to content without frontmatter", func(t *testing.T) {
		content := []byte("# Hello World\n\nBody text.")
		fm := map[string]any{"title": "Hello", "weight": 10}
		result := string(injectFrontmatter(content, fm))

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
		result := string(injectFrontmatter(content, fm))

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
		result := string(injectFrontmatter([]byte{}, map[string]any{"title": "Test"}))
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
	t.Run("matching H1 removed", func(t *testing.T) {
		input := "# my-repo\n\nBody text"
		result := stripLeadingH1(input, "my-repo")
		if strings.Contains(result, "# my-repo") {
			t.Error("matching H1 should be removed")
		}
		if !strings.Contains(result, "Body text") {
			t.Error("body should be preserved")
		}
	})

	t.Run("case insensitive match", func(t *testing.T) {
		input := "# My-Repo\n\nBody"
		result := stripLeadingH1(input, "my-repo")
		if strings.Contains(result, "# My-Repo") {
			t.Error("case-insensitive H1 should be removed")
		}
	})

	t.Run("non-matching H1 preserved", func(t *testing.T) {
		input := "# Different Title\n\nBody"
		result := stripLeadingH1(input, "my-repo")
		if !strings.Contains(result, "# Different Title") {
			t.Error("non-matching H1 should be preserved")
		}
	})

	t.Run("no H1", func(t *testing.T) {
		input := "Body text without heading"
		result := stripLeadingH1(input, "my-repo")
		if result != input {
			t.Error("content without H1 should be unchanged")
		}
	})

	t.Run("H1 only line", func(t *testing.T) {
		input := "# my-repo"
		result := stripLeadingH1(input, "my-repo")
		if result != "" {
			t.Errorf("H1-only content should return empty string, got %q", result)
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

func TestIsValidRepoName(t *testing.T) {
	valid := []string{"my-repo", "repo123", "a", "repo.name"}
	for _, name := range valid {
		if !isValidRepoName(name) {
			t.Errorf("isValidRepoName(%q) = false, want true", name)
		}
	}

	invalid := []string{"", ".", "..", "path/sep", "back\\slash", "dotdot..name"}
	for _, name := range invalid {
		if isValidRepoName(name) {
			t.Errorf("isValidRepoName(%q) = true, want false", name)
		}
	}
}

// ---------------------------------------------------------------------------
// T016: Integration tests with httptest mock
// ---------------------------------------------------------------------------

// urlRewriter intercepts HTTP requests and redirects them to the test server,
// allowing the apiClient to use its hardcoded githubAPI constant while actually
// hitting the mock server.
type urlRewriter struct {
	targetHost string
	targetPort string
}

func (r *urlRewriter) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = r.targetHost + ":" + r.targetPort
	return http.DefaultTransport.RoundTrip(req)
}

func newTestClient(serverURL string) *apiClient {
	parts := strings.TrimPrefix(serverURL, "http://")
	hostPort := strings.SplitN(parts, ":", 2)
	host, port := hostPort[0], hostPort[1]

	return &apiClient{
		token: "test-token",
		http: &http.Client{
			Transport: &urlRewriter{targetHost: host, targetPort: port},
		},
	}
}

func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func TestProcessRepo(t *testing.T) {
	readmeContent := "# test-repo\n\nThis is a test README."
	branchSHA := "abc123def456"

	mux := http.NewServeMux()

	mux.HandleFunc("/repos/testorg/test-repo/readme", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(FileResponse{
			Content:  b64(readmeContent),
			Encoding: "base64",
			SHA:      "sha-readme",
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/branches/main", func(w http.ResponseWriter, r *http.Request) {
		resp := BranchResponse{}
		resp.Commit.SHA = branchSHA
		json.NewEncoder(w).Encode(resp)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	ctx := context.Background()

	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "A test repository",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
	}

	result := &syncResult{}
	oldState := map[string]repoState{}

	work := processRepo(ctx, gh, "testorg", output, repo, true, false, result, oldState, nil)

	if work == nil {
		t.Fatal("processRepo returned nil")
	}
	if work.card.Name != "test-repo" {
		t.Errorf("card.Name = %q, want %q", work.card.Name, "test-repo")
	}
	if work.card.Language != "Go" {
		t.Errorf("card.Language = %q, want %q", work.card.Language, "Go")
	}
	if work.card.Description != "A test repository" {
		t.Errorf("card.Description = %q, want %q", work.card.Description, "A test repository")
	}

	indexPath := filepath.Join(output, "content", "docs", "projects", "test-repo", "_index.md")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("section index not written: %v", err)
	}
	index := string(data)
	if !strings.Contains(index, "title:") {
		t.Error("section index should contain frontmatter title")
	}
	if !strings.Contains(index, "readme_sha:") {
		t.Error("section index should contain readme_sha in frontmatter")
	}
	if !strings.Contains(index, "sha-readme") {
		t.Error("section index should contain the README blob SHA value")
	}
	if strings.Contains(index, "This is a test README.") {
		t.Error("section index should be frontmatter-only, no README body")
	}

	overviewPath := filepath.Join(output, "content", "docs", "projects", "test-repo", "overview.md")
	overviewData, err := os.ReadFile(overviewPath)
	if err != nil {
		t.Fatalf("overview page not written: %v", err)
	}
	overview := string(overviewData)
	if !strings.Contains(overview, "This is a test README.") {
		t.Error("overview page should contain README body")
	}
	if strings.Contains(overview, "# test-repo") {
		t.Error("leading H1 matching repo name should be stripped from overview")
	}
	if !strings.Contains(overview, `title: "Overview"`) {
		t.Error("overview page should have title 'Overview'")
	}
	if work.unchanged {
		t.Error("unchanged should be false for new repos")
	}
}

func TestProcessRepo_BranchUnchanged(t *testing.T) {
	branchSHA := "abc123def456"
	readmeCalls := 0

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/testorg/test-repo/branches/main", func(w http.ResponseWriter, r *http.Request) {
		resp := BranchResponse{}
		resp.Commit.SHA = branchSHA
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/repos/testorg/test-repo/readme", func(w http.ResponseWriter, r *http.Request) {
		readmeCalls++
		json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# test-repo\n\nContent"),
			Encoding: "base64",
			SHA:      "sha-readme",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	ctx := context.Background()

	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "A test repository",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
	}

	oldState := map[string]repoState{
		"test-repo": {branchSHA: branchSHA, readmeSHA: "sha-readme"},
	}
	oldManifest := map[string]bool{
		"content/docs/projects/test-repo/_index.md": true,
	}

	result := &syncResult{}
	work := processRepo(ctx, gh, "testorg", output, repo, true, false, result, oldState, oldManifest)

	if work == nil {
		t.Fatal("processRepo returned nil for unchanged repo in write mode")
	}
	if work.card.Name != "test-repo" {
		t.Errorf("card.Name = %q, want %q", work.card.Name, "test-repo")
	}
	if readmeCalls != 0 {
		t.Errorf("README was fetched %d times, want 0 (fast path should skip)", readmeCalls)
	}
	if !work.unchanged {
		t.Error("unchanged should be true when branch SHA matches")
	}
	if len(result.unchanged) != 1 || result.unchanged[0] != "test-repo" {
		t.Errorf("unchanged = %v, want [test-repo]", result.unchanged)
	}
	if len(result.writtenFiles) != 1 {
		t.Errorf("writtenFiles = %d, want 1 (carried forward from manifest)", len(result.writtenFiles))
	}
}

func TestProcessRepo_BranchChangedReadmeUnchanged(t *testing.T) {
	readmeContent := "# test-repo\n\nThis is a test README."
	readmeSHA := "sha-readme-stable"

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/testorg/test-repo/readme", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(FileResponse{
			Content:  b64(readmeContent),
			Encoding: "base64",
			SHA:      readmeSHA,
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/branches/main", func(w http.ResponseWriter, r *http.Request) {
		resp := BranchResponse{}
		resp.Commit.SHA = "new-branch-sha"
		json.NewEncoder(w).Encode(resp)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	ctx := context.Background()

	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "A test repository",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
	}

	oldState := map[string]repoState{
		"test-repo": {branchSHA: "old-branch-sha", readmeSHA: readmeSHA},
	}

	result := &syncResult{}
	work := processRepo(ctx, gh, "testorg", output, repo, true, false, result, oldState, nil)

	if work == nil {
		t.Fatal("processRepo returned nil")
	}
	if len(result.unchanged) != 1 || result.unchanged[0] != "test-repo" {
		t.Errorf("repo should be classified as unchanged when README SHA matches, got unchanged=%v updated=%v", result.unchanged, result.updated)
	}
}

func TestSyncConfigSource(t *testing.T) {
	fileContent := "[![badge](https://img.svg)](https://ci)\n\n# complyctl\n\nSome [link](docs/guide.md) here."

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/complyctl/contents/README.md", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(FileResponse{
			Content:  b64(fileContent),
			Encoding: "base64",
			SHA:      "sha-file",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	ctx := context.Background()

	src := Source{
		Repo:   "org/complyctl",
		Branch: "main",
		Files: []FileSpec{
			{
				Src:  "README.md",
				Dest: "content/docs/projects/complyctl/_index.md",
				Transform: Transform{
					InjectFrontmatter: map[string]any{
						"title":       "complyctl",
						"description": "CLI tool",
						"weight":      10,
					},
					RewriteLinks: true,
					StripBadges:  true,
				},
			},
		},
	}

	t.Run("write mode applies transforms", func(t *testing.T) {
		result := &syncResult{}
		syncConfigSource(ctx, gh, src, Defaults{Branch: "main"}, output, true, result)

		if result.errors > 0 {
			t.Fatalf("syncConfigSource had %d errors", result.errors)
		}
		if result.synced != 1 {
			t.Errorf("synced = %d, want 1", result.synced)
		}

		destPath := filepath.Join(output, "content", "docs", "projects", "complyctl", "_index.md")
		data, err := os.ReadFile(destPath)
		if err != nil {
			t.Fatalf("config file not written: %v", err)
		}
		content := string(data)

		if !strings.Contains(content, "title: complyctl") {
			t.Error("injected frontmatter should contain title")
		}
		if strings.Contains(content, "[![badge") {
			t.Error("badges should be stripped")
		}
		if strings.Contains(content, "](docs/guide.md)") {
			t.Error("relative links should be rewritten")
		}
		if !strings.Contains(content, "https://github.com/org/complyctl/blob/main/docs/guide.md") {
			t.Error("relative link should become absolute GitHub URL")
		}
	})

	t.Run("dry-run writes nothing", func(t *testing.T) {
		dryOutput := t.TempDir()
		result := &syncResult{}
		syncConfigSource(ctx, gh, src, Defaults{Branch: "main"}, dryOutput, false, result)

		if result.synced != 1 {
			t.Errorf("dry-run synced = %d, want 1", result.synced)
		}

		destPath := filepath.Join(dryOutput, "content", "docs", "projects", "complyctl", "_index.md")
		if _, err := os.Stat(destPath); !os.IsNotExist(err) {
			t.Error("dry-run should not create files")
		}
	})
}

func TestConcurrentSyncResult(t *testing.T) {
	result := &syncResult{}
	var wg sync.WaitGroup

	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result.mu.Lock()
			result.synced++
			result.mu.Unlock()
		}()
	}

	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result.mu.Lock()
			result.errors++
			result.mu.Unlock()
		}()
	}

	wg.Wait()

	if result.synced != 100 {
		t.Errorf("synced = %d, want 100", result.synced)
	}
	if result.errors != 50 {
		t.Errorf("errors = %d, want 50", result.errors)
	}
}

// ---------------------------------------------------------------------------
// Manifest-based orphan cleanup tests
// ---------------------------------------------------------------------------

func TestManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"content/docs/projects/complyctl/_index.md",
		"content/docs/projects/complyctl/quick-start.md",
	}

	if err := writeManifest(dir, files); err != nil {
		t.Fatalf("writeManifest: %v", err)
	}

	got := readManifest(dir)
	if got == nil {
		t.Fatal("readManifest returned nil")
	}
	for _, f := range files {
		if !got[f] {
			t.Errorf("manifest missing %q", f)
		}
	}
	if len(got) != len(files) {
		t.Errorf("manifest has %d entries, want %d", len(got), len(files))
	}
}

func TestReadManifest_Missing(t *testing.T) {
	dir := t.TempDir()
	got := readManifest(dir)
	if got != nil {
		t.Errorf("readManifest for missing file should return nil, got %v", got)
	}
}

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

func TestRecordFile(t *testing.T) {
	result := &syncResult{}
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			result.recordFile(fmt.Sprintf("file-%d.md", n))
		}(i)
	}
	wg.Wait()
	if len(result.writtenFiles) != 50 {
		t.Errorf("writtenFiles = %d, want 50", len(result.writtenFiles))
	}
}

// ---------------------------------------------------------------------------
// Provenance / insertAfterFrontmatter tests
// ---------------------------------------------------------------------------

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

func TestSyncConfigSourceProvenance(t *testing.T) {
	fileContent := "# complyctl\n\nSome content."
	fileSHA := "abc123def456789"

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/complyctl/contents/README.md", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(FileResponse{
			Content:  b64(fileContent),
			Encoding: "base64",
			SHA:      fileSHA,
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	ctx := context.Background()

	src := Source{
		Repo:   "org/complyctl",
		Branch: "main",
		Files: []FileSpec{
			{
				Src:  "README.md",
				Dest: "content/docs/projects/complyctl/_index.md",
				Transform: Transform{
					InjectFrontmatter: map[string]any{"title": "complyctl"},
				},
			},
		},
	}

	result := &syncResult{}
	syncConfigSource(ctx, gh, src, Defaults{Branch: "main"}, output, true, result)

	destPath := filepath.Join(output, "content", "docs", "projects", "complyctl", "_index.md")
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "<!-- synced from org/complyctl/README.md@main (abc123def456) -->") {
		t.Errorf("provenance comment missing or incorrect, got:\n%s", content)
	}
}

// ---------------------------------------------------------------------------
// Discovery tests
// ---------------------------------------------------------------------------

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

func TestListDirMD(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/org/repo/contents/docs", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]DirEntry{
			{Name: "guide.md", Path: "docs/guide.md", Type: "file"},
			{Name: "image.png", Path: "docs/image.png", Type: "file"},
			{Name: "sub", Path: "docs/sub", Type: "dir"},
		})
	})

	mux.HandleFunc("/repos/org/repo/contents/docs/sub", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]DirEntry{
			{Name: "nested.md", Path: "docs/sub/nested.md", Type: "file"},
			{Name: "data.json", Path: "docs/sub/data.json", Type: "file"},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	ctx := context.Background()

	files, err := gh.listDirMD(ctx, "org", "repo", "docs")
	if err != nil {
		t.Fatalf("listDirMD: %v", err)
	}

	want := map[string]bool{
		"docs/guide.md":      true,
		"docs/sub/nested.md": true,
	}
	got := make(map[string]bool)
	for _, f := range files {
		got[f] = true
	}

	if len(got) != len(want) {
		t.Errorf("got %d files, want %d: %v", len(got), len(want), files)
	}
	for w := range want {
		if !got[w] {
			t.Errorf("missing expected file %q", w)
		}
	}
}

// ---------------------------------------------------------------------------
// syncRepoDocPages tests
// ---------------------------------------------------------------------------

func TestSyncRepoDocPages(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/testorg/test-repo/contents/docs", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]DirEntry{
			{Name: "installation.md", Path: "docs/installation.md", Type: "file"},
			{Name: "usage.md", Path: "docs/usage.md", Type: "file"},
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/contents/docs/installation.md", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# Installation\n\nRun `go install`."),
			Encoding: "base64",
			SHA:      "sha-install",
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/contents/docs/usage.md", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# Usage\n\nRun the CLI tool."),
			Encoding: "base64",
			SHA:      "sha-usage",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	ctx := context.Background()

	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "A test repository",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
		PushedAt:      "2025-01-15T00:00:00Z",
	}

	discovery := Discovery{ScanPaths: []string{"docs"}}
	result := &syncResult{}
	syncRepoDocPages(ctx, gh, "testorg", repo, output, true, discovery, nil, nil, result)

	if result.errors != 0 {
		t.Fatalf("errors = %d, want 0", result.errors)
	}
	if result.synced != 2 {
		t.Errorf("synced = %d, want 2", result.synced)
	}

	cases := []struct {
		relPath string
		title   string
		provSrc string
	}{
		{
			relPath: "content/docs/projects/test-repo/installation.md",
			title:   "Installation",
			provSrc: "testorg/test-repo/docs/installation.md@main",
		},
		{
			relPath: "content/docs/projects/test-repo/usage.md",
			title:   "Usage",
			provSrc: "testorg/test-repo/docs/usage.md@main",
		},
	}

	for _, tc := range cases {
		fullPath := filepath.Join(output, tc.relPath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("file not written: %s: %v", tc.relPath, err)
		}
		content := string(data)

		if !strings.Contains(content, fmt.Sprintf("title: %q", tc.title)) {
			t.Errorf("%s: missing title %q in frontmatter:\n%s", tc.relPath, tc.title, content)
		}
		if !strings.Contains(content, "draft: false") {
			t.Errorf("%s: missing draft: false", tc.relPath)
		}
		if !strings.Contains(content, "weight: 10") {
			t.Errorf("%s: missing weight: 10", tc.relPath)
		}
		if !strings.Contains(content, "date: 2025-01-15T00:00:00Z") {
			t.Errorf("%s: missing or wrong date", tc.relPath)
		}
		if !strings.Contains(content, "<!-- synced from "+tc.provSrc) {
			t.Errorf("%s: missing provenance comment for %s:\n%s", tc.relPath, tc.provSrc, content)
		}
	}

	if len(result.writtenFiles) != 2 {
		t.Errorf("writtenFiles = %d, want 2", len(result.writtenFiles))
	}
}

func TestSyncRepoDocPages_SkipsConfigTracked(t *testing.T) {
	fetchedFiles := make(map[string]bool)

	mux := http.NewServeMux()

	mux.HandleFunc("/repos/testorg/test-repo/contents/docs", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]DirEntry{
			{Name: "installation.md", Path: "docs/installation.md", Type: "file"},
			{Name: "usage.md", Path: "docs/usage.md", Type: "file"},
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/contents/docs/installation.md", func(w http.ResponseWriter, r *http.Request) {
		fetchedFiles["docs/installation.md"] = true
		json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# Installation\n\nSteps."),
			Encoding: "base64",
			SHA:      "sha-install",
		})
	})
	mux.HandleFunc("/repos/testorg/test-repo/contents/docs/usage.md", func(w http.ResponseWriter, r *http.Request) {
		fetchedFiles["docs/usage.md"] = true
		json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# Usage\n\nRun it."),
			Encoding: "base64",
			SHA:      "sha-usage",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	ctx := context.Background()

	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "A test repository",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
		PushedAt:      "2025-01-15T00:00:00Z",
	}

	discovery := Discovery{ScanPaths: []string{"docs"}}
	configTracked := map[string]bool{"docs/installation.md": true}

	result := &syncResult{}
	syncRepoDocPages(ctx, gh, "testorg", repo, output, true, discovery, nil, configTracked, result)

	if fetchedFiles["docs/installation.md"] {
		t.Error("config-tracked file docs/installation.md should not have been fetched")
	}
	if !fetchedFiles["docs/usage.md"] {
		t.Error("non-tracked file docs/usage.md should have been fetched")
	}

	trackedPath := filepath.Join(output, "content", "docs", "projects", "test-repo", "installation.md")
	if _, err := os.Stat(trackedPath); !os.IsNotExist(err) {
		t.Error("config-tracked file should not have been written")
	}

	untrackedPath := filepath.Join(output, "content", "docs", "projects", "test-repo", "usage.md")
	if _, err := os.Stat(untrackedPath); err != nil {
		t.Fatalf("non-tracked file should have been written: %v", err)
	}

	if result.synced != 1 {
		t.Errorf("synced = %d, want 1 (only the non-tracked file)", result.synced)
	}
}

// --- Hardening Tests (T037) ---

func TestIsUnderDir(t *testing.T) {
	base := t.TempDir()

	tests := []struct {
		name   string
		target string
		want   bool
	}{
		{"child file", filepath.Join(base, "content", "file.md"), true},
		{"same dir", base, true},
		{"traversal", filepath.Join(base, "..", "etc", "passwd"), false},
		{"double traversal", filepath.Join(base, "content", "..", "..", "etc"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUnderDir(base, tt.target)
			if got != tt.want {
				t.Errorf("isUnderDir(%q, %q) = %v, want %v", base, tt.target, got, tt.want)
			}
		})
	}
}

func TestPathTraversalRejection(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/complyctl/contents/README.md", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(FileResponse{
			Content:  b64("# README\nContent here."),
			Encoding: "base64",
			SHA:      "sha-traversal",
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()

	src := Source{
		Repo:   "org/complyctl",
		Branch: "main",
		Files: []FileSpec{
			{
				Src:  "README.md",
				Dest: "../../etc/cron.d/backdoor.md",
			},
		},
	}

	result := &syncResult{}
	syncConfigSource(context.Background(), gh, src, Defaults{Branch: "main"}, output, true, result)

	if result.errors != 1 {
		t.Errorf("expected 1 error for path traversal, got %d", result.errors)
	}

	escapedPath := filepath.Join(output, "../../etc/cron.d/backdoor.md")
	if _, err := os.Stat(escapedPath); !os.IsNotExist(err) {
		t.Error("traversal file should not have been written")
	}
}

func TestContextCancellationDuringRetry(t *testing.T) {
	callCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/test-endpoint", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"message":"rate limited"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	var result map[string]any
	err := gh.getJSON(ctx, server.URL+"/test-endpoint", &result)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if elapsed > 2*time.Second {
		t.Errorf("cancellation took %v, expected < 2s", elapsed)
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

func TestEscapePathSegments(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"docs/guide.md", "docs/guide.md"},
		{"docs/my file.md", "docs/my%20file.md"},
		{"path/with spaces/file#1.md", "path/with%20spaces/file%231.md"},
	}
	for _, tt := range tests {
		got := escapePathSegments(tt.input)
		if got != tt.want {
			t.Errorf("escapePathSegments(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDryRunReturnsCard(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/testorg/test-repo/branches/main", func(w http.ResponseWriter, r *http.Request) {
		resp := BranchResponse{}
		resp.Commit.SHA = "new-sha"
		json.NewEncoder(w).Encode(resp)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	output := t.TempDir()
	repo := Repo{
		Name:          "test-repo",
		FullName:      "testorg/test-repo",
		Description:   "Test",
		Language:      "Go",
		HTMLURL:       "https://github.com/testorg/test-repo",
		DefaultBranch: "main",
	}

	result := &syncResult{}
	work := processRepo(context.Background(), gh, "testorg", output, repo, false, false, result, map[string]repoState{}, nil)

	if work == nil {
		t.Fatal("dry-run processRepo should return non-nil repoWork")
	}
	if work.card.Name != "test-repo" {
		t.Errorf("card.Name = %q, want %q", work.card.Name, "test-repo")
	}
}
