package openapi

import (
	"encoding/json"
	"strings"
	"testing"

	"api-mock-system/internal/models"

	"github.com/goccy/go-yaml"
)

// yamlUnmarshal decodes YAML using the same library ToYAML marshals with, so the
// round-trip stays within one serializer's type model.
func yamlUnmarshal(b []byte, v any) error { return yaml.Unmarshal(b, v) }

// publishedAPI is a helper that builds a published API with the given method.
func publishedAPI(method, path string) models.API {
	return models.API{
		Method: method,
		Path:   path,
		Status: "published",
		Name:   method + " " + path,
	}
}

// TestBuildIncludesAllPublishedMethods guards against the method-case mismatch
// that previously dropped every operation: stored methods are upper-case
// (normalized at create time), but Build lower-cases them before dispatching to
// assignOp. Every supported verb must land on the matching PathItem field.
func TestBuildIncludesAllPublishedMethods(t *testing.T) {
	apis := []models.API{
		publishedAPI("GET", "/users"),
		publishedAPI("POST", "/users"),
		publishedAPI("PUT", "/users/{id}"),
		publishedAPI("DELETE", "/users/{id}"),
		publishedAPI("PATCH", "/users/{id}"),
		publishedAPI("OPTIONS", "/users"),
	}
	doc := Build(&models.Project{Name: "p"}, apis)

	cases := []struct {
		path      string
		methodKey string
		want      string // the summary we set, so a nil op fails loudly
	}{
		{"/users", "get", "GET /users"},
		{"/users", "post", "POST /users"},
		{"/users/{id}", "put", "PUT /users/{id}"},
		{"/users/{id}", "delete", "DELETE /users/{id}"},
		{"/users/{id}", "patch", "PATCH /users/{id}"},
		{"/users", "options", "OPTIONS /users"},
	}
	for _, c := range cases {
		item, ok := doc.Paths[c.path]
		if !ok {
			t.Errorf("path %q missing from document (paths=%v)", c.path, doc.Paths)
			continue
		}
		op := opByMethod(item, c.methodKey)
		if op == nil {
			t.Errorf("method %q on path %q was dropped (assignOp case mismatch)", strings.ToUpper(c.methodKey), c.path)
			continue
		}
		if op.Summary != c.want {
			t.Errorf("path %q method %q: summary=%q want %q", c.path, c.methodKey, op.Summary, c.want)
		}
	}
}

// opByMethod reads the lowercase field name a JSON consumer (and the SPA) uses,
// mirroring how the frontend walks the document. Using json tags here keeps the
// test honest about what clients actually see.
func opByMethod(item PathItem, method string) *Operation {
	switch method {
	case "get":
		return item.Get
	case "post":
		return item.Post
	case "put":
		return item.Put
	case "delete":
		return item.Delete
	case "patch":
		return item.Patch
	case "options":
		return item.Options
	}
	return nil
}

// TestBuildOmitsUnpublished confirms designing/deprecated APIs stay out.
func TestBuildOmitsUnpublished(t *testing.T) {
	apis := []models.API{
		publishedAPI("GET", "/kept"),
		{Method: "GET", Path: "/designing", Status: "designing", Name: "draft"},
		{Method: "GET", Path: "/deprecated", Status: "deprecated", Name: "old"},
	}
	doc := Build(&models.Project{Name: "p"}, apis)
	if _, ok := doc.Paths["/designing"]; ok {
		t.Error("designing API leaked into the document")
	}
	if _, ok := doc.Paths["/deprecated"]; ok {
		t.Error("deprecated API leaked into the document")
	}
	if _, ok := doc.Paths["/kept"]; !ok {
		t.Error("published API was omitted")
	}
}

// TestBuildSkipsUnknownMethod confirms an unrecognized verb is skipped, not
// silently dropped into a never-matching switch branch.
func TestBuildSkipsUnknownMethod(t *testing.T) {
	apis := []models.API{
		publishedAPI("TRACE", "/x"),
		publishedAPI("GET", "/y"),
	}
	doc := Build(&models.Project{Name: "p"}, apis)
	if _, ok := doc.Paths["/x"]; ok {
		t.Error("unknown-method API produced a (likely empty) path item")
	}
	if _, ok := doc.Paths["/y"]; !ok {
		t.Error("known method dropped alongside unknown one")
	}
}

// TestToJSONIsJSON ensures ToJSON round-trips through encoding/json.
func TestToJSONIsJSON(t *testing.T) {
	doc := Build(&models.Project{Name: "p"}, []models.API{publishedAPI("GET", "/users")})
	data, err := ToJSON(doc)
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("ToJSON output is not valid JSON: %v", err)
	}
	if got["openapi"] != "3.0.3" {
		t.Errorf("openapi field = %v, want 3.0.3", got["openapi"])
	}
}

// TestToYAMLIsYAML is the direct regression for the bug where OpenAPIYAML
// returned JSON bytes: ToYAML must produce YAML that parses back via a YAML
// reader, and it must NOT be the same bytes as ToJSON.
func TestToYAMLIsYAML(t *testing.T) {
	doc := Build(&models.Project{Name: "p"}, []models.API{publishedAPI("GET", "/users")})
	yamlBytes, err := ToYAML(doc)
	if err != nil {
		t.Fatalf("ToYAML: %v", err)
	}
	jsonBytes, _ := ToJSON(doc)

	// A YAML document starts with a bare `openapi:` key; JSON starts with `{`.
	yamlStr := string(yamlBytes)
	if !strings.Contains(yamlStr, "\nopenapi:") && !strings.HasPrefix(strings.TrimSpace(yamlStr), "openapi:") {
		t.Errorf("ToYAML output does not look like YAML (first line=%q)", strings.SplitN(yamlStr, "\n", 2)[0])
	}
	if strings.HasPrefix(strings.TrimSpace(string(yamlBytes)), "{") {
		t.Error("ToYAML returned JSON-looking output (starts with '{')")
	}
	if string(yamlBytes) == string(jsonBytes) {
		t.Error("ToYAML returned the same bytes as ToJSON")
	}

	// Round-trip: the YAML must decode back to a document carrying the operation.
	parsed := struct {
		OpenAPI string `yaml:"openapi"`
		Paths   map[string]map[string]struct {
			Summary string `yaml:"summary"`
		} `yaml:"paths"`
	}{}
	if err := yamlUnmarshal(yamlBytes, &parsed); err != nil {
		t.Fatalf("ToYAML output did not parse as YAML: %v", err)
	}
	if parsed.OpenAPI != "3.0.3" {
		t.Errorf("yaml openapi = %q, want 3.0.3", parsed.OpenAPI)
	}
	if parsed.Paths["/users"]["get"].Summary != "GET /users" {
		t.Errorf("yaml paths missing GET /users, got %#v", parsed.Paths)
	}
}
