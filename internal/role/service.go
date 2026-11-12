package role

import (
	"context"
	"errors"
	"strings"
)

var ErrValidation = errors.New("role name must contain 1–100 characters")

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(
	ctx context.Context,
	tenantID, actorID int64,
	name string,
) (Role, error) {
	name = strings.ToLower(strings.TrimSpace(name))

	if name == "" || len(name) > 100 {
		return Role{}, ErrValidation
	}

	return s.repo.Create(ctx, tenantID, actorID, name)
}

func (s *Service) List(
	ctx context.Context,
	tenantID int64,
	search string,
	limit, offset int,
) ([]Role, error) {
	return s.repo.List(ctx, tenantID, search, limit, offset)
}
