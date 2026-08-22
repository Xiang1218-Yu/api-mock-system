// Package id provides UUID generation isolated from any third-party generator
// so the rest of the codebase never imports a UUID library directly.
package id

import "crypto/rand"

// NewUUID returns a version-4 UUID string (lowercase, hyphenated). It panics only
// if the system PRNG is unavailable, which is a fatal startup condition.
func NewUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	// RFC 4122 v4: set version and variant bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return format(b)
}

// format renders 16 raw bytes as the canonical 8-4-4-4-12 hex string.
func format(b []byte) string {
	const hex = "0123456789abcdef"
	out := make([]byte, 36)
	pos := 0
	for i, c := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out[pos] = '-'
			pos++
		}
		out[pos] = hex[c>>4]
		out[pos+1] = hex[c&0x0f]
		pos += 2
	}
	return string(out)
}
