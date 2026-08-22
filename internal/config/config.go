// Package config loads runtime configuration from environment variables with
// sensible defaults. All values are resolved exactly once at startup; callers
// read the returned *Config and never touch the environment directly.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds every value the application reads from the environment.
// Fields are grouped by concern but the struct stays flat so wiring is trivial.
type Config struct {
	ServerPort  string
	DBDSN       string
	JWTSecret   string
	JWTExpiry   time.Duration
	MockBaseURL string
	AggrTimeout time.Duration
	LogLevel    string
	Env         string
	// RateRPS is the per-key refill rate for the API rate limiter (spec §3.2).
	RateRPS float64
	// RateBurst is the maximum burst a single key may accumulate.
	RateBurst int
}

// Load reads environment variables and returns a populated Config.
// Missing values fall back to defaults that work for local development.
func Load() *Config {
	return &Config{
		ServerPort:  envStr("SERVER_PORT", "8080"),
		DBDSN:       envStr("DB_DSN", "api_mock.db"),
		JWTSecret:   envStr("JWT_SECRET", "dev-secret-change-me"),
		JWTExpiry:   envDur("JWT_EXPIRY", 24*time.Hour),
		MockBaseURL: envStr("MOCK_BASE_URL", "http://localhost:8080/mock"),
		AggrTimeout: envDur("AGGREGATE_TIMEOUT", 3000*time.Millisecond),
		LogLevel:    envStr("LOG_LEVEL", "info"),
		Env:         envStr("APP_ENV", "development"),
		RateRPS:     envFloat("RATE_RPS", 50),
		RateBurst:   envInt("RATE_BURST", 100),
	}
}

// IsDev reports whether the process is running in a development environment.
func (c *Config) IsDev() bool { return strings.EqualFold(c.Env, "development") }

// envStr returns the named env var or the fallback when unset/empty.
func envStr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// envDur returns the named env var parsed as a duration or the fallback.
func envDur(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	if ms, err := strconv.Atoi(v); err == nil {
		return time.Duration(ms) * time.Millisecond
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	return fallback
}

// envFloat returns the named env var parsed as a float64 or the fallback.
func envFloat(key string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return fallback
}

// envInt returns the named env var parsed as an int or the fallback.
func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	return fallback
}
