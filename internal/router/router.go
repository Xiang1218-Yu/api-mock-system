// Package router is the wiring layer. It builds the gin engine with the middleware
// stack and mounts every handler at its path. No business logic lives here —
// adding a route is the only reason to edit this file.
package router

import (
	"net/http"
	"strings"

	"api-mock-system/internal/aggregatehandler"
	"api-mock-system/internal/apihandler"
	"api-mock-system/internal/auth"
	"api-mock-system/internal/dashboardhandler"
	"api-mock-system/internal/debughandler"
	"api-mock-system/internal/dochandler"
	"api-mock-system/internal/middleware"
	"api-mock-system/internal/mockhandler"
	"api-mock-system/internal/projecthandler"
	"api-mock-system/internal/userhandler"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Deps holds every handler the router needs to mount.
// Composed as a struct so the caller wires them explicitly; no hidden globals.
type Deps struct {
	Users     *userhandler.Handler
	Projects  *projecthandler.Handler
	APIs      *apihandler.Handler
	Mock      *mockhandler.Handler
	Aggregate *aggregatehandler.Handler
	Docs      *dochandler.Handler
	Debug     *debughandler.Handler
	Dashboard *dashboardhandler.Handler
	StaticFS  http.FileSystem // optional: embedded web assets
}

// RateConfig carries the token-bucket limits for the API rate limiter (spec §3.2).
// A zero RPS disables limiting.
type RateConfig struct {
	RPS   float64
	Burst int
}

// New assembles the gin engine. rate configures per-key rate limiting; pass a
// zero RPS to disable.
func New(deps Deps, a *auth.Auth, rate RateConfig, log *zap.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(
		middleware.Recover(log),
		middleware.Logger(log),
		middleware.CORS(),
	)

	// Health check — no auth, for liveness probes.
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	// --- Public auth routes ---
	authGrp := r.Group("/api/v1/auth")
	{
		authGrp.POST("/register", deps.Users.Register)
		authGrp.POST("/login", deps.Users.Login)
	}

	// --- Authenticated API routes ---
	api := r.Group("/api/v1")
	api.Use(middleware.RequireAuth(a))
	if rate.RPS > 0 {
		// Per-user limiting on the management API.
		api.Use(middleware.RateLimit(rate.RPS, rate.Burst, func(c *gin.Context) string {
			if uid, ok := c.Get(string(middleware.UserIDKey)); ok {
				if s, ok := uid.(string); ok && s != "" {
					return "u:" + strings.TrimSpace(s)
				}
			}
			return "ip:" + c.ClientIP()
		}))
	}
	{
		api.GET("/auth/me", deps.Users.Me)

		// Projects + members
		// Projects + members. Parameter name ":projectId" is consistent across
		// every /projects/.../... route so Gin's wildcard tree never conflicts.
		api.POST("/projects", deps.Projects.Create)
		api.GET("/projects", deps.Projects.List)
		api.GET("/projects/:projectId", deps.Projects.Get)
		api.PUT("/projects/:projectId", deps.Projects.Update)
		api.DELETE("/projects/:projectId", deps.Projects.Delete)
		api.POST("/projects/:projectId/members", deps.Projects.InviteMember)
		api.DELETE("/projects/:projectId/members/:userId", deps.Projects.RemoveMember)
		api.GET("/projects/:projectId/members", deps.Projects.ListMembers)
		api.GET("/projects/:projectId/stats", deps.Dashboard.ProjectStats)

		// APIs (per project + by id)
		api.POST("/projects/:projectId/apis", deps.APIs.Create)
		api.GET("/projects/:projectId/apis", deps.APIs.List)
		api.GET("/apis/:id", deps.APIs.Get)
		api.PUT("/apis/:id", deps.APIs.Update)
		api.DELETE("/apis/:id", deps.APIs.Delete)
		api.POST("/apis/:id/publish", deps.APIs.Publish)
		api.GET("/apis/:id/versions", deps.APIs.Versions)
		api.POST("/apis/:id/rollback/:version", deps.APIs.Rollback)

		// Mock overrides (management; the live route is public below)
		api.POST("/apis/:id/mock/override", deps.Mock.SetOverride)
		api.DELETE("/apis/:id/mock/override", deps.Mock.ClearOverride)
		api.GET("/apis/:id/mock/override", deps.Mock.ListOverrides)

		// Aggregates
		api.POST("/projects/:projectId/aggregates", deps.Aggregate.Create)
		api.GET("/projects/:projectId/aggregates", deps.Aggregate.List)
		api.PUT("/aggregates/:id", deps.Aggregate.Update)
		api.DELETE("/aggregates/:id", deps.Aggregate.Delete)

		// Docs + debug
		api.GET("/projects/:projectId/docs/openapi.json", deps.Docs.OpenAPIJSON)
		api.GET("/projects/:projectId/docs/openapi.yaml", deps.Docs.OpenAPIYAML)
		api.GET("/projects/:projectId/docs/preview", deps.Docs.Preview)
		api.POST("/apis/:id/debug", deps.Debug.Debug)
		api.GET("/apis/:id/debug/history", deps.Debug.History)

		// Dashboard trends/duration (spec §2.7)
		api.GET("/projects/:projectId/stats/trends", deps.Dashboard.Trends)
		api.GET("/projects/:projectId/stats/duration", deps.Dashboard.Duration)
	}

	// --- Live aggregate route (authenticated) ---
	agg := r.Group("/aggregate", middleware.RequireAuth(a))
	agg.Any("/:projectId/*path", deps.Aggregate.Serve)

	// --- Live mock route (public, optionally rate-limited per project) ---
	// Mounted last so it doesn't shadow the typed routes. The wildcard matches
	// any method + any path under /mock/:projectId. Limiting is keyed by project
	// so one noisy consumer can't crowd out others (spec §3.2).
	mockGrp := r.Group("/mock")
	if rate.RPS > 0 {
		mockGrp.Use(middleware.RateLimit(rate.RPS, rate.Burst, func(c *gin.Context) string {
			pid := c.Param("projectId")
			if pid == "" {
				return "ip:" + c.ClientIP()
			}
			return "p:" + strings.TrimSpace(pid)
		}))
	}
	mockGrp.Any("/:projectId/*path", deps.Mock.Serve)

	// --- Web UI: static assets + SPA fallback ---
	if deps.StaticFS != nil {
		// Serve bundled assets (app.js, app.css, index.html) from embed.FS.
		r.StaticFS("/static", deps.StaticFS)
		// SPA fallback: any unmatched non-API path returns the shell page so the
		// hash router can take over client-side. API/mock/aggregate 404s pass
		// through unchanged.
		r.NoRoute(func(c *gin.Context) {
			p := c.Request.URL.Path
			if strings.HasPrefix(p, "/api/") ||
				strings.HasPrefix(p, "/mock/") ||
				strings.HasPrefix(p, "/aggregate/") ||
				strings.HasPrefix(p, "/static/") {
				c.Data(http.StatusNotFound, "text/plain; charset=utf-8", []byte("not found"))
				return
			}
			f, err := deps.StaticFS.Open("/index.html")
			if err != nil {
				c.Data(http.StatusNotFound, "text/plain; charset=utf-8", []byte("not found"))
				return
			}
			defer f.Close()
			c.Data(http.StatusOK, "text/html; charset=utf-8", readAll(f))
		})
	}

	return r
}

// readAll drains an http.File into a byte slice for inline serving.
func readAll(f http.File) []byte {
	stat, err := f.Stat()
	if err != nil {
		b := make([]byte, 0)
		return b
	}
	buf := make([]byte, 0, stat.Size())
	tmp := make([]byte, 4096)
	for {
		n, err := f.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf
}
