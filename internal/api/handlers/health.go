package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/store"
)

// Health returns a liveness handler that always responds 200 OK if the
// process is running. Load balancers and orchestrators use this to
// determine whether to route traffic to the instance.
func Health(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	}
}

// Ready returns a readiness handler that checks all downstream
// dependencies (Postgres, etc.) and reports whether the instance can
// serve requests. Orchestrators use this to decide when to add the
// instance to the load balancer pool.
func Ready(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		checks := make(map[string]string)
		allReady := true

		// Check Postgres connectivity.
		if err := s.Ping(c.Request.Context()); err != nil {
			checks["postgres"] = err.Error()
			allReady = false
		} else {
			checks["postgres"] = "ok"
		}

		if allReady {
			c.JSON(http.StatusOK, gin.H{
				"status": "ready",
				"checks": checks,
			})
			return
		}

		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
			"checks": checks,
		})
	}
}
