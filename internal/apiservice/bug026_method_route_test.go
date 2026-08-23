package apiservice

import "testing"

func TestBug026MethodRouteUsesCanonicalHTTPMethod(t *testing.T) {
	if got := normalizeMethod("post"); got != "POST" {
		t.Fatalf("normalizeMethod(post)=%q, want POST", got)
	}
}
