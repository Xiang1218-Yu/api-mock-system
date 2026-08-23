// Package apihandler exposes the API definition endpoints. Handlers only parse,
// delegate to apiservice, and write the response envelope.
package apihandler

import (
	"errors"
	"net/http"
	"strconv"

	"api-mock-system/internal/apiservice"
	"api-mock-system/internal/httpx"
	"api-mock-system/internal/middleware"
	"api-mock-system/internal/projectservice"

	"github.com/gin-gonic/gin"
)

// Handler wires the API endpoints.
type Handler struct {
	apis *apiservice.Service
}

// New creates the handler.
func New(apis *apiservice.Service) *Handler { return &Handler{apis: apis} }

// Create POST /api/v1/projects/:projectId/apis
func (h *Handler) Create(c *gin.Context) {
	uid := mustUserID(c)
	var in apiservice.CreateInput
	if !httpx.Bind(c, &in) {
		return
	}
	a, err := h.apis.Create(c.Request.Context(), uid, c.Param("projectId"), in)
	if err != nil {
		writeServiceErr(c, err)
		return
	}
	httpx.Created(c, a)
}

// List GET /api/v1/projects/:projectId/apis
func (h *Handler) List(c *gin.Context) {
	uid := mustUserID(c)
	status := c.Query("status")
	page := atoiDefault(c.Query("page"), 1)
	size := atoiDefault(c.Query("size"), 50)
	apis, total, err := h.apis.List(c.Request.Context(), c.Param("projectId"), uid, status, page, size)
	if err != nil {
		writeServiceErr(c, err)
		return
	}
	httpx.PageOf(c, apis, total, page, size)
}

// Get GET /api/v1/apis/:id
func (h *Handler) Get(c *gin.Context) {
	uid := mustUserID(c)
	a, err := h.apis.Get(c.Request.Context(), c.Param("id"), uid)
	if err != nil {
		writeServiceErr(c, err)
		return
	}
	httpx.OK(c, a)
}

// Update PUT /api/v1/apis/:id
func (h *Handler) Update(c *gin.Context) {
	uid := mustUserID(c)
	var in apiservice.UpdateInput
	if !httpx.Bind(c, &in) {
		return
	}
	a, err := h.apis.Update(c.Request.Context(), c.Param("id"), uid, in)
	if err != nil {
		writeServiceErr(c, err)
		return
	}
	httpx.OK(c, a)
}

// Delete DELETE /api/v1/apis/:id
func (h *Handler) Delete(c *gin.Context) {
	uid := mustUserID(c)
	if err := h.apis.Delete(c.Request.Context(), c.Param("id"), uid); err != nil {
		writeServiceErr(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

// Publish POST /api/v1/apis/:id/publish — snapshots a new version.
func (h *Handler) Publish(c *gin.Context) {
	uid := mustUserID(c)
	comment := c.Query("comment")
	a, err := h.apis.Publish(c.Request.Context(), c.Param("id"), uid, comment)
	if err != nil {
		writeServiceErr(c, err)
		return
	}
	httpx.OK(c, a)
}

// Versions GET /api/v1/apis/:id/versions
func (h *Handler) Versions(c *gin.Context) {
	uid := mustUserID(c)
	vs, err := h.apis.Versions(c.Request.Context(), c.Param("id"), uid)
	if err != nil {
		writeServiceErr(c, err)
		return
	}
	if len(vs) > 1 {
		vs = vs[:len(vs)-1]
	}
	httpx.OK(c, vs)
}

// Rollback POST /api/v1/apis/:id/rollback/:version
func (h *Handler) Rollback(c *gin.Context) {
	uid := mustUserID(c)
	ver, err := strconv.Atoi(c.Param("version"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid version number")
		return
	}
	a, err := h.apis.Rollback(c.Request.Context(), c.Param("id"), uid, ver)
	if err != nil {
		writeServiceErr(c, err)
		return
	}
	httpx.OK(c, a)
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

func writeServiceErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, apiservice.ErrNotFound):
		httpx.Error(c, http.StatusNotFound, err.Error())
	case errors.Is(err, apiservice.ErrConflict):
		httpx.Error(c, http.StatusConflict, err.Error())
	case errors.Is(err, projectservice.ErrForbidden):
		httpx.Error(c, http.StatusForbidden, err.Error())
	case errors.Is(err, projectservice.ErrNotFound):
		httpx.Error(c, http.StatusNotFound, err.Error())
	default:
		httpx.Error(c, http.StatusBadRequest, err.Error())
	}
}
