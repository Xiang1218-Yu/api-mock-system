package dashboardservice

import (
	"context"
	"testing"
	"time"

	"api-mock-system/internal/aggregaterepo"
	"api-mock-system/internal/apirepo"
	"api-mock-system/internal/calllogrepo"
	"api-mock-system/internal/id"
	"api-mock-system/internal/models"
	"api-mock-system/internal/projectrepo"
	"api-mock-system/internal/projectservice"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTestService wires a dashboard service over an in-memory SQLite DB with one
// public project owned by the given user (so RequireViewer passes).
func newTestService(t *testing.T, ownerID string) (*Service, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.CallLog{}, &models.API{}, &models.Aggregate{}, &models.Project{}, &models.ProjectMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pr := projectrepo.New(db)
	ps := projectservice.New(pr)
	pid := id.NewUUID()
	if err := pr.Create(context.Background(), &models.Project{Base: models.Base{ID: pid, CreatedAt: time.Now(), UpdatedAt: time.Now()}, Name: "p", Visibility: "public", OwnerID: ownerID}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := pr.AddMember(context.Background(), &models.ProjectMember{Base: models.Base{ID: id.NewUUID(), CreatedAt: time.Now(), UpdatedAt: time.Now()}, ProjectID: pid, UserID: ownerID, Role: "admin"}); err != nil {
		t.Fatalf("add member: %v", err)
	}
	s := New(db, apirepo.New(db), aggregaterepo.New(db), ps, calllogrepo.New(db))
	return s, pid
}

// TestDurationDistributionCounts is a regression test for the pointer-to-loop-var
// aliasing bug, where counts were written through a map of pointers but the
// returned slice held value copies — so every bucket reported 0.
func TestDurationDistributionCounts(t *testing.T) {
	ownerID := id.NewUUID()
	s, pid := newTestService(t, ownerID)
	ctx := context.Background()
	// 2 mock + 1 aggregate, all <10ms.
	aid := id.NewUUID()
	for i := 0; i < 2; i++ {
		if err := s.calllogs.Save(ctx, &models.CallLog{Base: models.Base{ID: id.NewUUID(), CreatedAt: time.Now()}, ProjectID: pid, Kind: "mock", APIID: &aid, Method: "GET", Path: "/u", StatusCode: 200, Duration: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.calllogs.Save(ctx, &models.CallLog{Base: models.Base{ID: id.NewUUID(), CreatedAt: time.Now()}, ProjectID: pid, Kind: "aggregate", Method: "GET", Path: "/c", StatusCode: 200, Duration: 5}); err != nil {
		t.Fatal(err)
	}

	dist, err := s.DurationDistribution(ctx, pid, ownerID)
	if err != nil {
		t.Fatalf("DurationDistribution: %v", err)
	}
	var first LatencyBucket
	for _, b := range dist.Buckets {
		if b.Bucket == "0-10ms" {
			first = b
		}
	}
	if first.Mock != 2 {
		t.Errorf("0-10ms mock = %d, want 2 (pointer-aliasing regression)", first.Mock)
	}
	if first.Aggregate != 1 {
		t.Errorf("0-10ms aggregate = %d, want 1 (pointer-aliasing regression)", first.Aggregate)
	}
}

// TestCallTrendCounts is the trend counterpart of the aliasing regression test.
func TestCallTrendCounts(t *testing.T) {
	ownerID := id.NewUUID()
	s, pid := newTestService(t, ownerID)
	ctx := context.Background()
	aid := id.NewUUID()
	if err := s.calllogs.Save(ctx, &models.CallLog{Base: models.Base{ID: id.NewUUID(), CreatedAt: time.Now()}, ProjectID: pid, Kind: "mock", APIID: &aid, Method: "GET", Path: "/u", StatusCode: 200, Duration: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.calllogs.Save(ctx, &models.CallLog{Base: models.Base{ID: id.NewUUID(), CreatedAt: time.Now()}, ProjectID: pid, Kind: "aggregate", Method: "GET", Path: "/c", StatusCode: 200, Duration: 1}); err != nil {
		t.Fatal(err)
	}

	trend, err := s.CallTrend(ctx, pid, ownerID, 3)
	if err != nil {
		t.Fatalf("CallTrend: %v", err)
	}
	today := time.Now().Format("2006-01-02")
	var todayPoint *TrendPoint
	for i := range trend.Points {
		if trend.Points[i].Date == today {
			todayPoint = &trend.Points[i]
		}
	}
	if todayPoint == nil {
		t.Fatalf("no point for today %q in %+v", today, trend.Points)
	}
	if todayPoint.Mock != 1 {
		t.Errorf("today mock = %d, want 1 (pointer-aliasing regression)", todayPoint.Mock)
	}
	if todayPoint.Aggregate != 1 {
		t.Errorf("today aggregate = %d, want 1 (pointer-aliasing regression)", todayPoint.Aggregate)
	}
}
