// Package projectrepo persists projects and their membership rows. Membership
// queries live here because project-level authorization needs them, and keeping
// them next to the project table avoids a separate package with a one-to-one
// dependency on this one.
package projectrepo

import (
	"context"
	"errors"
	"fmt"

	"api-mock-system/internal/models"

	"gorm.io/gorm"
)

// ErrNotFound is returned when a project lookup misses.
var ErrNotFound = errors.New("project not found")

// MemberRole is one of admin|editor|viewer.
type MemberRole string

const (
	RoleAdmin  MemberRole = "admin"
	RoleEditor MemberRole = "editor"
	RoleViewer MemberRole = "viewer"
)

// Repository is the data-access contract for projects and members.
type Repository interface {
	Create(ctx context.Context, p *models.Project) error
	FindByID(ctx context.Context, id string) (*models.Project, error)
	Update(ctx context.Context, p *models.Project) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, visibleToUserID string, q string, limit, offset int) ([]models.Project, int64, error)

	AddMember(ctx context.Context, m *models.ProjectMember) error
	RemoveMember(ctx context.Context, projectID, userID string) error
	ListMembers(ctx context.Context, projectID string) ([]models.ProjectMember, error)
	MemberRole(ctx context.Context, projectID, userID string) (MemberRole, bool, error)
}

type repo struct{ db *gorm.DB }

// New wires the repository to a gorm.DB.
func New(db *gorm.DB) Repository { return &repo{db: db} }

func (r *repo) Create(ctx context.Context, p *models.Project) error {
	if err := r.db.WithContext(ctx).Create(p).Error; err != nil {
		return fmt.Errorf("projectrepo: create: %w", err)
	}
	return nil
}

func (r *repo) FindByID(ctx context.Context, id string) (*models.Project, error) {
	var p models.Project
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("projectrepo: find by id: %w", err)
	}
	return &p, nil
}

func (r *repo) Update(ctx context.Context, p *models.Project) error {
	if err := r.db.WithContext(ctx).Save(p).Error; err != nil {
		return fmt.Errorf("projectrepo: update: %w", err)
	}
	return nil
}

func (r *repo) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Project{}).Error; err != nil {
		return fmt.Errorf("projectrepo: delete: %w", err)
	}
	return nil
}

// List returns projects the user can see: public projects always, private
// projects where they are an owner or member, plus optional name filtering.
// An empty visibleToUserID returns public projects only.
func (r *repo) List(ctx context.Context, visibleToUserID string, q string, limit, offset int) ([]models.Project, int64, error) {
	tx := r.db.WithContext(ctx).Model(&models.Project{})
	if visibleToUserID == "" {
		tx = tx.Where("visibility = ?", "public")
	} else {
		tx = tx.Where(
			"visibility = ? OR owner_id = ? OR id IN (SELECT project_id FROM project_members WHERE user_id = ?)",
			"public", visibleToUserID, visibleToUserID,
		)
	}
	if q != "" {
		tx = tx.Where("name LIKE ?", "%"+q+"%")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("projectrepo: count: %w", err)
	}
	var ps []models.Project
	if err := tx.Order("created_at DESC").Limit(limit).Offset(offset).Find(&ps).Error; err != nil {
		return nil, 0, fmt.Errorf("projectrepo: list: %w", err)
	}
	return ps, total, nil
}

func (r *repo) AddMember(ctx context.Context, m *models.ProjectMember) error {
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return fmt.Errorf("projectrepo: add member conflict: %w", err)
	}
	return nil
}

func (r *repo) RemoveMember(ctx context.Context, projectID, userID string) error {
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Delete(&models.ProjectMember{}).Error; err != nil {
		return fmt.Errorf("projectrepo: remove member: %w", err)
	}
	return nil
}

func (r *repo) ListMembers(ctx context.Context, projectID string) ([]models.ProjectMember, error) {
	var ms []models.ProjectMember
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&ms).Error; err != nil {
		return nil, fmt.Errorf("projectrepo: list members: %w", err)
	}
	return ms, nil
}

// MemberRole returns the user's role on the project and whether they are a
// member at all. Non-members return (empty, false, nil).
func (r *repo) MemberRole(ctx context.Context, projectID, userID string) (MemberRole, bool, error) {
	var m models.ProjectMember
	err := r.db.WithContext(ctx).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("projectrepo: member role: %w", err)
	}
	return MemberRole(m.Role), true, nil
}
