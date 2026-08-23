// Package aggregaterepo persists aggregate endpoint definitions.
package aggregaterepo

import (
	"context"
	"errors"
	"fmt"

	"api-mock-system/internal/models"

	"gorm.io/gorm"
)

// ErrNotFound is returned when an aggregate lookup misses.
var ErrNotFound = errors.New("aggregate not found")

// Repository is the data-access contract for aggregates.
type Repository interface {
	Create(ctx context.Context, a *models.Aggregate) error
	FindByID(ctx context.Context, id string) (*models.Aggregate, error)
	FindByProjectAndPath(ctx context.Context, projectID, path string) (*models.Aggregate, error)
	ListByProject(ctx context.Context, projectID string, limit, offset int) ([]models.Aggregate, int64, error)
	Update(ctx context.Context, a *models.Aggregate) error
	Delete(ctx context.Context, id string) error
}

type repo struct{ db *gorm.DB }

// New wires the repository to a gorm.DB.
func New(db *gorm.DB) Repository { return &repo{db: db} }

func (r *repo) Create(ctx context.Context, a *models.Aggregate) error {
	if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
		return fmt.Errorf("aggregaterepo: create: %w", err)
	}
	return nil
}

func (r *repo) FindByID(ctx context.Context, id string) (*models.Aggregate, error) {
	var a models.Aggregate
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&a).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("aggregaterepo: find by id: %w", err)
	}
	return &a, nil
}

func (r *repo) FindByProjectAndPath(ctx context.Context, projectID, path string) (*models.Aggregate, error) {
	var a models.Aggregate
	err := r.db.WithContext(ctx).
		Where("project_id = ? AND path = ?", projectID, path).
		First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("aggregaterepo: find by path: %w", err)
	}
	return &a, nil
}

func (r *repo) ListByProject(ctx context.Context, projectID string, limit, offset int) ([]models.Aggregate, int64, error) {
	tx := r.db.WithContext(ctx).Model(&models.Aggregate{}).Where("project_id = ?", projectID)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("aggregaterepo: count: %w", err)
	}
	var as []models.Aggregate
	if err := tx.Order("created_at DESC").Limit(limit).Offset(offset).Find(&as).Error; err != nil {
		return nil, 0, fmt.Errorf("aggregaterepo: list: %w", err)
	}
	return as, total, nil
}

func (r *repo) Update(ctx context.Context, a *models.Aggregate) error {
	if err := r.db.WithContext(ctx).Save(a).Error; err != nil {
		return fmt.Errorf("aggregaterepo: update: %w", err)
	}
	return nil
}

func (r *repo) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Aggregate{}).Error; err != nil {
		return fmt.Errorf("aggregaterepo: delete: %w", err)
	}
	return nil
}
