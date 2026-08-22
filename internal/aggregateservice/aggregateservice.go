// Package aggregateservice manages aggregate endpoint definitions and executes
// fan-out at request time. It delegates the actual merging to package aggregator
// and persists definitions to aggregaterepo.
package aggregateservice

import (
	"context"
	"errors"
	"net/http"
	"time"

	"api-mock-system/internal/aggregaterepo"
	"api-mock-system/internal/aggregator"
	"api-mock-system/internal/apiservice"
	"api-mock-system/internal/id"
	"api-mock-system/internal/models"
	"api-mock-system/internal/projectservice"
	"go.uber.org/zap"
)

var (
	// ErrNotFound is returned when an aggregate lookup misses.
	ErrNotFound = errors.New("aggregate not found")
	// ErrInvalidMode is returned for an unrecognized aggregation mode.
	ErrInvalidMode = errors.New("mode must be serial, parallel, or conditional")
)

// CreateInput captures the fields needed to define an aggregate.
type CreateInput struct {
	Name           string         `json:"name" binding:"required"`
	Description    string         `json:"description"`
	Path           string         `json:"path" binding:"required"`
	Mode           string         `json:"mode" binding:"required"`
	Timeout        int            `json:"timeout"` // milliseconds
	DownstreamAPIs models.JSONMap `json:"downstream_apis"`
	FieldMappings  models.JSONMap `json:"field_mappings"`
}

// UpdateInput captures the mutable fields of an aggregate.
type UpdateInput struct {
	Name           *string         `json:"name"`
	Description    *string         `json:"description"`
	Path           *string         `json:"path"`
	Mode           *string         `json:"mode"`
	Timeout        *int            `json:"timeout"`
	DownstreamAPIs *models.JSONMap `json:"downstream_apis"`
	FieldMappings  *models.JSONMap `json:"field_mappings"`
}

// Service orchestrates aggregates.
type Service struct {
	aggregates     aggregaterepo.Repository
	apis           *apiservice.Service
	projects       *projectservice.Service
	executor       *aggregator.Aggregator
	baseURL        string        // resolved mock/aggregate base, used to build downstream URLs
	defaultTimeout time.Duration // fallback when an aggregate sets no timeout
	log            *zap.Logger
}

// New wires the service. baseURL is the internal origin used to call downstream
// mock/aggregate APIs (e.g. http://localhost:8080). defaultTimeout is applied
// when an aggregate definition omits its own timeout (spec env AGGREGATE_TIMEOUT).
func New(aggregates aggregaterepo.Repository, apis *apiservice.Service, projects *projectservice.Service, baseURL string, defaultTimeout time.Duration, log *zap.Logger) *Service {
	if defaultTimeout <= 0 {
		defaultTimeout = 3 * time.Second
	}
	s := &Service{
		aggregates:     aggregates,
		apis:           apis,
		projects:       projects,
		executor:       aggregator.New(&http.Client{Timeout: 10 * time.Second}, log),
		baseURL:        baseURL,
		defaultTimeout: defaultTimeout,
		log:            log,
	}
	return s
}

// Create defines a new aggregate. Editor+.
func (s *Service) Create(ctx context.Context, actorID, projectID string, in CreateInput) (*models.Aggregate, error) {
	if err := s.projects.RequireEditor(ctx, projectID, actorID); err != nil {
		return nil, err
	}
	if !validMode(in.Mode) {
		return nil, ErrInvalidMode
	}
	a := &models.Aggregate{
		Base:           models.Base{ID: id.NewUUID()},
		ProjectID:      projectID,
		Name:           in.Name,
		Description:    in.Description,
		Path:           in.Path,
		Mode:           in.Mode,
		Timeout:        defaultTimeout(in.Timeout, int(s.defaultTimeout/time.Millisecond)),
		DownstreamAPIs: in.DownstreamAPIs,
		FieldMappings:  in.FieldMappings,
	}
	if err := s.aggregates.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// List returns aggregates under a project. Viewer+.
func (s *Service) List(ctx context.Context, projectID, userID string, page, size int) ([]models.Aggregate, int64, error) {
	if err := s.projects.RequireViewer(ctx, projectID, userID); err != nil {
		return nil, 0, err
	}
	if size <= 0 {
		size = 20
	}
	if page <= 0 {
		page = 1
	}
	return s.aggregates.ListByProject(ctx, projectID, size, (page-1)*size)
}

// Get returns an aggregate by id, after viewer+ check on its project.
func (s *Service) Get(ctx context.Context, id, userID string) (*models.Aggregate, error) {
	a, err := s.aggregates.FindByID(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	if err := s.projects.RequireViewer(ctx, a.ProjectID, userID); err != nil {
		return nil, err
	}
	return a, nil
}

// Update applies partial changes. Editor+.
func (s *Service) Update(ctx context.Context, id, userID string, in UpdateInput) (*models.Aggregate, error) {
	a, err := s.aggregates.FindByID(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	if err := s.projects.RequireEditor(ctx, a.ProjectID, userID); err != nil {
		return nil, err
	}
	if in.Name != nil {
		a.Name = *in.Name
	}
	if in.Description != nil {
		a.Description = *in.Description
	}
	if in.Path != nil {
		a.Path = *in.Path
	}
	if in.Mode != nil {
		if !validMode(*in.Mode) {
			return nil, ErrInvalidMode
		}
		a.Mode = *in.Mode
	}
	if in.Timeout != nil {
		a.Timeout = *in.Timeout
	}
	if in.DownstreamAPIs != nil {
		a.DownstreamAPIs = *in.DownstreamAPIs
	}
	if in.FieldMappings != nil {
		a.FieldMappings = *in.FieldMappings
	}
	if err := s.aggregates.Update(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// Delete removes an aggregate. Editor+.
func (s *Service) Delete(ctx context.Context, id, userID string) error {
	a, err := s.aggregates.FindByID(ctx, id)
	if err != nil {
		return mapErr(err)
	}
	if err := s.projects.RequireEditor(ctx, a.ProjectID, userID); err != nil {
		return err
	}
	return s.aggregates.Delete(ctx, id)
}
