package email

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"User@Example.com", "user@example.com"},
		{"  user@example.com  ", "user@example.com"},
		{"\tUSER@X.COM\n", "user@x.com"},
		{"already@lower.com", "already@lower.com"},
		{"", ""},
		{"   ", ""},
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Normalize must be idempotent: applying it twice must equal applying it once,
// so the service and repository can both normalize without double-processing.
func TestNormalizeIdempotent(t *testing.T) {
	for _, in := range []string{"User@Example.com", "  A@B.com  ", "x@y.com"} {
		once := Normalize(in)
		twice := Normalize(once)
		if once != twice {
			t.Errorf("Normalize not idempotent for %q: %q != %q", in, once, twice)
		}
	}
}
