package calllogrepo

import "testing"

func TestBug024DurationBucketsRespectBoundaries(t *testing.T) {
	if got := bucketLabel(10); got != "10-50ms" {
		t.Fatalf("bucketLabel(10)=%q, want 10-50ms", got)
	}
	if got := bucketLabel(50); got != "50-100ms" {
		t.Fatalf("bucketLabel(50)=%q, want 50-100ms", got)
	}
}
