package projectservice

import (
	"context"
	"errors"
	"testing"

	"api-mock-system/internal/models"
	"api-mock-system/internal/projectrepo"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRemoveMemberDoesNotBypassOwnerProtectionWithWhitespace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.Project{}, &models.ProjectMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	project := &models.Project{Base: models.Base{ID: "project"}, Name: "project", OwnerID: "owner", Visibility: "private"}
	if err := db.Create(project).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := db.Create(&models.ProjectMember{Base: models.Base{ID: "admin-member"}, ProjectID: project.ID, UserID: "admin", Role: string(projectrepo.RoleAdmin)}).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	service := New(projectrepo.New(db))
	err = service.RemoveMember(context.Background(), project.ID, "admin", " owner ")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("owner removal with whitespace error=%v, want forbidden", err)
	}
}
