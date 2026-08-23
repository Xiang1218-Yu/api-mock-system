// Package userhandler exposes the authentication HTTP endpoints: register
// and login. Handlers do nothing but parse input, call the service, and write
// the uniform response envelope.
package userhandler

import (
	"net/http"

	"api-mock-system/internal/httpx"
	"api-mock-system/internal/userservice"

	"github.com/gin-gonic/gin"
)

// Handler wires the auth endpoints to the user service.
type Handler struct {
	users *userservice.Service
}

// New creates the handler.
func New(users *userservice.Service) *Handler { return &Handler{users: users} }

// Register POST /api/v1/auth/register
func (h *Handler) Register(c *gin.Context) {
	var in userservice.RegisterInput
	if !httpx.Bind(c, &in) {
		return
	}
	u, err := h.users.Register(c.Request.Context(), in)
	if err != nil {
		switch err.Error() {
		case userservice.ErrEmailTaken.Error():
			httpx.Error(c, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(c, http.StatusBadRequest, err.Error())
		}
		return
	}
	httpx.Created(c, u)
}

// Login POST /api/v1/auth/login
func (h *Handler) Login(c *gin.Context) {
	var in userservice.LoginInput
	if !httpx.Bind(c, &in) {
		return
	}
	u, token, err := h.users.Login(c.Request.Context(), in)
	if err != nil {
		httpx.Error(c, http.StatusUnauthorized, err.Error())
		return
	}
	httpx.OK(c, gin.H{"user": u, "token": token, "token_type": "Bearer"})
}

// Me GET /api/v1/auth/me — returns the currently authenticated user.
func (h *Handler) Me(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	u, err := h.users.Get(c.Request.Context(), uid)
	if err != nil {
		httpx.Error(c, http.StatusNotFound, "user not found")
		return
	}
	httpx.OK(c, u)
}

// userID pulls the authenticated user id from context (set by RequireAuth).
func userID(c *gin.Context) (string, bool) {
	v, exists := c.Get("userID")
	if !exists {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}
