package api

import (
	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/handlers"
	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/artifacts"
	"github.com/devs-group/skillbox/internal/config"
	"github.com/devs-group/skillbox/internal/github"
	"github.com/devs-group/skillbox/internal/registry"
	"github.com/devs-group/skillbox/internal/runner"
	"github.com/devs-group/skillbox/internal/sandbox"
	"github.com/devs-group/skillbox/internal/scanner"
	"github.com/devs-group/skillbox/internal/store"
)

// NewRouter constructs the Gin engine with all routes, middleware, and
// handler bindings. It wires up:
//
//   - /health and /ready (unauthenticated, for orchestrators)
//   - /v1/* (authenticated via Bearer token, tenant-scoped)
//
// The router uses gin.New() (no default middleware) and explicitly adds
// Recovery and structured RequestLogger middleware so the log output is
// fully controlled.
func NewRouter(cfg *config.Config, s *store.Store, r *runner.Runner, reg *registry.Registry, sc scanner.Scanner, sm *sandbox.SessionManager, pipeline *scanner.Pipeline, worker *scanner.Worker, col ...*artifacts.Collector) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(middleware.CORSMiddleware())
	engine.Use(middleware.RequestLogger())

	// Health endpoints — no authentication required.
	engine.GET("/health", handlers.Health(s))
	engine.GET("/ready", handlers.Ready(s))

	// API v1 — requires valid API key and tenant context.
	v1 := engine.Group("/v1")
	v1.Use(middleware.AuthMiddleware(s, cfg.HydraAdminURL))
	v1.Use(middleware.TenantMiddleware())
	{
		// Execution endpoints
		v1.POST("/executions", handlers.CreateExecution(r))
		v1.GET("/executions/:id", handlers.GetExecution(s))
		v1.GET("/executions/:id/logs", handlers.GetExecutionLogs(s))

		// Skill management endpoints
		v1.POST("/skills", handlers.UploadSkill(reg, s, cfg, sc, worker))
		v1.POST("/skills/from-fields", handlers.CreateFromFields(reg, s, cfg, worker))
		v1.POST("/skills/validate", handlers.ValidateSkill(cfg, sc))
		v1.GET("/skills", handlers.ListSkills(s, reg))
		v1.GET("/skills/:name/:version", handlers.GetSkill(reg, s))
		v1.GET("/skills/:name/:version/files", handlers.GetSkillFiles(reg, s))
		v1.DELETE("/skills/:name/:version", handlers.DeleteSkill(reg, s))

		// Scanner admin endpoints — require admin token in addition to API key.
		admin := v1.Group("/admin")
		admin.Use(middleware.AdminMiddleware(cfg.AdminToken))
		{
			admin.GET("/scanner/stats", handlers.ScannerStats(pipeline))
			admin.GET("/scanner/patterns", handlers.ScannerGetPatterns(pipeline))
			admin.PUT("/scanner/patterns", handlers.ScannerSetPatterns(pipeline))
			admin.GET("/scanner/config", handlers.GetScannerConfig(s))
			admin.PUT("/scanner/config", handlers.UpdateScannerConfig(s))
			admin.GET("/skills/review", handlers.ListSkillsForReview(s))
			admin.PUT("/skills/:name/:version/review", handlers.ReviewSkill(reg, s))
		}

		// File/artifact endpoints
		if len(col) > 0 && col[0] != nil {
			filesHandler := handlers.NewFilesHandler(s, col[0], cfg.MaxSkillSize)
			files := v1.Group("/files")
			{
				files.POST("", filesHandler.Upload)
				files.GET("", filesHandler.List)
				files.GET("/:id", filesHandler.Get)
				files.GET("/:id/download", filesHandler.Download)
				files.PUT("/:id", filesHandler.Update)
				files.DELETE("/:id", filesHandler.Delete)
				files.GET("/:id/versions", filesHandler.Versions)
			}

			// Session workspace endpoints
			sessionsHandler := handlers.NewSessionsHandler(s, col[0])
			sessions := v1.Group("/sessions")
			{
				sessions.GET("/:external_id/files", sessionsHandler.ListFiles)
				sessions.GET("/:external_id/files/:filename", sessionsHandler.DownloadFile)
				sessions.DELETE("/:external_id/files/:filename", sessionsHandler.DeleteFile)
				sessions.DELETE("/:external_id", sessionsHandler.Delete)
			}
		}

		// Sandbox shell endpoints
		if sm != nil {
			sandboxHandler := handlers.NewSandboxHandler(sm, reg, s)
			sbGroup := v1.Group("/sandbox")
			sbGroup.Use(sandboxHandler.LimitBody())
			{
				sbGroup.POST("/execute", sandboxHandler.Execute)
				sbGroup.POST("/read-file", sandboxHandler.ReadFile)
				sbGroup.POST("/write-file", sandboxHandler.WriteFile)
				sbGroup.POST("/list-dir", sandboxHandler.ListDir)
				sbGroup.POST("/sync", sandboxHandler.Sync)
				sbGroup.POST("/upload-skill", sandboxHandler.UploadSkill)
				sbGroup.DELETE("/:session", sandboxHandler.Destroy)
			}
		}
	}

	// Marketplace (public — auth optional)
	marketplace := engine.Group("/v1/marketplace")
	marketplace.Use(middleware.OptionalAuthMiddleware(s, cfg.HydraAdminURL))
	{
		marketplace.GET("/skills", handlers.ListMarketplaceSkills(s))
		marketplace.GET("/skills/:name", handlers.GetMarketplaceSkill(s))
	}

	// User endpoints (authenticated)
	users := v1.Group("/users")
	{
		users.GET("/me", handlers.GetCurrentUser(s))
		users.GET("", middleware.RequireRole(s, "admin"), handlers.ListUsers(s))
		users.PUT("/:id/role", middleware.RequireRole(s, "admin"), handlers.UpdateUserRole(s))
	}

	// Group endpoints (admin only)
	groups := v1.Group("/groups")
	groups.Use(middleware.RequireRole(s, "admin"))
	{
		groups.POST("", handlers.CreateGroup(s))
		groups.GET("", handlers.ListGroups(s))
		groups.PUT("/:id", handlers.UpdateGroup(s))
		groups.DELETE("/:id", handlers.DeleteGroup(s))
		groups.POST("/:id/members", handlers.AddGroupMember(s))
		groups.DELETE("/:id/members/:userId", handlers.RemoveGroupMember(s))
		groups.GET("/:id/members", handlers.ListGroupMembers(s))
	}

	// Approval endpoints
	approvals := v1.Group("/approvals")
	{
		approvals.POST("", handlers.CreateApprovalRequest(s))
		approvals.GET("", handlers.ListApprovalRequests(s))
		approvals.PUT("/:id", middleware.RequireRole(s, "admin"), handlers.UpdateApprovalRequest(s))
	}

	// Invite endpoints
	invites := v1.Group("/invites")
	{
		invites.POST("", middleware.RequireRole(s, "admin"), handlers.CreateInviteCode(s))
		invites.GET("", middleware.RequireRole(s, "admin"), handlers.ListInviteCodes(s))
		invites.POST("/redeem", handlers.RedeemInviteCode(s))
	}

	// GitHub marketplace (public search/preview + authenticated install).
	if cfg.GitHubToken != "" {
		ghMarketplace := github.NewMarketplaceService(cfg.GitHubToken, reg, s)
		gh := engine.Group("/v1/github")
		{
			gh.GET("/search", handlers.SearchGitHub(ghMarketplace))
			gh.GET("/preview", handlers.PreviewGitHub(ghMarketplace))
		}
		ghAuth := engine.Group("/v1/github")
		ghAuth.Use(middleware.AuthMiddleware(s, cfg.HydraAdminURL))
		ghAuth.Use(middleware.TenantMiddleware())
		{
			ghAuth.POST("/install", handlers.InstallFromGitHub(ghMarketplace, worker))
		}
	}

	return engine
}
