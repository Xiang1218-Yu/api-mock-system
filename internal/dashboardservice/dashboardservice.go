// Package dashboardservice computes the stats shown on the project overview
// page. It reads across repositories but writes nothing. Because the counts are
// simple and read-mostly, each metric is a straight COUNT or GROUP BY rather
// than a precomputed materialized view.
package dashboardservice

import (
	"context"
	"errors"
	"fmt"
	"time"

	"api-mock-system/internal/aggregaterepo"
	"api-mock-system/internal/apirepo"
	"api-mock-system/internal/calllogrepo"
	"api-mock-system/internal/models"
	"api-mock-system/internal/projectservice"

	"gorm.io/gorm"
)

// Stats is the payload returned for the project overview page.
type Stats struct {
	APICount           int64            `json:"api_count"`
	PublishedCount     int64            `json:"published_count"`
	DesigningCount     int64            `json:"designing_count"`
	DeprecatedCount    int64            `json:"deprecated_count"`
	AggregateCount     int64            `json:"aggregate_count"`
	StatusBreakdown    map[string]int64 `json:"status_breakdown"`
	MockCallCount      int64            `json:"mock_call_count"`
	AggregateCallCount int64            `json:"aggregate_call_count"`
}

// Service computes dashboard metrics.
type Service struct {
	db         *gorm.DB
	apis       apirepo.Repository
	aggregates aggregaterepo.Repository
	projects   *projectservice.Service
	calllogs   calllogrepo.Repository
}

// New wires the service. The *gorm.DB is used only for the GROUP BY status
// query, which doesn't fit neatly into the per-entity repository surface.
func New(db *gorm.DB, apis apirepo.Repository, aggregates aggregaterepo.Repository, projects *projectservice.Service, calllogs calllogrepo.Repository) *Service {
	return &Service{db: db, apis: apis, aggregates: aggregates, projects: projects, calllogs: calllogs}
}

// ProjectStats returns aggregate metrics for a project. Viewer+ enforced.
func (s *Service) ProjectStats(ctx context.Context, projectID, userID string) (*Stats, error) {
	if err := s.projects.RequireViewer(ctx, projectID, userID); err != nil {
		return nil, err
	}

	// Status breakdown via a single GROUP BY rather than three COUNT queries.
	type row struct {
		Status string
		Count  int64
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Table("apis").
		Select("status, count(*) as count").
		Where("project_id = ?", projectID).
		Group("status").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("dashboard: status breakdown: %w", err)
	}
	breakdown := make(map[string]int64, len(rows))
	var total int64
	for _, r := range rows {
		breakdown[r.Status] = r.Count
		total += r.Count
	}

	aggCount, err := s.countAggregates(ctx, projectID)
	if err != nil {
		return nil, err
	}

	byKind, err := s.calllogs.CountByKind(ctx, projectID)
	if err != nil {
		return nil, err
	}

	return &Stats{
		APICount:           total,
		PublishedCount:     breakdown["published"],
		DesigningCount:     breakdown["designing"],
		DeprecatedCount:    breakdown["deprecated"],
		AggregateCount:     aggCount,
		StatusBreakdown:    breakdown,
		MockCallCount:      byKind["mock"],
		AggregateCallCount: byKind["aggregate"],
	}, nil
}

// countAggregates returns the number of aggregates in a project by querying
// ListByProject with size 1 and reading the returned total.
func (s *Service) countAggregates(ctx context.Context, projectID string) (int64, error) {
	_, total, err := s.aggregates.ListByProject(ctx, projectID, 1, 0)
	if err != nil {
		return 0, err
	}
	return total, nil
}

// TrendPoint is one day's call counts, split by kind.
type TrendPoint struct {
	Date      string `json:"date"`
	Mock      int64  `json:"mock"`
	Aggregate int64  `json:"aggregate"`
}

// Trend is the per-day call-count series for the trend chart.
type Trend struct {
	Points []TrendPoint `json:"points"`
}

// CallTrend returns the per-day mock/aggregate call counts over the last `days`
// days (default 7). Viewer+ enforced.
func (s *Service) CallTrend(ctx context.Context, projectID, userID string, days int) (*Trend, error) {
	if err := s.projects.RequireViewer(ctx, projectID, userID); err != nil {
		return nil, err
	}
	if days <= 0 || days > 90 {
		days = 7
	}
	rows, err := s.calllogs.DailyCounts(ctx, projectID, days)
	if err != nil {
		return nil, err
	}
	// Bucket counts by date so missing days still appear as zero rows. We index
	// into the slice directly (not via &loop-var) so mutations land on the same
	// entries the caller receives — a pointer-to-loop-var would alias and the
	// appended copies would never reflect the counts.
	byDate := make(map[string]int, days)
	today := time.Now()
	points := make([]TrendPoint, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := today.AddDate(0, 0, -i).Format("2006-01-02")
		byDate[d] = len(points)
		points = append(points, TrendPoint{Date: d})
	}
	for _, r := range rows {
		if idx, ok := byDate[r.Date]; ok {
			if r.Kind == "mock" {
				points[idx].Mock = r.Count
			} else if r.Kind == "aggregate" {
				points[idx].Aggregate = r.Count
			}
		}
	}
	return &Trend{Points: points}, nil
}

// LatencyBucket is one latency range's counts, split by kind.
type LatencyBucket struct {
	Bucket    string `json:"bucket"`
	Mock      int64  `json:"mock"`
	Aggregate int64  `json:"aggregate"`
}

// LatencyDistribution is the per-range latency series for the distribution chart.
type LatencyDistribution struct {
	Buckets []LatencyBucket `json:"buckets"`
}

// DurationDistribution returns the latency distribution per kind for a project.
// Viewer+ enforced.
func (s *Service) DurationDistribution(ctx context.Context, projectID, userID string) (*LatencyDistribution, error) {
	if err := s.projects.RequireViewer(ctx, projectID, userID); err != nil {
		return nil, err
	}
	rows, err := s.calllogs.DurationBuckets(ctx, projectID)
	if err != nil {
		return nil, err
	}
	// Re-key into {bucket -> slice index} so mutations land on the exact slice
	// entries the caller receives (pointer-to-loop-var would alias and the
	// appended copies would never see the counts).
	bk := make(map[string]int, len(calllogrepo.Buckets))
	out := make([]LatencyBucket, 0, len(calllogrepo.Buckets))
	for _, b := range calllogrepo.Buckets {
		bk[b.Label] = len(out)
		out = append(out, LatencyBucket{Bucket: b.Label})
	}
	for _, r := range rows {
		idx, ok := bk[r.Bucket]
		if !ok {
			continue
		}
		if r.Kind == "mock" {
			out[idx].Mock = r.Count
		} else if r.Kind == "aggregate" {
			out[idx].Aggregate = r.Count
		}
	}
	return &LatencyDistribution{Buckets: out}, nil
}

// ListForUser returns the projects visible to the user, for the projects index.
// Re-exposed here so the dashboard doesn't depend on a projectservice method
// shape the dashboard doesn't control.
func (s *Service) ListForUser(ctx context.Context, userID, q string, page, size int) ([]models.Project, int64, error) {
	if size <= 0 {
		size = 20
	}
	if page <= 0 {
		page = 1
	}
	return s.projects.List(ctx, userID, q, page, size)
}

// Ensure projectservice.ErrNotFound is referenced so the import isn't dropped
// if other methods are removed — keeps the dependency explicit.
var _ = errors.Is
