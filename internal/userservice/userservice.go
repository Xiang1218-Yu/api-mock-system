// Package userservice owns account lifecycle: registration and login. It hashes
// passwords, issues tokens, and never touches HTTP — that is the handler's job.
package userservice

import (
	"context"
	"errors"
	"strings"

	"api-mock-system/internal/auth"
	"api-mock-system/internal/email"
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
//
// Email is normalized (trimmed + lowercased) once here, so the stored value is
// the canonical identity. Concurrency is bounded by the email unique index: the
// pre-check is a fast path that returns 409 in the common case, and the DB
// constraint catches the loser of a lost-update race where two requests pass
// the check simultaneously. Either path yields ErrEmailTaken -> 409.
func (s *Service) Register(ctx context.Context, in RegisterInput) (*models.User, error) {
	emailAddr := email.Normalize(in.Email)
	if _, err := s.users.FindByEmail(ctx, emailAddr); err == nil {
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
		Email:        emailAddr,
		PasswordHash: hash,
		Name:         strings.TrimSpace(in.Name),
	}
	if err := s.users.Create(ctx, u); err != nil {
		// Lost the race: the other request committed first and the unique
		// index rejected this insert. Surface it as a conflict, not a 500.
		if errors.Is(err, userrepo.ErrEmailConflict) {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return u, nil
}

// Login validates credentials and returns the user plus a signed JWT. The email
// is normalized before lookup so the casing used at registration time does not
// matter: "User@X.com" registered, "user@x.com" logs in — same row.
func (s *Service) Login(ctx context.Context, in LoginInput) (*models.User, string, error) {
	u, err := s.users.FindByEmail(ctx, email.Normalize(in.Email))
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
	return u, token, nil
}

// Get returns a user by id; used by handlers to look up the current user.
func (s *Service) Get(ctx context.Context, userID string) (*models.User, error) {
	return s.users.FindByID(ctx, userID)
}

// GetByEmail looks up a user by email; used when resolving an invitee.
// Normalizes the input so an invite issued with mismatched casing still
// resolves to the registered account.
func (s *Service) GetByEmail(ctx context.Context, emailAddr string) (*models.User, error) {
	return s.users.FindByEmail(ctx, email.Normalize(emailAddr))
}
