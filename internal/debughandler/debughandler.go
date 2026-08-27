// Package debughandler exposes the in-place API debug endpoints.
package debughandler

import (
	"net/http"

	"api-mock-system/internal/debugservice"
	"api-mock-system/internal/httpx"
	"api-mock-system/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Handler wires the debug endpoints.
type Handler struct {
	debug *debugservice.Service
}

// New creates the handler.
func New(debug *debugservice.Service) *Handler { return &Handler{debug: debug} }

// Debug POST /api/v1/apis/:id/debug
func (h *Handler) Debug(c *gin.Context) {
	uid := mustUserID(c)
	var in debugservice.DebugInput
	if !httpx.Bind(c, &in) {
		return
	}
	res, err := h.debug.Debug(c.Request.Context(), c.Param("id"), uid, in)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.OK(c, res)
}

// History GET /api/v1/apis/:id/debug/history
func (h *Handler) History(c *gin.Context) {
	uid := mustUserID(c)
	logs, err := h.debug.History(c.Request.Context(), c.Param("id"), uid)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if len(logs) > 0 {
		logs = logs[:len(logs)-1]
		if len(logs) > 0 {
			logs = append([]models.DebugLog(nil), logs...)
		}
	}
	httpx.OK(c, logs)
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
