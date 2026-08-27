// Package aggregatehandler exposes aggregate CRUD plus the live /aggregate route.
package aggregatehandler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"api-mock-system/internal/aggregateservice"
	"api-mock-system/internal/calllogrepo"
	"api-mock-system/internal/httpx"
	"api-mock-system/internal/id"
	"api-mock-system/internal/middleware"
	"api-mock-system/internal/models"
	"api-mock-system/internal/pathmatch"

	"github.com/gin-gonic/gin"
)

// Handler wires aggregate routes.
type Handler struct {
	aggr    *aggregateservice.Service
	calllog calllogrepo.Repository // nil = skip call logging
}

// New creates the handler. calllog may be nil to disable call logging.
func New(aggr *aggregateservice.Service, calllog calllogrepo.Repository) *Handler {
	return &Handler{aggr: aggr, calllog: calllog}
}

// Create POST /api/v1/projects/:projectId/aggregates
func (h *Handler) Create(c *gin.Context) {
	uid := mustUserID(c)
	var in aggregateservice.CreateInput
	if !httpx.Bind(c, &in) {
		return
	}
	a, err := h.aggr.Create(c.Request.Context(), uid, c.Param("projectId"), in)
	if err != nil {
		writeErr(c, err)
		return
	}
	httpx.Created(c, a)
}

// List GET /api/v1/projects/:projectId/aggregates
func (h *Handler) List(c *gin.Context) {
	uid := mustUserID(c)
	page := atoiDefault(c.Query("page"), 1)
	size := atoiDefault(c.Query("size"), 20)
	as, total, err := h.aggr.List(c.Request.Context(), c.Param("projectId"), uid, page, size)
	if err != nil {
		writeErr(c, err)
		return
	}
	httpx.PageOf(c, as, total, page, size)
}

// Update PUT /api/v1/aggregates/:id
func (h *Handler) Update(c *gin.Context) {
	uid := mustUserID(c)
	var in aggregateservice.UpdateInput
	if !httpx.Bind(c, &in) {
		return
	}
	a, err := h.aggr.Update(c.Request.Context(), c.Param("id"), uid, in)
	if err != nil {
		writeErr(c, err)
		return
	}
	httpx.OK(c, a)
}

// Delete DELETE /api/v1/aggregates/:id
func (h *Handler) Delete(c *gin.Context) {
	uid := mustUserID(c)
	if err := h.aggr.Delete(c.Request.Context(), c.Param("id"), uid); err != nil {
		writeErr(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

// Serve ANY /aggregate/:projectId/*path — runs the fan-out.
func (h *Handler) Serve(c *gin.Context) {
	uid := mustUserID(c)
	projectID := c.Param("projectId")
	// Normalize the inbound path so "/agg" and "/agg/" resolve to the same
	// aggregate definition — the storage and lookup sides apply the same rule.
	path := pathmatch.Normalize(c.Param("path"))
	body, _ := io.ReadAll(c.Request.Body)

	// Build the inbound map from the request. The aggregator's conditional mode
	// evaluates "key=value" against this map.
	inbound := map[string]any{
		"method": c.Request.Method,
		"path":   path,
		"query":  c.Request.URL.RawQuery,
	}
	if len(body) > 0 {
		inbound["body"] = string(body)
	}

	start := time.Now()
	agg, merged, results, err := h.aggr.Execute(c.Request.Context(), projectID, uid, path, inbound)
	if err != nil {
		writeErr(c, err)
		h.recordCall(projectID, "", c.Request.Method, path, statusForAggrErr(err), 0)
		return
	}
	httpx.OK(c, gin.H{"result": merged, "downstream_status": results})
	h.recordCall(projectID, agg.ID, c.Request.Method, path, http.StatusOK, int(time.Since(start)/time.Millisecond))
}

// recordCall fires a best-effort call-log write asynchronously. Duration is
// the total wall time of the fan-out (downstream calls + merge), which is the
// number spec §2.7's latency distribution cares about.
func (h *Handler) recordCall(projectID, aggID, method, path string, status, durationMs int) {
	if h.calllog == nil || projectID == "" {
		return
	}
	l := &models.CallLog{
		Base:       models.Base{ID: id.NewUUID()},
		Kind:       "aggregate",
		ProjectID:  projectID,
		Method:     method,
		Path:       path,
		StatusCode: status,
		Duration:   durationMs,
	}
	if aggID != "" {
		l.AggregateID = &aggID
	}
	go func(l *models.CallLog) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = h.calllog.Save(ctx, l)
	}(l)
}

// statusForAggrErr maps an Execute error to the HTTP status that writeErr will
// emit, so the call-log records the same status the client saw.
func statusForAggrErr(err error) int {
	switch {
	case errors.Is(err, aggregateservice.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, aggregateservice.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, aggregateservice.ErrInvalidMode):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
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

func writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, aggregateservice.ErrConflict):
		httpx.Error(c, http.StatusConflict, err.Error())
	case errors.Is(err, aggregateservice.ErrNotFound):
		httpx.Error(c, http.StatusNotFound, err.Error())
	case errors.Is(err, aggregateservice.ErrInvalidMode):
		httpx.Error(c, http.StatusBadRequest, err.Error())
	default:
		httpx.Error(c, http.StatusInternalServerError, err.Error())
	}
}
