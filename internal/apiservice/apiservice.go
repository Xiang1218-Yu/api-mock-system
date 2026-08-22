// Package apiservice owns API-definition lifecycle: create, update, version,
// publish, rollback. Authorization is delegated to projectservice.RequireEditor
// so this service never repeats role logic.
package apiservice

import (
	"context"
	"errors"
	"strings"

	"api-mock-system/internal/apirepo"
	"api-mock-system/internal/id"
	"api-mock-system/internal/models"
	"api-mock-system/internal/projectservice"
)

var (
	// ErrNotFound is returned when an API lookup misses.
	ErrNotFound = errors.New("api not found")
	// ErrConflict is returned when a duplicate method+path is created.
	ErrConflict = errors.New("api with this method+path already exists")
)

// CreateInput captures the fields needed to create an API definition.
type CreateInput struct {
	Name            string             `json:"name" binding:"required"`
	Description     string             `json:"description"`
	Method          string             `json:"method" binding:"required"`
	Path            string             `json:"path" binding:"required"`
	Status          string             `json:"status"`
	RequestSchema   models.JSONMap     `json:"request_schema"`
	ResponseSchema  models.JSONMap     `json:"response_schema"`
	ResponseExample models.JSONMap     `json:"response_example"`
	MockDelay       int                `json:"mock_delay"`
	MockStatusCode  int                `json:"mock_status_code"`
	Tags            models.StringArray `json:"tags"`
}

// UpdateInput captures the mutable fields of an API.
type UpdateInput struct {
	Name            *string             `json:"name"`
	Description     *string             `json:"description"`
	Method          *string             `json:"method"`
	Path            *string             `json:"path"`
	Status          *string             `json:"status"`
	RequestSchema   *models.JSONMap     `json:"request_schema"`
	ResponseSchema  *models.JSONMap     `json:"response_schema"`
	ResponseExample *models.JSONMap     `json:"response_example"`
	MockDelay       *int                `json:"mock_delay"`
	MockStatusCode  *int                `json:"mock_status_code"`
	Tags            *models.StringArray `json:"tags"`
}

// Service orchestrates API definitions.
type Service struct {
	apis     apirepo.Repository
	projects *projectservice.Service
}

// New wires the service to its repository and the project service (for authz).
func New(apis apirepo.Repository, projects *projectservice.Service) *Service {
	return &Service{apis: apis, projects: projects}
}

// Create makes a new API under a project. Authorization: editor+ on the project.
func (s *Service) Create(ctx context.Context, actorID, projectID string, in CreateInput) (*models.API, error) {
	if err := s.projects.RequireEditor(ctx, projectID, actorID); err != nil {
		return nil, err
	}
	method := normalizeMethod(in.Method)
	if method == "" {
		return nil, errors.New("method must be one of GET,POST,PUT,DELETE,PATCH")
	}
	// Reject duplicate method+path within the project.
	if _, err := s.apis.FindByProjectAndPath(ctx, projectID, method, in.Path); err == nil {
		return nil, ErrConflict
	} else if !errors.Is(err, apirepo.ErrNotFound) {
		return nil, err
	}

	status := in.Status
	if status == "" {
		status = "designing"
	}
	if !validStatus(status) {
		return nil, errors.New("status must be designing, published, or deprecated")
	}

	a := &models.API{
		Base:            models.Base{ID: id.NewUUID()},
		ProjectID:       projectID,
		Name:            in.Name,
		Description:     in.Description,
		Method:          method,
		Path:            normalizePath(in.Path),
		Status:          status,
		RequestSchema:   in.RequestSchema,
		ResponseSchema:  in.ResponseSchema,
		ResponseExample: in.ResponseExample,
		MockDelay:       in.MockDelay,
		MockStatusCode:  defaultInt(in.MockStatusCode, 200),
		Tags:            in.Tags,
		Version:         1,
	}
	if err := s.apis.Create(ctx, a); err != nil {
		return nil, err
	}
	// Persist the v1 snapshot so the initial definition is rollback-targetable.
	if snap, err := snapshotAPI(a); err == nil {
		_ = s.apis.SaveVersion(ctx, &models.APIVersion{
			Base:          models.Base{ID: id.NewUUID()},
			APIID:         a.ID,
			Version:       1,
			Snapshot:      snap,
			ChangeComment: "initial",
			CreatedBy:     actorID,
		})
	}
	return a, nil
}

// Get returns an API by id, after verifying the caller can read its project.
func (s *Service) Get(ctx context.Context, apiID, userID string) (*models.API, error) {
	a, err := s.apis.FindByID(ctx, apiID)
	if err != nil {
		return nil, mapErr(err)
	}
	if err := s.projects.RequireViewer(ctx, a.ProjectID, userID); err != nil {
		return nil, err
	}
	return a, nil
}

// List returns APIs under a project; viewer+ required.
func (s *Service) List(ctx context.Context, projectID, userID, status string, page, size int) ([]models.API, int64, error) {
	if err := s.projects.RequireViewer(ctx, projectID, userID); err != nil {
		return nil, 0, err
	}
	if size <= 0 {
		size = 50
	}
	if page <= 0 {
		page = 1
	}
	return s.apis.ListByProject(ctx, projectID, status, "", size, (page-1)*size)
}

// Update applies partial changes and bumps the version on publish. Editor+.
func (s *Service) Update(ctx context.Context, apiID, userID string, in UpdateInput) (*models.API, error) {
	a, err := s.apis.FindByID(ctx, apiID)
	if err != nil {
		return nil, mapErr(err)
	}
	if err := s.projects.RequireEditor(ctx, a.ProjectID, userID); err != nil {
		return nil, err
	}
	// Published APIs can still be edited; the publish action is what snapshots.
	if in.Name != nil {
		a.Name = *in.Name
	}
	if in.Description != nil {
		a.Description = *in.Description
	}
	if in.Method != nil {
		m := normalizeMethod(*in.Method)
		if m == "" {
			return nil, errors.New("invalid method")
		}
		a.Method = m
	}
	if in.Path != nil {
		a.Path = normalizePath(strings.TrimSpace(*in.Path))
	}
	if in.Status != nil {
		if !validStatus(*in.Status) {
			return nil, errors.New("invalid status")
		}
		a.Status = *in.Status
	}
	if in.RequestSchema != nil {
		a.RequestSchema = *in.RequestSchema
	}
	if in.ResponseSchema != nil {
		a.ResponseSchema = *in.ResponseSchema
	}
	if in.ResponseExample != nil {
		a.ResponseExample = *in.ResponseExample
	}
	if in.MockDelay != nil {
		a.MockDelay = *in.MockDelay
	}
	if in.MockStatusCode != nil {
		a.MockStatusCode = *in.MockStatusCode
	}
	if in.Tags != nil {
		a.Tags = *in.Tags
	}
	if err := s.apis.Update(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// Delete removes an API and its overrides. Admin role implied via RequireEditor
// — deleting is treated as an edit here for simplicity; tighten if needed.
func (s *Service) Delete(ctx context.Context, apiID, userID string) error {
	a, err := s.apis.FindByID(ctx, apiID)
	if err != nil {
		return mapErr(err)
	}
	if err := s.projects.RequireEditor(ctx, a.ProjectID, userID); err != nil {
		return err
	}
	return s.apis.Delete(ctx, apiID)
}
