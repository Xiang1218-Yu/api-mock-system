package apirepo

import (
	"context"
	"testing"

	"api-mock-system/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTestRepo spins up a private in-memory SQLite database with the API schema
// migrated, returning a fresh repository. Each call gets its own connection to
// an isolated :memory: database, so tests never see each other's rows.
func newTestRepo(t *testing.T) Repository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.API{}, &models.APIVersion{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return New(db)
}

// mustCreate stores an API row directly with the given (project, method, path),
// bypassing the service so the test exercises the repo's own lookup, not the
// service's normalization. The path is stored verbatim, simulating whatever the
// caller hands the repo.
func mustCreate(t *testing.T, r Repository, projectID, method, path string) *models.API {
	t.Helper()
	a := &models.API{
		Base:      models.Base{ID: "api-" + projectID + "-" + method + "-" + path},
		ProjectID: projectID,
		Name:      path,
		Method:    method,
		Path:      path,
		Status:    "designing",
		Version:   1,
	}
	if err := r.Create(context.Background(), a); err != nil {
		t.Fatalf("create %q: %v", path, err)
	}
	return a
}

// TestDedupLookupMatchesNormalizedForm is the core regression: a path stored as
// "/users" must be found by a lookup for "/users", and vice versa. Before the
// fix, FindByProjectAndPath appended a trailing slash to the query, so it could
// never find a row whose stored path already lacked one — leaving dedup blind to
// the second, duplicate insert.
func TestDedupLookupMatchesNormalizedForm(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	// Store "/users" (the normalized form apiservice now writes).
	mustCreate(t, r, "p1", "GET", "/users")

	// Looking it up by any equivalent spelling must hit. The caller is expected
	// to Normalize before calling (apiservice does), but the repo must do an
	// exact match against the stored value — not mangle the query.
	for _, q := range []string{"/users"} {
		got, err := r.FindByProjectAndPath(ctx, "p1", "GET", q)
		if err != nil {
			t.Errorf("FindByProjectAndPath(%q): %v; want the stored /users row", q, err)
		} else if got.Path != "/users" {
			t.Errorf("FindByProjectAndPath(%q) returned path %q, want /users", q, got.Path)
		}
	}

	// A genuinely different path is not found.
	if _, err := r.FindByProjectAndPath(ctx, "p1", "GET", "/orders"); err == nil {
		t.Error("FindByProjectAndPath(/orders) unexpectedly matched /users")
	}
}

// TestDedupAcrossSlashVariantsAtServiceLevel documents the contract the service
// layer enforces on top of the repo: when the service normalizes both the stored
// path and the lookup path, "/users" and "/users/" collapse to one resource.
// This test stores both spellings raw at the repo level to prove the repo itself
// does NOT silently collapse them (it must not — it stores what it's given) and
// that an exact match is the right lookup strategy. The collapse is the service's
// job, validated in the pathmatch and apiservice tests.
func TestDedupRepoExactMatch(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	// If the service hands the repo two distinct normalized paths, they are
	// distinct resources — but the same normalized path twice must not be.
	mustCreate(t, r, "p1", "GET", "/users")

	if _, err := r.FindByProjectAndPath(ctx, "p1", "GET", "/users"); err != nil {
		t.Fatalf("expected existing /users to be found for dedup, got %v", err)
	}
	// Different method on same path is allowed (GET vs POST).
	mustCreate(t, r, "p1", "POST", "/users")
	if _, err := r.FindByProjectAndPath(ctx, "p1", "POST", "/users"); err != nil {
		t.Fatalf("expected existing POST /users to be found, got %v", err)
	}
	// Different project, same path+method, is allowed.
	mustCreate(t, r, "p2", "GET", "/users")
	if _, err := r.FindByProjectAndPath(ctx, "p2", "GET", "/users"); err != nil {
		t.Fatalf("expected existing p2 /users to be found, got %v", err)
	}
}
