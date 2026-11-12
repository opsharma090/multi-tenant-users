package tenant

import (
	"context"
	"errors"
	"strings"
)

var ErrValidation = errors.New(
	"name must contain 1–150 bytes; status must be active or suspended",
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Get(ctx context.Context, tenantID int64) (Tenant, error) {
	return s.repo.Get(ctx, tenantID)
}

func (s *Service) Update(
	ctx context.Context,
	tenantID, actorID int64,
	name, status string,
) (Tenant, error) {
	name = strings.TrimSpace(name)
	status = strings.TrimSpace(status)

	if name == "" || len(name) > 150 ||
		(status != "active" && status != "suspended") {
		return Tenant{}, ErrValidation
	}

	return s.repo.Update(ctx, tenantID, actorID, name, status)
}
