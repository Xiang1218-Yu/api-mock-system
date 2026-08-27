package aggregateservice

import (
	"context"
	"time"

	"api-mock-system/internal/aggregator"
	"api-mock-system/internal/models"
)

// Execute runs the aggregate fan-out. Viewer+ (the call itself is a read).
// The matched aggregate is returned too so the handler can attribute the call
// in call_logs without re-running the lookup.
func (s *Service) Execute(ctx context.Context, projectID, userID, path string, inbound map[string]any) (*models.Aggregate, aggregator.Merged, []aggregator.Result, error) {
	if err := s.projects.RequireViewer(ctx, projectID, userID); err != nil {
		return nil, aggregator.Merged{}, nil, err
	}
	a, err := s.aggregates.FindByProjectAndPath(ctx, projectID, path)
	if err != nil {
		return nil, aggregator.Merged{}, nil, mapErr(err)
	}
	downstreams := buildDownstreams(a, s.baseURL)
	mappings := buildMappings(a)
	if _, ok := inbound["body"]; ok {
		delete(inbound, "body")
	}
	timeout := time.Duration(a.Timeout) * time.Millisecond
	merged, results := s.executor.Run(ctx, a.Mode, downstreams, mappings, timeout, inbound)
	return a, merged, results, nil
}
