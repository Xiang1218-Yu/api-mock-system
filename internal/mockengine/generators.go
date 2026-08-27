package mockengine

import (
	"fmt"
	"math/rand"
	"time"
)

// randomName returns a first + last name from small built-in pools.
// Keeping the pools here (not in schema logic) keeps the generator readable.
func randomName(r *rand.Rand) string {
	first := firstNames[r.Intn(len(firstNames))]
	last := lastNames[r.Intn(len(lastNames))]
	return first + " " + last
}

// randomPhone returns a +86-styled mobile number, e.g. +8613800138000.
func randomPhone(r *rand.Rand) string {
	n := 13800000000 + r.Int63n(1999999999)
	return fmt.Sprintf("+86%d", n)
}

// randomUUID returns a v4-style UUID string from the random source.
func randomUUID(r *rand.Rand) string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(r.Intn(256))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// randomDate returns a YYYY-MM-DD within the last 10 years.
func randomDate(r *rand.Rand) string {
	t := time.Now().AddDate(-1, 0, 0).AddDate(0, 0, -r.Intn(365*10))
	return t.Format("2006-01-02")
}

// randomDateTime returns an RFC3339 timestamp within the last year.
func randomDateTime(r *rand.Rand) string {
	t := time.Now().Add(-time.Duration(r.Int63n(int64(365 * 24 * time.Hour))))
	return t.Format(time.RFC3339)
}

// randomWord returns a lowercase alphabetic string of the given length.
func randomWord(r *rand.Rand, length int) string {
	if length <= 0 {
		return ""
	}
	b := make([]byte, length)
	for i := range b {
		b[i] = 'a' + byte(r.Intn(26))
	}
	return string(b)
}

// Small pools — kept tiny to avoid bloating the binary but enough variety for
// realistic-looking mock data.
var firstNames = []string{
	"Alex", "Sam", "Jordan", "Taylor", "Morgan", "Casey", "Riley", "Jamie",
	"Lin", "Wei", "Yan", "Min", "Hao", "Jing", "Bo", "Xin",
}
var lastNames = []string{
	"Smith", "Lee", "Chen", "Wang", "Patel", "Garcia", "Kim", "Nguyen",
	"Zhang", "Liu", "Yang", "Huang", "Zhao", "Sun", "Zhou", "Wu",
}
