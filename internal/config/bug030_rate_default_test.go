package config

import "testing"

func TestBug030RateDefaultHasUsableBurst(t *testing.T) {
	t.Setenv("RATE_RPS", "")
	t.Setenv("RATE_BURST", "")
	cfg := Load()
	if cfg.RateBurst != 100 {
		t.Fatalf("default rate burst=%d, want 100", cfg.RateBurst)
	}
}
