package pathmatch

import "testing"

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"":            "/",
		"/":           "/",
		"//":          "/",
		"users":       "/users",
		"/users":      "/users",
		"/users/":     "/users",
		"/users//":    "/users",
		"/users/42":   "/users/42",
		"/users/42/":  "/users/42",
		"users/42":    "/users/42",
		"users/42/":   "/users/42",
		"/a/b/c/":     "/a/b/c",
		"////":        "/",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeIdempotent(t *testing.T) {
	// Normalizing the output must be a fixed point — otherwise a path stored
	// pre-normalize and one looked up post-normalize could still diverge.
	cases := []string{"/", "/users", "/users/42", "/a/b/c", "/users/:id"}
	for _, p := range cases {
		once := Normalize(p)
		twice := Normalize(once)
		if once != twice {
			t.Errorf("Normalize not idempotent: Normalize(%q)=%q, Normalize(%q)=%q", p, once, once, twice)
		}
	}
}

func TestMatchTrailingSlashAgnostic(t *testing.T) {
	// The bug: a pattern stored with a trailing slash and the same pattern
	// without one used to behave differently. Match must treat them as one.
	pattern := "/users/:id"
	requests := []string{"/users/42", "/users/42/", "users/42", "/users/42//"}
	for _, req := range requests {
		params, ok := Match(pattern, req)
		if !ok {
			t.Errorf("Match(%q, %q) = false, want true (trailing slash must not matter)", pattern, req)
			continue
		}
		if params["id"] != "42" {
			t.Errorf("Match(%q, %q): id param = %q, want 42", pattern, req, params["id"])
		}
	}

	// A pattern defined with a trailing slash must match a request without one.
	if _, ok := Match("/users/:id/", "/users/42"); !ok {
		t.Errorf("Match(\"/users/:id/\", \"/users/42\") = false, want true")
	}
}

func TestMatchSegmentCount(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"/users", "/users", true},
		{"/users", "/users/", true}, // trailing slash collapsed
		{"/users", "/users/42", false},
		{"/users/:id", "/users/42", true},
		{"/users/:id", "/users/42/comments", false},
		{"/users/:id/comments/:cid", "/users/7/comments/9", true},
		{"/", "/", true},
		{"/", "/anything", false},
	}
	for _, c := range cases {
		_, ok := Match(c.pattern, c.path)
		if ok != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.path, ok, c.want)
		}
	}
}

func TestMatchParams(t *testing.T) {
	params, ok := Match("/orgs/:org/repos/:repo", "/orgs/acme/repos/portal")
	if !ok {
		t.Fatal("expected match")
	}
	if params["org"] != "acme" || params["repo"] != "portal" {
		t.Errorf("params = %v, want org=acme repo=portal", params)
	}
}

// TestMatchDoesNotTreatTrailingSlashAsExtraSegment guards the exact regression:
// before Normalize, splitSlash("/users/") yielded ["users", ""] (two segments),
// so a stored "/users/" never matched a one-segment request "/users".
func TestMatchDoesNotTreatTrailingSlashAsExtraSegment(t *testing.T) {
	if _, ok := Match("/users/", "/users"); !ok {
		t.Error(`Match("/users/", "/users") = false; trailing slash created a phantom empty segment`)
	}
}
