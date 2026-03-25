package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/store"
)

// GetScannerConfig handles GET /v1/admin/scanner/config.
// Returns the scanner configuration for the tenant.
func GetScannerConfig(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)

		cfg, err := s.GetScannerConfig(c.Request.Context(), tenantID)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to get scanner config")
			return
		}

		c.JSON(http.StatusOK, cfg)
	}
}

// updateScannerConfigRequest is the JSON body for PUT /v1/admin/scanner/config.
type updateScannerConfigRequest struct {
	ApprovalPolicy *string `json:"approval_policy"`
	Tier1Enabled   *bool   `json:"tier1_enabled"`
	Tier2Enabled   *bool   `json:"tier2_enabled"`
	Tier3Enabled   *bool   `json:"tier3_enabled"`
	Tier3APIKey    *string `json:"tier3_api_key"`
	Tier3Model     *string `json:"tier3_model"`
}

// UpdateScannerConfig handles PUT /v1/admin/scanner/config.
// Updates the scanner configuration for the tenant.
func UpdateScannerConfig(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)

		var req updateScannerConfigRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body: "+err.Error())
			return
		}

		// Validate approval policy if provided.
		if req.ApprovalPolicy != nil {
			switch *req.ApprovalPolicy {
			case store.ApprovalPolicyAuto, store.ApprovalPolicyAlways, store.ApprovalPolicyNone:
				// valid
			default:
				response.RespondError(c, http.StatusBadRequest, "bad_request",
					"approval_policy must be 'auto', 'always', or 'none'")
				return
			}
		}

		// Get current config, then apply partial updates.
		current, err := s.GetScannerConfig(c.Request.Context(), tenantID)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to get current config")
			return
		}

		if req.ApprovalPolicy != nil {
			current.ApprovalPolicy = *req.ApprovalPolicy
		}
		if req.Tier1Enabled != nil {
			current.Tier1Enabled = *req.Tier1Enabled
		}
		if req.Tier2Enabled != nil {
			current.Tier2Enabled = *req.Tier2Enabled
		}
		if req.Tier3Enabled != nil {
			current.Tier3Enabled = *req.Tier3Enabled
		}
		if req.Tier3APIKey != nil {
			current.Tier3APIKey = req.Tier3APIKey
		}
		if req.Tier3Model != nil {
			current.Tier3Model = *req.Tier3Model
		}

		if err := s.UpsertScannerConfig(c.Request.Context(), current); err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to update scanner config")
			return
		}

		c.JSON(http.StatusOK, current)
	}
}
