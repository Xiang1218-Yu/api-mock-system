package calllogrepo

import "testing"

func TestBucketLabel(t *testing.T) {
	cases := []struct {
		name string
		ms   int
		want string
	}{
		// Within-range.
		{"zero", 0, "0-10ms"},
		{"nine", 9, "0-10ms"},

		// Boundaries: an exact hit on a shared edge belongs to the next range up
		// (ranges are half-open [lower, upper)), not the one whose max it equals.
		{"ten", 10, "10-50ms"},
		{"fifty", 50, "50-100ms"},
		{"hundred", 100, "100-500ms"},
		{"fivehundred", 500, "500ms+"},

		// Just inside each subsequent range.
		{"eleven", 11, "10-50ms"},
		{"forty-nine", 49, "10-50ms"},
		{"fivehundred-one", 501, "500ms+"},

		// Negative durations clamp to the first bucket.
		{"negative", -5, "0-10ms"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := bucketLabel(c.ms); got != c.want {
				t.Fatalf("bucketLabel(%d) = %q, want %q", c.ms, got, c.want)
			}
		})
	}
}

func TestBucketLabelOrder(t *testing.T) {
	// Walking every boundary in turn must walk the buckets in ascending order —
	// a regression here surfaces as labels arriving out of the Buckets order,
	// which the dashboard chart renders in whatever scrambled order it gets.
	wantOrder := []string{"0-10ms", "10-50ms", "50-100ms", "100-500ms", "500ms+"}
	if len(Buckets) != len(wantOrder) {
		t.Fatalf("Buckets has %d entries, test expects %d", len(Buckets), len(wantOrder))
	}
	for i, b := range Buckets {
		if b.Label != wantOrder[i] {
			t.Fatalf("Buckets[%d].Label = %q, want %q", i, b.Label, wantOrder[i])
		}
	}
	// A duration at the very top of each range must roll into the next range up,
	// so the labels traced by sweeping the boundaries are exactly wantOrder[1:].
	for i := 0; i < len(Buckets)-1; i++ {
		edge := Buckets[i].Max // exclusive upper bound of bucket i
		if got := bucketLabel(edge); got != wantOrder[i+1] {
			t.Fatalf("bucketLabel(%d) at edge %d = %q, want %q", edge, i, got, wantOrder[i+1])
		}
	}
}
