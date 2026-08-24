package mockservice

import (
	"context"
	"testing"

	"api-mock-system/internal/apiservice"
	"api-mock-system/internal/apirepo"
	"api-mock-system/internal/cache"
	"api-mock-system/internal/id"
	"api-mock-system/internal/models"
	"api-mock-system/internal/mockdatarepo"
	"api-mock-system/internal/projectrepo"
	"api-mock-system/internal/projectservice"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTestStack builds the apiservice + mockservice stack on an in-memory SQLite
// DB so a test can exercise create -> publish -> resolve the way the live mock
// route does.
func newTestStack(t *testing.T) (*Service, *apiservice.Service, string, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Project{}, &models.ProjectMember{},
		&models.API{}, &models.APIVersion{}, &models.MockData{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Exec("DELETE FROM apis").Error })

	projR := projectrepo.New(db)
	apiR := apirepo.New(db)
	mockR := mockdatarepo.New(db)

	projects := projectservice.New(projR)
	apis := apiservice.New(apiR, projects)
	mock := New(apis, mockR, cache.New(), zap.NewNop())

	userID := id.NewUUID()
	ctx := context.Background()
	p, err := projects.Create(ctx, userID, projectservice.CreateInput{
		Name: "p", Visibility: "public",
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return mock, apis, userID, p.ID
}

// createPublished sets up a published API with the given method+path and
// returns it. It exercises the same Create -> Publish path a user follows when
// registering an endpoint.
func createPublished(t *testing.T, apis *apiservice.Service, userID, projectID, method, path string) *models.API {
	t.Helper()
	ctx := context.Background()
	a, err := apis.Create(ctx, userID, projectID, apiservice.CreateInput{
		Name:   method + " " + path,
		Method: method,
		Path:   path,
		Status: "designing",
	})
	if err != nil {
		t.Fatalf("create api (%s %s): %v", method, path, err)
	}
	if _, err := apis.Publish(ctx, a.ID, userID, "publish"); err != nil {
		t.Fatalf("publish api (%s %s): %v", method, path, err)
	}
	return a
}

// TestResolvePublishedByMethod guards against the regression where a published
// API was unreachable by its registered method. The mock resolver compares the
// stored method against the inbound request method; an earlier version of
// normalizeMethod stored lowercase ("post") while http.Request.Method is
// uppercase ("POST"), so no published API ever matched — every call came back
// "not found". POST is called out explicitly because that's how the bug was
// reported, but all supported methods are covered.
func TestResolvePublishedByMethod(t *testing.T) {
	mock, apis, userID, projectID := newTestStack(t)
	ctx := context.Background()

	cases := []struct {
		method string
		path   string
	}{
		{"GET", "/orders"},
		{"POST", "/orders"},
		{"PUT", "/orders"},
		{"DELETE", "/orders"},
		{"PATCH", "/orders"},
	}

	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			createPublished(t, apis, userID, projectID, tc.method, tc.path)
			resp, err := mock.Resolve(ctx, projectID, tc.method, tc.path, "", "")
			if err != nil {
				t.Fatalf("Resolve %s %s: %v", tc.method, tc.path, err)
			}
			if resp == nil {
				t.Fatalf("Resolve %s %s: nil response", tc.method, tc.path)
			}
			if resp.StatusCode == 0 {
				t.Fatalf("Resolve %s %s: zero status code", tc.method, tc.path)
			}
		})
	}
}
