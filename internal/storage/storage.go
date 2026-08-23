// Package storage wraps the *gorm.DB so the rest of the codebase imports a single
// concrete type instead of gorm directly. It is the only place that chooses the
// database driver — swapping SQLite for Postgres later means editing this file.
//
// NOTE: system.md specifies PostgreSQL + Redis. This implementation uses SQLite
// (pure-Go, no CGO) and an in-process cache so the binary runs with `go run`
// and no external services. The repository interfaces in each domain package
// keep storage pluggable; a Postgres-backed implementation can be dropped in
// without touching services or handlers.
package storage

import (
	"context"
	"fmt"

	"api-mock-system/internal/models"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Store is the application's handle to the database.
type Store struct {
	DB  *gorm.DB
	Log *zap.Logger
}

// Open connects to the database at dsn and auto-migrates the full schema.
// It enables foreign-key constraints (relevant for SQLite) and long-running
// pragmas for write safety.
func Open(ctx context.Context, dsn string, log *zap.Logger) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormLogger(log),
	})
	if err != nil {
		return nil, fmt.Errorf("open db %q: %w", dsn, err)
	}

	// SQLite pragmas: foreign keys on, WAL for concurrent readers, busy timeout.
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.PingContext(ctx)
		_, _ = sqlDB.ExecContext(ctx, "PRAGMA journal_mode=WAL;")
		_, _ = sqlDB.ExecContext(ctx, "PRAGMA foreign_keys=ON;")
		_, _ = sqlDB.ExecContext(ctx, "PRAGMA busy_timeout=5000;")
	}

	if err := db.WithContext(ctx).AutoMigrate(
		&models.User{},
		&models.Project{},
		&models.ProjectMember{},
		&models.API{},
		&models.APIVersion{},
		&models.Aggregate{},
		&models.MockData{},
		&models.DebugLog{},
		&models.CallLog{},
	); err != nil {
		return nil, fmt.Errorf("auto-migrate: %w", err)
	}

	// Drop the legacy single-column index on project_members(project_id).
	// An earlier schema version declared that column UNIQUE, which capped a
	// project at one member and made every second invitation fail with
	// "UNIQUE constraint failed: project_members.project_id". AutoMigrate
	// creates the corrected (project_id, user_id) composite unique index above
	// but will not remove indexes that no longer appear in the model, so we
	// drop the stale one explicitly. Ignore the error if it is already gone.
	if err := db.WithContext(ctx).Migrator().DropIndex(&models.ProjectMember{}, "idx_project_members_project_id"); err != nil {
		log.Debug("drop legacy project_members.project_id index (already absent?)",
			zap.Error(err))
	}

	log.Info("database ready", zap.String("dsn", dsn))
	return &Store{DB: db, Log: log}, nil
}

// Close releases the underlying connection pool.
func (s *Store) Close() error {
	if s.DB == nil {
		return nil
	}
	sqlDB, err := s.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
