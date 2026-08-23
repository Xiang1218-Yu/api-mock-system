package apiservice

import (
	"context"

	"api-mock-system/internal/models"
	"api-mock-system/internal/pathmatch"
)

// ListPublished returns all published APIs for a project (used by the mock and
// openapi services). No authorization check — this is called internally by
// trusted services that have already checked access.
func (s *Service) ListPublished(ctx context.Context, projectID string) ([]models.API, error) {
	return s.apis.ListPublishedByProject(ctx, projectID)
}

// FindForMock looks up an API by project + method + path for the mock router.
// The path is concrete (e.g. "/users/42"); this matches it against parameterized
// stored paths (e.g. "/users/:id") and returns the first published API that
// matches. Returns apirepo.ErrNotFound on miss.
func (s *Service) FindForMock(ctx context.Context, projectID, method, path string) (*models.API, error) {
	apis, err := s.apis.ListPublishedByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for i := range apis {
		a := &apis[i]
		if a.Method != method {
			continue
		}
		if _, ok := pathmatch.Match(a.Path, path); ok {
			return a, nil
		}
	}
	// Return the service-level sentinel so callers recognize an unroutable path.
	return nil, ErrNotFound
}

// RequireEditor loads an API and enforces editor+ on its project. Exported for
// sibling services that authorize writes against an API's project.
func (s *Service) RequireEditor(ctx context.Context, apiID, userID string) (*models.API, error) {
	a, err := s.apis.FindByID(ctx, apiID)
	if err != nil {
		return nil, mapErr(err)
	}
	if err := s.projects.RequireEditor(ctx, a.ProjectID, userID); err != nil {
		return nil, err
	}
	return a, nil
}

// RequireViewer loads an API and enforces viewer+ on its project.
func (s *Service) RequireViewer(ctx context.Context, apiID, userID string) (*models.API, error) {
	return s.Get(ctx, apiID, userID)
}
