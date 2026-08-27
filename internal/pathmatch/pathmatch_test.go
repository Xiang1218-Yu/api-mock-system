package pathmatch

import "testing"

func TestMatch(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"/users/:id", "/users/42", true},
		{"/users/:id", "/users/42/", false}, // segment count mismatch
		{"/users/:id/posts", "/users/42/posts", true},
		{"/users/:id", "/orders/42", false},
		{"/users/:id", "/users", false},
		{"/health", "/health", true},
		{"users/:id", "users/1", true}, // tolerates missing leading slash
	}
	for _, c := range cases {
		params, ok := Match(c.pattern, c.path)
		if ok != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.path, ok, c.want)
		}
		if ok {
			if _, exists := params["id"]; c.pattern == "/users/:id" && !exists {
				t.Errorf("expected id param for %q", c.pattern)
			}
		}
	}
}

func TestMatchParams(t *testing.T) {
	params, ok := Match("/users/:userId/posts/:postId", "/users/7/posts/3")
	if !ok {
		t.Fatal("expected match")
	}
	if params["userId"] != "7" || params["postId"] != "3" {
		t.Errorf("params wrong: %v", params)
	}
}

func TestEmptyPath(t *testing.T) {
	_, ok := Match("", "")
	if !ok {
		t.Error("empty pattern should match empty path")
	}
}
