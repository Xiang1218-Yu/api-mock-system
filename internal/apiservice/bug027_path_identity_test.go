package apiservice

import "testing"

func TestBug027PathIdentitySharesCanonicalForm(t *testing.T) {
	if got := normalizePath("users/"); got != "/users" {
		t.Fatalf("normalizePath(users/)=%q, want /users", got)
	}
	if got := normalizePath("/users"); got != "/users" {
		t.Fatalf("normalizePath(/users)=%q, want /users", got)
	}
}
