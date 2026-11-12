package user

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrConflict = errors.New("email already registered")
var ErrSelfDelete = errors.New("cannot delete your own membership")

func (r *Repository) Create(
	ctx context.Context,
	tenantID, actorID int64,
	name, email, passwordHash string,
) (User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)

	var u User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (name, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, name, email, created_at
	`, name, email, passwordHash).Scan(
		&u.ID, &u.Name, &u.Email, &u.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, ErrConflict
		}
		return User{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO tenant_users (tenant_id, user_id, display_name)
		VALUES ($1, $2, $3)
	`, tenantID, u.ID, name)
	if err != nil {
		return User{}, err
	}

	if err := writeAudit(ctx, tx, tenantID, actorID, u.ID, "user:create"); err != nil {
		return User{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return u, nil
}

func (r *Repository) Update(
	ctx context.Context,
	tenantID, actorID, userID int64,
	name string,
) (User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE tenant_users
		SET display_name = $1
		WHERE tenant_id = $2 AND user_id = $3
		  AND deleted_at IS NULL
	`, name, tenantID, userID)
	if err != nil {
		return User{}, err
	}
	if tag.RowsAffected() == 0 {
		return User{}, ErrNotFound
	}

	var u User
	err = tx.QueryRow(ctx, `
		SELECT u.id, tu.display_name, u.email, u.created_at
		FROM users u
		JOIN tenant_users tu ON tu.user_id = u.id
		WHERE tu.tenant_id = $1 AND tu.user_id = $2
		  AND tu.deleted_at IS NULL
	`, tenantID, userID).Scan(
		&u.ID, &u.Name, &u.Email, &u.CreatedAt,
	)
	if err != nil {
		return User{}, err
	}

	if err := writeAudit(ctx, tx, tenantID, actorID, userID, "user:update"); err != nil {
		return User{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return u, nil
}

func (r *Repository) Delete(
	ctx context.Context,
	tenantID, actorID, userID int64,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE tenant_users
		SET deleted_at = NOW()
		WHERE tenant_id = $1 AND user_id = $2
		  AND deleted_at IS NULL
	`, tenantID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM user_roles
		WHERE tenant_id = $1 AND user_id = $2
	`, tenantID, userID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW()
		WHERE tenant_id = $1 AND user_id = $2
		  AND revoked_at IS NULL
	`, tenantID, userID)
	if err != nil {
		return err
	}

	if err := writeAudit(ctx, tx, tenantID, actorID, userID, "user:delete"); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func writeAudit(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, actorID, userID int64,
	action string,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO audit_logs
			(tenant_id, actor_id, action, target_type, target_id)
		VALUES ($1, $2, $3, 'user', $4)
	`, tenantID, actorID, action, userID)
	return err
}
