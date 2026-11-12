package tenant

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var ErrCodeConflict = errors.New("organization code already exists")

func (r *Repository) Create(
	ctx context.Context,
	sourceTenantID, actorID int64,
	name, code string,
) (Tenant, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Tenant{}, err
	}
	defer tx.Rollback(ctx)

	var t Tenant
	err = tx.QueryRow(ctx, `
		INSERT INTO tenants (name, code)
		VALUES ($1, $2)
		RETURNING id, name, code, status
	`, name, code).Scan(&t.ID, &t.Name, &t.Code, &t.Status)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Tenant{}, ErrCodeConflict
		}
		return Tenant{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO tenant_users (tenant_id, user_id)
		VALUES ($1, $2)
	`, t.ID, actorID)
	if err != nil {
		return Tenant{}, err
	}

	var roleID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO roles (tenant_id, name)
		VALUES ($1, 'admin')
		RETURNING id
	`, t.ID).Scan(&roleID)
	if err != nil {
		return Tenant{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO role_permissions
			(tenant_id, role_id, permission_id)
		SELECT $1, $2, id FROM permissions
	`, t.ID, roleID)
	if err != nil {
		return Tenant{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO user_roles (tenant_id, user_id, role_id)
		VALUES ($1, $2, $3)
	`, t.ID, actorID, roleID)
	if err != nil {
		return Tenant{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs
			(tenant_id, actor_id, action, target_type, target_id, metadata)
		VALUES
			($1, $2, 'tenant:create', 'tenant', $1,
			 jsonb_build_object('source_tenant_id', $3::bigint)),
			($1, $2, 'user:join', 'user', $2, '{}'::jsonb),
			($1, $2, 'role:create', 'role', $4, '{}'::jsonb),
			($1, $2, 'role:assign', 'user', $2,
			 jsonb_build_object('role_id', $4::bigint)),
			($1, $2, 'permission:assign', 'role', $4,
			 jsonb_build_object('permissions',
			     (SELECT jsonb_agg(code ORDER BY code) FROM permissions)))
	`, t.ID, actorID, sourceTenantID, roleID)
	if err != nil {
		return Tenant{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Tenant{}, err
	}
	return t, nil
}
