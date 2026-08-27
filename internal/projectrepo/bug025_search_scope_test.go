package projectrepo

import (
	"context"
	"testing"

	"api-mock-system/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBug025SearchScopeExcludesUnmatchedPublicProjects(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:bug025?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Project{}, &models.ProjectMember{}); err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	ctx := context.Background()
	if err := repo.Create(ctx, &models.Project{Base: models.Base{ID: "private-1"}, Name: "internal-api", OwnerID: "u1", Visibility: "private"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, &models.Project{Base: models.Base{ID: "public-1"}, Name: "public-docs", OwnerID: "u2", Visibility: "public"}); err != nil {
		t.Fatal(err)
	}
	projects, total, err := repo.List(ctx, "u1", "does-not-exist", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(projects) != 0 {
		t.Fatalf("unmatched search returned total=%d projects=%v", total, projects)
	}
}
