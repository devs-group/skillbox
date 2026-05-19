package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/github"
	"github.com/devs-group/skillbox/internal/scanner"
)

// isInvalidManifestErr returns true when err is a SKILL.md parse/validation
// failure rather than an upstream GitHub fetch failure. These should surface
// as 422 so the UI can show a "not a valid skill" message instead of a
// generic gateway error.
func isInvalidManifestErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "parse SKILL.md") ||
		strings.Contains(msg, "invalid skill:") ||
		strings.Contains(msg, "frontmatter delimiter")
}

// SearchGitHub handles GET /v1/github/search?q=...&page=1.
// Searches GitHub for repositories containing SKILL.md files.
// No authentication required (uses server-side GitHub token).
func SearchGitHub(m *github.MarketplaceService) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("q")
		if query == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "query parameter 'q' is required")
			return
		}

		page := 1
		if p := c.Query("page"); p != "" {
			if v, err := parsePositiveInt(p); err == nil {
				page = v
			}
		}

		results, err := m.Search(c.Request.Context(), query, page)
		if err != nil {
			response.RespondError(c, http.StatusBadGateway, "github_error", err.Error())
			return
		}

		c.JSON(http.StatusOK, results)
	}
}

// PreviewGitHub handles GET /v1/github/preview?owner=...&repo=...&path=...
// Fetches and parses a SKILL.md from GitHub, returning metadata and sibling files.
// No authentication required.
func PreviewGitHub(m *github.MarketplaceService) gin.HandlerFunc {
	return func(c *gin.Context) {
		owner := c.Query("owner")
		repo := c.Query("repo")
		filePath := c.Query("path")

		if owner == "" || repo == "" || filePath == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request",
				"query parameters 'owner', 'repo', and 'path' are required")
			return
		}

		preview, err := m.Preview(c.Request.Context(), owner, repo, filePath)
		if err != nil {
			if isInvalidManifestErr(err) {
				response.RespondError(c, http.StatusUnprocessableEntity, "invalid_skill_manifest", err.Error())
				return
			}
			response.RespondError(c, http.StatusBadGateway, "github_error", err.Error())
			return
		}

		c.JSON(http.StatusOK, preview)
	}
}

// InstallFromGitHub handles POST /v1/github/install.
// Fetches a skill from GitHub, submits it to the pending prefix for async
// scanning, and returns 202 Accepted.
func InstallFromGitHub(m *github.MarketplaceService, worker *scanner.Worker) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)

		var req github.InstallRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body: "+err.Error())
			return
		}

		if req.RepoOwner == "" || req.RepoName == "" || req.FilePath == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request",
				"'repo_owner', 'repo_name', and 'file_path' are required")
			return
		}

		result, err := m.Install(c.Request.Context(), tenantID, &req)
		if err != nil {
			if strings.Contains(err.Error(), "skill_blocked:") {
				_, after, _ := strings.Cut(err.Error(), "skill_blocked: ")
				response.RespondError(c, http.StatusForbidden, "skill_blocked", "Skill "+after)
				return
			}
			if strings.Contains(err.Error(), "skill_exists:") {
				_, after, _ := strings.Cut(err.Error(), "skill_exists: ")
				response.RespondError(c, http.StatusConflict, "skill_exists", "Skill "+after)
				return
			}
			if isInvalidManifestErr(err) {
				response.RespondError(c, http.StatusUnprocessableEntity, "invalid_skill_manifest",
					"not a valid skill manifest: "+err.Error())
				return
			}
			response.RespondError(c, http.StatusBadGateway, "github_error",
				"failed to install skill from GitHub: "+err.Error())
			return
		}

		// Queue async scan job.
		if worker != nil {
			worker.Submit(scanner.ScanJob{
				TenantID: tenantID,
				Skill:    result.Name,
				Version:  result.Version,
			})
		}

		c.JSON(http.StatusAccepted, result)
	}
}

// parsePositiveInt parses a string as a positive integer.
func parsePositiveInt(s string) (int, error) {
	var v int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number")
		}
		v = v*10 + int(c-'0')
	}
	if v <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return v, nil
}
