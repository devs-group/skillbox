package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/store"
)

// ListMarketplaceSkills handles GET /v1/marketplace/skills?q=&limit=&offset=.
// Returns paginated public skills from the marketplace.
func ListMarketplaceSkills(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("q")

		limit := 20
		if l := c.Query("limit"); l != "" {
			if v, err := strconv.Atoi(l); err == nil && v > 0 {
				limit = v
			}
		}

		offset := 0
		if o := c.Query("offset"); o != "" {
			if v, err := strconv.Atoi(o); err == nil && v >= 0 {
				offset = v
			}
		}

		skills, total, err := s.ListMarketplaceSkills(c.Request.Context(), query, limit, offset)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to list marketplace skills")
			return
		}

		if skills == nil {
			skills = []store.SkillRecord{}
		}

		c.JSON(http.StatusOK, gin.H{
			"skills": skills,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

// marketplaceSkillResponse extends SkillRecord with approval status.
type marketplaceSkillResponse struct {
	store.SkillRecord
	IsApproved *bool `json:"is_approved,omitempty"`
}

// GetMarketplaceSkill handles GET /v1/marketplace/skills/:name.
// Returns detail for a public skill. If the user is authenticated, includes
// the approval status for their tenant.
func GetMarketplaceSkill(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if name == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "skill name is required")
			return
		}

		rec, err := s.GetMarketplaceSkill(c.Request.Context(), name)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "skill not found")
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to retrieve skill")
			return
		}

		resp := marketplaceSkillResponse{
			SkillRecord: *rec,
		}

		// If user is authenticated, check approval status for their tenant.
		if tenantID, exists := c.Get(middleware.ContextKeyTenantID); exists {
			approved, err := s.IsSkillApprovedForTenant(c.Request.Context(), tenantID.(string), name)
			if err == nil {
				resp.IsApproved = &approved
			}
		}

		c.JSON(http.StatusOK, resp)
	}
}
