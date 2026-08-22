// Package mockhandler serves the live mock routes AND the override-management
// endpoints. The live route is mounted under /mock/:projectId/*path and answers
// any HTTP method. Management endpoints live under /api/v1/apis/:id/mock.
package mockhandler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"api-mock-system/internal/apiservice"
	"api-mock-system/internal/calllogrepo"
	"api-mock-system/internal/httpx"
	"api-mock-system/internal/id"
	"api-mock-system/internal/middleware"
	"api-mock-system/internal/mockservice"
	"api-mock-system/internal/models"
	"api-mock-system/internal/projectservice"

	"github.com/gin-gonic/gin"
)

// Handler wires mock routes to the mock service.
type Handler struct {
	mock    *mockservice.Service
	calllog calllogrepo.Repository // nil = skip call logging
}

// New creates the handler. calllog may be nil to disable call logging.
func New(mock *mockservice.Service, calllog calllogrepo.Repository) *Handler {
	return &Handler{mock: mock, calllog: calllog}
}

// Serve ANY /mock/:projectId/*path — the public mock endpoint.
//
// Authentication is intentionally NOT required here: mock endpoints are meant
// to be called by frontend developers during development. A project marked
// private still serves mock data; access control at the project level applies
// to management, not to the mock runtime. (If a project needs to gate its mock
// endpoints, that is a future feature; see system.md §3.2 which scopes project
// isolation to the management surface.)
func (h *Handler) Serve(c *gin.Context) {
	projectID := c.Param("projectId")
	// Gin collapses the wildcard into "path" with a leading slash; trim it.
	path := c.Param("path")
	query := c.Request.URL.RawQuery
	body, _ := io.ReadAll(c.Request.Body)

	start := time.Now()
	resp, err := h.mock.Resolve(c.Request.Context(), projectID, c.Request.Method, path, query, string(body))
	// Real processing time excludes the artificial mock delay so the dashboard's
	// latency distribution reflects engine/handler cost, not the configured lag.
	totalMs := int(time.Since(start) / time.Millisecond)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, mockservice.ErrNotFound), errors.Is(err, mockservice.ErrNotPublished):
			status = http.StatusNotFound
		case errors.Is(err, mockservice.ErrInvalidRequest):
			status = http.StatusBadRequest
		}
		httpx.Error(c, status, err.Error())
		h.recordCall(projectID, "", c.Request.Method, path, status, max1(totalMs))
		return
	}
	for k, v := range resp.Headers {
		c.Header(k, v)
	}
	c.JSON(resp.StatusCode, resp.Body)
	// Strip the configured delay so logged duration reflects real cost.
	delayMs := int(resp.Delay / time.Millisecond)
	h.recordCall(projectID, resp.APIID, c.Request.Method, path, resp.StatusCode, max1(totalMs-delayMs))
}

// recordCall fires a best-effort call-log write asynchronously. It never
// blocks the response or propagates errors — call logs are observability, not
// part of the request contract (mirrors debugservice.go's fire-and-forget).
func (h *Handler) recordCall(projectID, apiID, method, path string, status, durationMs int) {
	if h.calllog == nil || projectID == "" {
		return
	}
	l := &models.CallLog{
		Base:       models.Base{ID: id.NewUUID()},
		Kind:       "mock",
		ProjectID:  projectID,
		Method:     method,
		Path:       path,
		StatusCode: status,
		Duration:   durationMs,
	}
	if apiID != "" {
		l.APIID = &apiID
	}
	go func(l *models.CallLog) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = h.calllog.Save(ctx, l)
	}(l)
}

// max1 returns n or 1 when n < 1, so a sub-millisecond call still logs a
// nonzero duration (consistent with the aggregator's rounding).
func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// SetOverride POST /api/v1/apis/:id/mock/override
func (h *Handler) SetOverride(c *gin.Context) {
	uid := mustUserID(c)
	var in mockservice.OverrideInput
	if !httpx.Bind(c, &in) {
		return
	}
	if err := h.mock.SetOverride(c.Request.Context(), c.Param("id"), uid, in); err != nil {
		httpx.Error(c, statusForErr(err), err.Error())
		return
	}
	httpx.Created(c, gin.H{"set": true})
}

// ClearOverride DELETE /api/v1/apis/:id/mock/override?key=...
func (h *Handler) ClearOverride(c *gin.Context) {
	uid := mustUserID(c)
	key := c.Query("key")
	if key == "" {
		httpx.Error(c, http.StatusBadRequest, "key query param required")
		return
	}
	if err := h.mock.ClearOverride(c.Request.Context(), c.Param("id"), key, uid); err != nil {
		httpx.Error(c, statusForErr(err), err.Error())
		return
	}
	httpx.OK(c, gin.H{"cleared": true})
}

// ListOverrides GET /api/v1/apis/:id/mock/override
func (h *Handler) ListOverrides(c *gin.Context) {
	uid := mustUserID(c)
	overrides, err := h.mock.ListOverrides(c.Request.Context(), c.Param("id"), uid)
	if err != nil {
		httpx.Error(c, statusForErr(err), err.Error())
		return
	}
	httpx.OK(c, overrides)
}

// statusForErr maps a mock service error to an HTTP status. Forbidden/Not-Found
// come from the authorization layer; everything else is a client-side problem.
func statusForErr(err error) int {
	switch {
	case errors.Is(err, projectservice.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, projectservice.ErrNotFound),
		errors.Is(err, apiservice.ErrNotFound):
		return http.StatusNotFound
	default:
		return http.StatusBadRequest
	}
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
