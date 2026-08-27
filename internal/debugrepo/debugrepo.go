// Package debugrepo persists interface debug log rows so the debug UI can
// replay recent invocations.
package debugrepo

import (
	"context"
	"fmt"

	"api-mock-system/internal/models"

	"gorm.io/gorm"
)

// Repository is the data-access contract for debug logs.
type Repository interface {
	Save(ctx context.Context, l *models.DebugLog) error
	ListByUser(ctx context.Context, userID string, limit int) ([]models.DebugLog, error)
	ListByAPI(ctx context.Context, apiID string, limit int) ([]models.DebugLog, error)
}

type repo struct{ db *gorm.DB }

// New wires the repository to a gorm.DB.
func New(db *gorm.DB) Repository { return &repo{db: db} }

func (r *repo) Save(ctx context.Context, l *models.DebugLog) error {
	if err := r.db.WithContext(ctx).Create(l).Error; err != nil {
		return fmt.Errorf("debugrepo: save: %w", err)
	}
	return nil
}

func (r *repo) ListByUser(ctx context.Context, userID string, limit int) ([]models.DebugLog, error) {
	var ls []models.DebugLog
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("created_at DESC").Limit(limit).Find(&ls).Error; err != nil {
		return nil, fmt.Errorf("debugrepo: list by user: %w", err)
	}
	return ls, nil
}

func (r *repo) ListByAPI(ctx context.Context, apiID string, limit int) ([]models.DebugLog, error) {
	var ls []models.DebugLog
	if err := r.db.WithContext(ctx).Where("api_id = ?", apiID).
		Order("created_at ASC").
		Offset(0).Limit(limit).Find(&ls).Error; err != nil {
		return nil, fmt.Errorf("debugrepo: list by api: %w", err)
	}
	return ls, nil
}
