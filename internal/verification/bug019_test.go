package verification_test

import (
	"context"
	"testing"

	"api-mock-system/internal/models"
	"api-mock-system/internal/projectrepo"
	"api-mock-system/internal/projectservice"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// source marker: projectservice.InviteMember -> projectrepo.AddMember -> models.ProjectMember
func TestProjectCanInviteMultipleMembers(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:bug019?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Project{}, &models.ProjectMember{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	service := projectservice.New(projectrepo.New(db))
	project, err := service.Create(context.Background(), "owner-019", projectservice.CreateInput{
		Name:       "membership project",
		Visibility: "private",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.InviteMember(context.Background(), project.ID, "owner-019", "member-a", "viewer"); err != nil {
		t.Fatalf("first member invite failed: %v", err)
	}
	if err := service.InviteMember(context.Background(), project.ID, "owner-019", "member-b", "editor"); err != nil {
		t.Fatalf("second member invite failed: %v", err)
	}
	members, err := service.ListMembers(context.Background(), project.ID, "owner-019")
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 3 {
		t.Fatalf("member count = %d, want owner plus two invitees", len(members))
	}
}
