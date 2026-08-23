// Package projecthandler exposes project + member endpoints. Handlers resolve
// the invitee's email→id via the user service, then delegate to projectservice.
package projecthandler

import (
	"errors"
	"net/http"
	"strconv"

	"api-mock-system/internal/httpx"
	"api-mock-system/internal/middleware"
	"api-mock-system/internal/projectservice"
	"api-mock-system/internal/userservice"

	"github.com/gin-gonic/gin"
)

// Handler wires the project endpoints.
type Handler struct {
	projects *projectservice.Service
	users    *userservice.Service
}

// New creates the handler.
func New(projects *projectservice.Service, users *userservice.Service) *Handler {
	return &Handler{projects: projects, users: users}
}

// Create POST /api/v1/projects
func (h *Handler) Create(c *gin.Context) {
	uid := mustUserID(c)
	var in projectservice.CreateInput
	if !httpx.Bind(c, &in) {
		return
	}
	p, err := h.projects.Create(c.Request.Context(), uid, in)
	if err != nil {
		writeServiceErr(c, err)
		return
	}
	httpx.Created(c, p)
}

// List GET /api/v1/projects
func (h *Handler) List(c *gin.Context) {
	uid := mustUserID(c)
	q := c.Query("q")
	page, size := pagination(c)
	ps, total, err := h.projects.List(c.Request.Context(), uid, q, page, size)
	if err != nil {
		writeServiceErr(c, err)
		return
	}
	httpx.PageOf(c, ps, total, page, size)
}

// Get GET /api/v1/projects/:projectId
func (h *Handler) Get(c *gin.Context) {
	uid := mustUserID(c)
	p, err := h.projects.Get(c.Request.Context(), c.Param("projectId"), uid)
	if err != nil {
		writeServiceErr(c, err)
		return
	}
	httpx.OK(c, p)
}

// Update PUT /api/v1/projects/:projectId
func (h *Handler) Update(c *gin.Context) {
	uid := mustUserID(c)
	var in projectservice.UpdateInput
	if !httpx.Bind(c, &in) {
		return
	}
	p, err := h.projects.Update(c.Request.Context(), c.Param("projectId"), uid, in)
	if err != nil {
		writeServiceErr(c, err)
		return
	}
	httpx.OK(c, p)
}

// Delete DELETE /api/v1/projects/:projectId
func (h *Handler) Delete(c *gin.Context) {
	uid := mustUserID(c)
	if err := h.projects.Delete(c.Request.Context(), c.Param("projectId"), uid); err != nil {
		writeServiceErr(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

// InviteMember POST /api/v1/projects/:projectId/members
func (h *Handler) InviteMember(c *gin.Context) {
	uid := mustUserID(c)
	var in projectservice.MemberInput
	if !httpx.Bind(c, &in) {
		return
	}
	invitee, err := h.users.GetByEmail(c.Request.Context(), in.Email)
	if err != nil {
		httpx.Error(c, http.StatusNotFound, "invitee user not found")
		return
	}
	if err := h.projects.InviteMember(c.Request.Context(), c.Param("projectId"), uid, invitee.ID, in.Role); err != nil {
		writeServiceErr(c, err)
		return
	}
	httpx.Created(c, gin.H{"user_id": invitee.ID, "role": in.Role})
}

// RemoveMember DELETE /api/v1/projects/:projectId/members/:userId
func (h *Handler) RemoveMember(c *gin.Context) {
	uid := mustUserID(c)
	if err := h.projects.RemoveMember(c.Request.Context(), c.Param("projectId"), uid, c.Param("userId")); err != nil {
		writeServiceErr(c, err)
		return
	}
	httpx.OK(c, gin.H{"removed": true})
}

// ListMembers GET /api/v1/projects/:projectId/members
func (h *Handler) ListMembers(c *gin.Context) {
	uid := mustUserID(c)
	ms, err := h.projects.ListMembers(c.Request.Context(), c.Param("projectId"), uid)
	if err != nil {
		writeServiceErr(c, err)
		return
	}
	httpx.OK(c, ms)
}

// mustUserID extracts the authenticated user id or aborts with 401.
func mustUserID(c *gin.Context) string {
	v, ok := c.Get(string(middleware.UserIDKey))
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized")
		c.Abort()
		return ""
	}
	return v.(string)
}

// pagination reads page/size query params with sensible defaults.
func pagination(c *gin.Context) (int, int) {
	page := atoiDefault(c.Query("page"), 1)
	size := atoiDefault(c.Query("size"), 20)
	if size > 200 {
		size = 200
	}
	return page, size
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

// writeServiceErr maps a projectservice error to an HTTP status.
func writeServiceErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, projectservice.ErrNotFound):
		httpx.Error(c, http.StatusNotFound, err.Error())
	case errors.Is(err, projectservice.ErrForbidden):
		httpx.Error(c, http.StatusForbidden, err.Error())
	case errors.Is(err, projectservice.ErrInvalidRole):
		httpx.Error(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, projectservice.ErrMemberExists):
		httpx.Error(c, http.StatusConflict, err.Error())
	default:
		httpx.Error(c, http.StatusInternalServerError, err.Error())
	}
}
