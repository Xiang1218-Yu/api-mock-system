package verification_test

import (
	"bytes"
	"context"
	"testing"

	"api-mock-system/internal/apirepo"
	"api-mock-system/internal/apiservice"
	"api-mock-system/internal/docservice"
	"api-mock-system/internal/models"
	"api-mock-system/internal/openapi"
	"api-mock-system/internal/projectrepo"
	"api-mock-system/internal/projectservice"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// source marker: docservice.OpenAPIYAML -> dochandler.OpenAPIYAML -> openapi.Build
func TestDocumentExportPreservesYAMLAndPublishedOperations(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:bug020?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Project{}, &models.ProjectMember{}, &models.API{}, &models.APIVersion{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	project := &models.Project{
		Base:       models.Base{ID: "project-020"},
		Name:       "documentation project",
		OwnerID:    "owner-020",
		Visibility: "public",
	}
	api := &models.API{
		Base:      models.Base{ID: "api-020"},
		ProjectID: project.ID,
		Name:      "list users",
		Method:    "GET",
		Path:      "/users",
		Status:    "published",
	}
	if err := db.Create(project).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(api).Error; err != nil {
		t.Fatal(err)
	}

	projects := projectservice.New(projectrepo.New(db))
	apis := apiservice.New(apirepo.New(db), projects)
	docs := docservice.New(projects, apis)
	data, err := docs.OpenAPIYAML(context.Background(), project.ID, "viewer-020")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("openapi:")) {
		t.Fatalf("YAML export is not YAML: %s", data)
	}

	document := openapi.Build(project, []models.API{*api})
	item, ok := document.Paths["/users"]
	if !ok || item.Get == nil {
		t.Fatalf("published GET operation missing from OpenAPI document: %#v", document.Paths)
	}
}
