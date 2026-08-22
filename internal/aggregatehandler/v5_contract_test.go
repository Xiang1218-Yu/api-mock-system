package aggregatehandler

import (
	"context"
	"testing"
	"time"

	"api-mock-system/internal/calllogrepo"
	"api-mock-system/internal/models"
)

type contractCallLogRepo struct {
	saved chan error
}

func (r *contractCallLogRepo) Save(ctx context.Context, _ *models.CallLog) error {
	r.saved <- ctx.Err()
	return ctx.Err()
}

func (*contractCallLogRepo) CountByKind(context.Context, string) (map[string]int64, error) {
	return nil, nil
}

func (*contractCallLogRepo) DailyCounts(context.Context, string, int) ([]calllogrepo.DailyCount, error) {
	return nil, nil
}

func (*contractCallLogRepo) DurationBuckets(context.Context, string) ([]calllogrepo.BucketCount, error) {
	return nil, nil
}

func TestCancelledRequestStillPersistsAggregateCallLog(t *testing.T) {
	repo := &contractCallLogRepo{saved: make(chan error, 1)}
	h := &Handler{calllog: repo}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	h.recordCall(ctx, "project-1", "aggregate-1", "GET", "/summary", 200, 1)
	select {
	case err := <-repo.saved:
		if err != nil {
			t.Fatalf("cancelled request did not retain call log: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("call log save did not run")
	}
}
