// Package openapi builds OpenAPI 3.0 documents from stored API definitions.
// It is a pure transformer: schemas in, document out. JSON/YAML serialization
// lives with the caller; here we only assemble the document model.
package openapi

import (
	"encoding/json"
	"sort"
	"strings"

	"api-mock-system/internal/models"
)

// Document is a minimal OpenAPI 3.0 structure covering the fields this platform
// actually populates. Extra fields are omitted intentionally.
type Document struct {
	OpenAPI    string              `json:"openapi" yaml:"openapi"`
	Info       Info                `json:"info" yaml:"info"`
	Servers    []Server            `json:"servers,omitempty" yaml:"servers,omitempty"`
	Paths      map[string]PathItem `json:"paths" yaml:"paths"`
	Components map[string]any      `json:"components,omitempty" yaml:"components,omitempty"`
}

// Info describes the document's title and version.
type Info struct {
	Title       string `json:"title" yaml:"title"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string `json:"version" yaml:"version"`
}

// Server is a base URL override.
type Server struct {
	URL string `json:"url" yaml:"url"`
}

// PathItem holds the operations available at a path.
type PathItem struct {
	Get     *Operation `json:"get,omitempty" yaml:"get,omitempty"`
	Post    *Operation `json:"post,omitempty" yaml:"post,omitempty"`
	Put     *Operation `json:"put,omitempty" yaml:"put,omitempty"`
	Delete  *Operation `json:"delete,omitempty" yaml:"delete,omitempty"`
	Patch   *Operation `json:"patch,omitempty" yaml:"patch,omitempty"`
	Options *Operation `json:"options,omitempty" yaml:"options,omitempty"`
}

// Operation describes one HTTP method on a path.
type Operation struct {
	Summary     string           `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string           `json:"description,omitempty" yaml:"description,omitempty"`
	Tags        []string         `json:"tags,omitempty" yaml:"tags,omitempty"`
	Parameters  []Parameter      `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	RequestBody *RequestBody     `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
	Responses   map[int]Response `json:"responses" yaml:"responses"`
}

// Parameter is a query/path/header parameter.
type Parameter struct {
	Name        string         `json:"name" yaml:"name"`
	In          string         `json:"in" yaml:"in"` // query|path|header
	Required    bool           `json:"required,omitempty" yaml:"required,omitempty"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Schema      map[string]any `json:"schema,omitempty" yaml:"schema,omitempty"`
}

// RequestBody wraps the request schema.
type RequestBody struct {
	Description string               `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool                 `json:"required,omitempty" yaml:"required,omitempty"`
	Content     map[string]MediaType `json:"content" yaml:"content"`
}

// MediaType wraps a schema for a content type.
type MediaType struct {
	Schema  map[string]any `json:"schema,omitempty" yaml:"schema,omitempty"`
	Example any            `json:"example,omitempty" yaml:"example,omitempty"`
}

// Response is one status code's response definition.
type Response struct {
	Description string               `json:"description" yaml:"description"`
	Content     map[string]MediaType `json:"content,omitempty" yaml:"content,omitempty"`
}

// Build assembles an OpenAPI document from a project and its published APIs.
// Only published APIs are included; designing/deprecated ones are omitted.
func Build(project *models.Project, apis []models.API) *Document {
	doc := &Document{
		OpenAPI: "3.0.3",
		Info: Info{
			Title:       project.Name,
			Description: project.Description,
			Version:     "1.0.0",
		},
		Paths: make(map[string]PathItem),
	}
	if project.BasePath != "" {
		doc.Servers = []Server{{URL: project.BasePath}}
	}

	// Sort APIs by path then method for stable document output.
	sort.Slice(apis, func(i, j int) bool {
		if apis[i].Path != apis[j].Path {
			return apis[i].Path < apis[j].Path
		}
		return apis[i].Method < apis[j].Method
	})

	for i := range apis {
		a := &apis[i]
		if a.Status != "published" {
			continue
		}
		path := openAPIPath(a.Path)
		method := strings.ToLower(strings.TrimSpace(a.Method))
		if method == "" {
			continue
		}
		item := doc.Paths[path]
		op := buildOperation(a)
		if !assignOp(&item, method, op) {
			// Unknown HTTP method: skip rather than silently drop via a missed
			// switch branch, so every published API is accounted for.
			continue
		}
		doc.Paths[path] = item
	}
	return doc
}

// buildOperation translates one stored API into an OpenAPI Operation.
func buildOperation(a *models.API) *Operation {
	op := &Operation{
		Summary:     a.Name,
		Description: a.Description,
		Tags:        []string(a.Tags),
		Responses:   map[int]Response{},
	}
	if req := mapToMap(a.RequestSchema); len(req) > 0 {
		op.Parameters = extractParameters(req)
		if body, ok := req["body"].(map[string]any); ok {
			op.RequestBody = &RequestBody{
				Required: true,
				Content: map[string]MediaType{
					"application/json": {Schema: body},
				},
			}
		}
	}
	respSchema := mapToMap(a.ResponseSchema)
	status := a.MockStatusCode
	if status == 0 {
		status = 200
	}
	op.Responses[status] = Response{
		Description: statusText(status),
		Content: map[string]MediaType{
			"application/json": {
				Schema:  respSchema,
				Example: a.ResponseExample,
			},
		},
	}
	return op
}

// openAPIPath converts a Gin-style /users/:id path to OpenAPI /users/{id}.
func openAPIPath(p string) string {
	out := make([]byte, 0, len(p))
	depth := 0
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c == ':' {
			out = append(out, '{')
			depth++
			continue
		}
		if c == '/' && depth > 0 {
			out = append(out, '}')
			depth--
		}
		out = append(out, c)
	}
	if depth > 0 {
		out = append(out, '}')
	}
	return string(out)
}

// assignOp places an operation under the right method field of a PathItem.
// method must be lowercased; the returned bool is false when the method is not
// a recognized HTTP verb so the caller can skip instead of silently dropping it.
func assignOp(item *PathItem, method string, op *Operation) bool {
	switch method {
	case "get":
		item.Get = op
	case "post":
		item.Post = op
	case "put":
		item.Put = op
	case "delete":
		item.Delete = op
	case "patch":
		item.Patch = op
	case "options":
		item.Options = op
	default:
		return false
	}
	return true
}

// extractParameters pulls query/path/header parameter schemas out of a request
// schema map. The convention: request_schema = { "query": {...}, "path": {...},
// "header": {...}, "body": {...} }.
func extractParameters(req map[string]any) []Parameter {
	var params []Parameter
	for _, loc := range []string{"path", "query", "header"} {
		section, ok := req[loc].(map[string]any)
		if !ok {
			continue
		}
		props, _ := section["properties"].(map[string]any)
		required, _ := section["required"].([]any)
		for name, raw := range props {
			schema, _ := raw.(map[string]any)
			params = append(params, Parameter{
				Name:     name,
				In:       loc,
				Required: contains(required, name),
				Schema:   schema,
			})
		}
	}
	return params
}

// ToJSON renders a document to pretty JSON bytes.
func ToJSON(doc *Document) ([]byte, error) {
	return json.MarshalIndent(doc, "", "  ")
}

// mapToMap converts a models.JSONMap to a plain map[string]any, returning nil
// for empty input to keep the document clean.
func mapToMap(m models.JSONMap) map[string]any {
	if len(m) == 0 {
		return nil
	}
	return map[string]any(m)
}

func contains(items []any, target string) bool {
	for _, it := range items {
		if s, ok := it.(string); ok && s == target {
			return true
		}
	}
	return false
}

// statusText returns a human status description, used as the Response.Description
// since OpenAPI requires a non-empty description on every response.
func statusText(code int) string {
	switch code {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 204:
		return "No Content"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 500:
		return "Internal Server Error"
	default:
		return "Response"
	}
}
