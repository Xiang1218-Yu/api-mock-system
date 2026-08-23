// Package email centralizes email normalization so every code path that reads
// or writes an email address applies the identical rule. Normalization is the
// difference between one account and two: "User@Example.com" and
// "user@example.com" must resolve to the same identity, or registration, login,
// and membership lookups all diverge.
//
// The rule is intentionally minimal: trim surrounding whitespace and lowercase
// ASCII characters. It is idempotent, so applying it twice is harmless, and the
// repository may normalize defensively even when the caller already has. The
// email local-part is technically case-sensitive under RFC 5321, but in
// practice every major provider treats it case-insensitively; lowercasing is
// the de-facto standard and keeps the rule portable across SQLite and Postgres.
package email

import "strings"

// Normalize returns the canonical form of an address: trimmed and lowercased.
// Empty input yields empty output. It does not validate syntax — that remains
// the caller's responsibility (e.g. via binding tags).
func Normalize(addr string) string {
	return strings.ToLower(strings.TrimSpace(addr))
}
