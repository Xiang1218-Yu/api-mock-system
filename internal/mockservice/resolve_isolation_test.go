package mockservice

import (
	"context"
	"sync"
	"testing"

	"api-mock-system/internal/apiservice"
	"api-mock-system/internal/cache"
	"api-mock-system/internal/mockdatarepo"
	"api-mock-system/internal/models"

	"go.uber.org/zap"
)

// fakeAPIRepo is an in-memory apirepo.Repository sufficient to drive
// apiservice.FindForMock (the only path Resolve touches). The version methods
// are stubs because Resolve never calls them.
type fakeAPIRepo struct {
	mu   sync.Mutex
	apis map[string]*models.API
}

func newFakeAPIRepo(apis ...*models.API) *fakeAPIRepo {
	m := make(map[string]*models.API, len(apis))
	for _, a := range apis {
		m[a.ID] = a
	}
	return &fakeAPIRepo{apis: m}
}

func (r *fakeAPIRepo) Create(_ context.Context, a *models.API) error { return nil }
func (r *fakeAPIRepo) FindByID(_ context.Context, id string) (*models.API, error) {
	if a, ok := r.apis[id]; ok {
		return a, nil
	}
	return nil, apiservice.ErrNotFound
}
func (r *fakeAPIRepo) FindByProjectAndPath(_ context.Context, projectID, method, path string) (*models.API, error) {
	return nil, apiservice.ErrNotFound
}
func (r *fakeAPIRepo) ListByProject(_ context.Context, projectID, status, groupID string, limit, offset int) ([]models.API, int64, error) {
	return nil, 0, nil
}
func (r *fakeAPIRepo) Update(_ context.Context, a *models.API) error { return nil }
func (r *fakeAPIRepo) Delete(_ context.Context, id string) error     { return nil }
func (r *fakeAPIRepo) ListPublishedByProject(_ context.Context, projectID string) ([]models.API, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.API
	for _, a := range r.apis {
		if a.ProjectID == projectID && a.Status == "published" {
			out = append(out, *a)
		}
	}
	return out, nil
}
func (r *fakeAPIRepo) SaveVersion(_ context.Context, v *models.APIVersion) error { return nil }
func (r *fakeAPIRepo) ListVersions(_ context.Context, apiID string) ([]models.APIVersion, error) {
	return nil, nil
}
func (r *fakeAPIRepo) FindVersion(_ context.Context, apiID string, version int) (*models.APIVersion, error) {
	return nil, apiservice.ErrNotFound
}

// fakeMockRepo is an in-memory mockdatarepo.Repository for override CRUD.
type fakeMockRepo struct {
	mu   sync.Mutex
	rows []*models.MockData
}

func (r *fakeMockRepo) Set(_ context.Context, m *models.MockData) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.rows {
		if e.APIID == m.APIID && e.Key == m.Key {
			e.Value = m.Value
			e.Enabled = m.Enabled
			return nil
		}
	}
	clone := *m
	r.rows = append(r.rows, &clone)
	return nil
}
func (r *fakeMockRepo) Get(_ context.Context, apiID, key string) (*models.MockData, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.rows {
		if e.APIID == apiID && e.Key == key && e.Enabled {
			c := *e
			return &c, nil
		}
	}
	return nil, mockdatarepo.ErrNotFound
}
func (r *fakeMockRepo) Clear(_ context.Context, apiID, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, e := range r.rows {
		if e.APIID == apiID && e.Key == key {
			r.rows = append(r.rows[:i], r.rows[i+1:]...)
			return nil
		}
	}
	return nil
}
func (r *fakeMockRepo) ClearAll(_ context.Context, apiID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows = nil
	return nil
}
func (r *fakeMockRepo) List(_ context.Context, apiID string) ([]models.MockData, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.MockData
	for _, e := range r.rows {
		if e.APIID == apiID {
			out = append(out, *e)
		}
	}
	return out, nil
}

func publishedAPI() *models.API {
	return &models.API{
		Base:      models.Base{ID: "api-1"},
		ProjectID: "proj-1",
		Method:    "GET",
		Path:      "/users",
		Status:    "published",
		ResponseSchema: models.JSONMap{
			"type": "object",
			"properties": map[string]any{
				"id":   map[string]any{"type": "integer", "minimum": 1, "maximum": 100000},
				"name": map[string]any{"type": "string"},
			},
			"required": []any{"id"},
		},
	}
}

// newResolveService wires a real mockservice.Service against fake repos. The
// project service is nil: Resolve's only apiservice call is FindForMock, which
// does not touch authorization.
func newResolveService(t *testing.T, apis ...*models.API) *Service {
	t.Helper()
	apiRepo := newFakeAPIRepo(apis...)
	apisvc := apiservice.New(apiRepo, nil)
	return New(apisvc, &fakeMockRepo{}, cache.New(), zap.NewNop())
}

// TestResolve_IsolatesByQuery locks the headline bug: two calls to the same
// published API differing only by query params must return distinct bodies.
// Before the fix the cache key ignored query, so the second call echoed the
// first request's cached result.
func TestResolve_IsolatesByQuery(t *testing.T) {
	svc := newResolveService(t, publishedAPI())
	ctx := context.Background()

	r1, err := svc.Resolve(ctx, "proj-1", "GET", "/users", "page=1", "")
	if err != nil {
		t.Fatalf("resolve page=1: %v", err)
	}
	r2, err := svc.Resolve(ctx, "proj-1", "GET", "/users", "page=2", "")
	if err != nil {
		t.Fatalf("resolve page=2: %v", err)
	}
	if bodyJSON(t, r1) == bodyJSON(t, r2) {
		t.Fatalf("different query returned identical body; got %s for both", bodyJSON(t, r1))
	}
}

// TestResolve_IsolatesByBody locks the JSON-body half of the bug: two calls
// differing only by request body must return distinct results.
func TestResolve_IsolatesByBody(t *testing.T) {
	svc := newResolveService(t, publishedAPI())
	ctx := context.Background()

	r1, err := svc.Resolve(ctx, "proj-1", "GET", "/users", "", `{"id":1}`)
	if err != nil {
		t.Fatalf("resolve body1: %v", err)
	}
	r2, err := svc.Resolve(ctx, "proj-1", "GET", "/users", "", `{"id":2}`)
	if err != nil {
		t.Fatalf("resolve body2: %v", err)
	}
	if bodyJSON(t, r1) == bodyJSON(t, r2) {
		t.Fatalf("different body returned identical body; got %s for both", bodyJSON(t, r1))
	}
}

// TestResolve_CachesRepeatableForSameRequest locks the consistency half: the
// same request repeated must return the same (cached) value — isolation did
// not break the "same request returns same mock data" requirement.
func TestResolve_CachesRepeatableForSameRequest(t *testing.T) {
	svc := newResolveService(t, publishedAPI())
	ctx := context.Background()

	r1, err := svc.Resolve(ctx, "proj-1", "GET", "/users", "page=1", "")
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	r2, err := svc.Resolve(ctx, "proj-1", "GET", "/users", "page=1", "")
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if bodyJSON(t, r1) != bodyJSON(t, r2) {
		t.Fatalf("same request returned inconsistent bodies: %s vs %s", bodyJSON(t, r1), bodyJSON(t, r2))
	}
}

// TestResolve_FixedOverrideOnlyAnswersItsOwnRequest locks the "fixed response
// is not unexpectedly overwritten" half. A fixed override set for exactly one
// request signature (GET /users?page=1, empty body) must:
//   - answer the request it was set for with its fixed value, and
//   - NOT leak into a request that differs by query or body, which must still
//     generate its own response rather than serving the override's value.
func TestResolve_FixedOverrideOnlyAnswersItsOwnRequest(t *testing.T) {
	svc := newResolveService(t, publishedAPI())
	ctx := context.Background()

	// Set a fixed override for exactly one request signature, using the same
	// key derivation Resolve uses for override lookup.
	key := mockKey("GET", "/users", "page=1", "")
	overrideVal := models.JSONMap{"fixed": true}
	if err := svc.overrides.Set(ctx, &models.MockData{
		Base:    models.Base{ID: "ov-1"},
		APIID:   "api-1",
		Key:     key,
		Value:   overrideVal,
		Enabled: true,
	}); err != nil {
		t.Fatalf("set override: %v", err)
	}

	// The targeted request must receive the fixed override.
	rTarget, err := svc.Resolve(ctx, "proj-1", "GET", "/users", "page=1", "")
	if err != nil {
		t.Fatalf("resolve targeted request: %v", err)
	}
	if got := bodyJSON(t, rTarget); got != `{"fixed":true}` {
		t.Fatalf("targeted request did not return fixed override: got %s", got)
	}

	// A request differing only by query must NOT receive the override's value.
	rOtherQ, err := svc.Resolve(ctx, "proj-1", "GET", "/users", "page=2", "")
	if err != nil {
		t.Fatalf("resolve different query: %v", err)
	}
	if got := bodyJSON(t, rOtherQ); got == `{"fixed":true}` {
		t.Fatalf("override leaked into a different-query request: got %s", got)
	}

	// A request differing only by body must NOT receive the override's value.
	rOtherB, err := svc.Resolve(ctx, "proj-1", "GET", "/users", "page=1", `{"id":1}`)
	if err != nil {
		t.Fatalf("resolve different body: %v", err)
	}
	if got := bodyJSON(t, rOtherB); got == `{"fixed":true}` {
		t.Fatalf("override leaked into a different-body request: got %s", got)
	}
}

// bodyJSON renders a Response body canonically so two bodies can be compared
// byte-for-byte regardless of map key ordering.
func bodyJSON(t *testing.T, r *Response) string {
	t.Helper()
	b, err := r.MarshalBody()
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return string(b)
}
