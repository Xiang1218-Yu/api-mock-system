// Package mockservice resolves an inbound mock request to a concrete response:
// look up the API, check for a fixed override, otherwise generate from schema.
// It also manages override CRUD and caches generated responses per request.
package mockservice

import (
	"context"
	"encoding/json"
	"errors"
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
	// ErrInvalidSchema is returned when a published API's response schema is
	// structurally broken in a way that prevents generating a well-formed mock
	// (e.g. declares "type":"array" but the generator yields a non-array). It is
	// a controlled service error rather than a crashed request, so the client
	// still gets a normal HTTP response.
	ErrInvalidSchema = errors.New("mock response schema is malformed")
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
	switch {
	case len(api.ResponseSchema) > 0:
		// The engine dispatches on the schema's "type" itself, including
		// "array" (it returns a []any honoring items/minItems/maxItems), so the
		// full response schema is handed over verbatim. Do NOT unwrap "items"
		// here: that would both flatten an array response into a single element
		// (wrong shape) and require an unchecked type assertion that panics when
		// items is the JSON-decoded map[string]any rather than models.JSONMap.
		generated, err := generateMockBody(engine, api.ResponseSchema)
		if err != nil {
			return nil, err
		}
		mockBody = generated
	case len(api.ResponseExample) > 0:
		mockBody = api.ResponseExample
	default:
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

func defaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

// generateMockBody runs the engine against the response schema and turns any
// failure — a malformed schema that would otherwise panic, or a declared type
// the generator could not honor — into a controlled ErrInvalidSchema instead of
// a crashed request.
//
// "A well-formed schema should return normally": a schema whose declared
// "type" matches the generated value's shape succeeds. "A malformed schema
// should become a controlled service error": anything that panics, or yields a
// value whose kind contradicts a declared array type, is reported as an error
// the handler can map to a 500 without dropping the connection.
func generateMockBody(engine *mockengine.Engine, schema models.JSONMap) (any, error) {
	var (
		body any
		ok   bool
	)
	// The engine is pure and panic-safe for the cases it handles, but a schema
	// authored with an unexpected keyword shape could still trip an assertion
	// somewhere downstream. Recover so the caller sees an error, never a crash.
	func() {
		defer func() {
			if r := recover(); r != nil {
				ok = false
			}
		}()
		body = engine.Generate(schema)
		ok = true
	}()
	if !ok {
		return nil, ErrInvalidSchema
	}
	// Shape check: an array response must serialize as a JSON array. The engine
	// returns []any for "type":"array"; if it produced anything else, the schema
	// was structurally inconsistent — surface it as a controlled error rather
	// than returning a non-array body for a declared array.
	if schema["type"] == "array" {
		if _, isArr := body.([]any); !isArr {
			return nil, ErrInvalidSchema
		}
	}
	return body, nil
}

// MarshalBody renders a Response body to bytes for the HTTP writer.
func (r *Response) MarshalBody() ([]byte, error) {
	return json.Marshal(r.Body)
}
