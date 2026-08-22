// Package docservice assembles OpenAPI documents from a project's published
// APIs. It delegates document construction to package openapi and project/api
// lookups to their services, so this layer only wires the two together.
package docservice

import (
	"context"
	"errors"

	"api-mock-system/internal/apiservice"
	"api-mock-system/internal/models"
	"api-mock-system/internal/openapi"
	"api-mock-system/internal/projectservice"
)

// ErrNotFound is returned when the project doesn't exist or is inaccessible.
var ErrNotFound = errors.New("project not found")

// Service builds OpenAPI documents.
type Service struct {
	projects *projectservice.Service
	apis     *apiservice.Service
}

// New wires the service.
func New(projects *projectservice.Service, apis *apiservice.Service) *Service {
	return &Service{projects: projects, apis: apis}
}

// OpenAPIJSON returns the project's document as pretty JSON bytes.
func (s *Service) OpenAPIJSON(ctx context.Context, projectID, userID string) ([]byte, error) {
	doc, err := s.build(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	return openapi.ToJSON(doc)
}

// OpenAPIYAML returns the project's document as YAML bytes.
func (s *Service) OpenAPIYAML(ctx context.Context, projectID, userID string) ([]byte, error) {
	doc, err := s.build(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	return openapi.ToYAML(doc)
}

// build assembles the document. Viewer+ authorization is enforced on the
// project lookup; published APIs are then enumerated without a second check.
func (s *Service) build(ctx context.Context, projectID, userID string) (*openapi.Document, error) {
	p, err := s.projects.Get(ctx, projectID, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	apis, err := s.apis.ListPublished(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return openapi.Build(p, apis), nil
}

func mapErr(err error) error {
	if errors.Is(err, projectservice.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// Ensure models import survives (used transitively via openapi/apimodels).
var _ = models.Project{}
