package verification_test

import (
	"context"
	"testing"

	"api-mock-system/internal/apirepo"
	"api-mock-system/internal/apiservice"
	"api-mock-system/internal/models"
	"api-mock-system/internal/projectrepo"
	"api-mock-system/internal/projectservice"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openVersionDB(t *testing.T) (*gorm.DB, *models.Project, *models.API, *apiservice.Service) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:bug018?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Project{}, &models.ProjectMember{}, &models.API{}, &models.APIVersion{}); err != nil {
		t.Fatal(err)
	}
	project := &models.Project{
		Base:       models.Base{ID: "project-018"},
		Name:       "version project",
		OwnerID:    "owner-018",
		Visibility: "private",
	}
	api := &models.API{
		Base:      models.Base{ID: "api-018"},
		ProjectID: project.ID,
		Name:      "versioned endpoint",
		Method:    "GET",
		Path:      "/versioned",
		Status:    "published",
	}
	if err := db.Create(project).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ProjectMember{
		Base:      models.Base{ID: "member-018"},
		ProjectID: project.ID,
		UserID:    project.OwnerID,
		Role:      "admin",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(api).Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	projects := projectservice.New(projectrepo.New(db))
	apis := apiservice.New(apirepo.New(db), projects)
	return db, project, api, apis
}

// source marker: apiservice.Publish -> apirepo.SaveVersion -> apirepo.ListVersions
func TestPublishCreatesContinuousVersionHistory(t *testing.T) {
	db, _, api, service := openVersionDB(t)
	ctx := context.Background()
	if _, err := service.Publish(ctx, api.ID, "owner-018", "first change"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(ctx, api.ID, "owner-018", "second change"); err != nil {
		t.Fatal(err)
	}

	var versions []models.APIVersion
	if err := db.Where("api_id = ?", api.ID).Order("version ASC").Find(&versions).Error; err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 || versions[0].Version != 1 || versions[1].Version != 2 {
		t.Fatalf("version history = %#v, want versions 1 and 2", versions)
	}
}
