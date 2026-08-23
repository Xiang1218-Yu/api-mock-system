package docservice_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"api-mock-system/internal/apirepo"
	"api-mock-system/internal/apiservice"
	"api-mock-system/internal/docservice"
	"api-mock-system/internal/models"
	"api-mock-system/internal/projectrepo"
	"api-mock-system/internal/projectservice"
	"api-mock-system/internal/storage"

	"github.com/goccy/go-yaml"
	"go.uber.org/zap"
)

var nextID atomic.Uint64

// newID returns a unique-enough text primary key for test rows.
func newID() string { return "id-" + itoa(nextID.Add(1)) }

func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// yamlUnmarshal decodes YAML with the same library the service marshals with.
func yamlUnmarshal(b []byte, v any) error { return yaml.Unmarshal(b, v) }

// newStore opens an isolated SQLite database in a temp file so each test gets a
// fresh, migrated schema without colliding with the working repo's api_mock.db.
func newStore(t *testing.T) *storage.Store {
	t.Helper()
	dir := t.TempDir()
	log := zap.NewNop()
	store, err := storage.Open(context.Background(), filepath.Join(dir, "test.db"), log)
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// seedProjectAndAPI inserts a public project plus one published API directly
// via repositories, bypassing authorization — the doc layer re-checks viewer
// access on the project, and a public project satisfies any caller.
func seedProjectAndAPI(t *testing.T, db *storage.Store) (projectID, userID string) {
	t.Helper()
	projects := projectrepo.New(db.DB)
	projectID = newID()
	if err := projects.Create(context.Background(), &models.Project{
		Base:       models.Base{ID: projectID},
		Name:       "Orders API",
		BasePath:   "/v1",
		Visibility: "public",
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	apiID := newID()
	if err := apirepo.New(db.DB).Create(context.Background(), &models.API{
		Base:      models.Base{ID: apiID},
		ProjectID: projectID,
		Name:      "List orders",
		Method:    "GET",
		Path:      "/orders",
		Status:    "published",
	}); err != nil {
		t.Fatalf("create api: %v", err)
	}
	return projectID, ""
}

// TestOpenAPIJSONIncludesPublishedAPI is the end-to-end regression for the
// assignOp method-case bug: a published GET /orders must appear as an operation
// on the document's /orders path item, not be silently dropped.
func TestOpenAPIJSONIncludesPublishedAPI(t *testing.T) {
	store := newStore(t)
	pid, uid := seedProjectAndAPI(t, store)
	docs := docservice.New(projectservice.New(projectrepo.New(store.DB)), apiservice.New(apirepo.New(store.DB), projectservice.New(projectrepo.New(store.DB))))

	data, err := docs.OpenAPIJSON(context.Background(), pid, uid)
	if err != nil {
		t.Fatalf("OpenAPIJSON: %v", err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			Summary string `json:"summary"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("OpenAPIJSON output is not valid JSON: %v", err)
	}
	op, ok := doc.Paths["/orders"]["get"]
	if !ok {
		t.Fatalf("GET /orders missing from document (paths=%v); assignOp dropped it", doc.Paths)
	}
	if op.Summary != "List orders" {
		t.Errorf("GET /orders summary = %q, want %q", op.Summary, "List orders")
	}
}

// TestOpenAPIYAMLIsYAML is the regression for the bug where OpenAPIYAML
// returned JSON bytes: the YAML output must parse as YAML and must differ from
// the JSON bytes, and it must still carry the published operation.
func TestOpenAPIYAMLIsYAML(t *testing.T) {
	store := newStore(t)
	pid, uid := seedProjectAndAPI(t, store)
	projects := projectservice.New(projectrepo.New(store.DB))
	docs := docservice.New(projects, apiservice.New(apirepo.New(store.DB), projects))

	yamlBytes, err := docs.OpenAPIYAML(context.Background(), pid, uid)
	if err != nil {
		t.Fatalf("OpenAPIYAML: %v", err)
	}
	jsonBytes, _ := docs.OpenAPIJSON(context.Background(), pid, uid)

	yamlStr := string(yamlBytes)
	if strings.HasPrefix(strings.TrimSpace(yamlStr), "{") {
		t.Fatalf("OpenAPIYAML returned JSON-looking output (starts with '{'):\n%s", yamlStr)
	}
	if string(yamlBytes) == string(jsonBytes) {
		t.Fatalf("OpenAPIYAML returned the same bytes as OpenAPIJSON")
	}

	parsed := struct {
		OpenAPI string `yaml:"openapi"`
		Paths   map[string]map[string]struct {
			Summary string `yaml:"summary"`
		} `yaml:"paths"`
	}{}
	if err := yamlUnmarshal(yamlBytes, &parsed); err != nil {
		t.Fatalf("OpenAPIYAML output did not parse as YAML: %v", err)
	}
	if parsed.OpenAPI != "3.0.3" {
		t.Errorf("yaml openapi = %q, want 3.0.3", parsed.OpenAPI)
	}
	if op, ok := parsed.Paths["/orders"]["get"]; !ok || op.Summary != "List orders" {
		t.Errorf("yaml paths missing GET /orders, got %#v", parsed.Paths)
	}
}
