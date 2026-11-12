package audit

import (
	"context"
	"errors"
	"strings"
)

var ErrValidation = errors.New("invalid pagination or action filter")

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(
	ctx context.Context,
	tenantID int64,
	action string,
	page, limit int,
) ([]Log, error) {
	action = strings.TrimSpace(action)

	if page < 1 || page > 1000000 ||
		limit < 1 || limit > 100 ||
		len(action) > 100 {
		return nil, ErrValidation
	}

	return s.repo.List(
		ctx, tenantID, action, limit, (page-1)*limit,
	)
}
