package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/runner"
	"github.com/devs-group/skillbox/internal/store"
)

// createExecutionRequest is the JSON body for POST /v1/executions.
type createExecutionRequest struct {
	Skill   string            `json:"skill"`
	Version string            `json:"version"`
	Input   json.RawMessage   `json:"input"`
	Env     map[string]string `json:"env"`
}

// CreateExecution handles POST /v1/executions.
// It parses the request body, invokes the runner synchronously, and
// returns the full RunResult JSON. The "skill" field is required;
// "version" defaults to "latest" if omitted.
func CreateExecution(r *runner.Runner) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createExecutionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid JSON body: "+err.Error())
			return
		}

		if req.Skill == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "'skill' is required")
			return
		}

		if req.Version == "" {
			req.Version = "latest"
		}

		tenantID := middleware.GetTenantID(c)

		result, err := r.Run(c.Request.Context(), runner.RunRequest{
			Skill:    req.Skill,
			Version:  req.Version,
			Input:    req.Input,
			Env:      req.Env,
			TenantID: tenantID,
		})
		if err != nil {
			if errors.Is(err, runner.ErrSkillNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "skill not found: "+req.Skill+"@"+req.Version)
				return
			}
			if errors.Is(err, runner.ErrImageNotAllowed) {
				response.RespondError(c, http.StatusBadRequest, "image_not_allowed", "skill image is not in the allowlist")
				return
			}
			if errors.Is(err, runner.ErrTimeout) {
				response.RespondError(c, http.StatusGatewayTimeout, "timeout", "execution timed out")
				return
			}

			// Return a 500 with the error message for unexpected failures.
			errMsg := err.Error()
			c.JSON(http.StatusInternalServerError, runner.RunResult{
				Status: "failed",
				Error:  &errMsg,
			})
			return
		}

		c.JSON(http.StatusOK, result)
	}
}

// GetExecution handles GET /v1/executions/:id.
// It retrieves an execution record from the store and enforces tenant
// isolation: the caller's tenant must match the execution's tenant.
func GetExecution(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "execution id is required")
			return
		}

		tenantID := middleware.GetTenantID(c)

		exec, err := s.GetExecution(c.Request.Context(), id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "execution not found")
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to retrieve execution")
			return
		}

		// Enforce tenant isolation: a key must not see executions from
		// other tenants.
		if exec.TenantID != tenantID {
			response.RespondError(c, http.StatusNotFound, "not_found", "execution not found")
			return
		}

		c.JSON(http.StatusOK, exec)
	}
}

// GetExecutionLogs handles GET /v1/executions/:id/logs.
// It returns just the logs field as plain text.
func GetExecutionLogs(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "execution id is required")
			return
		}

		tenantID := middleware.GetTenantID(c)

		exec, err := s.GetExecution(c.Request.Context(), id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "execution not found")
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to retrieve execution")
			return
		}

		if exec.TenantID != tenantID {
			response.RespondError(c, http.StatusNotFound, "not_found", "execution not found")
			return
		}

		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(exec.Logs))
	}
}
