package storage

import (
	"context"
	"path/filepath"
	"testing"

	"api-mock-system/internal/models"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestOpen_DropsLegacyUniqueProjectIDIndex reproduces the reported runtime state:
// a database created by an older schema version where project_members.project_id
// carried a UNIQUE index. That index capped a project at one member, so the
// second invitation failed with "UNIQUE constraint failed: project_members.project_id".
// Open must drop it and leave only the corrected (project_id, user_id) composite
// unique index, after which distinct members can coexist.
func TestOpen_DropsLegacyUniqueProjectIDIndex(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "legacy.db")

	// Build the legacy schema by hand: single-column UNIQUE index on project_id.
	legacy, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open legacy: %v", err)
	}
	if err := legacy.Exec(`CREATE TABLE project_members (
		id text PRIMARY KEY,
		created_at datetime,
		updated_at datetime,
		project_id text,
		user_id text,
		role text
	)`).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	// This is the exact constraint the bug report hit.
	if err := legacy.Exec(`CREATE UNIQUE INDEX idx_project_members_project_id ON project_members(project_id)`).Error; err != nil {
		t.Fatalf("create legacy unique index: %v", err)
	}
	// Seed one member so we can prove a second insert used to fail.
	if err := legacy.Exec(`INSERT INTO project_members (id, project_id, user_id, role) VALUES ('m1','p1','u1','admin')`).Error; err != nil {
		t.Fatalf("seed legacy member: %v", err)
	}

	// Before the fix: inserting a second distinct member must fail under the
	// legacy unique index. This guards the test's premise.
	if err := legacy.Exec(`INSERT INTO project_members (id, project_id, user_id, role) VALUES ('m2','p1','u2','viewer')`).Error; err == nil {
		t.Fatalf("premise: expected UNIQUE failure for second member under legacy index")
	}
	// Flush and close so storage.Open sees the file on disk, not an in-memory
	// leftover from this connection's handle.
	if sqlDB, err := legacy.DB(); err == nil {
		_ = sqlDB.Close()
	}
	legacy = nil

	// Now run the real migration the same way the app boots.
	log := zap.NewNop()
	store, err := Open(context.Background(), dsn, log)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// The stale single-column unique index must be gone.
	var staleCount int64
	if err := store.DB.Raw(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_project_members_project_id'`).Scan(&staleCount).Error; err != nil {
		t.Fatalf("query stale index: %v", err)
	}
	if staleCount != 0 {
		t.Fatalf("legacy idx_project_members_project_id still present: %d", staleCount)
	}

	// The corrected composite unique index must exist.
	var compositeCount int64
	if err := store.DB.Raw(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_project_members_project_user'`).Scan(&compositeCount).Error; err != nil {
		t.Fatalf("query composite index: %v", err)
	}
	if compositeCount != 1 {
		t.Fatalf("composite unique index missing: count=%d", compositeCount)
	}

	// The headline assertion: a second, distinct member now inserts cleanly.
	m2 := &models.ProjectMember{
		Base:      models.Base{ID: "m2"},
		ProjectID: "p1",
		UserID:    "u2",
		Role:      "viewer",
	}
	if err := store.DB.Create(m2).Error; err != nil {
		t.Fatalf("insert second distinct member after migration: want nil, got %v", err)
	}

	// The duplicate-invite rule still holds: re-inserting u1 on p1 must fail.
	m1dup := &models.ProjectMember{
		Base:      models.Base{ID: "m1dup"},
		ProjectID: "p1",
		UserID:    "u1",
		Role:      "editor",
	}
	if err := store.DB.Create(m1dup).Error; err == nil {
		t.Fatalf("duplicate (project_id,user_id): expected UNIQUE failure")
	}
}
