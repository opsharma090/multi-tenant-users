package auth

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type Identity struct {
	UserID       int64
	TenantID     int64
	PasswordHash string
}

func (r *Repository) FindIdentity(
	ctx context.Context,
	email, tenantCode string,
) (Identity, error) {
	var identity Identity

	err := r.db.QueryRow(ctx, `
		SELECT u.id, t.id, u.password_hash
		FROM users u
		JOIN tenant_users tu ON tu.user_id = u.id
		JOIN tenants t ON t.id = tu.tenant_id
		WHERE u.email = $1
		  AND t.code = $2
		  AND t.status = 'active'
		  AND tu.deleted_at IS NULL
	`, email, tenantCode).Scan(
		&identity.UserID,
		&identity.TenantID,
		&identity.PasswordHash,
	)

	return identity, err
}
