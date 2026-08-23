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

// newTestDB spins up an in-memory SQLite DB with the corrected schema (the same
// AutoMigrate the app runs) plus the legacy-index drop that fixes existing DBs.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&mode=memory"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Project{}, &models.ProjectMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Exec("DELETE FROM project_members").Error; _ = db.Exec("DELETE FROM projects").Error })
	return db
}

// TestInviteMember_AllowsDistinctMembers reproduces the reported bug: after the
// first member is invited, a second distinct member must also be accepted. With
// the old single-column UNIQUE(project_id) index, the second insert failed and
// the service returned ErrForbidden — masking the real cause.
func TestInviteMember_AllowsDistinctMembers(t *testing.T) {
	db := newTestDB(t)
	svc := New(projectrepo.New(db))

	ctx := context.Background()
	ownerID := "owner-1"
	memberA := "invitee-a"
	memberB := "invitee-b"

	// Seed the project + owner-admin row the way Service.Create does.
	proj := &models.Project{Base: models.Base{ID: "proj-1"}, OwnerID: ownerID, Visibility: "public"}
	if err := projectrepo.New(db).Create(ctx, proj); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if err := svc.projects.AddMember(ctx, &models.ProjectMember{
		Base: models.Base{ID: "pm-owner"}, ProjectID: proj.ID, UserID: ownerID, Role: string(projectrepo.RoleAdmin),
	}); err != nil {
		t.Fatalf("seed owner member: %v", err)
	}

	// First invite of a distinct member.
	if err := svc.InviteMember(ctx, proj.ID, ownerID, memberA, "editor"); err != nil {
		t.Fatalf("first invite: want nil, got %v", err)
	}
	// Second invite of a *different* member must succeed — this is the bug.
	if err := svc.InviteMember(ctx, proj.ID, ownerID, memberB, "viewer"); err != nil {
		t.Fatalf("second invite of distinct member: want nil, got %v", err)
	}

	// The roster must reflect both invited members plus the owner.
	ms, err := svc.ListMembers(ctx, proj.ID, ownerID)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(ms) != 3 {
		t.Fatalf("roster size: want 3 (owner+a+b), got %d", len(ms))
	}
}

// TestInviteMember_DuplicateStillRejected confirms the duplicate-invite rule
// still holds under the corrected (project_id, user_id) unique index — the fix
// relaxes the over-broad constraint, not the intended one.
func TestInviteMember_DuplicateStillRejected(t *testing.T) {
	db := newTestDB(t)
	svc := New(projectrepo.New(db))

	ctx := context.Background()
	ownerID := "owner-1"
	dup := "invitee-dup"

	proj := &models.Project{Base: models.Base{ID: "proj-2"}, OwnerID: ownerID, Visibility: "public"}
	if err := projectrepo.New(db).Create(ctx, proj); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if err := svc.projects.AddMember(ctx, &models.ProjectMember{
		Base: models.Base{ID: "pm-owner2"}, ProjectID: proj.ID, UserID: ownerID, Role: string(projectrepo.RoleAdmin),
	}); err != nil {
		t.Fatalf("seed owner member: %v", err)
	}

	if err := svc.InviteMember(ctx, proj.ID, ownerID, dup, "editor"); err != nil {
		t.Fatalf("first invite: want nil, got %v", err)
	}
	err := svc.InviteMember(ctx, proj.ID, ownerID, dup, "editor")
	if !errors.Is(err, ErrMemberExists) {
		t.Fatalf("duplicate invite: want ErrMemberExists, got %v", err)
	}
}

// TestInviteMember_InvalidRoleStillRejected confirms role validation still
// fires before any DB write — the original rules are unchanged.
func TestInviteMember_InvalidRoleStillRejected(t *testing.T) {
	db := newTestDB(t)
	svc := New(projectrepo.New(db))
	ctx := context.Background()
	ownerID := "owner-1"
	proj := &models.Project{Base: models.Base{ID: "proj-3"}, OwnerID: ownerID, Visibility: "public"}
	if err := projectrepo.New(db).Create(ctx, proj); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if err := svc.projects.AddMember(ctx, &models.ProjectMember{
		Base: models.Base{ID: "pm-owner3"}, ProjectID: proj.ID, UserID: ownerID, Role: string(projectrepo.RoleAdmin),
	}); err != nil {
		t.Fatalf("seed owner member: %v", err)
	}

	if err := svc.InviteMember(ctx, proj.ID, ownerID, "someone", "superuser"); !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("invalid role: want ErrInvalidRole, got %v", err)
	}
}

// TestInviteMember_NonAdminForbidden confirms authz still precedes everything.
func TestInviteMember_NonAdminForbidden(t *testing.T) {
	db := newTestDB(t)
	svc := New(projectrepo.New(db))
	ctx := context.Background()
	ownerID := "owner-1"
	viewerID := "viewer-1"
	proj := &models.Project{Base: models.Base{ID: "proj-4"}, OwnerID: ownerID, Visibility: "public"}
	if err := projectrepo.New(db).Create(ctx, proj); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if err := svc.projects.AddMember(ctx, &models.ProjectMember{
		Base: models.Base{ID: "pm-owner4"}, ProjectID: proj.ID, UserID: ownerID, Role: string(projectrepo.RoleAdmin),
	}); err != nil {
		t.Fatalf("seed owner member: %v", err)
	}
	if err := svc.projects.AddMember(ctx, &models.ProjectMember{
		Base: models.Base{ID: "pm-viewer4"}, ProjectID: proj.ID, UserID: viewerID, Role: string(projectrepo.RoleViewer),
	}); err != nil {
		t.Fatalf("seed viewer member: %v", err)
	}

	if err := svc.InviteMember(ctx, proj.ID, viewerID, "someone-else", "editor"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-admin invite: want ErrForbidden, got %v", err)
	}
}
