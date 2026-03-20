package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/store"
)

// Context keys used by auth middleware. Both JWT and API-key paths set
// ContextKeyTenantID so downstream handlers work unchanged.
const (
	ContextKeyAPIKey   = "api_key"
	ContextKeyTenantID = "tenant_id"
	ContextKeyUserID   = "user_id"
	ContextKeyAuthType = "auth_type"
)

// AuthType constants identify which authentication path was used.
const (
	AuthTypeAPIKey = "api_key"
	AuthTypeJWT    = "jwt"
)

// isJWT checks whether a token looks like a JWT (has 2 dots and starts with eyJ).
func isJWT(token string) bool {
	return strings.HasPrefix(token, "eyJ") && strings.Count(token, ".") == 2
}

// AuthMiddleware extracts a Bearer token from the Authorization header and
// routes to either JWT validation (for Hydra-issued tokens) or SHA-256
// API-key lookup. Both paths set ContextKeyTenantID so downstream handlers
// work unchanged.
func AuthMiddleware(s *store.Store, hydraAdminURL string) gin.HandlerFunc {
	// Hydra token introspection endpoint
	introspectURL := strings.TrimRight(hydraAdminURL, "/") + "/admin/oauth2/introspect"

	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			response.RespondError(c, http.StatusUnauthorized, "unauthorized", "missing Authorization header")
			c.Abort()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.RespondError(c, http.StatusUnauthorized, "unauthorized", "Authorization header must use Bearer scheme")
			c.Abort()
			return
		}

		token := strings.TrimSpace(parts[1])
		if token == "" {
			response.RespondError(c, http.StatusUnauthorized, "unauthorized", "empty bearer token")
			c.Abort()
			return
		}

		if isJWT(token) {
			authenticateJWT(c, s, introspectURL, token)
		} else {
			authenticateAPIKey(c, s, token)
		}
	}
}

// OptionalAuthMiddleware is like AuthMiddleware but does not reject
// unauthenticated requests. If valid credentials are present, it sets
// context keys; otherwise it passes through silently. Used for endpoints
// like marketplace browse that enrich responses when authenticated.
func OptionalAuthMiddleware(s *store.Store, hydraAdminURL string) gin.HandlerFunc {
	introspectURL := strings.TrimRight(hydraAdminURL, "/") + "/admin/oauth2/introspect"

	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.Next()
			return
		}

		token := strings.TrimSpace(parts[1])
		if token == "" {
			c.Next()
			return
		}

		// Try to authenticate but don't abort on failure
		if isJWT(token) {
			authenticateJWTOptional(c, s, introspectURL, token)
		} else {
			authenticateAPIKeyOptional(c, s, token)
		}
		c.Next()
	}
}

// authenticateJWT validates a JWT token via Hydra introspection and resolves
// the user from the Skillbox users table.
func authenticateJWT(c *gin.Context, s *store.Store, introspectURL, token string) {
	claims, err := introspectToken(introspectURL, token)
	if err != nil {
		response.RespondError(c, http.StatusUnauthorized, "unauthorized", "invalid token")
		c.Abort()
		return
	}

	if !claims.Active {
		response.RespondError(c, http.StatusUnauthorized, "unauthorized", "token is inactive or expired")
		c.Abort()
		return
	}

	if claims.Sub == "" {
		response.RespondError(c, http.StatusUnauthorized, "unauthorized", "token missing subject claim")
		c.Abort()
		return
	}

	// Resolve user by Kratos identity ID (the sub claim)
	user, err := s.GetUserByKratosID(c.Request.Context(), claims.Sub)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			response.RespondError(c, http.StatusUnauthorized, "unauthorized", "user not found — redeem an invite code first")
			c.Abort()
			return
		}
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to resolve user")
		c.Abort()
		return
	}

	c.Set(ContextKeyUserID, user.ID)
	c.Set(ContextKeyTenantID, user.TenantID)
	c.Set(ContextKeyAuthType, AuthTypeJWT)
	c.Next()
}

// authenticateJWTOptional is like authenticateJWT but silently skips on failure.
func authenticateJWTOptional(c *gin.Context, s *store.Store, introspectURL, token string) {
	claims, err := introspectToken(introspectURL, token)
	if err != nil || !claims.Active || claims.Sub == "" {
		return
	}

	user, err := s.GetUserByKratosID(c.Request.Context(), claims.Sub)
	if err != nil {
		return
	}

	c.Set(ContextKeyUserID, user.ID)
	c.Set(ContextKeyTenantID, user.TenantID)
	c.Set(ContextKeyAuthType, AuthTypeJWT)
}

// authenticateAPIKey validates a token via SHA-256 hash lookup (existing logic).
func authenticateAPIKey(c *gin.Context, s *store.Store, token string) {
	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])

	key, err := s.GetAPIKeyByHash(c.Request.Context(), hashHex)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			response.RespondError(c, http.StatusUnauthorized, "unauthorized", "invalid or revoked API key")
			c.Abort()
			return
		}
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to validate API key")
		c.Abort()
		return
	}

	if key.RevokedAt != nil {
		response.RespondError(c, http.StatusUnauthorized, "unauthorized", "invalid or revoked API key")
		c.Abort()
		return
	}

	c.Set(ContextKeyAPIKey, key)
	c.Set(ContextKeyTenantID, key.TenantID)
	c.Set(ContextKeyAuthType, AuthTypeAPIKey)
	c.Next()
}

// authenticateAPIKeyOptional is like authenticateAPIKey but silently skips on failure.
func authenticateAPIKeyOptional(c *gin.Context, s *store.Store, token string) {
	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])

	key, err := s.GetAPIKeyByHash(c.Request.Context(), hashHex)
	if err != nil || key.RevokedAt != nil {
		return
	}

	c.Set(ContextKeyAPIKey, key)
	c.Set(ContextKeyTenantID, key.TenantID)
	c.Set(ContextKeyAuthType, AuthTypeAPIKey)
}

// introspectionResponse represents the response from Hydra's token introspection endpoint.
type introspectionResponse struct {
	Active   bool   `json:"active"`
	Sub      string `json:"sub"`
	ClientID string `json:"client_id"`
	Scope    string `json:"scope"`
	Exp      int64  `json:"exp"`
	Iat      int64  `json:"iat"`
}

// introspectToken calls Hydra's introspection endpoint to validate an access token.
func introspectToken(introspectURL, token string) (*introspectionResponse, error) {
	body := strings.NewReader("token=" + token)
	req, err := http.NewRequest(http.MethodPost, introspectURL, body)
	if err != nil {
		return nil, fmt.Errorf("create introspection request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("introspection request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("introspection returned %d", resp.StatusCode)
	}

	var result introspectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode introspection response: %w", err)
	}
	return &result, nil
}
