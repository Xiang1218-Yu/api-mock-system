// Package projectservice orchestrates project CRUD and membership, including
// role-based authorization. Authorization lives here, not in handlers or repos,
// because it's a business rule: only an admin can change members, only editors
// and admins can modify projects, etc.
package projectservice

import (
	"context"
	"errors"
	"fmt"

	"api-mock-system/internal/id"
	"api-mock-system/internal/models"
	"api-mock-system/internal/projectrepo"
)

var (
	// ErrNotFound is returned when a project lookup misses.
	ErrNotFound = errors.New("project not found")
	// ErrForbidden is returned when a user lacks the required role.
	ErrForbidden = errors.New("forbidden")
	// ErrInvalidRole is returned when an unrecognized role is supplied.
	ErrInvalidRole = errors.New("invalid role")
)

// CreateInput captures the fields needed to create a project.
type CreateInput struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	BasePath    string `json:"base_path"`
	Visibility  string `json:"visibility"` // public|private
}

// UpdateInput captures the fields that may change on a project.
type UpdateInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	BasePath    *string `json:"base_path"`
	Visibility  *string `json:"visibility"`
}

// MemberInput captures the fields needed to invite a member.
type MemberInput struct {
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role" binding:"required"`
}

// Service orchestrates projects.
type Service struct {
	projects projectrepo.Repository
}

// New wires the service to its repository.
func New(p projectrepo.Repository) *Service { return &Service{projects: p} }

// Create makes a new project owned by userID, who becomes an admin member.
func (s *Service) Create(ctx context.Context, ownerID string, in CreateInput) (*models.Project, error) {
	vis := in.Visibility
	if vis != "public" && vis != "private" {
		vis = "private"
	}
	p := &models.Project{
		Base:        models.Base{ID: id.NewUUID()},
		Name:        in.Name,
		Description: in.Description,
		BasePath:    in.BasePath,
		OwnerID:     ownerID,
		Visibility:  vis,
	}
	if err := s.projects.Create(ctx, p); err != nil {
		return nil, err
	}
	// Owner is always an admin member.
	if err := s.projects.AddMember(ctx, &models.ProjectMember{
		Base:      models.Base{ID: id.NewUUID()},
		ProjectID: p.ID,
		UserID:    ownerID,
		Role:      string(projectrepo.RoleAdmin),
	}); err != nil {
		return nil, fmt.Errorf("add owner member: %w", err)
	}
	return p, nil
}

// Get returns a project if the user can see it (public, or a member).
func (s *Service) Get(ctx context.Context, projectID, userID string) (*models.Project, error) {
	p, err := s.projects.FindByID(ctx, projectID)
	if err != nil {
		return nil, mapErr(err)
	}
	if p.Visibility == "public" {
		return p, nil
	}
	if userID == "" {
		return nil, ErrForbidden
	}
	if _, ok, err := s.projects.MemberRole(ctx, projectID, userID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrForbidden
	}
	return p, nil
}

// Update applies partial changes; requires editor+ role.
func (s *Service) Update(ctx context.Context, projectID, userID string, in UpdateInput) (*models.Project, error) {
	if err := s.requireRole(ctx, projectID, userID, projectrepo.RoleEditor); err != nil {
		return nil, err
	}
	p, err := s.projects.FindByID(ctx, projectID)
	if err != nil {
		return nil, mapErr(err)
	}
	if in.Name != nil {
		p.Name = *in.Name
	}
	if in.Description != nil {
		p.Description = *in.Description
	}
	if in.BasePath != nil {
		p.BasePath = *in.BasePath
	}
	if in.Visibility != nil {
		if *in.Visibility != "public" && *in.Visibility != "private" {
			return nil, ErrInvalidRole
		}
		p.Visibility = *in.Visibility
	}
	if err := s.projects.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Delete removes a project; admin-only.
func (s *Service) Delete(ctx context.Context, projectID, userID string) error {
	if err := s.requireRole(ctx, projectID, userID, projectrepo.RoleAdmin); err != nil {
		return err
	}
	return s.projects.Delete(ctx, projectID)
}

// List returns projects visible to the user with optional name search.
func (s *Service) List(ctx context.Context, userID, query string, page, size int) ([]models.Project, int64, error) {
	if size <= 0 {
		size = 20
	}
	if page <= 0 {
		page = 1
	}
	return s.projects.List(ctx, userID, query, size, (page-1)*size)
}

// InviteMember adds a member with the given role; admin-only. The member's
// user id is resolved by the caller (the handler looks up the email), so this
// method takes the resolved user id directly.
func (s *Service) InviteMember(ctx context.Context, projectID, actorID, inviteeID, role string) error {
	if err := s.requireRole(ctx, projectID, actorID, projectrepo.RoleAdmin); err != nil {
		return err
	}
	if !validRole(role) {
		return ErrInvalidRole
	}
	return s.projects.AddMember(ctx, &models.ProjectMember{
		Base:      models.Base{ID: id.NewUUID()},
		ProjectID: projectID,
		UserID:    inviteeID,
		Role:      role,
	})
}

// RemoveMember deletes a member; admin-only. The owner cannot be removed.
func (s *Service) RemoveMember(ctx context.Context, projectID, actorID, targetID string) error {
	p, err := s.projects.FindByID(ctx, projectID)
	if err != nil {
		return mapErr(err)
	}
	if p.OwnerID == targetID {
		return ErrForbidden
	}
	if err := s.requireRole(ctx, projectID, actorID, projectrepo.RoleAdmin); err != nil {
		return err
	}
	return s.projects.RemoveMember(ctx, projectID, targetID)
}

// ListMembers returns the membership roster; any member may view it.
func (s *Service) ListMembers(ctx context.Context, projectID, userID string) ([]models.ProjectMember, error) {
	if err := s.requireRole(ctx, projectID, userID, projectrepo.RoleViewer); err != nil {
		return nil, err
	}
	return s.projects.ListMembers(ctx, projectID)
}

// requireRole enforces that the user holds at least the given role on the
// project. Admins satisfy every role; editors satisfy editor and viewer;
// viewers satisfy only viewer.
func (s *Service) requireRole(ctx context.Context, projectID, userID string, want projectrepo.MemberRole) error {
	if userID == "" {
		return ErrForbidden
	}
	role, ok, err := s.projects.MemberRole(ctx, projectID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	if !roleSatisfies(role, want) {
		return ErrForbidden
	}
	return nil
}

func roleSatisfies(have, want projectrepo.MemberRole) bool {
	if have == projectrepo.RoleAdmin {
		return true
	}
	if have == projectrepo.RoleEditor {
		return want == projectrepo.RoleEditor || want == projectrepo.RoleViewer
	}
	if have == projectrepo.RoleViewer {
		return want == projectrepo.RoleViewer
	}
	return false
}

func validRole(role string) bool {
	switch projectrepo.MemberRole(role) {
	case projectrepo.RoleAdmin, projectrepo.RoleEditor, projectrepo.RoleViewer:
		return true
	}
	return false
}

// mapErr translates repo-not-found into the service-level not-found.
func mapErr(err error) error {
	if errors.Is(err, projectrepo.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// RequireEditor is exported for sibling services (apiservice, aggregateservice)
// that need to authorize edits against project membership without each one
// reimplementing the role ladder.
func (s *Service) RequireEditor(ctx context.Context, projectID, userID string) error {
	return s.requireRole(ctx, projectID, userID, projectrepo.RoleEditor)
}

// RequireViewer checks read access for a project.
func (s *Service) RequireViewer(ctx context.Context, projectID, userID string) error {
	return s.requireRole(ctx, projectID, userID, projectrepo.RoleViewer)
}

// IsMember reports membership without enforcing a role.
func (s *Service) IsMember(ctx context.Context, projectID, userID string) (bool, error) {
	_, ok, err := s.projects.MemberRole(ctx, projectID, userID)
	return ok, err
}
