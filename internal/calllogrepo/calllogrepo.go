// Package calllogrepo persists runtime call-log rows (mock/aggregate calls) so
// the dashboard can report call counts, per-day trends, and latency
// distributions (spec §2.7). Writes are best-effort: the handler fires them
// asynchronously, so storage latency never blocks a mock or aggregate response.
package calllogrepo

import (
	"context"
	"fmt"
	"time"

	"api-mock-system/internal/models"

	"gorm.io/gorm"
)

// Repository is the data-access contract for call logs.
type Repository interface {
	// Save persists one call-log row. Callers pass an already-constructed
	// models.CallLog; the repo only concerns itself with persistence.
	Save(ctx context.Context, l *models.CallLog) error
	// CountByKind returns the total row count per kind ("mock"/"aggregate")
	// for a project, as a map. Kinds with zero rows are still present with 0.
	CountByKind(ctx context.Context, projectID string) (map[string]int64, error)
	// DailyCounts returns, per kind, the per-day call count over the last `days`
	// days (including today). Each entry is {Date, Kind, Count}.
	DailyCounts(ctx context.Context, projectID string, days int) ([]DailyCount, error)
	// DurationBuckets returns the latency distribution per kind, bucketed into
	// fixed ranges. Each entry is {Bucket, Kind, Count}.
	DurationBuckets(ctx context.Context, projectID string) ([]BucketCount, error)
}

// DailyCount is one (date, kind) tally for the trend chart.
type DailyCount struct {
	Date  string `json:"date"`
	Kind  string `json:"kind"`
	Count int64  `json:"count"`
}

// BucketCount is one (latency range, kind) tally for the distribution chart.
type BucketCount struct {
	Bucket string `json:"bucket"`
	Kind   string `json:"kind"`
	Count  int64  `json:"count"`
}

// Buckets are the latency ranges (in milliseconds) reported by
// DurationBuckets, in ascending order. A call lands in the first range whose
// upper bound it does not exceed; the final range is open-ended.
var Buckets = []struct {
	Label string
	Max   int // exclusive upper bound; 0 means open-ended
}{
	{"0-10ms", 10},
	{"10-50ms", 50},
	{"50-100ms", 100},
	{"100-500ms", 500},
	{"500ms+", 0},
}

type repo struct{ db *gorm.DB }

// New wires the repository to a gorm.DB.
func New(db *gorm.DB) Repository { return &repo{db: db} }

func (r *repo) Save(ctx context.Context, l *models.CallLog) error {
	if err := r.db.WithContext(ctx).Create(l).Error; err != nil {
		return fmt.Errorf("calllogrepo: save: %w", err)
	}
	return nil
}

func (r *repo) CountByKind(ctx context.Context, projectID string) (map[string]int64, error) {
	type row struct {
		Kind  string
		Count int64
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Table("call_logs").
		Select("kind, count(*) as count").
		Where("project_id = ?", projectID).
		Group("kind").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("calllogrepo: count by kind: %w", err)
	}
	out := map[string]int64{"mock": 0, "aggregate": 0}
	for _, r := range rows {
		out[r.Kind] = r.Count
	}
	return out, nil
}

func (r *repo) DailyCounts(ctx context.Context, projectID string, days int) ([]DailyCount, error) {
	if days <= 0 {
		days = 7
	}
	// Filter by date(created_at) >= the start date string, rather than by the
	// raw timestamp. Comparing date strings sidesteps timezone boundary issues
	// (a UTC-midnight truncation would mis-bucket off-UTC local calls) and keeps
	// the window aligned to calendar days as the chart displays them.
	sinceDate := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	type row struct {
		Date  string
		Kind  string
		Count int64
	}
	var rows []row
	// date(created_at) returns YYYY-MM-DD on SQLite. The date-string comparison
	// is lexicographic and correct for ISO dates.
	if err := r.db.WithContext(ctx).
		Table("call_logs").
		Select("date(created_at) as date, kind, count(*) as count").
		Where("project_id = ? AND date(created_at) >= ?", projectID, sinceDate).
		Group("date, kind").
		Order("date ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("calllogrepo: daily counts: %w", err)
	}
	out := make([]DailyCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, DailyCount{Date: r.Date, Kind: r.Kind, Count: r.Count})
	}
	return out, nil
}

// DurationBuckets reports per-kind counts in each latency range. SQLite lacks a
// CASE-based bucket here would need a raw query; we instead pull all (kind,
// duration) rows for the project and bucket in Go — fine for the volumes a
// dashboard trends (call logs are summarized, not the full request stream).
func (r *repo) DurationBuckets(ctx context.Context, projectID string) ([]BucketCount, error) {
	type row struct {
		Kind     string
		Duration int
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Table("call_logs").
		Select("kind, duration").
		Where("project_id = ?", projectID).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("calllogrepo: duration buckets: %w", err)
	}
	// index: kind -> bucketLabel -> count
	counts := map[string]map[string]int64{"mock": {}, "aggregate": {}}
	for _, r := range rows {
		bucket := bucketLabel(r.Duration)
		if counts[r.Kind] == nil {
			counts[r.Kind] = map[string]int64{}
		}
		counts[r.Kind][bucket]++
	}
	out := make([]BucketCount, 0, len(Buckets)*2)
	for _, b := range Buckets {
		for _, kind := range []string{"mock", "aggregate"} {
			out = append(out, BucketCount{Bucket: b.Label, Kind: kind, Count: counts[kind][b.Label]})
		}
	}
	return out, nil
}

// bucketLabel returns the label of the latency range a duration falls into.
func bucketLabel(ms int) string {
	for _, b := range Buckets {
		if b.Max == 0 || ms < b.Max {
			return b.Label
		}
	}
	return Buckets[len(Buckets)-1].Label
}
