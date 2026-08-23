package mockservice

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"api-mock-system/internal/apirepo"
	"api-mock-system/internal/apiservice"
	"api-mock-system/internal/cache"
	"api-mock-system/internal/id"
	"api-mock-system/internal/mockdatarepo"
	"api-mock-system/internal/models"
	"api-mock-system/internal/projectrepo"
	"api-mock-system/internal/projectservice"
	"api-mock-system/internal/storage"

	"go.uber.org/zap"
)

// openTestStore opens a migrated SQLite DB in the test's temp dir.
func openTestStore(t *testing.T) *storage.Store {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := storage.Open(ctx, dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// newMockService builds the real apiservice + mockservice against a throwaway
// SQLite database, mirroring app.Run's wiring but without the HTTP layer. It
// exercises the genuine Resolve code path (FindForMock -> generateMockBody).
func newMockService(t *testing.T) *Service {
	t.Helper()
	return newMockServiceFromStore(t, openTestStore(t))
}

// newMockServiceFromStore wires the services against an already-open store,
// so a test can seed rows into the same DB it resolves against.
func newMockServiceFromStore(t *testing.T, store *storage.Store) *Service {
	t.Helper()
	projects := projectservice.New(projectrepo.New(store.DB))
	apis := apiservice.New(apirepo.New(store.DB), projects)
	overrides := mockdatarepo.New(store.DB)
	return New(apis, overrides, cache.New(), zap.NewNop())
}

// seedPublishedAPI inserts a published API with the given response schema.
func seedPublishedAPI(t *testing.T, store *storage.Store, schema models.JSONMap) {
	t.Helper()
	api := &models.API{
		Base:           models.Base{ID: id.NewUUID()},
		ProjectID:      "proj-test",
		Name:           "list users",
		Method:         "GET",
		Path:           "/users",
		Status:         "published",
		Version:        1,
		ResponseSchema: schema,
	}
	if err := store.DB.Create(api).Error; err != nil {
		t.Fatalf("seed api: %v", err)
	}
}

func TestResolve_ArrayResponseSchema_ReturnsArray(t *testing.T) {
	store := openTestStore(t)
	svc := newMockServiceFromStore(t, store)
	seedPublishedAPI(t, store, models.JSONMap{
		"type": "array",
		"items": models.JSONMap{
			"type": "object",
			"properties": models.JSONMap{
				"id":   models.JSONMap{"type": "integer"},
				"name": models.JSONMap{"type": "string"},
			},
			"required": []any{"id", "name"},
		},
		"minItems": 2,
		"maxItems": 2,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The original bug: this call panicked on the unchecked items assertion, so
	// the client got a 500 with no mock body. It must now return a 200 array.
	resp, err := svc.Resolve(ctx, "proj-test", "GET", "/users", "", "")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("Resolve returned nil response")
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// The body must serialize as a JSON array, not a single object.
	b, err := json.Marshal(resp.Body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	var arr []any
	if err := json.Unmarshal(b, &arr); err != nil {
		t.Fatalf("response body is not a JSON array: %v (body=%s)", err, b)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d (body=%s)", len(arr), b)
	}
	obj, ok := arr[0].(map[string]any)
	if !ok {
		t.Fatalf("array element is not an object: %T (body=%s)", arr[0], b)
	}
	if _, ok := obj["id"]; !ok {
		t.Errorf("missing required field id in element (body=%s)", b)
	}
}

// TestResolve_ArrayResponseSchema_MalformedNeverCrashes exercises the exact
// crash path: before the fix, calling a published array-response API panicked.
// Here we use the previously-crashing schema variants and confirm Resolve
// completes (either with data or a controlled ErrInvalidSchema), never crashing.
func TestResolve_ArrayResponseSchema_MalformedNeverCrashes(t *testing.T) {
	cases := map[string]models.JSONMap{
		"no-items":     {"type": "array", "minItems": 2, "maxItems": 3},
		"tuple-items":  {"type": "array", "items": []any{models.JSONMap{"type": "integer"}}},
		"items-false":  {"type": "array", "items": false},
		"items-number": {"type": "array", "items": 42},
	}
	for name, schema := range cases {
		t.Run(name, func(t *testing.T) {
			store := openTestStore(t)
			svc := newMockServiceFromStore(t, store)
			seedPublishedAPI(t, store, schema)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Resolve panicked for %s: %v", name, r)
				}
			}()
			resp, err := svc.Resolve(ctx, "proj-test", "GET", "/users", "", "")
			if err != nil {
				// A controlled service error is acceptable for a broken schema;
				// the requirement is "not a crash". Confirm it's our sentinel.
				if err != ErrInvalidSchema {
					t.Fatalf("expected ErrInvalidSchema, got %v", err)
				}
				return
			}
			if resp == nil {
				t.Fatalf("got nil response with nil error for %s", name)
			}
		})
	}
}
