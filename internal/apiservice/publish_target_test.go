package apiservice

import (
	"context"
	"testing"

	"api-mock-system/internal/apirepo"
	"api-mock-system/internal/models"
	"api-mock-system/internal/projectrepo"
	"api-mock-system/internal/projectservice"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPublishRepairsNonPositiveVersionBeforeMakingRouteLive(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.Project{}, &models.ProjectMember{}, &models.API{}, &models.APIVersion{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	project := &models.Project{Base: models.Base{ID: "project"}, Name: "project", OwnerID: "editor", Visibility: "private"}
	if err := db.Create(project).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := db.Create(&models.ProjectMember{Base: models.Base{ID: "editor-member"}, ProjectID: project.ID, UserID: "editor", Role: string(projectrepo.RoleEditor)}).Error; err != nil {
		t.Fatalf("create editor: %v", err)
	}
	api := &models.API{Base: models.Base{ID: "api"}, ProjectID: project.ID, Name: "orders", Method: "GET", Path: "/orders", Status: "designing", Version: -1}
	if err := db.Create(api).Error; err != nil {
		t.Fatalf("create api: %v", err)
	}
	service := New(apirepo.New(db), projectservice.New(projectrepo.New(db)))
	published, err := service.Publish(context.Background(), api.ID, "editor", "publish")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.Version != 1 || published.Status != "published" {
		t.Fatalf("published version=%d status=%q, want version 1 and published", published.Version, published.Status)
	}
}
