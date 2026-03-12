package api

import (
	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/handlers"
	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/artifacts"
	"github.com/devs-group/skillbox/internal/config"
	"github.com/devs-group/skillbox/internal/registry"
	"github.com/devs-group/skillbox/internal/runner"
	"github.com/devs-group/skillbox/internal/sandbox"
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
func NewRouter(cfg *config.Config, s *store.Store, r *runner.Runner, reg *registry.Registry, sm *sandbox.SessionManager, col ...*artifacts.Collector) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(middleware.RequestLogger())

	// Health endpoints — no authentication required.
	engine.GET("/health", handlers.Health(s))
	engine.GET("/ready", handlers.Ready(s))

	// API v1 — requires valid API key and tenant context.
	v1 := engine.Group("/v1")
	v1.Use(middleware.AuthMiddleware(s))
	v1.Use(middleware.TenantMiddleware())
	{
		// Execution endpoints
		v1.POST("/executions", handlers.CreateExecution(r))
		v1.GET("/executions/:id", handlers.GetExecution(s))
		v1.GET("/executions/:id/logs", handlers.GetExecutionLogs(s))

		// Skill management endpoints
		v1.POST("/skills", handlers.UploadSkill(reg, s, cfg))
		v1.GET("/skills", handlers.ListSkills(s, reg))
		v1.GET("/skills/:name/:version", handlers.GetSkill(reg, s))
		v1.GET("/skills/:name/:version/files", handlers.GetSkillFiles(reg, s))
		v1.DELETE("/skills/:name/:version", handlers.DeleteSkill(reg, s))

		// File/artifact endpoints
		if len(col) > 0 && col[0] != nil {
			filesHandler := handlers.NewFilesHandler(s, col[0])
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

	return engine
}
