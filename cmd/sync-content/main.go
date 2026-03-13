// SPDX-License-Identifier: Apache-2.0

// Command sync-content pulls documentation from upstream GitHub repositories
// into the website's Hugo content tree. It operates in hybrid mode:
//
//  1. Org scan — lists all non-archived, non-fork repos in the GitHub org,
//     fetches each README, and generates project pages and landing-page cards.
//  2. Config sync — reads sync-config.yaml and pulls specific files with
//     transforms (frontmatter injection, link rewriting, badge stripping).
//
// The tool is fully idempotent: unchanged files are not rewritten. A sync
// manifest (.sync-manifest.json) tracks written files so orphaned content
// from previous runs is cleaned up automatically.
//
// Usage:
//
//	go run ./cmd/sync-content --org complytime --config sync-config.yaml          # dry-run
//	go run ./cmd/sync-content --org complytime --config sync-config.yaml --write  # apply
//	go run ./cmd/sync-content --config sync-config.yaml --repo complytime/complyctl --write  # single repo
//	go run ./cmd/sync-content --org complytime --config sync-config.yaml --discover  # find new content
//
// Environment:
//
//	GITHUB_TOKEN   GitHub API token (recommended; unauthenticated rate limit is 60 req/hr)
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	githubAPI      = "https://api.github.com"
	pageSize       = 100
	maxRetries     = 3
	defaultWorkers = 5
)

// GitHub API response types

type Repo struct {
	Name            string   `json:"name"`
	FullName        string   `json:"full_name"`
	Description     string   `json:"description"`
	Language        string   `json:"language"`
	StargazersCount int      `json:"stargazers_count"`
	HTMLURL         string   `json:"html_url"`
	DefaultBranch   string   `json:"default_branch"`
	PushedAt        string   `json:"pushed_at"`
	Archived        bool     `json:"archived"`
	Fork            bool     `json:"fork"`
	Topics          []string `json:"topics"`
}

type FileResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	SHA      string `json:"sha"`
}

type DirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

type BranchResponse struct {
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

// repoState holds the SHAs read from an existing project page for two-tier
// change detection: branchSHA is a fast pre-filter, readmeSHA enables
// content-level comparison when the branch has moved.
type repoState struct {
	branchSHA string
	readmeSHA string
}

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

// SyncConfig is the top-level structure parsed from sync-config.yaml.
type SyncConfig struct {
	Defaults  Defaults  `yaml:"defaults"`
	Sources   []Source  `yaml:"sources"`
	Discovery Discovery `yaml:"discovery"`
}

// Discovery configures automatic detection of new repos and doc files
// that are not yet declared in sources.
type Discovery struct {
	IgnoreRepos []string `yaml:"ignore_repos"`
	IgnoreFiles []string `yaml:"ignore_files"`
	ScanPaths   []string `yaml:"scan_paths"`
}

// Defaults holds fallback values applied to every source unless overridden.
type Defaults struct {
	Branch string `yaml:"branch"`
}

// Source is a single upstream repository declared in the config file.
type Source struct {
	Repo        string     `yaml:"repo"`
	Branch      string     `yaml:"branch"`
	SkipOrgSync bool       `yaml:"skip_org_sync"`
	Files       []FileSpec `yaml:"files"`
}

// FileSpec describes one file to fetch from a source repo and where to place it.
type FileSpec struct {
	Src       string    `yaml:"src"`
	Dest      string    `yaml:"dest"`
	Transform Transform `yaml:"transform"`
}

// Transform describes optional mutations applied to fetched content.
type Transform struct {
	InjectFrontmatter map[string]any `yaml:"inject_frontmatter"`
	RewriteLinks      bool           `yaml:"rewrite_links"`
	StripBadges       bool           `yaml:"strip_badges"`
}

// loadConfig reads a sync-config.yaml file and returns the parsed configuration.
// It applies default values (e.g. branch) and validates that every source has
// the required fields.
func loadConfig(path string) (*SyncConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg SyncConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	if cfg.Defaults.Branch == "" {
		cfg.Defaults.Branch = "main"
	}

	for i := range cfg.Sources {
		src := &cfg.Sources[i]
		if src.Repo == "" {
			return nil, fmt.Errorf("config %s: source[%d] missing required field 'repo'", path, i)
		}
		if src.Branch == "" {
			src.Branch = cfg.Defaults.Branch
		}
		for j, f := range src.Files {
			if f.Src == "" {
				return nil, fmt.Errorf("config %s: source[%d] (%s) file[%d] missing 'src'", path, i, src.Repo, j)
			}
			if f.Dest == "" {
				return nil, fmt.Errorf("config %s: source[%d] (%s) file[%d] missing 'dest'", path, i, src.Repo, j)
			}
		}
	}

	return &cfg, nil
}

// syncResult tracks outcomes for the final summary and exit code.
type syncResult struct {
	mu           sync.Mutex
	synced       int
	skipped      int
	warnings     int
	errors       int
	added        []string
	updated      []string
	removed      []string
	unchanged    []string
	writtenFiles []string
}

// recordFile appends a relative file path to the manifest of files written
// during this sync run. Thread-safe.
func (r *syncResult) recordFile(relPath string) {
	r.mu.Lock()
	r.writtenFiles = append(r.writtenFiles, relPath)
	r.mu.Unlock()
}

func (r *syncResult) hasChanges() bool {
	return len(r.added) > 0 || len(r.updated) > 0 || len(r.removed) > 0
}

func (r *syncResult) toMarkdown() string {
	var b strings.Builder
	b.WriteString("## Content Sync Summary\n\n")

	if len(r.added) > 0 {
		b.WriteString("### New Repositories\n\n")
		for _, name := range r.added {
			fmt.Fprintf(&b, "- `%s`\n", name)
		}
		b.WriteString("\n")
	}
	if len(r.updated) > 0 {
		b.WriteString("### Updated\n\n")
		for _, name := range r.updated {
			fmt.Fprintf(&b, "- `%s`\n", name)
		}
		b.WriteString("\n")
	}
	if len(r.removed) > 0 {
		b.WriteString("### Removed\n\n")
		for _, name := range r.removed {
			fmt.Fprintf(&b, "- `%s`\n", name)
		}
		b.WriteString("\n")
	}
	if !r.hasChanges() {
		b.WriteString("No changes detected.\n\n")
	}

	fmt.Fprintf(&b, "**Stats**: %d synced, %d skipped",
		r.synced, r.skipped)
	if r.warnings > 0 {
		fmt.Fprintf(&b, ", %d warnings", r.warnings)
	}
	if r.errors > 0 {
		fmt.Fprintf(&b, ", %d errors", r.errors)
	}
	b.WriteString("\n")

	return b.String()
}

func (r *syncResult) printSummary() {
	slog.Info("sync summary",
		"synced", r.synced,
		"skipped", r.skipped,
		"warnings", r.warnings,
		"errors", r.errors,
	)
	if len(r.added) > 0 {
		slog.Info("new repos", "repos", strings.Join(r.added, ", "))
	}
	if len(r.updated) > 0 {
		slog.Info("updated repos", "repos", strings.Join(r.updated, ", "))
	}
	if len(r.removed) > 0 {
		slog.Info("removed repos", "repos", strings.Join(r.removed, ", "))
	}
	if !r.hasChanges() {
		slog.Info("no content changes detected")
	}
}

// apiClient wraps net/http for authenticated GitHub REST API calls.
type apiClient struct {
	token string
	http  *http.Client
}

func (c *apiClient) do(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.http.Do(req)
}

// getJSON fetches a URL and decodes JSON, retrying on rate limit (403/429)
// with exponential backoff and respect for Retry-After / X-RateLimit-Reset.
func (c *apiClient) getJSON(ctx context.Context, url string, dst any) error {
	var lastErr error
	for attempt := range maxRetries + 1 {
		resp, err := c.do(ctx, url)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusOK {
			err = json.NewDecoder(resp.Body).Decode(dst)
			resp.Body.Close()
			return err
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		lastErr = fmt.Errorf("GET %s: %d %s", url, resp.StatusCode, body)

		if !isRateLimited(resp) || attempt == maxRetries {
			return lastErr
		}

		wait := retryWait(resp, attempt)
		slog.Warn("rate limited, retrying", "url", url, "attempt", attempt+1, "wait", wait.Round(time.Second))
		time.Sleep(wait)
	}
	return lastErr
}

func isRateLimited(resp *http.Response) bool {
	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if resp.StatusCode == http.StatusForbidden {
		return resp.Header.Get("X-RateLimit-Remaining") == "0"
	}
	return false
}

func retryWait(resp *http.Response, attempt int) time.Duration {
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if seconds, err := strconv.Atoi(ra); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
		if ts, err := strconv.ParseInt(reset, 10, 64); err == nil {
			wait := time.Until(time.Unix(ts, 0)) + time.Second
			if wait > 0 && wait < 5*time.Minute {
				return wait
			}
			if wait >= 5*time.Minute {
				slog.Warn("rate limit reset too far in future, using backoff", "reset_in", wait.Round(time.Second))
			}
		}
	}
	return time.Duration(1<<uint(attempt)) * time.Second
}

func (c *apiClient) listOrgRepos(ctx context.Context, org string) ([]Repo, error) {
	var all []Repo
	for page := 1; ; page++ {
		url := fmt.Sprintf("%s/orgs/%s/repos?per_page=%d&page=%d&type=public",
			githubAPI, org, pageSize, page)
		var batch []Repo
		if err := c.getJSON(ctx, url, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < pageSize {
			break
		}
	}
	return all, nil
}

func (c *apiClient) getREADME(ctx context.Context, owner, repo string) (string, string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/readme", githubAPI, owner, repo)
	var f FileResponse
	if err := c.getJSON(ctx, url, &f); err != nil {
		return "", "", err
	}
	content, err := decodeContent(f)
	return content, f.SHA, err
}

func (c *apiClient) getFileContent(ctx context.Context, owner, repo, path string) (string, string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", githubAPI, owner, repo, path)
	var f FileResponse
	if err := c.getJSON(ctx, url, &f); err != nil {
		return "", "", err
	}
	content, err := decodeContent(f)
	return content, f.SHA, err
}

func (c *apiClient) listDir(ctx context.Context, owner, repo, path string) ([]DirEntry, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", githubAPI, owner, repo, path)
	var entries []DirEntry
	if err := c.getJSON(ctx, url, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (c *apiClient) getBranchSHA(ctx context.Context, owner, repo, branch string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/branches/%s", githubAPI, owner, repo, branch)
	var b BranchResponse
	if err := c.getJSON(ctx, url, &b); err != nil {
		return "", err
	}
	return b.Commit.SHA, nil
}

// listDirMD recursively lists .md files under a directory, reusing listDir.
// Returns paths relative to the repo root (e.g. "docs/guide.md").
func (c *apiClient) listDirMD(ctx context.Context, owner, repo, dir string) ([]string, error) {
	entries, err := c.listDir(ctx, owner, repo, dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		switch {
		case e.Type == "file" && strings.HasSuffix(e.Name, ".md"):
			if e.Path != "" {
				files = append(files, e.Path)
			} else {
				files = append(files, dir+"/"+e.Name)
			}
		case e.Type == "dir":
			subDir := dir + "/" + e.Name
			if e.Path != "" {
				subDir = e.Path
			}
			sub, err := c.listDirMD(ctx, owner, repo, subDir)
			if err != nil {
				slog.Warn("could not list subdir", "repo", owner+"/"+repo, "dir", subDir, "error", err)
				continue
			}
			files = append(files, sub...)
		}
	}
	return files, nil
}

func decodeContent(f FileResponse) (string, error) {
	if f.Encoding != "base64" {
		return f.Content, nil
	}
	raw := strings.NewReplacer("\n", "", "\r", "").Replace(f.Content)
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	return string(decoded), nil
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

func isValidRepoName(name string) bool {
	return name != "" &&
		name != "." && name != ".." &&
		!strings.Contains(name, "/") &&
		!strings.Contains(name, "\\") &&
		!strings.Contains(name, "..")
}

func languageOrDefault(lang string) string {
	if lang == "" {
		return "Unknown"
	}
	return lang
}

// mdLinkRe matches Markdown links and images: [text](url) and ![alt](url).
var mdLinkRe = regexp.MustCompile(`(!?\[[^\]]*\])\(([^)]+)\)`)

// badgeRe matches Markdown badge patterns at the start of a line:
// [![alt](img-url)](link-url) followed by optional whitespace/newline.
var badgeRe = regexp.MustCompile(`(?m)^\[!\[[^\]]*\]\([^\)]*\)\]\([^\)]*\)\s*\n?`)

// rewriteRelativeLinks converts relative Markdown links and images to absolute
// GitHub URLs so they work when the content is rendered on the Hugo site.
// Regular links point to /blob/ (rendered view), images point to raw.githubusercontent (raw content).
// The optional basePath resolves relative URLs from a subdirectory (e.g. "docs/")
// rather than the repo root. Pass "" for repo-root files like README.
func rewriteRelativeLinks(markdown, owner, repo, branch string, basePath ...string) string {
	prefix := ""
	if len(basePath) > 0 && basePath[0] != "" {
		prefix = strings.TrimSuffix(basePath[0], "/") + "/"
	}
	baseBlob := fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", owner, repo, branch, prefix)
	baseRaw := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, branch, prefix)

	return mdLinkRe.ReplaceAllStringFunc(markdown, func(match string) string {
		subs := mdLinkRe.FindStringSubmatch(match)
		if len(subs) < 3 {
			return match
		}
		bracket := subs[1]
		href := subs[2]

		if isAbsoluteURL(href) {
			return match
		}

		href = strings.TrimPrefix(href, "./")

		if strings.HasPrefix(bracket, "!") {
			return bracket + "(" + baseRaw + href + ")"
		}
		return bracket + "(" + baseBlob + href + ")"
	})
}

func isAbsoluteURL(href string) bool {
	return strings.HasPrefix(href, "http://") ||
		strings.HasPrefix(href, "https://") ||
		strings.HasPrefix(href, "//") ||
		strings.HasPrefix(href, "#") ||
		strings.HasPrefix(href, "mailto:")
}

// stripLeadingH1 removes the first line of a README if it's an H1 heading
// that matches the repo name, preventing duplicate titles on Hugo pages.
func stripLeadingH1(readme, repoName string) string {
	lines := strings.SplitN(readme, "\n", 2)
	if len(lines) == 0 {
		return readme
	}
	firstLine := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(firstLine, "# ") {
		return readme
	}
	heading := strings.TrimSpace(strings.TrimPrefix(firstLine, "# "))
	if !strings.EqualFold(heading, repoName) {
		return readme
	}
	if len(lines) > 1 {
		return strings.TrimLeft(lines[1], "\n")
	}
	return ""
}

// stripBadges removes Markdown badge lines from the start of content.
func stripBadges(content string) string {
	return strings.TrimLeft(badgeRe.ReplaceAllString(content, ""), "\n")
}

// injectFrontmatter prepends YAML frontmatter to the content. If the content
// already has frontmatter (starts with "---"), it is replaced.
func injectFrontmatter(content []byte, fm map[string]any) []byte {
	var buf bytes.Buffer
	buf.WriteString("---\n")

	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		slog.Warn("failed to marshal frontmatter, skipping injection", "error", err)
		return content
	}
	buf.Write(fmBytes)
	buf.WriteString("---\n\n")

	body := content
	if bytes.HasPrefix(body, []byte("---")) {
		end := bytes.Index(body[3:], []byte("\n---"))
		if end != -1 {
			body = bytes.TrimLeft(body[end+3+4:], "\n")
		}
	}

	buf.Write(body)
	return buf.Bytes()
}

// insertAfterFrontmatter inserts extra bytes right after the closing "---"
// of YAML frontmatter. If there is no frontmatter, content is prepended.
func insertAfterFrontmatter(content, insert []byte) []byte {
	if !bytes.HasPrefix(content, []byte("---\n")) {
		return append(append(insert, '\n'), content...)
	}
	idx := bytes.Index(content[4:], []byte("\n---\n"))
	if idx == -1 {
		return append(append(insert, '\n'), content...)
	}
	insertPoint := 4 + idx + len("\n---\n")
	var buf bytes.Buffer
	buf.Write(content[:insertPoint])
	buf.Write(insert)
	buf.Write(content[insertPoint:])
	return buf.Bytes()
}

// syncConfigSource processes all FileSpec entries for a single config source,
// fetching each declared file from GitHub, applying the requested transforms,
// and writing the result to the output directory.
func syncConfigSource(ctx context.Context, gh *apiClient, src Source, defaults Defaults, output string, write bool, result *syncResult) {
	branch := src.Branch
	if branch == "" {
		branch = defaults.Branch
	}

	parts := strings.SplitN(src.Repo, "/", 2)
	if len(parts) != 2 {
		slog.Error("invalid repo format in config, expected owner/name", "repo", src.Repo)
		result.mu.Lock()
		result.errors++
		result.mu.Unlock()
		return
	}
	owner, repoName := parts[0], parts[1]
	logger := slog.With("config_repo", src.Repo)

	for _, file := range src.Files {
		content, fileSHA, err := gh.getFileContent(ctx, owner, repoName, file.Src)
		if err != nil {
			logger.Error("could not fetch config file", "src", file.Src, "error", err)
			result.mu.Lock()
			result.errors++
			result.mu.Unlock()
			continue
		}

		if file.Transform.StripBadges {
			content = stripBadges(content)
		}
		if file.Transform.RewriteLinks {
			content = rewriteRelativeLinks(content, owner, repoName, branch)
		}

		out := []byte(content)
		if len(file.Transform.InjectFrontmatter) > 0 {
			out = injectFrontmatter(out, file.Transform.InjectFrontmatter)
		}

		shortSHA := fileSHA
		if len(shortSHA) > 12 {
			shortSHA = shortSHA[:12]
		}
		provenance := fmt.Sprintf(
			"<!-- synced from %s/%s@%s (%s) -->\n",
			src.Repo, file.Src, branch, shortSHA,
		)
		out = insertAfterFrontmatter(out, []byte(provenance))

		destPath := filepath.Join(output, file.Dest)

		if !write {
			logger.Info("would write config file (dry-run)", "src", file.Src, "dest", destPath)
			result.mu.Lock()
			result.synced++
			result.mu.Unlock()
			continue
		}

		written, err := writeFileSafe(destPath, out)
		if err != nil {
			logger.Error("error writing config file", "src", file.Src, "dest", destPath, "error", err)
			result.mu.Lock()
			result.errors++
			result.mu.Unlock()
			continue
		}

		result.recordFile(file.Dest)

		if written {
			logger.Info("wrote config file", "src", file.Src, "dest", destPath)
		} else {
			logger.Info("config file unchanged", "src", file.Src, "dest", destPath)
		}

		result.mu.Lock()
		result.synced++
		result.mu.Unlock()
	}
}

func parseNameList(raw string) map[string]bool {
	set := make(map[string]bool)
	for _, name := range strings.Split(raw, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			set[name] = true
		}
	}
	return set
}

// readExistingState reads source_sha and readme_sha from existing project
// pages to enable two-tier change detection: branch SHA as a fast pre-filter,
// readme SHA for content-level comparison.
func readExistingState(outputDir string) map[string]repoState {
	state := make(map[string]repoState)
	dir := filepath.Join(outputDir, "content", "docs", "projects")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return state
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		repoName := e.Name()
		indexPath := filepath.Join(dir, repoName, "_index.md")
		branchSHA := readFrontmatterField(indexPath, "source_sha")
		if branchSHA != "" {
			state[repoName] = repoState{
				branchSHA: branchSHA,
				readmeSHA: readFrontmatterField(indexPath, "readme_sha"),
			}
		}
	}
	return state
}

// carryForwardManifest records files from the previous manifest that belong to
// a repo being skipped on the fast path, preventing orphan cleanup from
// deleting them.
func carryForwardManifest(result *syncResult, repoName string, oldManifest map[string]bool) {
	prefix := "content/docs/projects/" + repoName + "/"
	for relPath := range oldManifest {
		if strings.HasPrefix(relPath, prefix) {
			result.recordFile(relPath)
		}
	}
}

// hasDocPagesInManifest reports whether the old manifest contains any doc page
// files (other than _index.md) for the given repo. Used to detect whether doc
// pages have ever been synced, so the first run with scan_paths support
// correctly populates them even when the branch SHA is unchanged.
func hasDocPagesInManifest(manifest map[string]bool, repoName string) bool {
	prefix := "content/docs/projects/" + repoName + "/"
	for relPath := range manifest {
		if strings.HasPrefix(relPath, prefix) && filepath.Base(relPath) != "_index.md" {
			return true
		}
	}
	return false
}

func readFrontmatterField(path, field string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		return ""
	}
	endIdx := strings.Index(content[4:], "\n---")
	if endIdx < 0 {
		return ""
	}
	frontMatter := content[4 : 4+endIdx]

	prefix := field + ":"
	for _, line := range strings.Split(frontMatter, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
			return strings.Trim(val, "\"")
		}
	}
	return ""
}

// cleanStaleContent removes synced content for repos that are no longer active,
// preserving the section _index.md. Only removes the sync tool's _index.md from
// each repo directory; Hugo module-mounted sub-pages are virtual and unaffected.
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
		indexPath := filepath.Join(projectsDir, repoName, "_index.md")
		if err := os.Remove(indexPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing stale %s: %w", indexPath, err)
		}
		os.Remove(filepath.Join(projectsDir, repoName))
	}

	return nil
}

const manifestFile = ".sync-manifest.json"

// readManifest loads the set of files written by the previous sync run.
// Returns nil if no manifest exists (e.g. first run).
func readManifest(outputDir string) map[string]bool {
	data, err := os.ReadFile(filepath.Join(outputDir, manifestFile))
	if err != nil {
		return nil
	}
	var files []string
	if err := json.Unmarshal(data, &files); err != nil {
		slog.Warn("could not parse sync manifest", "error", err)
		return nil
	}
	m := make(map[string]bool, len(files))
	for _, f := range files {
		m[f] = true
	}
	return m
}

// writeManifest persists the list of files written during this sync run.
func writeManifest(outputDir string, files []string) error {
	sorted := make([]string, len(files))
	copy(sorted, files)
	sort.Strings(sorted)
	data, err := json.MarshalIndent(sorted, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, manifestFile), append(data, '\n'), 0o644)
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
			if err := os.Remove(dir); err != nil {
				break
			}
			slog.Info("removed empty directory", "path", dir)
			dir = filepath.Dir(dir)
		}
	}
	return removed
}

// ---------------------------------------------------------------------------
// Discovery: detect new repos and untracked doc files
// ---------------------------------------------------------------------------

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
			if _, _, err := gh.getREADME(ctx, org, repo.Name); err == nil {
				result.NewFiles[fullName] = append(result.NewFiles[fullName], "README.md")
			}
		}
		if len(scanPaths) == 0 {
			continue
		}
		for _, scanPath := range scanPaths {
			files, err := gh.listDirMD(ctx, org, repo.Name, scanPath)
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
		for repo, files := range result.NewFiles {
			for _, f := range files {
				fmt.Fprintf(os.Stderr, "  - %s: %s\n", repo, f)
			}
		}
	} else {
		fmt.Fprintln(os.Stderr, "No new doc files found in tracked repos.")
	}

	if summaryPath := os.Getenv("GITHUB_STEP_SUMMARY"); summaryPath != "" {
		f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
		if err != nil {
			return
		}
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
			for repo, files := range result.NewFiles {
				for _, file := range files {
					fmt.Fprintf(f, "- `%s`: [`%s`](https://github.com/%s/blob/main/%s)\n", repo, file, repo, file)
				}
			}
			fmt.Fprintln(f)
		}
		if len(result.NewRepos) == 0 && totalNewFiles == 0 {
			fmt.Fprintln(f, "All repos and doc files are tracked. Nothing new to add.")
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

// writeFileSafe writes data to path, skipping the write if the file already
// exists with identical content. Returns true if the file was actually written.
func writeFileSafe(path string, data []byte) (bool, error) {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, data) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, data, 0o644)
}

// buildSectionIndex generates a lightweight Hugo section index (_index.md) for a
// project. Contains only frontmatter metadata so the Doks sidebar renders the
// section heading as a collapsible toggle with child pages listed underneath.
func buildSectionIndex(repo Repo, sha, readmeSHA string) string {
	lang := languageOrDefault(repo.Language)

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %q\n", repo.Name)
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
	fmt.Fprintf(&b, "    title: %q\n", repo.Name+" | ComplyTime")
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

// titleFromFilename converts a Markdown filename stem to a human-readable title.
// E.g. "quick-start" → "Quick Start", "sync_cac_content" → "Sync Cac Content".
func titleFromFilename(name string) string {
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.NewReplacer("-", " ", "_", " ").Replace(name)
	words := strings.Fields(name)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
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

// syncRepoDocPages auto-syncs Markdown files found under each scan_path in the
// discovery config. Files already tracked by explicit config sources or listed
// in ignoreFiles are skipped. Intermediate directories get auto-generated
// _index.md section pages.
func syncRepoDocPages(ctx context.Context, gh *apiClient, org string, repo Repo, output string, write bool, discovery Discovery, ignoreFiles map[string]bool, configTracked map[string]bool, result *syncResult) {
	logger := slog.With("repo", repo.Name, "phase", "doc-pages")

	for _, scanPath := range discovery.ScanPaths {
		files, err := gh.listDirMD(ctx, org, repo.Name, scanPath)
		if err != nil {
			logger.Debug("scan path not found", "path", scanPath, "error", err)
			continue
		}

		neededDirs := make(map[string]bool)

		for _, filePath := range files {
			baseName := filepath.Base(filePath)
			if ignoreFiles[baseName] {
				continue
			}
			if configTracked[filePath] {
				continue
			}

			relPath := strings.TrimPrefix(filePath, scanPath+"/")
			destRel := filepath.Join("content", "docs", "projects", repo.Name, relPath)
			destPath := filepath.Join(output, destRel)

			dir := filepath.Dir(relPath)
			for dir != "." && dir != "" {
				neededDirs[dir] = true
				dir = filepath.Dir(dir)
			}

			if !write {
				logger.Info("would write doc page (dry-run)", "src", filePath, "dest", destRel)
				result.mu.Lock()
				result.synced++
				result.mu.Unlock()
				continue
			}

			content, sha, err := gh.getFileContent(ctx, org, repo.Name, filePath)
			if err != nil {
				logger.Warn("could not fetch doc file", "path", filePath, "error", err)
				result.mu.Lock()
				result.warnings++
				result.mu.Unlock()
				continue
			}

			content = stripBadges(content)
			fileDir := filepath.Dir(filePath)
			content = rewriteRelativeLinks(content, org, repo.Name, repo.DefaultBranch, fileDir)

			page := buildDocPage(filePath, repo.FullName, repo.Description, repo.PushedAt, repo.DefaultBranch, sha, content)

			written, err := writeFileSafe(destPath, []byte(page))
			if err != nil {
				logger.Error("error writing doc page", "path", destPath, "error", err)
				result.mu.Lock()
				result.errors++
				result.mu.Unlock()
				continue
			}

			result.recordFile(destRel)
			if written {
				logger.Info("wrote doc page", "src", filePath, "dest", destPath)
			} else {
				logger.Info("doc page unchanged", "src", filePath, "dest", destPath)
			}

			result.mu.Lock()
			result.synced++
			result.mu.Unlock()
		}

		for dir := range neededDirs {
			indexRel := filepath.Join("content", "docs", "projects", repo.Name, dir, "_index.md")
			indexPath := filepath.Join(output, indexRel)

			if _, err := os.Stat(indexPath); err == nil {
				result.recordFile(indexRel)
				continue
			}

			if !write {
				continue
			}

			title := titleFromFilename(filepath.Base(dir))
			var b strings.Builder
			b.WriteString("---\n")
			fmt.Fprintf(&b, "title: %q\n", title)
			fmt.Fprintf(&b, "description: %q\n", repo.Description+" — "+title)
			fmt.Fprintf(&b, "date: %s\n", repo.PushedAt)
			fmt.Fprintf(&b, "lastmod: %s\n", repo.PushedAt)
			b.WriteString("draft: false\n")
			b.WriteString("---\n")

			written, err := writeFileSafe(indexPath, []byte(b.String()))
			if err != nil {
				logger.Error("error writing section index", "path", indexPath, "error", err)
				continue
			}

			result.recordFile(indexRel)
			if written {
				logger.Info("wrote section index", "path", indexPath)
			}
		}
	}
}

// repoWork holds the inputs and outputs for processing a single repo.
type repoWork struct {
	repo      Repo
	sha       string
	card      ProjectCard
	unchanged bool
}

// processRepo handles a single repository: fetches content, writes pages.
// When skipReadme is true, README fetching and project page generation are
// skipped but the ProjectCard is still produced.
//
// Two-tier change detection:
//  1. Branch SHA unchanged → skip all fetches (fast path).
//  2. Branch SHA changed  → fetch README, compare blob SHA for accurate
//     content-level change reporting.
//
// All shared state mutations go through result.mu.
func processRepo(ctx context.Context, gh *apiClient, org, output string, repo Repo, write bool, skipReadme bool, result *syncResult, oldState map[string]repoState, oldManifest map[string]bool) *repoWork {
	logger := slog.With("repo", repo.Name)

	sha, err := gh.getBranchSHA(ctx, org, repo.Name, repo.DefaultBranch)
	if err != nil {
		logger.Warn("could not get branch SHA", "error", err)
		sha = "unknown"
		result.mu.Lock()
		result.warnings++
		result.mu.Unlock()
	}

	old, existed := oldState[repo.Name]

	// Fast path: branch hasn't changed since last sync — skip all fetches.
	if existed && old.branchSHA == sha {
		result.mu.Lock()
		result.unchanged = append(result.unchanged, repo.Name)
		result.synced++
		result.mu.Unlock()

		if !write {
			logger.Info("unchanged (branch SHA match), skipping", "sha", sha)
			return nil
		}

		logger.Info("unchanged (branch SHA match), skipping fetches", "sha", sha)
		if oldManifest != nil {
			carryForwardManifest(result, repo.Name, oldManifest)
		}

		docURL := fmt.Sprintf("/docs/projects/%s/", repo.Name)
		card := ProjectCard{
			Name:        repo.Name,
			Language:    languageOrDefault(repo.Language),
			Type:        deriveProjectType(repo),
			Description: repo.Description,
			URL:         docURL,
			Repo:        repo.HTMLURL,
			Stars:       repo.StargazersCount,
		}
		return &repoWork{repo: repo, sha: sha, card: card, unchanged: true}
	}

	// Dry-run: report what would happen without fetching content.
	if !write {
		result.mu.Lock()
		if !existed {
			result.added = append(result.added, repo.Name)
		} else {
			result.updated = append(result.updated, repo.Name)
		}
		result.synced++
		result.mu.Unlock()
		logger.Info("would sync (dry-run)", "sha", sha)
		return nil
	}

	// Slow path: branch SHA changed — fetch content and compare file-level SHAs.
	contentChanged := !existed
	var readmeSHA string

	if !skipReadme {
		readme, rSHA, err := gh.getREADME(ctx, org, repo.Name)
		readmeSHA = rSHA
		if err != nil {
			logger.Warn("no README found", "error", err)
			result.mu.Lock()
			result.warnings++
			result.mu.Unlock()
		}

		if existed && old.readmeSHA != "" && old.readmeSHA == readmeSHA {
			logger.Info("README unchanged despite branch update", "branch_sha", sha, "readme_sha", readmeSHA)
		} else {
			contentChanged = true
		}

		if readme != "" {
			readme = stripLeadingH1(readme, repo.Name)
			readme = stripBadges(readme)
			readme = rewriteRelativeLinks(readme, org, repo.Name, repo.DefaultBranch)
		} else {
			readme = fmt.Sprintf(
				"*No README available.* Visit the [repository on GitHub](%s) for more information.\n",
				repo.HTMLURL,
			)
		}

		indexPage := buildSectionIndex(repo, sha, readmeSHA)
		indexRel := filepath.Join("content", "docs", "projects", repo.Name, "_index.md")
		indexPath := filepath.Join(output, indexRel)
		written, err := writeFileSafe(indexPath, []byte(indexPage))
		if err != nil {
			logger.Error("error writing section index", "path", indexPath, "error", err)
			result.mu.Lock()
			result.errors++
			result.mu.Unlock()
			return nil
		}
		result.recordFile(indexRel)
		if written {
			logger.Info("wrote section index", "path", indexPath)
		} else {
			logger.Info("section index unchanged", "path", indexPath)
		}

		overviewPage := buildOverviewPage(repo, readme)
		overviewRel := filepath.Join("content", "docs", "projects", repo.Name, "overview.md")
		overviewPath := filepath.Join(output, overviewRel)
		written, err = writeFileSafe(overviewPath, []byte(overviewPage))
		if err != nil {
			logger.Error("error writing overview page", "path", overviewPath, "error", err)
			result.mu.Lock()
			result.errors++
			result.mu.Unlock()
			return nil
		}
		result.recordFile(overviewRel)
		if written {
			logger.Info("wrote overview page", "path", overviewPath)
		} else {
			logger.Info("overview page unchanged", "path", overviewPath)
		}
	}

	docURL := fmt.Sprintf("/docs/projects/%s/", repo.Name)

	card := ProjectCard{
		Name:        repo.Name,
		Language:    languageOrDefault(repo.Language),
		Type:        deriveProjectType(repo),
		Description: repo.Description,
		URL:         docURL,
		Repo:        repo.HTMLURL,
		Stars:       repo.StargazersCount,
	}

	result.mu.Lock()
	if !existed {
		result.added = append(result.added, repo.Name)
	} else if contentChanged {
		result.updated = append(result.updated, repo.Name)
	} else {
		result.unchanged = append(result.unchanged, repo.Name)
	}
	result.synced++
	result.mu.Unlock()

	return &repoWork{repo: repo, sha: sha, card: card}
}

// writeGitHubOutputs writes structured outputs for GitHub Actions integration.
func writeGitHubOutputs(result *syncResult) {
	if ghOutput := os.Getenv("GITHUB_OUTPUT"); ghOutput != "" {
		f, err := os.OpenFile(ghOutput, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
		if err == nil {
			defer f.Close()
			hasChanges := "false"
			if result.hasChanges() {
				hasChanges = "true"
			}
			fmt.Fprintf(f, "has_changes=%s\n", hasChanges)
			fmt.Fprintf(f, "changed_count=%d\n", len(result.added)+len(result.updated))
			fmt.Fprintf(f, "error_count=%d\n", result.errors)
		}
	}

	if summaryPath := os.Getenv("GITHUB_STEP_SUMMARY"); summaryPath != "" {
		f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
		if err == nil {
			defer f.Close()
			fmt.Fprint(f, result.toMarkdown())
		}
	}
}

func main() {
	org := flag.String("org", "complytime", "GitHub organization name")
	token := flag.String("token", "", "GitHub API token (or set GITHUB_TOKEN env var)")
	output := flag.String("output", ".", "Hugo site root directory")
	include := flag.String("include", "", "Comma-separated repo allowlist (empty = auto-discover all)")
	exclude := flag.String("exclude", ".github,website,community,org-infra,complytime-demos,complytime-policies,complytime-collector-distro", "Comma-separated repo names to skip")
	write := flag.Bool("write", false, "Apply changes to disk (default: dry-run)")
	summaryFile := flag.String("summary", "", "Write markdown change summary to this file (for PR body)")
	timeout := flag.Duration("timeout", 3*time.Minute, "Overall timeout for all API operations")
	workers := flag.Int("workers", defaultWorkers, "Maximum concurrent repo processing goroutines")
	configPath := flag.String("config", "", "Path to sync-config.yaml for declarative file syncs")
	repoFilter := flag.String("repo", "", "Sync only this repo (e.g. complytime/complyctl)")
	discover := flag.Bool("discover", false, "Scan org for new repos and untracked doc files, then exit")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	apiToken := *token
	if apiToken == "" {
		apiToken = os.Getenv("GITHUB_TOKEN")
	}
	if apiToken == "" {
		slog.Warn("no GitHub token provided; API rate limits will be restrictive")
	}

	includeSet := parseNameList(*include)
	excludeSet := parseNameList(*exclude)

	if *repoFilter != "" {
		parts := strings.SplitN(*repoFilter, "/", 2)
		shortName := *repoFilter
		if len(parts) == 2 {
			shortName = parts[1]
		}
		includeSet = map[string]bool{shortName: true}
		slog.Info("filtering to single repo", "repo", *repoFilter)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	gh := &apiClient{
		token: apiToken,
		http:  &http.Client{Timeout: 30 * time.Second},
	}

	var cfg *SyncConfig
	if *configPath != "" {
		var err error
		cfg, err = loadConfig(*configPath)
		if err != nil {
			slog.Error("error loading config", "error", err)
			os.Exit(1)
		}
		slog.Info("loaded sync config", "path", *configPath, "sources", len(cfg.Sources))
	}

	if *discover {
		result, err := runDiscovery(ctx, gh, *org, cfg, excludeSet)
		if err != nil {
			slog.Error("discovery failed", "error", err)
			os.Exit(1)
		}
		printDiscoveryReport(result)
		return
	}

	configSources := make(map[string]Source)
	if cfg != nil {
		for _, src := range cfg.Sources {
			if *repoFilter != "" && src.Repo != *repoFilter {
				continue
			}
			configSources[src.Repo] = src
		}
	}

	oldState := readExistingState(*output)
	oldManifest := readManifest(*output)

	slog.Info("fetching repositories", "org", *org)
	repos, err := gh.listOrgRepos(ctx, *org)
	if err != nil {
		slog.Error("error listing repos", "error", err)
		os.Exit(1)
	}
	slog.Info("found repositories", "count", len(repos))

	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Name < repos[j].Name
	})

	var result syncResult
	newState := make(map[string]bool)

	var eligible []Repo
	for _, repo := range repos {
		if !isValidRepoName(repo.Name) {
			slog.Warn("skipping repo with invalid name", "name", repo.Name)
			result.skipped++
			continue
		}
		included := len(includeSet) == 0 || includeSet[repo.Name]
		if repo.Archived || repo.Fork || !included || excludeSet[repo.Name] {
			result.skipped++
			continue
		}
		newState[repo.Name] = true
		eligible = append(eligible, repo)
	}

	ignoreFiles := make(map[string]bool)
	if cfg != nil {
		for _, f := range cfg.Discovery.IgnoreFiles {
			ignoreFiles[f] = true
		}
	}

	sem := make(chan struct{}, *workers)
	var wg sync.WaitGroup
	var cardsMu sync.Mutex
	var cards []ProjectCard
	var processedConfigMu sync.Mutex
	processedConfig := make(map[string]bool)

	for _, repo := range eligible {
		wg.Add(1)
		sem <- struct{}{}
		go func(r Repo) {
			defer wg.Done()
			defer func() { <-sem }()

			slog.Info("processing repo", "repo", r.Name)

			cfgSrc, inConfig := configSources[r.FullName]
			skipReadme := inConfig && cfgSrc.SkipOrgSync

			work := processRepo(ctx, gh, *org, *output, r, *write, skipReadme, &result, oldState, oldManifest)
			if work != nil {
				cardsMu.Lock()
				cards = append(cards, work.card)
				cardsMu.Unlock()
			}

			configTracked := make(map[string]bool)
			if inConfig {
				for _, f := range cfgSrc.Files {
					configTracked[f.Src] = true
				}
			}

			if work != nil && !skipReadme && cfg != nil && len(cfg.Discovery.ScanPaths) > 0 {
				if !work.unchanged || !hasDocPagesInManifest(oldManifest, r.Name) {
					syncRepoDocPages(ctx, gh, *org, r, *output, *write, cfg.Discovery, ignoreFiles, configTracked, &result)
				}
			}

			if inConfig {
				syncConfigSource(ctx, gh, cfgSrc, cfg.Defaults, *output, *write, &result)
				processedConfigMu.Lock()
				processedConfig[r.FullName] = true
				processedConfigMu.Unlock()
			}
		}(repo)
	}

	wg.Wait()

	if cfg != nil {
		for _, src := range cfg.Sources {
			if processedConfig[src.Repo] {
				continue
			}
			slog.Info("processing config-only source (not in org)", "repo", src.Repo)
			syncConfigSource(ctx, gh, src, cfg.Defaults, *output, *write, &result)

			parts := strings.SplitN(src.Repo, "/", 2)
			if len(parts) == 2 {
				newState[parts[1]] = true
			}
		}
	}

	sort.Slice(cards, func(i, j int) bool {
		return cards[i].Name < cards[j].Name
	})

	for name := range oldState {
		if !newState[name] {
			result.removed = append(result.removed, name)
		}
	}
	sort.Strings(result.removed)

	if *write {
		if oldManifest != nil {
			orphans := cleanOrphanedFiles(*output, oldManifest, result.writtenFiles)
			if orphans > 0 {
				slog.Info("cleaned orphaned files from previous sync", "count", orphans)
			}
		} else {
			if err := cleanStaleContent(*output, newState); err != nil {
				slog.Warn("could not clean stale content", "error", err)
				result.warnings++
			}
		}
		if err := writeManifest(*output, result.writtenFiles); err != nil {
			slog.Warn("could not write sync manifest", "error", err)
			result.warnings++
		}
	}

	if *write {
		jsonData, err := json.MarshalIndent(cards, "", "  ")
		if err != nil {
			slog.Error("error marshaling projects.json", "error", err)
			os.Exit(1)
		}
		jsonPath := filepath.Join(*output, "data", "projects.json")
		written, err := writeFileSafe(jsonPath, append(jsonData, '\n'))
		if err != nil {
			slog.Error("error writing projects.json", "error", err)
			os.Exit(1)
		}
		if written {
			slog.Info("wrote projects.json", "count", len(cards))
		} else {
			slog.Info("projects.json unchanged, skipped write")
		}
	}

	result.printSummary()
	writeGitHubOutputs(&result)

	if *summaryFile != "" {
		md := result.toMarkdown()
		if _, err := writeFileSafe(*summaryFile, []byte(md)); err != nil {
			slog.Error("error writing summary file", "path", *summaryFile, "error", err)
		} else {
			slog.Info("wrote change summary", "path", *summaryFile)
		}
	}

	if !*write {
		slog.Info("dry run complete, no files were written")
	} else {
		slog.Info("sync complete")
	}

	if result.errors > 0 {
		os.Exit(1)
	}
}
