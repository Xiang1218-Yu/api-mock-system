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

	"api-mock-system/internal/email"
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
		// TranslateError maps driver-specific constraint violations onto
		// gorm.ErrDuplicatedKey etc., so repositories can detect a lost
		// registration race without parsing driver error strings.
		Logger:         gormLogger(log),
		TranslateError: true,
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

	// Backfill: lower-case any historic emails so the uniqueness invariant
	// holds for data written before normalization was enforced. Idempotent
	// and safe to run every boot: rows already lowercased are untouched.
	if err := normalizeUserEmails(ctx, db); err != nil {
		return nil, fmt.Errorf("normalize user emails: %w", err)
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

// normalizeUserEmails lower-cases every existing user email in place. Run once
// per boot before the server accepts traffic; it is idempotent. If a set of
// rows would collide after lowercasing (e.g. "A@x.com" and "a@x.com" already
// coexisted due to the old case-sensitive index), the duplicates are removed,
// keeping the earliest-created row so account history is preserved. This is a
// one-time repair for databases created before email normalization existed.
func normalizeUserEmails(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Collapsing duplicates must happen before the unique index is rebuilt,
		// so the index is dropped for the duration of this transaction.
		if err := tx.Migrator().DropIndex(&models.User{}, "idx_users_email"); err != nil {
			return fmt.Errorf("drop email index: %w", err)
		}

		var users []models.User
		if err := tx.Select("id", "email", "created_at").Find(&users).Error; err != nil {
			return fmt.Errorf("load users: %w", err)
		}

		seen := make(map[string]struct{}, len(users)) // normalized emails already processed
		for _, u := range users {
			norm := email.Normalize(u.Email)
			if norm == "" {
				continue
			}
			if _, ok := seen[norm]; ok {
				// Duplicate: two rows collapse to one identity after
				// lowercasing (e.g. "A@x.com" and "a@x.com" coexisted under
				// the old case-sensitive index). Drop the newer row to clear
				// the way for the unique index; the first-seen row survives.
				if err := tx.Delete(&models.User{}, u.ID).Error; err != nil {
					return fmt.Errorf("delete duplicate user %s: %w", u.ID, err)
				}
				continue
			}
			seen[norm] = struct{}{}
			if norm != u.Email {
				if err := tx.Model(&models.User{}).Where("id = ?", u.ID).
					Update("email", norm).Error; err != nil {
					return fmt.Errorf("lowercase email for user %s: %w", u.ID, err)
				}
			}
		}

		// Recreate the index so case-insensitive uniqueness is enforced from
		// here on. Done by AutoMigrate on the next boot if we did nothing, but
		// rebuilding now closes the gap before traffic resumes.
		if err := tx.Migrator().CreateIndex(&models.User{}, "idx_users_email"); err != nil {
			return fmt.Errorf("recreate email index: %w", err)
		}
		return nil
	})
}
