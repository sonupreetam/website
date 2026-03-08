// Command sync-content reads a declarative sync-config.yaml manifest and
// pulls the declared files from upstream GitHub repositories into the
// website's content tree.
//
// It is designed to be invoked by a scheduled GitHub Actions workflow
// (or manually) and is fully idempotent: if no upstream content has
// changed, no files are written and the exit code is 0.
//
// Usage:
//
//	go run ./cmd/sync-content                        # default: sync-config.yaml, dry-run
//	go run ./cmd/sync-content --write                # apply changes to disk
//	go run ./cmd/sync-content --config custom.yaml   # use a custom manifest
//	go run ./cmd/sync-content --repo complytime/complyctl  # sync a single repo
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Configuration model (mirrors sync-config.yaml)
// ---------------------------------------------------------------------------

// Config is the top-level sync configuration.
type Config struct {
	Defaults  Defaults  `yaml:"defaults"`
	Sources   []Source  `yaml:"sources"`
	Discovery Discovery `yaml:"discovery"`
}

// Defaults holds values applied to every source unless overridden.
type Defaults struct {
	Branch    string `yaml:"branch"`
	GitHubAPI string `yaml:"github_api"`
}

// Source is a single upstream repository to sync from.
type Source struct {
	Repo   string     `yaml:"repo"`
	Branch string     `yaml:"branch"` // overrides Defaults.Branch
	Files  []FileSpec `yaml:"files"`
}

// FileSpec describes a single file to pull and where to place it.
type FileSpec struct {
	Src       string    `yaml:"src"`
	Dest      string    `yaml:"dest"`
	Transform Transform `yaml:"transform"`
}

// Transform describes optional mutations to apply to the fetched content.
type Transform struct {
	InjectFrontmatter map[string]any `yaml:"inject_frontmatter"`
	RewriteLinks      bool           `yaml:"rewrite_links"`
	StripBadges       bool           `yaml:"strip_badges"`
}

// Discovery configures automatic detection of new repos and doc files
// that are not yet declared in Sources.
type Discovery struct {
	Org         string   `yaml:"org"`
	IgnoreRepos []string `yaml:"ignore_repos"`
	IgnoreFiles []string `yaml:"ignore_files"`
	ScanPaths   []string `yaml:"scan_paths"`
}

// ---------------------------------------------------------------------------
// GitHub API helpers
// ---------------------------------------------------------------------------

// ghRawURL returns the raw content URL for a file in a GitHub repository.
func ghRawURL(apiBase, repo, branch, path string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", repo, branch, path)
}

// ghCommitSHA fetches the latest commit SHA for a branch via the GitHub API.
func ghCommitSHA(ctx context.Context, client *http.Client, apiBase, repo, branch, token string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/commits/%s", apiBase, repo, branch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GET %s: %d %s", url, resp.StatusCode, string(body))
	}

	var payload struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode commit response: %w", err)
	}
	return payload.SHA, nil
}

// fetchFile downloads a single file from a GitHub repository.
func fetchFile(ctx context.Context, client *http.Client, apiBase, repo, branch, path, token string) ([]byte, error) {
	url := ghRawURL(apiBase, repo, branch, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("file not found: %s/%s@%s", repo, path, branch)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s: %d %s", url, resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// ---------------------------------------------------------------------------
// Content transforms
// ---------------------------------------------------------------------------

// badgeRe matches common Markdown badge patterns: [![...](img)](link) at the
// start of a line, plus trailing blank lines.
var badgeRe = regexp.MustCompile(`(?m)^\[!\[[^\]]*\]\([^\)]*\)\]\([^\)]*\)\s*\n?`)

// applyTransforms mutates the raw Markdown content according to the spec.
func applyTransforms(content []byte, spec FileSpec, repo, branch string) []byte {
	out := content

	// 1. Strip badge images (CI status, coverage, etc.)
	if spec.Transform.StripBadges {
		out = badgeRe.ReplaceAll(out, nil)
		// Trim leading whitespace left after stripping.
		out = bytes.TrimLeft(out, "\n")
	}

	// 2. Rewrite relative links so they point to the GitHub source.
	if spec.Transform.RewriteLinks {
		out = rewriteRelativeLinks(out, repo, branch, spec.Src)
	}

	// 3. Inject Hugo frontmatter if specified.
	if len(spec.Transform.InjectFrontmatter) > 0 {
		out = injectFrontmatter(out, spec.Transform.InjectFrontmatter)
	}

	return out
}

// relLinkRe matches Markdown links with relative paths: [text](./path) or [text](path).
var relLinkRe = regexp.MustCompile(`(\[[^\]]+\])\(([^):#\s][^):#]*)\)`)

// relImgRe matches Markdown images with relative paths: ![alt](./path) or ![alt](path).
var relImgRe = regexp.MustCompile(`(!\[[^\]]*\])\(([^):#\s][^):#]*)\)`)

// rewriteRelativeLinks converts relative Markdown links/images to absolute
// GitHub URLs so they work correctly on the rendered website.
func rewriteRelativeLinks(content []byte, repo, branch, srcFile string) []byte {
	srcDir := filepath.Dir(srcFile)

	rewrite := func(re *regexp.Regexp, base string) []byte {
		return re.ReplaceAllFunc(content, func(match []byte) []byte {
			subs := re.FindSubmatch(match)
			if len(subs) < 3 {
				return match
			}
			linkText := string(subs[1])
			target := string(subs[2])

			// Skip absolute URLs, anchors, and Hugo shortcodes.
			if strings.HasPrefix(target, "http://") ||
				strings.HasPrefix(target, "https://") ||
				strings.HasPrefix(target, "#") ||
				strings.HasPrefix(target, "{{") {
				return match
			}

			// Resolve relative to the source file's directory.
			resolved := filepath.Join(srcDir, target)
			resolved = filepath.ToSlash(resolved)

			absURL := fmt.Sprintf("https://github.com/%s/blob/%s/%s", repo, branch, resolved)
			return []byte(fmt.Sprintf("%s(%s)", linkText, absURL))
		})
	}

	content = rewrite(relLinkRe, "blob")
	content = rewrite(relImgRe, "raw")
	return content
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

	// Strip existing frontmatter if present.
	body := content
	if bytes.HasPrefix(body, []byte("---")) {
		// Find the closing "---".
		end := bytes.Index(body[3:], []byte("\n---"))
		if end != -1 {
			body = bytes.TrimLeft(body[end+3+4:], "\n") // skip past closing ---\n
		}
	}

	buf.Write(body)
	return buf.Bytes()
}

// ---------------------------------------------------------------------------
// LLM-powered content enhancement
// ---------------------------------------------------------------------------

// LLMConfig holds settings for the optional LLM enhancement pass.
type LLMConfig struct {
	APIKey  string // from OPENAI_API_KEY
	BaseURL string // from OPENAI_BASE_URL (defaults to https://api.openai.com/v1)
	Model   string // from OPENAI_MODEL (defaults to gpt-4o-mini)
}

// llmConfigFromEnv builds an LLMConfig from environment variables.
// Returns nil if OPENAI_API_KEY is not set.
func llmConfigFromEnv() *LLMConfig {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil
	}
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &LLMConfig{APIKey: key, BaseURL: baseURL, Model: model}
}

// chatMessage is a single message in an OpenAI chat completion request.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest is the request body for OpenAI chat completions.
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// chatResponse is the (minimal) response from OpenAI chat completions.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

const enhanceSystemPrompt = `You are a technical documentation editor for the ComplyTime project (compliance automation tools). Your job is to improve Markdown documentation that has been pulled from GitHub repositories.

Rules:
1. PRESERVE all technical content, code blocks, links, and meaning exactly. Never remove or change factual information.
2. Fix inconsistent Markdown formatting (heading levels, list styles, code fence languages).
3. Ensure headings start at H2 (##) since H1 is generated from the frontmatter title.
4. Add language identifiers to bare code fences (e.g. use "` + "```yaml" + `" not just "` + "```" + `").
5. Improve readability: break up walls of text, improve transitions between sections.
6. Standardize admonitions/callouts to Hugo-compatible format: "> **Note:** ..." or "> **Warning:** ...".
7. Fix obvious typos and grammar issues.
8. Do NOT add new sections, features, or content that doesn't exist in the original.
9. Do NOT add comments about what you changed.
10. Return ONLY the improved Markdown body (no frontmatter — that is handled separately).
11. Keep the output concise. Do not pad with unnecessary filler text.`

// enhanceContent sends the Markdown body through an LLM to improve formatting
// and readability. Frontmatter is preserved and reattached after enhancement.
func enhanceContent(ctx context.Context, client *http.Client, cfg *LLMConfig, content []byte) ([]byte, error) {
	// Split frontmatter from body.
	frontmatter, body := splitFrontmatter(content)

	bodyStr := strings.TrimSpace(string(body))
	if bodyStr == "" {
		return content, nil
	}

	// Skip very short content (not worth enhancing).
	if len(bodyStr) < 100 {
		return content, nil
	}

	reqBody := chatRequest{
		Model: cfg.Model,
		Messages: []chatMessage{
			{Role: "system", Content: enhanceSystemPrompt},
			{Role: "user", Content: "Improve this documentation:\n\n" + bodyStr},
		},
		Temperature: 0.2,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return content, fmt.Errorf("marshal LLM request: %w", err)
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return content, fmt.Errorf("create LLM request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return content, fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return content, fmt.Errorf("read LLM response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return content, fmt.Errorf("LLM API returned %d: %s", resp.StatusCode, string(respBytes))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return content, fmt.Errorf("decode LLM response: %w", err)
	}

	if chatResp.Error != nil {
		return content, fmt.Errorf("LLM error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return content, fmt.Errorf("LLM returned no choices")
	}

	enhanced := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	if enhanced == "" {
		return content, nil
	}

	// Strip markdown code fences if the LLM wrapped the output in them.
	enhanced = stripWrappingCodeFence(enhanced)

	// Reassemble: frontmatter + enhanced body.
	var buf bytes.Buffer
	if len(frontmatter) > 0 {
		buf.Write(frontmatter)
		buf.WriteString("\n")
	}
	buf.WriteString(enhanced)
	buf.WriteString("\n")

	return buf.Bytes(), nil
}

// splitFrontmatter separates YAML frontmatter from the Markdown body.
// Returns (frontmatter including delimiters, body). If no frontmatter exists,
// frontmatter is nil.
func splitFrontmatter(content []byte) ([]byte, []byte) {
	if !bytes.HasPrefix(content, []byte("---\n")) {
		return nil, content
	}
	idx := bytes.Index(content[4:], []byte("\n---\n"))
	if idx == -1 {
		// Check for frontmatter at end of file (no trailing newline).
		idx = bytes.Index(content[4:], []byte("\n---"))
		if idx == -1 {
			return nil, content
		}
		end := 4 + idx + len("\n---")
		return content[:end], bytes.TrimLeft(content[end:], "\n")
	}
	end := 4 + idx + len("\n---\n")
	return content[:end], bytes.TrimLeft(content[end:], "\n")
}

// stripWrappingCodeFence removes a wrapping ```markdown ... ``` fence that
// LLMs sometimes add around their output.
func stripWrappingCodeFence(s string) string {
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "```") {
		// Find first newline after opening fence.
		nlIdx := strings.Index(trimmed, "\n")
		if nlIdx == -1 {
			return s
		}
		inner := trimmed[nlIdx+1:]
		// Find closing fence.
		lastFence := strings.LastIndex(inner, "```")
		if lastFence != -1 {
			inner = inner[:lastFence]
		}
		return strings.TrimSpace(inner)
	}
	return s
}

// ---------------------------------------------------------------------------
// Sync result tracking
// ---------------------------------------------------------------------------

// SyncResult captures the outcome of syncing a single file.
type SyncResult struct {
	Repo    string
	Src     string
	Dest    string
	Changed bool
	Skipped bool
	Error   error
}

// ---------------------------------------------------------------------------
// Core sync logic
// ---------------------------------------------------------------------------

// syncSource processes a single source repository and returns results for
// each declared file. If llmCfg is non-nil, content is enhanced via LLM.
func syncSource(ctx context.Context, client *http.Client, cfg Config, src Source, token string, write bool, llmCfg *LLMConfig) []SyncResult {
	branch := src.Branch
	if branch == "" {
		branch = cfg.Defaults.Branch
	}
	apiBase := cfg.Defaults.GitHubAPI

	logger := slog.With("repo", src.Repo, "branch", branch)

	// Fetch the latest commit SHA for provenance logging.
	sha, err := ghCommitSHA(ctx, client, apiBase, src.Repo, branch, token)
	if err != nil {
		logger.Warn("could not resolve HEAD commit", "error", err)
		sha = "unknown"
	} else {
		logger.Info("resolved HEAD", "sha", sha[:12])
	}

	results := make([]SyncResult, 0, len(src.Files))

	for _, file := range src.Files {
		res := SyncResult{Repo: src.Repo, Src: file.Src, Dest: file.Dest}

		raw, err := fetchFile(ctx, client, apiBase, src.Repo, branch, file.Src, token)
		if err != nil {
			res.Error = err
			results = append(results, res)
			logger.Error("fetch failed", "src", file.Src, "error", err)
			continue
		}

		transformed := applyTransforms(raw, file, src.Repo, branch)

		// Optional LLM enhancement pass.
		if llmCfg != nil {
			enhanced, err := enhanceContent(ctx, client, llmCfg, transformed)
			if err != nil {
				logger.Warn("LLM enhancement failed, using original", "src", file.Src, "error", err)
			} else {
				transformed = enhanced
				logger.Info("enhanced via LLM", "src", file.Src, "model", llmCfg.Model)
			}
		}

		// Add a sync provenance comment at the top of the file, after frontmatter.
		provenance := fmt.Sprintf(
			"<!-- synced from %s@%s (%s) on %s -->\n",
			src.Repo, branch, sha[:minLen(len(sha), 12)],
			time.Now().UTC().Format(time.RFC3339),
		)
		transformed = insertAfterFrontmatter(transformed, []byte(provenance))

		// Check if the destination already has identical content.
		destPath := file.Dest
		existing, readErr := os.ReadFile(destPath)
		if readErr == nil && bytes.Equal(existing, transformed) {
			res.Skipped = true
			results = append(results, res)
			logger.Info("unchanged", "dest", destPath)
			continue
		}

		res.Changed = true

		if write {
			if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
				res.Error = fmt.Errorf("mkdir: %w", err)
				results = append(results, res)
				continue
			}
			if err := os.WriteFile(destPath, transformed, 0o644); err != nil {
				res.Error = fmt.Errorf("write: %w", err)
				results = append(results, res)
				continue
			}
			logger.Info("written", "dest", destPath)
		} else {
			logger.Info("would write (dry-run)", "dest", destPath)
		}

		results = append(results, res)
	}

	return results
}

// insertAfterFrontmatter inserts extra bytes right after the closing "---"
// of YAML frontmatter. If there is no frontmatter, content is prepended.
func insertAfterFrontmatter(content, insert []byte) []byte {
	if !bytes.HasPrefix(content, []byte("---\n")) {
		return append(insert, content...)
	}
	// Find the closing frontmatter delimiter.
	idx := bytes.Index(content[4:], []byte("\n---\n"))
	if idx == -1 {
		return append(insert, content...)
	}
	insertPoint := 4 + idx + len("\n---\n")
	var buf bytes.Buffer
	buf.Write(content[:insertPoint])
	buf.Write(insert)
	buf.Write(content[insertPoint:])
	return buf.Bytes()
}

func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---------------------------------------------------------------------------
// Discovery: detect new repos and untracked doc files
// ---------------------------------------------------------------------------

// DiscoveryResult holds the output of an org-wide discovery scan.
type DiscoveryResult struct {
	NewRepos    []string            // repos in org not declared in sources
	NewFiles    map[string][]string // repo → list of untracked doc files
	ScannedOrg  string
	TotalRepos  int
	TrackedRepos int
}

// ghListOrgRepos returns all public repo full names (owner/name) for a GitHub org.
func ghListOrgRepos(ctx context.Context, client *http.Client, apiBase, org, token string) ([]string, error) {
	var allRepos []string
	page := 1

	for {
		url := fmt.Sprintf("%s/orgs/%s/repos?type=public&per_page=100&page=%d", apiBase, org, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("list repos: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("GET %s: %d %s", url, resp.StatusCode, string(body))
		}

		var repos []struct {
			FullName string `json:"full_name"`
			Archived bool   `json:"archived"`
			Fork     bool   `json:"fork"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
			return nil, fmt.Errorf("decode repos: %w", err)
		}

		if len(repos) == 0 {
			break
		}

		for _, r := range repos {
			if !r.Archived {
				allRepos = append(allRepos, r.FullName)
			}
		}

		page++
	}

	return allRepos, nil
}

// ghListDir lists files in a directory of a GitHub repo via the Contents API.
// Returns a list of file paths (relative to repo root). Recurses into subdirs.
func ghListDir(ctx context.Context, client *http.Client, apiBase, repo, branch, dir, token string) ([]string, error) {
	url := fmt.Sprintf("%s/repos/%s/contents/%s?ref=%s", apiBase, repo, dir, branch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list dir %s/%s: %w", repo, dir, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // directory doesn't exist — not an error
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s: %d %s", url, resp.StatusCode, string(body))
	}

	var entries []struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Type string `json:"type"` // "file" or "dir"
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode dir listing: %w", err)
	}

	var files []string
	for _, e := range entries {
		if e.Type == "file" && strings.HasSuffix(e.Name, ".md") {
			files = append(files, e.Path)
		} else if e.Type == "dir" {
			// Recurse into subdirectories.
			subFiles, err := ghListDir(ctx, client, apiBase, repo, branch, e.Path, token)
			if err != nil {
				slog.Warn("could not list subdir", "repo", repo, "dir", e.Path, "error", err)
				continue
			}
			files = append(files, subFiles...)
		}
	}

	return files, nil
}

// runDiscovery scans the org for repos and doc files not yet in the manifest.
func runDiscovery(ctx context.Context, client *http.Client, cfg Config, token string) (*DiscoveryResult, error) {
	disc := cfg.Discovery
	if disc.Org == "" {
		return nil, fmt.Errorf("discovery.org is not set in config")
	}

	apiBase := cfg.Defaults.GitHubAPI
	result := &DiscoveryResult{
		NewFiles:   make(map[string][]string),
		ScannedOrg: disc.Org,
	}

	// Build sets for fast lookups.
	ignoreRepoSet := make(map[string]bool)
	for _, r := range disc.IgnoreRepos {
		ignoreRepoSet[r] = true
	}

	ignoreFileSet := make(map[string]bool)
	for _, f := range disc.IgnoreFiles {
		ignoreFileSet[f] = true
	}

	trackedRepoSet := make(map[string]bool)
	trackedFileSet := make(map[string]bool) // "repo::path"
	for _, src := range cfg.Sources {
		trackedRepoSet[src.Repo] = true
		for _, f := range src.Files {
			trackedFileSet[src.Repo+"::"+f.Src] = true
		}
	}

	// 1. List all repos in the org.
	slog.Info("discovering repos", "org", disc.Org)
	orgRepos, err := ghListOrgRepos(ctx, client, apiBase, disc.Org, token)
	if err != nil {
		return nil, fmt.Errorf("list org repos: %w", err)
	}

	result.TotalRepos = len(orgRepos)
	result.TrackedRepos = len(trackedRepoSet)

	// 2. Identify new repos.
	for _, repo := range orgRepos {
		if ignoreRepoSet[repo] || trackedRepoSet[repo] {
			continue
		}
		result.NewRepos = append(result.NewRepos, repo)
	}

	// 3. For tracked repos, scan for new doc files.
	for _, src := range cfg.Sources {
		branch := src.Branch
		if branch == "" {
			branch = cfg.Defaults.Branch
		}

		logger := slog.With("repo", src.Repo)

		// Scan root for README.md (always check).
		if !trackedFileSet[src.Repo+"::README.md"] {
			// Check if README.md exists.
			_, err := fetchFile(ctx, client, apiBase, src.Repo, branch, "README.md", token)
			if err == nil {
				result.NewFiles[src.Repo] = append(result.NewFiles[src.Repo], "README.md")
			}
		}

		// Scan each configured scan path.
		for _, scanPath := range disc.ScanPaths {
			files, err := ghListDir(ctx, client, apiBase, src.Repo, branch, scanPath, token)
			if err != nil {
				logger.Warn("discovery scan failed", "path", scanPath, "error", err)
				continue
			}

			for _, filePath := range files {
				baseName := filepath.Base(filePath)

				// Skip ignored file names.
				if ignoreFileSet[baseName] {
					continue
				}

				// Skip if already tracked.
				if trackedFileSet[src.Repo+"::"+filePath] {
					continue
				}

				result.NewFiles[src.Repo] = append(result.NewFiles[src.Repo], filePath)
			}
		}
	}

	return result, nil
}

// printDiscoveryReport outputs the discovery results to stderr and optionally
// writes a GITHUB_STEP_SUMMARY for CI visibility.
func printDiscoveryReport(result *DiscoveryResult) {
	fmt.Fprintf(os.Stderr, "\n══ Discovery Report (%s) ══\n", result.ScannedOrg)
	fmt.Fprintf(os.Stderr, "  org repos:     %d\n", result.TotalRepos)
	fmt.Fprintf(os.Stderr, "  tracked repos: %d\n", result.TrackedRepos)

	// New repos.
	if len(result.NewRepos) > 0 {
		fmt.Fprintf(os.Stderr, "\n⚡ New repos not in manifest (%d):\n", len(result.NewRepos))
		for _, repo := range result.NewRepos {
			fmt.Fprintf(os.Stderr, "  → %s\n", repo)
		}
	} else {
		fmt.Fprintln(os.Stderr, "\n✓ No new repos found.")
	}

	// New files in tracked repos.
	totalNewFiles := 0
	for _, files := range result.NewFiles {
		totalNewFiles += len(files)
	}

	if totalNewFiles > 0 {
		fmt.Fprintf(os.Stderr, "\n⚡ New doc files not in manifest (%d):\n", totalNewFiles)
		for repo, files := range result.NewFiles {
			for _, f := range files {
				fmt.Fprintf(os.Stderr, "  → %s: %s\n", repo, f)
			}
		}
	} else {
		fmt.Fprintln(os.Stderr, "✓ No new doc files found in tracked repos.")
	}

	// Write GitHub Actions step summary if available.
	if summaryPath := os.Getenv("GITHUB_STEP_SUMMARY"); summaryPath != "" {
		f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
		if err != nil {
			return
		}
		defer f.Close()

		fmt.Fprintln(f, "## 🔍 Content Discovery Report")
		fmt.Fprintf(f, "| Metric | Count |\n|---|---|\n")
		fmt.Fprintf(f, "| Org repos | %d |\n", result.TotalRepos)
		fmt.Fprintf(f, "| Tracked repos | %d |\n", result.TrackedRepos)
		fmt.Fprintf(f, "| New repos found | %d |\n", len(result.NewRepos))
		fmt.Fprintf(f, "| New doc files found | %d |\n\n", totalNewFiles)

		if len(result.NewRepos) > 0 {
			fmt.Fprintln(f, "### New Repositories")
			fmt.Fprintln(f, "These repos exist in the org but are not in `sync-config.yaml`:")
			fmt.Fprintln(f, "")
			for _, repo := range result.NewRepos {
				fmt.Fprintf(f, "- [`%s`](https://github.com/%s)\n", repo, repo)
			}
			fmt.Fprintln(f, "")
		}

		if totalNewFiles > 0 {
			fmt.Fprintln(f, "### New Documentation Files")
			fmt.Fprintln(f, "These files exist in tracked repos but are not declared in `sync-config.yaml`:")
			fmt.Fprintln(f, "")
			for repo, files := range result.NewFiles {
				for _, file := range files {
					fmt.Fprintf(f, "- `%s`: [`%s`](https://github.com/%s/blob/main/%s)\n", repo, file, repo, file)
				}
			}
			fmt.Fprintln(f, "")
		}

		if len(result.NewRepos) == 0 && totalNewFiles == 0 {
			fmt.Fprintln(f, "✅ All repos and doc files are tracked. Nothing new to add.")
		}
	}

	// Write discovery counts to GITHUB_OUTPUT for downstream steps.
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

// ---------------------------------------------------------------------------
// CLI
// ---------------------------------------------------------------------------

func main() {
	configPath := flag.String("config", "sync-config.yaml", "Path to sync manifest")
	write := flag.Bool("write", false, "Apply changes to disk (default: dry-run)")
	repoFilter := flag.String("repo", "", "Only sync this repo (e.g. complytime/complyctl)")
	discover := flag.Bool("discover", false, "Scan org for new repos and untracked doc files (report only)")
	enhance := flag.Bool("enhance", false, "Use LLM to beautify/improve content formatting (requires OPENAI_API_KEY)")
	timeout := flag.Duration("timeout", 2*time.Minute, "HTTP timeout for all operations")
	flag.Parse()

	// Structured logging.
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))

	// Load configuration.
	data, err := os.ReadFile(*configPath)
	if err != nil {
		slog.Error("failed to read config", "path", *configPath, "error", err)
		os.Exit(1)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		slog.Error("failed to parse config", "path", *configPath, "error", err)
		os.Exit(1)
	}

	// Apply defaults.
	if cfg.Defaults.Branch == "" {
		cfg.Defaults.Branch = "main"
	}
	if cfg.Defaults.GitHubAPI == "" {
		cfg.Defaults.GitHubAPI = "https://api.github.com"
	}

	// Optional GitHub token from environment.
	token := os.Getenv("GITHUB_TOKEN")

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := &http.Client{Timeout: 30 * time.Second}

	// Discovery mode: scan for new repos / untracked files, then exit.
	if *discover {
		result, err := runDiscovery(ctx, client, cfg, token)
		if err != nil {
			slog.Error("discovery failed", "error", err)
			os.Exit(1)
		}
		printDiscoveryReport(result)
		return
	}

	// LLM enhancement config (optional).
	var llmCfg *LLMConfig
	if *enhance {
		llmCfg = llmConfigFromEnv()
		if llmCfg == nil {
			slog.Error("--enhance requires OPENAI_API_KEY environment variable")
			os.Exit(1)
		}
		slog.Info("LLM enhancement enabled", "model", llmCfg.Model, "base_url", llmCfg.BaseURL)
	}

	// Filter sources if --repo is set.
	sources := cfg.Sources
	if *repoFilter != "" {
		filtered := make([]Source, 0)
		for _, s := range sources {
			if s.Repo == *repoFilter {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			slog.Error("no matching source found", "repo", *repoFilter)
			os.Exit(1)
		}
		sources = filtered
	}

	// Process all sources concurrently.
	var (
		mu         sync.Mutex
		allResults []SyncResult
		wg         sync.WaitGroup
	)

	for _, src := range sources {
		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			results := syncSource(ctx, client, cfg, s, token, *write, llmCfg)
			mu.Lock()
			allResults = append(allResults, results...)
			mu.Unlock()
		}(src)
	}

	wg.Wait()

	// Summary.
	var changed, skipped, errored int
	for _, r := range allResults {
		switch {
		case r.Error != nil:
			errored++
		case r.Changed:
			changed++
		case r.Skipped:
			skipped++
		}
	}

	mode := "DRY-RUN"
	if *write {
		mode = "WRITE"
	}

	fmt.Fprintf(os.Stderr, "\n── sync-content summary (%s) ──\n", mode)
	fmt.Fprintf(os.Stderr, "  sources:   %d\n", len(sources))
	fmt.Fprintf(os.Stderr, "  changed:   %d\n", changed)
	fmt.Fprintf(os.Stderr, "  unchanged: %d\n", skipped)
	fmt.Fprintf(os.Stderr, "  errors:    %d\n", errored)

	if errored > 0 {
		fmt.Fprintln(os.Stderr, "\nFiles with errors:")
		for _, r := range allResults {
			if r.Error != nil {
				fmt.Fprintf(os.Stderr, "  ✗ %s/%s → %s: %v\n", r.Repo, r.Src, r.Dest, r.Error)
			}
		}
	}

	if changed > 0 {
		fmt.Fprintln(os.Stderr, "\nFiles changed:")
		for _, r := range allResults {
			if r.Changed && r.Error == nil {
				fmt.Fprintf(os.Stderr, "  ✓ %s/%s → %s\n", r.Repo, r.Src, r.Dest)
			}
		}
	}

	// Set GitHub Actions output if running in CI.
	if ghOutput := os.Getenv("GITHUB_OUTPUT"); ghOutput != "" {
		f, err := os.OpenFile(ghOutput, os.O_APPEND|os.O_WRONLY, 0o644)
		if err == nil {
			defer f.Close()
			hasChanges := "false"
			if changed > 0 {
				hasChanges = "true"
			}
			fmt.Fprintf(f, "has_changes=%s\n", hasChanges)
			fmt.Fprintf(f, "changed_count=%d\n", changed)
			fmt.Fprintf(f, "error_count=%d\n", errored)
		}
	}

	if errored > 0 {
		os.Exit(1)
	}
}
