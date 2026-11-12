package user

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("user not found")

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(
	ctx context.Context,
	tenantID int64,
	search string,
	limit, offset int,
) ([]User, error) {
	return s.repo.List(ctx, tenantID, search, limit, offset)
}

func (s *Service) Get(
	ctx context.Context,
	tenantID, userID int64,
) (User, error) {
	u, err := s.repo.Get(ctx, tenantID, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}
