package apiservice

import (
	"context"
	"encoding/json"
	"fmt"

	"api-mock-system/internal/id"
	"api-mock-system/internal/models"
)

// Publish snapshots the current API definition into api_versions and advances
// the version counter, setting status to "published". Editor+.
func (s *Service) Publish(ctx context.Context, apiID, userID, comment string) (*models.API, error) {
	a, err := s.apis.FindByID(ctx, apiID)
	if err != nil {
		return nil, mapErr(err)
	}
	if err := s.projects.RequireEditor(ctx, a.ProjectID, userID); err != nil {
		return nil, err
	}

	snapshot, err := snapshotAPI(a)
	if err != nil {
		return nil, err
	}
	nextVersion := a.Version + 1
	ver := &models.APIVersion{
		Base:          models.Base{ID: id.NewUUID()},
		APIID:         a.ID,
		Version:       nextVersion,
		Snapshot:      snapshot,
		ChangeComment: comment,
		CreatedBy:     userID,
	}
	if err := s.apis.SaveVersion(ctx, ver); err != nil {
		return nil, err
	}
	a.Version = nextVersion
	a.Status = "published"
	if err := s.apis.Update(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// Versions returns the version history (newest first).
func (s *Service) Versions(ctx context.Context, apiID, userID string) ([]models.APIVersion, error) {
	a, err := s.apis.FindByID(ctx, apiID)
	if err != nil {
		return nil, mapErr(err)
	}
	if err := s.projects.RequireViewer(ctx, a.ProjectID, userID); err != nil {
		return nil, err
	}
	return s.apis.ListVersions(ctx, apiID)
}

// Rollback restores the API to a prior snapshot and re-publishes at a new
// version number (history is append-only). Editor+.
func (s *Service) Rollback(ctx context.Context, apiID, userID string, version int) (*models.API, error) {
	a, err := s.apis.FindByID(ctx, apiID)
	if err != nil {
		return nil, mapErr(err)
	}
	if err := s.projects.RequireEditor(ctx, a.ProjectID, userID); err != nil {
		return nil, err
	}
	v, err := s.apis.FindVersion(ctx, apiID, version)
	if err != nil {
		return nil, mapErr(err)
	}
	// Preserve the current publish status: rolling back a definition should
	// not silently un-publish an API (which would take its mock route offline).
	currentStatus := a.Status
	restoreAPIFromSnapshot(a, v.Snapshot)
	a.Status = currentStatus

	nextVersion := a.Version + 1
	newSnap, err := snapshotAPI(a)
	if err != nil {
		return nil, err
	}
	if err := s.apis.SaveVersion(ctx, &models.APIVersion{
		Base:          models.Base{ID: id.NewUUID()},
		APIID:         a.ID,
		Version:       nextVersion,
		Snapshot:      newSnap,
		ChangeComment: fmt.Sprintf("rollback to v%d", version),
		CreatedBy:     userID,
	}); err != nil {
		return nil, err
	}
	a.Version = nextVersion
	if err := s.apis.Update(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// snapshotAPI serializes the mutable fields of an API into a JSONMap.
func snapshotAPI(a *models.API) (models.JSONMap, error) {
	raw, err := json.Marshal(struct {
		Name, Description, Method, Path, Status        string
		RequestSchema, ResponseSchema, ResponseExample models.JSONMap
		MockDelay, MockStatusCode                      int
		Tags                                           models.StringArray
	}{
		Name: a.Name, Description: a.Description, Method: a.Method, Path: a.Path, Status: a.Status,
		RequestSchema: a.RequestSchema, ResponseSchema: a.ResponseSchema, ResponseExample: a.ResponseExample,
		MockDelay: a.MockDelay, MockStatusCode: a.MockStatusCode, Tags: a.Tags,
	})
	if err != nil {
		return nil, err
	}
	var out models.JSONMap
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// restoreAPIFromSnapshot copies snapshot fields back onto the API struct.
func restoreAPIFromSnapshot(a *models.API, snap models.JSONMap) {
	if v, ok := snap["Name"].(string); ok {
		a.Name = v
	}
	if v, ok := snap["Description"].(string); ok {
		a.Description = v
	}
	if v, ok := snap["Method"].(string); ok {
		a.Method = v
	}
	if v, ok := snap["Path"].(string); ok {
		a.Path = v
	}
	if v, ok := snap["Status"].(string); ok {
		a.Status = v
	}
	if v, ok := snap["MockDelay"].(float64); ok {
		a.MockDelay = int(v)
	}
	if v, ok := snap["MockStatusCode"].(float64); ok {
		a.MockStatusCode = int(v)
	}
	if v, ok := snap["RequestSchema"].(map[string]any); ok {
		a.RequestSchema = v
	}
	if v, ok := snap["ResponseSchema"].(map[string]any); ok {
		a.ResponseSchema = v
	}
	if v, ok := snap["ResponseExample"].(map[string]any); ok {
		a.ResponseExample = v
	}
	if v, ok := snap["Tags"].([]any); ok {
		tags := make(models.StringArray, 0, len(v))
		for _, t := range v {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
		a.Tags = tags
	}
}
