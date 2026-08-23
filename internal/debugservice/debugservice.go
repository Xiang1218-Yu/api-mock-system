// Package debugservice lets a user invoke an API in a sandboxed way and keeps
// the last N invocations. It calls the mock service to produce the response
// (debugging a published API means running its mock) and records the result.
package debugservice

import (
	"context"
	"encoding/json"
	"time"

	"api-mock-system/internal/apiservice"
	"api-mock-system/internal/debugrepo"
	"api-mock-system/internal/id"
	"api-mock-system/internal/mockservice"
	"api-mock-system/internal/models"
	"go.uber.org/zap"
)

// DebugInput is what the caller wants to invoke.
type DebugInput struct {
	Method string         `json:"method" binding:"required"`
	Path   string         `json:"path" binding:"required"`
	Query  string         `json:"query"`
	Body   map[string]any `json:"body"`
}

// DebugResult is the recorded outcome of one debug call.
type DebugResult struct {
	StatusCode int    `json:"status_code"`
	Response   any    `json:"response"`
	Duration   int    `json:"duration_ms"`
	LogID      string `json:"log_id"`
}

// Service executes debug calls and persists history.
type Service struct {
	apis *apiservice.Service
	mock *mockservice.Service
	logs debugrepo.Repository
	log  *zap.Logger
}

// New wires the service.
func New(apis *apiservice.Service, mock *mockservice.Service, logs debugrepo.Repository, log *zap.Logger) *Service {
	return &Service{apis: apis, mock: mock, logs: logs, log: log}
}

// Debug runs the request against the API's mock and saves a debug log row.
func (s *Service) Debug(ctx context.Context, apiID, userID string, in DebugInput) (*DebugResult, error) {
	api, err := s.apis.Get(ctx, apiID, userID)
	if err != nil {
		return nil, err
	}
	bodyStr := encodeBody(in.Body)

	start := time.Now()
	resp, err := s.mock.Resolve(ctx, api.ProjectID, in.Method, in.Path, in.Query, bodyStr)
	duration := time.Since(start)
	if err != nil {
		// Record the failure as a debug log too — debugging is about seeing
		// failures, not just successes.
		s.save(ctx, &models.DebugLog{
			Base:       models.Base{ID: id.NewUUID()},
			UserID:     userID,
			APIID:      &apiID,
			Request:    models.JSONMap{"method": in.Method, "path": in.Path, "query": in.Query, "body": in.Body},
			Response:   models.JSONMap{"error": err.Error()},
			StatusCode: 0,
			Duration:   int(duration / time.Millisecond),
		})
		return nil, err
	}

	result := &DebugResult{
		StatusCode: resp.StatusCode,
		Response:   resp.Body,
		Duration:   int(duration / time.Millisecond),
	}
	// Persist the log asynchronously so the caller isn't blocked by storage.
	go s.save(context.Background(), &models.DebugLog{
		Base:       models.Base{ID: id.NewUUID()},
		UserID:     userID,
		APIID:      &apiID,
		Request:    models.JSONMap{"method": in.Method, "path": in.Path, "query": in.Query, "body": in.Body},
		Response:   models.JSONMap{"response": resp.Body},
		StatusCode: resp.StatusCode,
		Duration:   result.Duration,
	})
	result.LogID = ""
	return result, nil
}

// History returns recent debug logs for an API.
func (s *Service) History(ctx context.Context, apiID, userID string) ([]models.DebugLog, error) {
	if _, err := s.apis.Get(ctx, apiID, userID); err != nil {
		return nil, err
	}
	return s.logs.ListByAPI(ctx, apiID, 50)
}

// save writes a debug log row, swallowing errors (debug logs are best-effort).
func (s *Service) save(ctx context.Context, l *models.DebugLog) {
	if err := s.logs.Save(ctx, l); err != nil {
		s.log.Warn("debug log save failed", zap.Error(err))
	}
}

func encodeBody(b map[string]any) string {
	if len(b) == 0 {
		return ""
	}
	out, err := json.Marshal(b)
	if err != nil {
		return ""
	}
	return string(out)
}
