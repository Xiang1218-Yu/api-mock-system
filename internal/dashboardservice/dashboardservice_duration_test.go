package dashboardservice

import (
	"context"
	"strconv"
	"testing"

	"api-mock-system/internal/calllogrepo"
	"api-mock-system/internal/models"
	"api-mock-system/internal/projectrepo"
	"api-mock-system/internal/projectservice"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newDB spins up an in-memory SQLite database with the subset of tables the
// latency path touches, so DurationDistribution can be exercised end-to-end
// (calllogrepo -> dashboardservice) without standing up the whole app.
func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Project{}, &models.ProjectMember{}, &models.CallLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// seedViewer inserts a project + a viewer membership so RequireViewer passes.
func seedViewer(t *testing.T, db *gorm.DB, projectID, userID string) {
	t.Helper()
	if err := db.Create(&models.Project{Base: models.Base{ID: projectID}, Visibility: "public", OwnerID: userID}).Error; err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if err := db.Create(&models.ProjectMember{
		Base:      models.Base{ID: "m1"},
		ProjectID: projectID,
		UserID:    userID,
		Role:      "viewer",
	}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
}

// TestDurationDistribution_OrderAndBoundaries is the regression guard for the
// two latency-distribution bugs:
//  1. Boundary durations (exactly 10ms, 50ms, 100ms, 500ms) must fall into the
//     next range up, not the one whose max they equal.
//  2. The returned buckets must be in ascending order to match the chart.
func TestDurationDistribution_OrderAndBoundaries(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	seedViewer(t, db, "p1", "u1")

	logs := calllogrepo.New(db)
	// One mock call sitting on each boundary edge — these are the exact values
	// that previously got mis-bucketed and scrambled the chart.
	for _, ms := range []int{10, 50, 100, 500} {
		if err := logs.Save(ctx, &models.CallLog{
			Base:      models.Base{ID: "log-" + strconv.Itoa(ms)},
			ProjectID: "p1",
			Kind:      "mock",
			Duration:  ms,
		}); err != nil {
			t.Fatalf("save %dms: %v", ms, err)
		}
	}

	svc := New(db, nil, nil, projectservice.New(projectrepo.New(db)), logs)
	dist, err := svc.DurationDistribution(ctx, "p1", "u1")
	if err != nil {
		t.Fatalf("DurationDistribution: %v", err)
	}

	// Order: ascending, fast -> slow, matching calllogrepo.Buckets.
	wantOrder := []string{"0-10ms", "10-50ms", "50-100ms", "100-500ms", "500ms+"}
	if len(dist.Buckets) != len(wantOrder) {
		t.Fatalf("got %d buckets, want %d", len(dist.Buckets), len(wantOrder))
	}
	for i, b := range dist.Buckets {
		if b.Bucket != wantOrder[i] {
			t.Fatalf("buckets[%d] = %q, want %q (full order: %v)",
				i, b.Bucket, wantOrder[i], labels(dist.Buckets))
		}
	}

	// Boundaries: 10ms -> 10-50ms, 50ms -> 50-100ms, 100ms -> 100-500ms,
	// 500ms -> 500ms+. Each of those buckets must hold exactly one mock call,
	// and 0-10ms must hold none (nothing below 10ms was logged).
	want := map[string]int64{
		"0-10ms":    0,
		"10-50ms":   1,
		"50-100ms":  1,
		"100-500ms": 1,
		"500ms+":    1,
	}
	for _, b := range dist.Buckets {
		if b.Mock != want[b.Bucket] {
			t.Errorf("bucket %q: mock = %d, want %d", b.Bucket, b.Mock, want[b.Bucket])
		}
	}
}

func labels(bs []LatencyBucket) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Bucket
	}
	return out
}
