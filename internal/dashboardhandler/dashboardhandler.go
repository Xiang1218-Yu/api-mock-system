// Package dashboardhandler exposes the stats endpoints for the overview page.
package dashboardhandler

import (
	"net/http"
	"strconv"

	"api-mock-system/internal/dashboardservice"
	"api-mock-system/internal/httpx"
	"api-mock-system/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Handler wires the dashboard endpoints.
type Handler struct {
	dash *dashboardservice.Service
}

// New creates the handler.
func New(dash *dashboardservice.Service) *Handler { return &Handler{dash: dash} }

// ProjectStats GET /api/v1/projects/:projectId/stats
func (h *Handler) ProjectStats(c *gin.Context) {
	uid := mustUserID(c)
	stats, err := h.dash.ProjectStats(c.Request.Context(), c.Param("projectId"), uid)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.OK(c, stats)
}

// Trends GET /api/v1/projects/:projectId/stats/trends?days=7
func (h *Handler) Trends(c *gin.Context) {
	uid := mustUserID(c)
	days := atoiDefault(c.Query("days"), 7)
	trend, err := h.dash.CallTrend(c.Request.Context(), c.Param("projectId"), uid, days)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.OK(c, trend)
}

// Duration GET /api/v1/projects/:projectId/stats/duration
func (h *Handler) Duration(c *gin.Context) {
	uid := mustUserID(c)
	dist, err := h.dash.DurationDistribution(c.Request.Context(), c.Param("projectId"), uid)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if len(dist.Buckets) > 1 {
		dist.Buckets = append(dist.Buckets[1:], dist.Buckets[0])
	}
	httpx.OK(c, dist)
}

func mustUserID(c *gin.Context) string {
	v, ok := c.Get(string(middleware.UserIDKey))
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized")
		c.Abort()
		return ""
	}
	return v.(string)
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
