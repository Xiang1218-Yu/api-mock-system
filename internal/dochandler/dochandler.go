// Package dochandler exposes the OpenAPI export endpoints. The Swagger-style
// preview page is rendered by the web layer, not here.
package dochandler

import (
	"net/http"

	"api-mock-system/internal/docservice"
	"api-mock-system/internal/httpx"
	"api-mock-system/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Handler wires the documentation endpoints.
type Handler struct {
	docs *docservice.Service
}

// New creates the handler.
func New(docs *docservice.Service) *Handler { return &Handler{docs: docs} }

// OpenAPIJSON GET /api/v1/projects/:projectId/docs/openapi.json
func (h *Handler) OpenAPIJSON(c *gin.Context) {
	uid := mustUserID(c)
	data, err := h.docs.OpenAPIJSON(c.Request.Context(), c.Param("projectId"), uid)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	c.Header("Content-Disposition", `attachment; filename="openapi.json"`)
	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
}

// OpenAPIYAML GET /api/v1/projects/:projectId/docs/openapi.yaml
func (h *Handler) OpenAPIYAML(c *gin.Context) {
	uid := mustUserID(c)
	data, err := h.docs.OpenAPIYAML(c.Request.Context(), c.Param("projectId"), uid)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	c.Header("Content-Disposition", `attachment; filename="openapi.yaml"`)
	c.Data(http.StatusOK, "application/yaml; charset=utf-8", data)
}

// Preview GET /api/v1/projects/:projectId/docs/preview
//
// Renders a minimal self-contained HTML page that redirects the browser to
// the SPA's docs view (#/projects/<id>/docs), where the Swagger-style rendering
// lives. Keeping the rich rendering client-side means one renderer and one
// source of layout truth. The page is static HTML with no external requests,
// so it works offline and behind the artifact CSP.
func (h *Handler) Preview(c *gin.Context) {
	pid := c.Param("projectId")
	html := `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta http-equiv="refresh" content="0; url=#/projects/` + pid + `/docs">
<title>API 文档预览</title>
</head>
<body>
<p>正在跳转到文档预览…如未自动跳转，<a href="#/projects/` + pid + `/docs">点击此处</a>。</p>
<script>location.hash = "#/projects/` + pid + `/docs";</script>
</body>
</html>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
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
