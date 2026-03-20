package github

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devs-group/skillbox/internal/registry"
	"github.com/devs-group/skillbox/internal/skill"
	"github.com/devs-group/skillbox/internal/store"
)

// cacheEntry holds a cached API response with a TTL.
type cacheEntry struct {
	data      any
	expiresAt time.Time
}

const cacheTTL = 5 * time.Minute

// MarketplaceService provides GitHub-based skill discovery and installation.
type MarketplaceService struct {
	githubToken string
	httpClient  *http.Client
	reg         *registry.Registry
	store       *store.Store
	cache       sync.Map
}

// SearchResult represents a single result from GitHub Code Search.
// Field names match VectorChat's MarketplaceSearchResult for compatibility.
type SearchResult struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	RepoOwner   string `json:"repo_owner"`
	RepoName    string `json:"repo_name"`
	FilePath    string `json:"file_path"`
	Stars       int    `json:"stars"`
	HTMLURL     string `json:"html_url"`
}

// SearchResponse wraps the paginated search results.
// Field names match VectorChat's MarketplaceSearchResponse.
type SearchResponse struct {
	Results    []SearchResult `json:"results"`
	TotalCount int            `json:"total_count"`
	HasMore    bool           `json:"has_more"`
}

// FileEntry represents a file within a skill directory on GitHub.
type FileEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int    `json:"size"`
	Type string `json:"type"` // "file" or "dir"
}

// PreviewResponse contains the parsed skill metadata and sibling files.
// Field names match VectorChat's MarketplacePreviewResponse.
type PreviewResponse struct {
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Version      string      `json:"version"`
	Lang         string      `json:"lang"`
	Instructions string      `json:"instructions"`
	RepoOwner    string      `json:"repo_owner"`
	RepoName     string      `json:"repo_name"`
	FilePath     string      `json:"file_path"`
	Files        []FileEntry `json:"files"`
}

// InstallRequest specifies which GitHub skill to install.
// Field names match VectorChat's MarketplaceInstallRequest.
type InstallRequest struct {
	RepoOwner string `json:"repo_owner"`
	RepoName  string `json:"repo_name"`
	FilePath  string `json:"file_path"`
	SkillName string `json:"skill_name"` // optional override for name collision
}

// InstallResponse contains the result of a successful installation.
type InstallResponse struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// NewMarketplaceService creates a new MarketplaceService.
func NewMarketplaceService(githubToken string, reg *registry.Registry, s *store.Store) *MarketplaceService {
	return &MarketplaceService{
		githubToken: githubToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		reg:   reg,
		store: s,
	}
}

// IsConfigured returns true if the GitHub token is set.
func (m *MarketplaceService) IsConfigured() bool {
	return m.githubToken != ""
}

// Search queries the GitHub Code Search API for repositories containing SKILL.md files.
func (m *MarketplaceService) Search(ctx context.Context, query string, page int) (*SearchResponse, error) {
	if page < 1 {
		page = 1
	}

	cacheKey := fmt.Sprintf("search:%s:%d", query, page)
	if cached, ok := m.getCache(cacheKey); ok {
		return cached.(*SearchResponse), nil
	}

	// Build GitHub Code Search query: search for SKILL.md files matching the query.
	ghQuery := fmt.Sprintf("filename:SKILL.md %s", query)
	u := fmt.Sprintf("https://api.github.com/search/code?q=%s&page=%d&per_page=20",
		url.QueryEscape(ghQuery), page)

	body, err := m.githubGet(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("github search: %w", err)
	}

	var raw struct {
		TotalCount int `json:"total_count"`
		Items      []struct {
			Name       string `json:"name"`
			Path       string `json:"path"`
			HTMLURL    string `json:"html_url"`
			Repository struct {
				FullName        string `json:"full_name"`
				Description     string `json:"description"`
				StargazersCount int    `json:"stargazers_count"`
			} `json:"repository"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}

	results := make([]SearchResult, 0, len(raw.Items))
	for _, item := range raw.Items {
		parts := strings.SplitN(item.Repository.FullName, "/", 2)
		if len(parts) != 2 {
			continue
		}
		name := extractSkillName(item.Path)
		results = append(results, SearchResult{
			Name:        name,
			Description: item.Repository.Description,
			RepoOwner:   parts[0],
			RepoName:    parts[1],
			FilePath:    item.Path,
			Stars:       item.Repository.StargazersCount,
			HTMLURL:     item.HTMLURL,
		})
	}

	perPage := 30
	resp := &SearchResponse{
		Results:    results,
		TotalCount: raw.TotalCount,
		HasMore:    raw.TotalCount > page*perPage,
	}
	m.setCache(cacheKey, resp)
	return resp, nil
}

// Preview fetches a SKILL.md from GitHub, parses its frontmatter, and lists sibling files.
func (m *MarketplaceService) Preview(ctx context.Context, owner, repo, filePath string) (*PreviewResponse, error) {
	cacheKey := fmt.Sprintf("preview:%s/%s/%s", owner, repo, filePath)
	if cached, ok := m.getCache(cacheKey); ok {
		return cached.(*PreviewResponse), nil
	}

	// Fetch the raw SKILL.md content.
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/HEAD/%s",
		url.PathEscape(owner), url.PathEscape(repo), filePath)
	content, err := m.githubGet(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetch SKILL.md: %w", err)
	}

	parsed, err := skill.ParseSkillMD(content)
	if err != nil {
		return nil, fmt.Errorf("parse SKILL.md: %w", err)
	}

	// List sibling files in the same directory.
	dir := path.Dir(filePath)
	if dir == "." {
		dir = ""
	}

	contentsURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s",
		url.PathEscape(owner), url.PathEscape(repo), dir)
	contentsBody, err := m.githubGet(ctx, contentsURL)
	if err != nil {
		slog.Warn("failed to list sibling files", "error", err)
		// Non-fatal: return preview without file list.
		contentsBody = []byte("[]")
	}

	var ghFiles []struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Size int    `json:"size"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(contentsBody, &ghFiles); err != nil {
		slog.Warn("failed to parse contents response", "error", err)
	}

	files := make([]FileEntry, 0, len(ghFiles))
	for _, f := range ghFiles {
		files = append(files, FileEntry{
			Name: f.Name,
			Path: f.Path,
			Size: f.Size,
			Type: f.Type,
		})
	}

	resp := &PreviewResponse{
		Name:         parsed.Name,
		Description:  parsed.Description,
		Version:      parsed.Version,
		Lang:         parsed.Lang,
		Instructions: parsed.Instructions,
		RepoOwner:    owner,
		RepoName:     repo,
		FilePath:     filePath,
		Files:        files,
	}
	m.setCache(cacheKey, resp)
	return resp, nil
}

// Install fetches a skill from GitHub and uploads it to the tenant's registry.
func (m *MarketplaceService) Install(ctx context.Context, tenantID string, req *InstallRequest) (*InstallResponse, error) {
	// 1. Fetch SKILL.md from GitHub.
	skillPath := req.FilePath
	if !strings.HasSuffix(skillPath, "SKILL.md") {
		skillPath = path.Join(skillPath, "SKILL.md")
	}

	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/HEAD/%s",
		url.PathEscape(req.RepoOwner), url.PathEscape(req.RepoName), skillPath)
	content, err := m.githubGet(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetch SKILL.md: %w", err)
	}

	// 2. Parse SKILL.md.
	parsed, err := skill.ParseSkillMD(content)
	if err != nil {
		return nil, fmt.Errorf("parse SKILL.md: %w", err)
	}

	// 3. Fetch all sibling files from GitHub.
	dir := path.Dir(skillPath)
	if dir == "." {
		dir = ""
	}

	contentsURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s",
		url.PathEscape(req.RepoOwner), url.PathEscape(req.RepoName), dir)
	contentsBody, err := m.githubGet(ctx, contentsURL)
	if err != nil {
		return nil, fmt.Errorf("list skill files: %w", err)
	}

	var ghFiles []struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		Size        int    `json:"size"`
		Type        string `json:"type"`
		DownloadURL string `json:"download_url"`
	}
	if err := json.Unmarshal(contentsBody, &ghFiles); err != nil {
		return nil, fmt.Errorf("parse contents listing: %w", err)
	}

	// 4. Build zip archive.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for _, f := range ghFiles {
		if f.Type != "file" || f.DownloadURL == "" {
			continue
		}

		fileContent, err := m.githubGet(ctx, f.DownloadURL)
		if err != nil {
			slog.Warn("failed to fetch file, skipping", "path", f.Path, "error", err)
			continue
		}

		// For SKILL.md, use the original content we already fetched and parsed.
		if f.Name == "SKILL.md" {
			fileContent = content
		}

		w, err := zw.Create(f.Name)
		if err != nil {
			return nil, fmt.Errorf("create zip entry %s: %w", f.Name, err)
		}
		if _, err := w.Write(fileContent); err != nil {
			return nil, fmt.Errorf("write zip entry %s: %w", f.Name, err)
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finalize zip: %w", err)
	}

	zipBytes := buf.Bytes()

	// 5. Upload to registry.
	err = m.reg.Upload(ctx, tenantID, parsed.Name, parsed.Version, bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("upload to registry: %w", err)
	}

	// 6. Upsert metadata.
	err = m.store.UpsertSkill(ctx, &store.SkillRecord{
		TenantID:    tenantID,
		Name:        parsed.Name,
		Version:     parsed.Version,
		Description: parsed.Description,
		Lang:        parsed.Lang,
	})
	if err != nil {
		// Non-fatal: skill is already in the registry.
		slog.Warn("failed to upsert skill metadata", "error", err)
	}

	return &InstallResponse{
		Name:        parsed.Name,
		Version:     parsed.Version,
		Description: parsed.Description,
	}, nil
}

// githubGet performs an authenticated GET request to the GitHub API.
func (m *MarketplaceService) githubGet(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("User-Agent", "Skillbox-Marketplace/1.0")
	if m.githubToken != "" {
		req.Header.Set("Authorization", "Bearer "+m.githubToken)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MiB cap
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("not found: %s", rawURL)
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		// Check rate limit headers.
		if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
			if ts, err := strconv.ParseInt(reset, 10, 64); err == nil {
				resetTime := time.Unix(ts, 0)
				return nil, fmt.Errorf("GitHub rate limit exceeded, resets at %s", resetTime.Format(time.RFC3339))
			}
		}
		return nil, fmt.Errorf("GitHub API rate limit exceeded (status %d)", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// getCache retrieves a value from the TTL cache.
func (m *MarketplaceService) getCache(key string) (any, bool) {
	v, ok := m.cache.Load(key)
	if !ok {
		return nil, false
	}
	entry := v.(*cacheEntry)
	if time.Now().After(entry.expiresAt) {
		m.cache.Delete(key)
		return nil, false
	}
	return entry.data, true
}

// setCache stores a value in the TTL cache.
func (m *MarketplaceService) setCache(key string, data any) {
	m.cache.Store(key, &cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(cacheTTL),
	})
}

// extractSkillName derives a skill name from the SKILL.md file path.
// e.g., "skills/data-analysis/SKILL.md" → "data-analysis"
func extractSkillName(filePath string) string {
	dir := path.Dir(filePath)
	if dir == "." || dir == "" {
		return "imported-skill"
	}
	return path.Base(dir)
}
