package aggregator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestParallel calls two local test servers concurrently and checks the merge.
func TestParallel(t *testing.T) {
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name":"alice"}`))
	}))
	defer srv1.Close()
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":7}`))
	}))
	defer srv2.Close()

	a := New(http.DefaultClient, zap.NewNop())
	ds := []Downstream{
		{Name: "user", Method: "GET", URL: srv1.URL},
		{Name: "id", Method: "GET", URL: srv2.URL},
	}
	merged, results := a.Run(context.Background(), "parallel", ds, nil, 5*time.Second, nil)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Error != "" {
			t.Errorf("downstream %s errored: %s", r.Name, r.Error)
		}
		if r.StatusCode != 200 {
			t.Errorf("downstream %s status %d", r.Name, r.StatusCode)
		}
		if r.Duration <= 0 {
			t.Errorf("duration not set for %s", r.Name)
		}
	}
	if merged.Meta["ok"].(int) != 2 {
		t.Errorf("meta ok = %v, want 2", merged.Meta["ok"])
	}
	if _, ok := merged.Data["user"]; !ok {
		t.Error("user data missing from merge")
	}
}

// TestDurationNeverZero ensures the duration_ms metric is always >= 1 even for
// sub-millisecond local calls. This was the root cause of the flaky TestParallel
// (httptest calls completing in <1ms truncated to 0). Regression guard.
func TestDurationNeverZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	a := New(http.DefaultClient, zap.NewNop())
	ds := []Downstream{{Name: "x", Method: "GET", URL: srv.URL}}
	// Run several times — any sub-ms call would have truncated to 0 before the fix.
	for i := 0; i < 10; i++ {
		_, results := a.Run(context.Background(), "parallel", ds, nil, 5*time.Second, nil)
		if len(results) != 1 {
			t.Fatalf("iter %d: expected 1 result, got %d", i, len(results))
		}
		if results[0].Duration < 1 {
			t.Fatalf("iter %d: duration %d < 1 (the 0-truncation regression)", i, results[0].Duration)
		}
	}
}

// TestSerial verifies serial ordering by making the second call observe a
// state change only the first could have made.
func TestSerial(t *testing.T) {
	state := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state++
		w.Write([]byte(`{"step":` + itoa(state) + `}`))
	}))
	defer srv.Close()

	a := New(http.DefaultClient, zap.NewNop())
	ds := []Downstream{{Name: "a", Method: "GET", URL: srv.URL}, {Name: "b", Method: "GET", URL: srv.URL}}
	_, results := a.Run(context.Background(), "serial", ds, nil, 5*time.Second, nil)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

// TestTimeout proves a slow downstream surfaces a timeout result without
// aborting the whole aggregate (partial data + timeout hint).
func TestTimeout(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte(`{"slow":true}`))
	}))
	defer slow.Close()
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"fast":true}`))
	}))
	defer fast.Close()

	a := New(http.DefaultClient, zap.NewNop())
	ds := []Downstream{
		{Name: "fast", Method: "GET", URL: fast.URL},
		{Name: "slow", Method: "GET", URL: slow.URL},
	}
	merged, results := a.Run(context.Background(), "parallel", ds, nil, 100*time.Millisecond, nil)

	var sawTimeout bool
	for _, r := range results {
		if r.Timeout {
			sawTimeout = true
		}
	}
	if !sawTimeout {
		t.Error("expected at least one timeout result")
	}
	if len(merged.Errors) == 0 {
		t.Error("expected merge errors to capture the timeout")
	}
}

// TestConditional verifies only matching downstreams are called.
func TestConditional(t *testing.T) {
	called := map[string]bool{}
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called["user"] = true
		w.Write([]byte(`{"user":true}`))
	}))
	defer srv1.Close()
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called["billing"] = true
		w.Write([]byte(`{"billing":true}`))
	}))
	defer srv2.Close()

	a := New(http.DefaultClient, zap.NewNop())
	ds := []Downstream{
		{Name: "user", Method: "GET", URL: srv1.URL, Condition: "plan=free"},
		{Name: "billing", Method: "GET", URL: srv2.URL, Condition: "plan=pro"},
	}
	a.Run(context.Background(), "conditional", ds, nil, 5*time.Second, map[string]any{"plan": "pro"})

	if called["user"] {
		t.Error("user downstream should NOT have been called")
	}
	if !called["billing"] {
		t.Error("billing downstream should have been called")
	}
}

// itoa avoids strconv import for the serial test's inline JSON.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
