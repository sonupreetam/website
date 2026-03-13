// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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

	files, err := gh.listDirMD(ctx, "org", "repo", "docs", "")
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

func TestListDirMD_DepthLimit(t *testing.T) {
	callCount := 0

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/contents/", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode([]DirEntry{
			{Name: "file.md", Path: r.URL.Path[len("/repos/org/repo/contents/"):] + "/file.md", Type: "file"},
			{Name: "deeper", Path: r.URL.Path[len("/repos/org/repo/contents/"):] + "/deeper", Type: "dir"},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	ctx := context.Background()

	files, err := gh.listDirMD(ctx, "org", "repo", "docs", "")
	if err != nil {
		t.Fatalf("listDirMD: %v", err)
	}

	if callCount > maxDirDepth+1 {
		t.Errorf("API calls = %d, expected at most %d (depth limit should cap recursion)", callCount, maxDirDepth+1)
	}

	if len(files) == 0 {
		t.Error("expected at least some .md files to be found")
	}
	if len(files) > maxDirDepth+1 {
		t.Errorf("found %d files, expected at most %d", len(files), maxDirDepth+1)
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

func TestAppendRef(t *testing.T) {
	tests := []struct {
		url  string
		ref  string
		want string
	}{
		{"https://api.github.com/repos/o/r/readme", "", "https://api.github.com/repos/o/r/readme"},
		{"https://api.github.com/repos/o/r/readme", "abc123", "https://api.github.com/repos/o/r/readme?ref=abc123"},
		{"https://api.github.com/repos/o/r/contents/docs?per_page=100", "def456", "https://api.github.com/repos/o/r/contents/docs?per_page=100&ref=def456"},
	}
	for _, tt := range tests {
		got := appendRef(tt.url, tt.ref)
		if got != tt.want {
			t.Errorf("appendRef(%q, %q) = %q, want %q", tt.url, tt.ref, got, tt.want)
		}
	}
}

func TestGetREADME_WithRef(t *testing.T) {
	var receivedRef string

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/readme", func(w http.ResponseWriter, r *http.Request) {
		receivedRef = r.URL.Query().Get("ref")
		json.NewEncoder(w).Encode(FileResponse{
			Content:  "VEVTVA==",
			Encoding: "base64",
			SHA:      "sha123",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	ctx := context.Background()

	_, _, err := gh.getREADME(ctx, "org", "repo", "locked-sha-abc")
	if err != nil {
		t.Fatalf("getREADME: %v", err)
	}
	if receivedRef != "locked-sha-abc" {
		t.Errorf("ref = %q, want %q", receivedRef, "locked-sha-abc")
	}

	receivedRef = ""
	_, _, err = gh.getREADME(ctx, "org", "repo", "")
	if err != nil {
		t.Fatalf("getREADME (no ref): %v", err)
	}
	if receivedRef != "" {
		t.Errorf("ref should be empty when not provided, got %q", receivedRef)
	}
}

func TestListDirMD_WithRef(t *testing.T) {
	var receivedRef string

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/contents/docs", func(w http.ResponseWriter, r *http.Request) {
		receivedRef = r.URL.Query().Get("ref")
		json.NewEncoder(w).Encode([]DirEntry{
			{Name: "guide.md", Path: "docs/guide.md", Type: "file"},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	gh := newTestClient(server.URL)
	ctx := context.Background()

	_, err := gh.listDirMD(ctx, "org", "repo", "docs", "pinned-sha")
	if err != nil {
		t.Fatalf("listDirMD: %v", err)
	}
	if receivedRef != "pinned-sha" {
		t.Errorf("ref = %q, want %q", receivedRef, "pinned-sha")
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
