package dashboardservice

import (
	"context"
	"errors"
	"testing"

	"api-mock-system/internal/models"
	"api-mock-system/internal/projectrepo"
	"api-mock-system/internal/projectservice"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAnonymousProjectIndexRequiresIdentity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.Project{}, &models.ProjectMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&models.Project{Base: models.Base{ID: "public"}, Name: "public", OwnerID: "owner", Visibility: "public"}).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	service := New(db, nil, nil, projectservice.New(projectrepo.New(db)), nil)
	_, _, err = service.ListForUser(context.Background(), "", "", 1, 20)
	if !errors.Is(err, projectservice.ErrForbidden) {
		t.Fatalf("anonymous project index error=%v, want forbidden", err)
	}
}
