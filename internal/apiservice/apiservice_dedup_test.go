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

// allowAllProjects is a projectrepo stub whose MemberRole always grants admin,
// so service-level authz (RequireEditor/RequireViewer) passes for every actor.
// Only MemberRole is exercised by apiservice.Create; the rest are unreachable
// from this path and left as no-ops that fail loudly if ever called.
type allowAllProjects struct{}

func (allowAllProjects) MemberRole(_ context.Context, _, _ string) (projectrepo.MemberRole, bool, error) {
	return projectrepo.RoleAdmin, true, nil
}
func (allowAllProjects) Create(context.Context, *models.Project) error                  { panic("unused") }
func (allowAllProjects) FindByID(context.Context, string) (*models.Project, error)      { panic("unused") }
func (allowAllProjects) Update(context.Context, *models.Project) error                  { panic("unused") }
func (allowAllProjects) Delete(context.Context, string) error                            { panic("unused") }
func (allowAllProjects) List(context.Context, string, string, int, int) ([]models.Project, int64, error) {
	panic("unused")
}
func (allowAllProjects) AddMember(context.Context, *models.ProjectMember) error { panic("unused") }
func (allowAllProjects) RemoveMember(context.Context, string, string) error    { panic("unused") }
func (allowAllProjects) ListMembers(context.Context, string) ([]models.ProjectMember, error) {
	panic("unused")
}

// newTestService builds an apiservice.Service backed by a fresh, isolated
// in-memory SQLite DB and an always-authorized project service.
func newTestService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.API{}, &models.APIVersion{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	apis := apirepo.New(db)
	projects := projectservice.New(allowAllProjects{})
	return New(apis, projects)
}

// TestCreateRejectsDuplicatePathAcrossSlashVariants is the headline fix: an API
// created with path "/users" must conflict with a second API created at "/users/"
// — and vice versa. Before the fix the dedup query appended a trailing slash to
// an already-trailing-slashed path, so the lookup never matched and both rows
// were stored, later causing duplicate matches at mock time.
func TestCreateRejectsDuplicatePathAcrossSlashVariants(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	if _, err := s.Create(ctx, "u1", "p1", CreateInput{
		Name: "users", Method: "GET", Path: "/users",
	}); err != nil {
		t.Fatalf("first create: %v", err)
	}

	// Same path with a trailing slash must be seen as the same resource.
	if _, err := s.Create(ctx, "u1", "p1", CreateInput{
		Name: "users-dup", Method: "GET", Path: "/users/",
	}); err != ErrConflict {
		t.Errorf("create /users/ after /users: err = %v, want ErrConflict", err)
	}

	// And the reverse order — leading with the trailing slash — must also conflict.
	if _, err := s.Create(ctx, "u1", "p2", CreateInput{
		Name: "orders", Method: "GET", Path: "/orders/",
	}); err != nil {
		t.Fatalf("create /orders/ (first in p2): %v", err)
	}
	if _, err := s.Create(ctx, "u1", "p2", CreateInput{
		Name: "orders-dup", Method: "GET", Path: "/orders",
	}); err != ErrConflict {
		t.Errorf("create /orders after /orders/: err = %v, want ErrConflict", err)
	}
}

// TestCreateStoresNormalizedPath verifies the persisted path is the normalized
// (trailing-slash-free) form, so lookups and matching never see the raw variant.
func TestCreateStoresNormalizedPath(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	a, err := s.Create(ctx, "u1", "p1", CreateInput{
		Name: "users", Method: "GET", Path: "/users//",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.Path != "/users" {
		t.Errorf("stored path = %q, want %q", a.Path, "/users")
	}
}

// TestCreateAllowsSamePathDifferentMethod confirms dedup keys on method+path,
// not path alone — GET /users and POST /users are independent resources.
func TestCreateAllowsSamePathDifferentMethod(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	if _, err := s.Create(ctx, "u1", "p1", CreateInput{
		Name: "get-users", Method: "GET", Path: "/users",
	}); err != nil {
		t.Fatalf("create GET: %v", err)
	}
	if _, err := s.Create(ctx, "u1", "p1", CreateInput{
		Name: "post-users", Method: "POST", Path: "/users/",
	}); err != nil {
		t.Errorf("create POST /users/ after GET /users: err = %v, want nil (different method)", err)
	}
}
