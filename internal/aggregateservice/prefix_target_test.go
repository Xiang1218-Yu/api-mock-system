package aggregateservice

import "testing"

func TestAggregatePrefixTargetKeepsSinglePathSeparator(t *testing.T) {
	got := resolveURL(map[string]any{"api_id": "orders"}, "https://mock.example/gateway/mock/")
	want := "https://mock.example/gateway/mock/internal/api/orders"
	if got != want {
		t.Fatalf("aggregate prefix target=%q, want %q", got, want)
	}
}
