// Package mockdatarepo persists fixed Mock overrides — rows that replace
// generated mock output for a matching request key.
package mockdatarepo

import (
	"context"
	"errors"
	"fmt"

	"api-mock-system/internal/id"
	"api-mock-system/internal/models"

	"gorm.io/gorm"
)

// ErrNotFound is returned when an override lookup misses.
var ErrNotFound = errors.New("mock override not found")

// Repository is the data-access contract for Mock overrides.
type Repository interface {
	Set(ctx context.Context, m *models.MockData) error
	Get(ctx context.Context, apiID, key string) (*models.MockData, error)
	Clear(ctx context.Context, apiID, key string) error
	ClearAll(ctx context.Context, apiID string) error
	List(ctx context.Context, apiID string) ([]models.MockData, error)
}

type repo struct{ db *gorm.DB }

// New wires the repository to a gorm.DB.
func New(db *gorm.DB) Repository { return &repo{db: db} }

// Set upserts the override: one row per (api_id, key). On conflict it updates
// value and enabled; otherwise it inserts. Implemented as update-or-insert
// because SQLite's ON CONFLICT target on (api_id, key) isn't portable across
// drivers in GORM's high-level API.
func (r *repo) Set(ctx context.Context, m *models.MockData) error {
	tx := r.db.WithContext(ctx)
	res := tx.Model(&models.MockData{}).
		Where("api_id = ? AND key = ?", m.APIID, m.Key).
		Updates(map[string]any{"value": m.Value, "enabled": m.Enabled})
	if res.Error != nil {
		return fmt.Errorf("mockdatarepo: set update: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		// No existing row — insert a fresh one with a generated id.
		if m.ID == "" {
			m.ID = id.NewUUID()
		}
		if err := tx.Create(m).Error; err != nil {
			return fmt.Errorf("mockdatarepo: set insert: %w", err)
		}
	}
	return nil
}

func (r *repo) Get(ctx context.Context, apiID, key string) (*models.MockData, error) {
	var m models.MockData
	err := r.db.WithContext(ctx).
		Where("api_id = ? AND key = ? AND enabled = ?", apiID, key, true).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mockdatarepo: get: %w", err)
	}
	return &m, nil
}

func (r *repo) Clear(ctx context.Context, apiID, key string) error {
	if err := r.db.WithContext(ctx).
		Where("api_id = ? AND key = ?", apiID, key).
		Delete(&models.MockData{}).Error; err != nil {
		return fmt.Errorf("mockdatarepo: clear: %w", err)
	}
	return nil
}

func (r *repo) ClearAll(ctx context.Context, apiID string) error {
	if err := r.db.WithContext(ctx).
		Where("api_id = ?", apiID).
		Delete(&models.MockData{}).Error; err != nil {
		return fmt.Errorf("mockdatarepo: clear all: %w", err)
	}
	return nil
}

func (r *repo) List(ctx context.Context, apiID string) ([]models.MockData, error) {
	var ms []models.MockData
	if err := r.db.WithContext(ctx).Where("api_id = ?", apiID).Find(&ms).Error; err != nil {
		return nil, fmt.Errorf("mockdatarepo: list: %w", err)
	}
	return ms, nil
}
