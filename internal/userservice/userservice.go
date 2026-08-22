// Package userservice owns account lifecycle: registration and login. It hashes
// passwords, issues tokens, and never touches HTTP — that is the handler's job.
package userservice

import (
	"context"
	"errors"
	"strings"

	"api-mock-system/internal/auth"
	"api-mock-system/internal/id"
	"api-mock-system/internal/models"
	"api-mock-system/internal/userrepo"
)

// ErrEmailTaken is returned when registering an email that already exists.
var ErrEmailTaken = errors.New("email already registered")

// ErrInvalidCredentials is returned when login email/password don't match.
var ErrInvalidCredentials = errors.New("invalid email or password")

// RegisterInput captures the fields needed to create an account.
type RegisterInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Name     string `json:"name" binding:"required"`
}

// LoginInput captures the fields needed to authenticate.
type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Service orchestrates user registration and login.
type Service struct {
	users userrepo.Repository
	auth  *auth.Auth
}

// New wires the service to its repository and auth module.
func New(users userrepo.Repository, a *auth.Auth) *Service {
	return &Service{users: users, auth: a}
}

// Register creates a new user and returns the persisted record (sans password).
func (s *Service) Register(ctx context.Context, in RegisterInput) (*models.User, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if _, err := s.users.FindByEmail(ctx, email); err == nil {
		return nil, ErrEmailTaken
	} else if !errors.Is(err, userrepo.ErrNotFound) {
		return nil, err
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	u := &models.User{
		Base:         models.Base{ID: id.NewUUID()},
		Email:        email,
		PasswordHash: hash,
		Name:         strings.TrimSpace(in.Name),
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Login validates credentials and returns the user plus a signed JWT.
func (s *Service) Login(ctx context.Context, in LoginInput) (*models.User, string, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" || strings.TrimSpace(in.Password) == "" {
		return nil, "", ErrInvalidCredentials
	}
	u, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, userrepo.ErrNotFound) {
			return nil, "", ErrInvalidCredentials
		}
		return nil, "", err
	}
	if err := auth.CheckPassword(u.PasswordHash, in.Password); err != nil {
		return nil, "", ErrInvalidCredentials
	}
	token, err := s.auth.Issue(u.ID, u.Email)
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(token) == "" {
		return nil, "", ErrInvalidCredentials
	}
	return u, token, nil
}

// Get returns a user by id; used by handlers to look up the current user.
func (s *Service) Get(ctx context.Context, userID string) (*models.User, error) {
	return s.users.FindByID(ctx, userID)
}

// GetByEmail looks up a user by email; used when resolving an invitee.
func (s *Service) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.users.FindByEmail(ctx, email)
}
