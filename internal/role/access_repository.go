package role

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("resource not found")
var ErrPermissions = errors.New("unknown permission code")

type Permission struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
}

func (r *Repository) Permissions(ctx context.Context) ([]Permission, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, code FROM permissions ORDER BY code
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Permission, 0)
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.ID, &p.Code); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *Repository) SetPermissions(
	ctx context.Context,
	tenantID, actorID, roleID int64,
	codes []string,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var id int64
	err = tx.QueryRow(ctx, `
		SELECT id FROM roles
		WHERE tenant_id = $1 AND id = $2
		FOR UPDATE
	`, tenantID, roleID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	var count int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM permissions
		WHERE code = ANY($1::text[])
	`, codes).Scan(&count)
	if err != nil {
		return err
	}
	if count != len(codes) {
		return ErrPermissions
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM role_permissions
		WHERE tenant_id = $1 AND role_id = $2
	`, tenantID, roleID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO role_permissions
			(tenant_id, role_id, permission_id)
		SELECT $1, $2, id
		FROM permissions
		WHERE code = ANY($3::text[])
	`, tenantID, roleID, codes)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs
			(tenant_id, actor_id, action, target_type, target_id, metadata)
		VALUES (
			$1, $2, 'permission:assign', 'role', $3,
			jsonb_build_object('permissions', $4::text[])
		)
	`, tenantID, actorID, roleID, codes)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *Repository) Assign(
	ctx context.Context,
	tenantID, actorID, userID, roleID int64,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// A role from another tenant must return 404.
	var id int64
	err = tx.QueryRow(ctx, `
		SELECT id FROM roles
		WHERE tenant_id = $1 AND id = $2
		FOR SHARE
	`, tenantID, roleID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	// The user must have an active membership in this tenant.
	err = tx.QueryRow(ctx, `
		SELECT user_id FROM tenant_users
		WHERE tenant_id = $1 AND user_id = $2
		  AND deleted_at IS NULL
		FOR UPDATE
	`, tenantID, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO user_roles (tenant_id, user_id, role_id)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`, tenantID, userID, roleID)
	if err != nil {
		return err
	}

	if tag.RowsAffected() > 0 {
		_, err = tx.Exec(ctx, `
			INSERT INTO audit_logs
				(tenant_id, actor_id, action, target_type, target_id, metadata)
			VALUES (
				$1, $2, 'role:assign', 'user', $3,
				jsonb_build_object('role_id', $4::bigint)
			)
		`, tenantID, actorID, userID, roleID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
