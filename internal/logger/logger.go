// Package logger wraps zap with a single-purpose initializer so every other
// package depends on a *zap.Logger without knowing how it was constructed.
package logger

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New builds a zap.Logger for the given level string ("debug"|"info"|"warn"|"error").
// It uses a console encoder for human-readable output in development.
func New(level string) (*zap.Logger, error) {
	var lvl zapcore.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = zapcore.DebugLevel
	case "warn":
		lvl = zapcore.WarnLevel
	case "error":
		lvl = zapcore.ErrorLevel
	default:
		lvl = zapcore.InfoLevel
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(lvl)
	cfg.EncoderConfig = zap.NewDevelopmentEncoderConfig()
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.Encoding = "console"
	cfg.Development = true
	cfg.DisableStacktrace = false
	cfg.DisableCaller = false

	return cfg.Build()
}
