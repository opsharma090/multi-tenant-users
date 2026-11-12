package tenant

import (
	"context"
	"regexp"
	"strings"
)

var validCode = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,49}$`)

func (s *Service) Create(
	ctx context.Context,
	sourceTenantID, actorID int64,
	name, code string,
) (Tenant, error) {
	name = strings.TrimSpace(name)
	code = strings.ToLower(strings.TrimSpace(code))

	if name == "" || len(name) > 150 || !validCode.MatchString(code) {
		return Tenant{}, ErrCreateValidation
	}

	return s.repo.Create(ctx, sourceTenantID, actorID, name, code)
}
