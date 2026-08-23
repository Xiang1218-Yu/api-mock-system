// Package userrepo persists users. It owns no logic beyond CRUD and lookups;
// password hashing and JWT live in package auth, not here.
package userrepo

import (
	"context"
	"errors"
	"fmt"

	"api-mock-system/internal/email"
	"api-mock-system/internal/models"

	"gorm.io/gorm"
)

// ErrNotFound is returned when a single-row lookup misses.
var ErrNotFound = errors.New("user not found")

// ErrEmailConflict is returned by Create when the email uniqueness constraint
// rejects an insert. It is the signal that a concurrent registration race lost
// to the other request — the caller maps it to a 409, not a 500.
var ErrEmailConflict = errors.New("email already registered")

// Repository is the contract every user-service consumer depends on.
// A future Postgres-backed implementation need only satisfy this interface.
type Repository interface {
	Create(ctx context.Context, u *models.User) error
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByID(ctx context.Context, id string) (*models.User, error)
	List(ctx context.Context, limit, offset int) ([]models.User, int64, error)
}

// repo is the concrete GORM implementation.
type repo struct{ db *gorm.DB }

// New wires the repository to a gorm.DB.
func New(db *gorm.DB) Repository { return &repo{db: db} }

func (r *repo) Create(ctx context.Context, u *models.User) error {
	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		// The unique index on email is the last line of defense against the
		// register race: when the check-then-create window loses, the DB
		// rejects the insert and we translate it to a domain conflict.
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrEmailConflict
		}
		return fmt.Errorf("userrepo: create: %w", err)
	}
	return nil
}

func (r *repo) FindByEmail(ctx context.Context, emailAddr string) (*models.User, error) {
	var u models.User
	if err := r.db.WithContext(ctx).Where("email = ?", email.Normalize(emailAddr)).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("userrepo: find by email: %w", err)
	}
	return &u, nil
}

func (r *repo) FindByID(ctx context.Context, id string) (*models.User, error) {
	var u models.User
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("userrepo: find by id: %w", err)
	}
	return &u, nil
}

func (r *repo) List(ctx context.Context, limit, offset int) ([]models.User, int64, error) {
	var users []models.User
	var total int64
	if err := r.db.WithContext(ctx).Model(&models.User{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("userrepo: count: %w", err)
	}
	if err := r.db.WithContext(ctx).Order("created_at DESC").
		Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("userrepo: list: %w", err)
	}
	return users, total, nil
}
