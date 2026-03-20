package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/store"
)

// CreateInviteCode handles POST /v1/invites (admin only).
// Creates a new invite code for the authenticated tenant.
func CreateInviteCode(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)

		var createdBy *string
		if userID, exists := c.Get(middleware.ContextKeyUserID); exists {
			uid := userID.(string)
			createdBy = &uid
		}

		inv, err := s.CreateInviteCode(c.Request.Context(), tenantID, createdBy)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to create invite code")
			return
		}

		c.JSON(http.StatusCreated, inv)
	}
}

// ListInviteCodes handles GET /v1/invites (admin only).
// Returns all invite codes for the authenticated tenant.
func ListInviteCodes(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)

		codes, err := s.ListInviteCodes(c.Request.Context(), tenantID)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to list invite codes")
			return
		}

		if codes == nil {
			codes = []*store.InviteCode{}
		}

		c.JSON(http.StatusOK, codes)
	}
}

// redeemInviteCodeRequest is the request body for RedeemInviteCode.
type redeemInviteCodeRequest struct {
	Code string `json:"code"`
}

// RedeemInviteCode handles POST /v1/invites/redeem.
// Redeems an invite code for the authenticated user.
func RedeemInviteCode(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get(middleware.ContextKeyUserID)
		if !exists {
			response.RespondError(c, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		var req redeemInviteCodeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body")
			return
		}

		if req.Code == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "code is required")
			return
		}

		inv, err := s.RedeemInviteCode(c.Request.Context(), req.Code, userID.(string))
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "invite code not found, already used, or expired")
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to redeem invite code")
			return
		}

		c.JSON(http.StatusOK, inv)
	}
}
