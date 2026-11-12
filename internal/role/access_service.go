package role

import (
	"context"
	"strings"
)

func (s *Service) Permissions(ctx context.Context) ([]Permission, error) {
	return s.repo.Permissions(ctx)
}

func (s *Service) SetPermissions(
	ctx context.Context,
	tenantID, actorID, roleID int64,
	codes []string,
) error {
	if codes == nil || len(codes) > 100 {
		return ErrPermissions
	}

	unique := make([]string, 0, len(codes))
	seen := make(map[string]bool)

	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" || len(code) > 100 {
			return ErrPermissions
		}
		if !seen[code] {
			unique = append(unique, code)
			seen[code] = true
		}
	}

	return s.repo.SetPermissions(ctx, tenantID, actorID, roleID, unique)
}

func (s *Service) Assign(
	ctx context.Context,
	tenantID, actorID, userID, roleID int64,
) error {
	return s.repo.Assign(ctx, tenantID, actorID, userID, roleID)
}
