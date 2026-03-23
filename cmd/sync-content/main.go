// SPDX-License-Identifier: Apache-2.0
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
	"unicode"
	"unicode/utf8"

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
	Type string `json:"type"`
}

type BranchResponse struct {
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
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
	Defaults Defaults `yaml:"defaults"`
	Sources  []Source `yaml:"sources"`
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
	mu        sync.Mutex
	synced    int
	skipped   int
	warnings  int
	errors    int
	specs     int
	added     []string
	updated   []string
	removed   []string
	unchanged []string
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

	fmt.Fprintf(&b, "**Stats**: %d synced, %d skipped, %d specs",
		r.synced, r.skipped, r.specs)
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
		"specs", r.specs,
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

func (c *apiClient) getREADME(ctx context.Context, owner, repo string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/readme", githubAPI, owner, repo)
	var f FileResponse
	if err := c.getJSON(ctx, url, &f); err != nil {
		return "", err
	}
	return decodeContent(f)
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

func (c *apiClient) hasDir(ctx context.Context, owner, repo, dir string) bool {
	_, err := c.listDir(ctx, owner, repo, dir)
	return err == nil
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

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// mdLinkRe matches Markdown links and images: [text](url) and ![alt](url).
var mdLinkRe = regexp.MustCompile(`(!?\[[^\]]*\])\(([^)]+)\)`)

// badgeRe matches Markdown badge patterns at the start of a line:
// [![alt](img-url)](link-url) followed by optional whitespace/newline.
var badgeRe = regexp.MustCompile(`(?m)^\[!\[[^\]]*\]\([^\)]*\)\]\([^\)]*\)\s*\n?`)

// rewriteRelativeLinks converts relative Markdown links and images to absolute
// GitHub URLs so they work when the README is rendered on the Hugo site.
// Regular links point to /blob/ (rendered view), images point to raw.githubusercontent (raw content).
func rewriteRelativeLinks(markdown, owner, repo, branch string) string {
	baseBlob := fmt.Sprintf("https://github.com/%s/%s/blob/%s/", owner, repo, branch)
	baseRaw := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/", owner, repo, branch)

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
		content, _, err := gh.getFileContent(ctx, owner, repoName, file.Src)
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

// readExistingState reads the source_sha from existing project pages
// to enable change detection across sync runs.
func readExistingState(outputDir string) map[string]string {
	state := make(map[string]string)
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
		if sha := readFrontmatterField(indexPath, "source_sha"); sha != "" {
			state[repoName] = sha
		}
	}
	return state
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

	specsDir := filepath.Join(outputDir, "content", "specs")
	entries, err = os.ReadDir(specsDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading specs dir: %w", err)
	}
	for _, e := range entries {
		if e.Name() == "_index.md" || !e.IsDir() {
			continue
		}
		if activeRepos[e.Name()] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(specsDir, e.Name())); err != nil {
			return fmt.Errorf("removing stale spec dir %s: %w", e.Name(), err)
		}
	}

	return nil
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

// buildProjectPage generates a Hugo section index page for a project.
// Uses repo.PushedAt for lastmod so output is deterministic across runs.
func buildProjectPage(repo Repo, readme, sha string) string {
	lang := languageOrDefault(repo.Language)

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %q\n", repo.Name)
	fmt.Fprintf(&b, "description: %q\n", repo.Description)
	fmt.Fprintf(&b, "date: %s\n", repo.PushedAt)
	fmt.Fprintf(&b, "lastmod: %s\n", repo.PushedAt)
	b.WriteString("draft: false\n")
	b.WriteString("toc: true\n")
	b.WriteString("params:\n")
	fmt.Fprintf(&b, "  language: %q\n", lang)
	fmt.Fprintf(&b, "  stars: %d\n", repo.StargazersCount)
	fmt.Fprintf(&b, "  repo: %q\n", repo.HTMLURL)
	fmt.Fprintf(&b, "  source_sha: %q\n", sha)
	b.WriteString("  seo:\n")
	fmt.Fprintf(&b, "    title: %q\n", repo.Name+" | ComplyTime")
	fmt.Fprintf(&b, "    description: %q\n", repo.Description)
	b.WriteString("---\n\n")
	b.WriteString(readme)

	return b.String()
}

// Spec pages are ordered: constitution (1), spec (2), plan (3).
var specWeights = map[string]int{
	"constitution.md": 1,
	"spec.md":         2,
	"plan.md":         3,
}

func buildSpecPage(repoName, specName, content, pushedAt string) string {
	label := capitalizeFirst(strings.TrimSuffix(specName, filepath.Ext(specName)))
	weight := specWeights[specName]

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %q\n", label+" — "+repoName)
	fmt.Fprintf(&b, "description: %q\n", label+" specification for "+repoName)
	fmt.Fprintf(&b, "date: %s\n", pushedAt)
	fmt.Fprintf(&b, "lastmod: %s\n", pushedAt)
	fmt.Fprintf(&b, "weight: %d\n", weight)
	b.WriteString("draft: false\n")
	b.WriteString("---\n\n")
	b.WriteString(content)

	return b.String()
}

func buildSpecIndex(repoName, description, pushedAt string) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %q\n", repoName)
	fmt.Fprintf(&b, "description: %q\n", "Spec-kit artifacts for "+repoName)
	fmt.Fprintf(&b, "date: %s\n", pushedAt)
	fmt.Fprintf(&b, "lastmod: %s\n", pushedAt)
	b.WriteString("draft: false\n")
	b.WriteString("---\n\n")
	if description != "" {
		b.WriteString(description)
		b.WriteString("\n")
	}

	return b.String()
}

// repoWork holds the inputs and outputs for processing a single repo.
type repoWork struct {
	repo Repo
	sha  string
	card ProjectCard
}

// processRepo handles a single repository: fetches content, writes pages.
// When skipReadme is true, README fetching and project page generation are
// skipped but the ProjectCard and .specify/ artifacts are still produced.
// All shared state mutations go through result.mu.
func processRepo(ctx context.Context, gh *apiClient, org, output string, repo Repo, write bool, skipReadme bool, result *syncResult, oldState map[string]string) *repoWork {
	logger := slog.With("repo", repo.Name)

	sha, err := gh.getBranchSHA(ctx, org, repo.Name, repo.DefaultBranch)
	if err != nil {
		logger.Warn("could not get branch SHA", "error", err)
		sha = "unknown"
		result.mu.Lock()
		result.warnings++
		result.mu.Unlock()
	}

	result.mu.Lock()
	oldSHA, existed := oldState[repo.Name]
	switch {
	case !existed:
		result.added = append(result.added, repo.Name)
	case oldSHA != sha:
		result.updated = append(result.updated, repo.Name)
	default:
		result.unchanged = append(result.unchanged, repo.Name)
	}
	result.mu.Unlock()

	if !write {
		logger.Info("would sync (dry-run)", "sha", sha)
		result.mu.Lock()
		result.synced++
		result.mu.Unlock()
		return nil
	}

	if !skipReadme {
		readme, err := gh.getREADME(ctx, org, repo.Name)
		if err != nil {
			logger.Warn("no README found", "error", err)
			result.mu.Lock()
			result.warnings++
			result.mu.Unlock()
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

		page := buildProjectPage(repo, readme, sha)
		pagePath := filepath.Join(output, "content", "docs", "projects", repo.Name, "_index.md")
		written, err := writeFileSafe(pagePath, []byte(page))
		if err != nil {
			logger.Error("error writing project page", "path", pagePath, "error", err)
			result.mu.Lock()
			result.errors++
			result.mu.Unlock()
			return nil
		}
		if written {
			logger.Info("wrote project page", "path", pagePath)
		} else {
			logger.Info("project page unchanged, skipped write", "path", pagePath)
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

	specEntries, err := gh.listDir(ctx, org, repo.Name, ".specify")
	if err != nil {
		result.mu.Lock()
		result.synced++
		result.mu.Unlock()
		return &repoWork{repo: repo, sha: sha, card: card}
	}

	indexContent := buildSpecIndex(repo.Name, repo.Description, repo.PushedAt)
	indexPath := filepath.Join(output, "content", "specs", repo.Name, "_index.md")
	if _, err := writeFileSafe(indexPath, []byte(indexContent)); err != nil {
		logger.Error("error writing spec index", "path", indexPath, "error", err)
		result.mu.Lock()
		result.errors++
		result.mu.Unlock()
	}

	wanted := map[string]bool{"constitution.md": true, "spec.md": true, "plan.md": true}
	for _, entry := range specEntries {
		if entry.Type != "file" || !wanted[entry.Name] {
			continue
		}
		content, _, err := gh.getFileContent(ctx, org, repo.Name, ".specify/"+entry.Name)
		if err != nil {
			logger.Warn("could not fetch spec file", "file", ".specify/"+entry.Name, "error", err)
			result.mu.Lock()
			result.warnings++
			result.mu.Unlock()
			continue
		}
		specPage := buildSpecPage(repo.Name, entry.Name, content, repo.PushedAt)
		specPath := filepath.Join(output, "content", "specs", repo.Name, entry.Name)
		written, err := writeFileSafe(specPath, []byte(specPage))
		if err != nil {
			logger.Error("error writing spec file", "path", specPath, "error", err)
			result.mu.Lock()
			result.errors++
			result.mu.Unlock()
			continue
		}
		result.mu.Lock()
		result.specs++
		result.mu.Unlock()
		if written {
			logger.Info("wrote spec file", "path", specPath)
		}
	}

	result.mu.Lock()
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
	exclude := flag.String("exclude", ".github", "Comma-separated repo names to skip")
	write := flag.Bool("write", false, "Apply changes to disk (default: dry-run)")
	summaryFile := flag.String("summary", "", "Write markdown change summary to this file (for PR body)")
	timeout := flag.Duration("timeout", 3*time.Minute, "Overall timeout for all API operations")
	workers := flag.Int("workers", defaultWorkers, "Maximum concurrent repo processing goroutines")
	configPath := flag.String("config", "", "Path to sync-config.yaml for declarative file syncs")
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

	configSources := make(map[string]Source)
	if cfg != nil {
		for _, src := range cfg.Sources {
			configSources[src.Repo] = src
		}
	}

	oldState := readExistingState(*output)

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

			work := processRepo(ctx, gh, *org, *output, r, *write, skipReadme, &result, oldState)
			if work != nil {
				cardsMu.Lock()
				cards = append(cards, work.card)
				cardsMu.Unlock()
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
		if err := cleanStaleContent(*output, newState); err != nil {
			slog.Warn("could not clean stale content", "error", err)
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
