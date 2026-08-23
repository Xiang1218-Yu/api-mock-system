package openapi

import (
	"encoding/json"
	"testing"

	"api-mock-system/internal/models"
)

func TestBuild(t *testing.T) {
	project := &models.Project{
		Base:        models.Base{ID: "p1"},
		Name:        "Demo",
		Description: "test",
		BasePath:    "/api/v1",
	}
	apis := []models.API{
		{
			Base:           models.Base{ID: "a1"},
			ProjectID:      "p1",
			Name:           "Get User",
			Method:         "GET",
			Path:           "/users/:id",
			Status:         "published",
			ResponseSchema: models.JSONMap{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "integer"}}},
			Tags:           models.StringArray{"user"},
		},
		{
			Base:   models.Base{ID: "a2"},
			Method: "GET",
			Path:   "/internal",
			Status: "designing", // must be excluded
		},
	}
	doc := Build(project, apis)
	if doc.OpenAPI != "3.0.3" {
		t.Errorf("openapi version = %q", doc.OpenAPI)
	}
	if doc.Info.Title != "Demo" {
		t.Errorf("title = %q", doc.Info.Title)
	}
	if len(doc.Servers) != 1 || doc.Servers[0].URL != "/api/v1" {
		t.Errorf("server not set: %v", doc.Servers)
	}
	// :id -> {id} conversion
	if _, ok := doc.Paths["/users/{id}"]; !ok {
		t.Errorf("path /users/{id} missing; paths = %v", doc.Paths)
	}
	// designing API excluded
	if _, ok := doc.Paths["/internal"]; ok {
		t.Error("designing API should be excluded")
	}
	if doc.Paths["/users/{id}"].Get == nil {
		t.Error("GET operation missing")
	}
	if doc.Paths["/users/{id}"].Get.Summary != "Get User" {
		t.Errorf("summary = %q", doc.Paths["/users/{id}"].Get.Summary)
	}
}

func TestPathConversion(t *testing.T) {
	cases := map[string]string{
		"/users":           "/users",
		"/users/:id":       "/users/{id}",
		"/users/:id/posts": "/users/{id}/posts",
		"/u/:a/:b":         "/u/{a}/{b}",
	}
	for in, want := range cases {
		if got := openAPIPath(in); got != want {
			t.Errorf("openAPIPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToJSON(t *testing.T) {
	doc := Build(&models.Project{Base: models.Base{ID: "p"}, Name: "X"}, nil)
	b, err := ToJSON(doc)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if m["openapi"] != "3.0.3" {
		t.Errorf("openapi field wrong: %v", m["openapi"])
	}
}
