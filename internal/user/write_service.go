package user

import (
	"context"

	"golang.org/x/crypto/bcrypt"
)

func (s *Service) Create(
	ctx context.Context,
	tenantID, actorID int64,
	name, email, password string,
) (User, error) {
	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password), bcrypt.DefaultCost,
	)
	if err != nil {
		return User{}, err
	}

	return s.repo.Create(ctx, tenantID, actorID, name, email, string(hash))
}

func (s *Service) Update(
	ctx context.Context,
	tenantID, actorID, userID int64,
	name string,
) (User, error) {
	return s.repo.Update(ctx, tenantID, actorID, userID, name)
}

func (s *Service) Delete(
	ctx context.Context,
	tenantID, actorID, userID int64,
) error {
	if actorID == userID {
		return ErrSelfDelete
	}
	return s.repo.Delete(ctx, tenantID, actorID, userID)
}
