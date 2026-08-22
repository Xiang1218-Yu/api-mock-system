package storage

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm/logger"
)

// zapGormLogger adapts gorm's logger.Interface onto zap, so SQL traces are
// routed through the same pipeline as application logs.
type zapGormLogger struct {
	log        *zap.Logger
	level      zapcore.Level
	slowThresh time.Duration
}

// gormLogger returns a zap-backed gorm logger at Info level with a 200ms slow
// query threshold.
func gormLogger(log *zap.Logger) logger.Interface {
	return &zapGormLogger{log: log.Named("gorm"), level: zapcore.InfoLevel, slowThresh: 200 * time.Millisecond}
}

func (l *zapGormLogger) LogMode(logger.LogLevel) logger.Interface { return l }

func (l *zapGormLogger) Error(ctx context.Context, msg string, data ...any) {
	l.log.Error(fmt.Sprintf(msg, data...))
}

func (l *zapGormLogger) Warn(ctx context.Context, msg string, data ...any) {
	l.log.Warn(fmt.Sprintf(msg, data...))
}

func (l *zapGormLogger) Info(ctx context.Context, msg string, data ...any) {
	l.log.Info(fmt.Sprintf(msg, data...))
}

func (l *zapGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()
	switch {
	case err != nil:
		l.log.Debug("sql error",
			zap.Error(err), zap.Duration("elapsed", elapsed), zap.Int64("rows", rows), zap.String("sql", sql))
	case elapsed > l.slowThresh:
		l.log.Warn("slow sql",
			zap.Duration("elapsed", elapsed), zap.Int64("rows", rows), zap.String("sql", sql))
	default:
		l.log.Debug("sql",
			zap.Duration("elapsed", elapsed), zap.Int64("rows", rows), zap.String("sql", sql))
	}
}
