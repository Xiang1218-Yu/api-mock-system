// Package mockservice resolves an inbound mock request to a concrete response:
// look up the API, check for a fixed override, otherwise generate from schema.
// It also manages override CRUD and caches generated responses per request.
package mockservice

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"api-mock-system/internal/apiservice"
	"api-mock-system/internal/cache"
	"api-mock-system/internal/id"
	"api-mock-system/internal/mockdatarepo"
	"api-mock-system/internal/mockengine"
	"api-mock-system/internal/models"
	"go.uber.org/zap"
)

var (
	// ErrNotFound is returned when no published API matches the request.
	ErrNotFound = errors.New("mock route not found")
	// ErrNotPublished is returned when the matched API is not published.
	ErrNotPublished = errors.New("api is not published")
)

// Response is the mock service's resolved answer.
type Response struct {
	StatusCode int
	Body       any
	Headers    map[string]string
	Delay      time.Duration
	// APIID is the matched API's id, surfaced so the handler can attribute the
	// call in call_logs without re-running the lookup.
	APIID string
}

// OverrideInput sets a fixed mock value for a request key.
type OverrideInput struct {
	Key     string         `json:"key"`
	Value   models.JSONMap `json:"value"`
	Enabled *bool          `json:"enabled,omitempty"`
}

// Service resolves mock responses and manages overrides.
type Service struct {
	apis      *apiservice.Service // pointer; methods are defined on the pointer receiver
	overrides mockdatarepo.Repository
	engine    *mockengine.Engine // base engine; per-request engines are seeded
	cache     *cache.Cache
	ttl       time.Duration
	log       *zap.Logger
}

// New wires the service together. ttl bounds the response cache.
func New(apis *apiservice.Service, overrides mockdatarepo.Repository, c *cache.Cache, log *zap.Logger) *Service {
	// The base engine is unused for actual generation (we seed per-request),
	// but kept for callers that want an unseeded generator.
	return &Service{apis: apis, overrides: overrides, engine: mockengine.New(nil), cache: c, ttl: 30 * time.Second, log: log}
}

// Resolve builds a mock response for the inbound request. It applies delay and
// status overrides, consults fixed overrides, and generates from schema when
// no override matches. Responses are cached by request signature.
func (s *Service) Resolve(ctx context.Context, projectID, method, path, query, body string) (*Response, error) {
	method = normalizeMethod(method)
	path = normalizeRequestPath(path)
	api, err := s.apis.FindForMock(ctx, projectID, method, path)
	if err != nil {
		if errors.Is(err, apiservice.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if api.Status != "published" {
		return nil, ErrNotPublished
	}
	if err := validateRequest(api.RequestSchema, body); err != nil {
		return nil, err
	}

	cacheKey := "mock:" + api.ID + "|" + method + "|" + path + "|" + query
	if v, ok := s.cache.Get(cacheKey); ok {
		if r, ok := v.(*Response); ok {
			// Delay still applies to cached responses, per "response delay simulation".
			s.sleep(api.MockDelay)
			return r, nil
		}
	}

	// 1. Fixed override keyed by request signature wins over generation.
	if ov, err := s.overrides.Get(ctx, api.ID, mockKey(method, path, query, body)); err == nil {
		resp := &Response{
			StatusCode: defaultInt(api.MockStatusCode, 200),
			Body:       ov.Value,
			Headers:    map[string]string{"Content-Type": "application/json"},
			APIID:      api.ID,
		}
		s.cache.Set(cacheKey, resp, s.ttl)
		s.sleep(api.MockDelay)
		return resp, nil
	}

	// 2. Otherwise generate from the response schema, deterministically.
	engine := mockengine.NewSeeded(api.ID, method, path, query, body)
	var mockBody any
	if len(api.ResponseSchema) > 0 {
		mockBody = engine.Generate(api.ResponseSchema)
	} else if len(api.ResponseExample) > 0 {
		mockBody = api.ResponseExample
	} else {
		mockBody = map[string]any{"message": "mock response", "ok": true}
	}
	resp := &Response{
		StatusCode: defaultInt(api.MockStatusCode, 200),
		Body:       mockBody,
		Headers:    map[string]string{"Content-Type": "application/json"},
		APIID:      api.ID,
	}
	s.cache.Set(cacheKey, resp, s.ttl)
	s.sleep(api.MockDelay)
	return resp, nil
}

// SetOverride stores a fixed mock value for a request key. Editor+.
func (s *Service) SetOverride(ctx context.Context, apiID, userID string, in OverrideInput) error {
	// Editor authorization on the API's project. The previous implementation
	// relied on the handler; the service now enforces it itself so the rule can
	// never be bypassed by a new caller.
	if _, err := s.apis.RequireEditor(ctx, apiID, userID); err != nil {
		return err
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	if err := s.overrides.Set(ctx, &models.MockData{
		Base:    models.Base{ID: id.NewUUID()},
		APIID:   apiID,
		Key:     in.Key,
		Value:   in.Value,
		Enabled: enabled,
	}); err != nil {
		return err
	}
	// Drop cached responses for this API so the override takes effect at once.
	s.Invalidate(ctx, apiID)
	return nil
}

// ClearOverride removes a single override by key. Editor+.
func (s *Service) ClearOverride(ctx context.Context, apiID, key, userID string) error {
	if _, err := s.apis.RequireEditor(ctx, apiID, userID); err != nil {
		return err
	}
	if err := s.overrides.Clear(ctx, apiID, key); err != nil {
		return err
	}
	s.Invalidate(context.Background(), apiID)
	return nil
}

// ListOverrides returns all overrides for an API. Viewer+.
func (s *Service) ListOverrides(ctx context.Context, apiID, userID string) ([]models.MockData, error) {
	if _, err := s.apis.RequireViewer(ctx, apiID, userID); err != nil {
		return nil, err
	}
	return s.overrides.List(ctx, apiID)
}

// Invalidate drops cached mock responses for an API (e.g. after publish/override).
// The cache keys this API's entries under the "mock:<apiID>|" prefix, so the
// purge is scoped — other APIs' cached responses survive.
func (s *Service) Invalidate(ctx context.Context, apiID string) {
	_ = ctx
	s.cache.DeleteByPrefix("mock:" + apiID + "|")
}

// sleep applies the configured mock delay, capped at 5s per the spec range.
func (s *Service) sleep(ms int) {
	if ms <= 0 {
		return
	}
	if ms > 5000 {
		ms = 5000
	}
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

// mockKey derives the override key from the request. Override entries are
// matched on this string so a user can fix a response for one specific call.
func mockKey(method, path, query, body string) string {
	return method + " " + path + "?" + query + " " + body
}

func normalizeMethod(method string) string {
	return strings.ToUpper(strings.TrimSpace(method))
}

func normalizeRequestPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if path[0] != '/' {
		return "/" + path
	}
	return path
}

func defaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

// MarshalBody renders a Response body to bytes for the HTTP writer.
func (r *Response) MarshalBody() ([]byte, error) {
	return json.Marshal(r.Body)
}
