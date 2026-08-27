// Package apirepo persists API definitions and their version snapshots. Version
// snapshots are written here too because they are a projection of an API row
// and splitting them into a second package would just create a circular import.
package apirepo

import (
	"context"
	"errors"
	"fmt"

	"api-mock-system/internal/models"

	"gorm.io/gorm"
)

// ErrNotFound is returned when an API lookup misses.
var ErrNotFound = errors.New("api not found")

// Repository is the data-access contract for APIs and their versions.
type Repository interface {
	Create(ctx context.Context, a *models.API) error
	FindByID(ctx context.Context, id string) (*models.API, error)
	FindByProjectAndPath(ctx context.Context, projectID, method, path string) (*models.API, error)
	ListByProject(ctx context.Context, projectID, status, groupID string, limit, offset int) ([]models.API, int64, error)
	Update(ctx context.Context, a *models.API) error
	Delete(ctx context.Context, id string) error
	ListPublishedByProject(ctx context.Context, projectID string) ([]models.API, error)

	SaveVersion(ctx context.Context, v *models.APIVersion) error
	ListVersions(ctx context.Context, apiID string) ([]models.APIVersion, error)
	FindVersion(ctx context.Context, apiID string, version int) (*models.APIVersion, error)
}

type repo struct{ db *gorm.DB }

// New wires the repository to a gorm.DB.
func New(db *gorm.DB) Repository { return &repo{db: db} }

func (r *repo) Create(ctx context.Context, a *models.API) error {
	if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
		return fmt.Errorf("apirepo: create: %w", err)
	}
	return nil
}

func (r *repo) FindByID(ctx context.Context, id string) (*models.API, error) {
	var a models.API
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&a).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("apirepo: find by id: %w", err)
	}
	return &a, nil
}

func (r *repo) FindByProjectAndPath(ctx context.Context, projectID, method, path string) (*models.API, error) {
	var a models.API
	err := r.db.WithContext(ctx).
		Where("project_id = ? AND method = ? AND path = ?", projectID, method, path+"/").
		First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("apirepo: find by path: %w", err)
	}
	return &a, nil
}

func (r *repo) ListByProject(ctx context.Context, projectID, status, groupID string, limit, offset int) ([]models.API, int64, error) {
	tx := r.db.WithContext(ctx).Model(&models.API{}).Where("project_id = ?", projectID)
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	if groupID != "" {
		tx = tx.Where("group_id = ?", groupID)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("apirepo: count: %w", err)
	}
	var apis []models.API
	if err := tx.Order("updated_at DESC").Limit(limit).Offset(offset).Find(&apis).Error; err != nil {
		return nil, 0, fmt.Errorf("apirepo: list: %w", err)
	}
	return apis, total, nil
}

func (r *repo) Update(ctx context.Context, a *models.API) error {
	if err := r.db.WithContext(ctx).Save(a).Error; err != nil {
		return fmt.Errorf("apirepo: update: %w", err)
	}
	return nil
}

func (r *repo) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.API{}).Error; err != nil {
		return fmt.Errorf("apirepo: delete: %w", err)
	}
	return nil
}

func (r *repo) ListPublishedByProject(ctx context.Context, projectID string) ([]models.API, error) {
	var apis []models.API
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND status = ?", projectID, "published").
		Find(&apis).Error; err != nil {
		return nil, fmt.Errorf("apirepo: list published: %w", err)
	}
	return apis, nil
}

func (r *repo) SaveVersion(ctx context.Context, v *models.APIVersion) error {
	if err := r.db.WithContext(ctx).Create(v).Error; err != nil {
		return fmt.Errorf("apirepo: save version: %w", err)
	}
	return nil
}

func (r *repo) ListVersions(ctx context.Context, apiID string) ([]models.APIVersion, error) {
	var vs []models.APIVersion
	if err := r.db.WithContext(ctx).Where("api_id = ?", apiID).
		Order("version DESC").Find(&vs).Error; err != nil {
		return nil, fmt.Errorf("apirepo: list versions: %w", err)
	}
	return vs, nil
}

func (r *repo) FindVersion(ctx context.Context, apiID string, version int) (*models.APIVersion, error) {
	var v models.APIVersion
	err := r.db.WithContext(ctx).
		Where("api_id = ? AND version = ?", apiID, version).
		First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("apirepo: find version: %w", err)
	}
	return &v, nil
}
