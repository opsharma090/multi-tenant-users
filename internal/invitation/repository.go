package invitation

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Account(
	ctx context.Context,
	email string,
) (Account, error) {
	var account Account

	err := r.db.QueryRow(ctx, `
		SELECT id, password_hash
		FROM users
		WHERE email = $1
	`, email).Scan(
		&account.ID,
		&account.PasswordHash,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrCredentials
	}

	return account, err
}

func (r *Repository) Create(
	ctx context.Context,
	tenantID, actorID int64,
	email, tokenHash string,
	expires time.Time,
) (Invitation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Invitation{}, err
	}
	defer tx.Rollback(ctx)

	// Same tenant ki concurrent invitations serialize hongi.
	var status string
	err = tx.QueryRow(ctx, `
		SELECT status
		FROM tenants
		WHERE id = $1
		FOR UPDATE
	`, tenantID).Scan(&status)

	if errors.Is(err, pgx.ErrNoRows) ||
		(err == nil && status != "active") {
		return Invitation{}, ErrForbidden
	}
	if err != nil {
		return Invitation{}, err
	}

	// Transaction ke andar membership aur permission recheck.
	var allowed bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM tenant_users tu
			JOIN user_roles ur
			  ON ur.tenant_id = tu.tenant_id
			 AND ur.user_id = tu.user_id
			JOIN role_permissions rp
			  ON rp.tenant_id = ur.tenant_id
			 AND rp.role_id = ur.role_id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE tu.tenant_id = $1
			  AND tu.user_id = $2
			  AND tu.deleted_at IS NULL
			  AND p.code = 'user:invite'
		)
	`, tenantID, actorID).Scan(&allowed)
	if err != nil {
		return Invitation{}, err
	}
	if !allowed {
		return Invitation{}, ErrForbidden
	}

	// Already active member ko dobara invite nahi karenge.
	var alreadyMember bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM tenant_users tu
			JOIN users u ON u.id = tu.user_id
			WHERE tu.tenant_id = $1
			  AND u.email = $2
			  AND tu.deleted_at IS NULL
		)
	`, tenantID, email).Scan(&alreadyMember)
	if err != nil {
		return Invitation{}, err
	}
	if alreadyMember {
		return Invitation{}, ErrAlreadyMember
	}

	// Re-invite par previous pending token invalidate hoga.
	_, err = tx.Exec(ctx, `
		UPDATE tenant_invitations
		SET revoked_at = NOW()
		WHERE tenant_id = $1
		  AND email = $2
		  AND accepted_at IS NULL
		  AND revoked_at IS NULL
	`, tenantID, email)
	if err != nil {
		return Invitation{}, err
	}

	var inv Invitation
	err = tx.QueryRow(ctx, `
		INSERT INTO tenant_invitations (
			tenant_id, email, token_hash, invited_by, expires_at
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, tenant_id, email, expires_at
	`, tenantID, email, tokenHash, actorID, expires).Scan(
		&inv.ID,
		&inv.TenantID,
		&inv.Email,
		&inv.ExpiresAt,
	)
	if err != nil {
		return Invitation{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs (
			tenant_id, actor_id, action, target_type, target_id
		)
		VALUES ($1, $2, 'invitation:create', 'invitation', $3)
	`, tenantID, actorID, inv.ID)
	if err != nil {
		return Invitation{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Invitation{}, err
	}

	return inv, nil
}

func (r *Repository) Accept(
	ctx context.Context,
	tokenHash string,
	userID int64,
	email string,
	now time.Time,
) (Membership, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Membership{}, err
	}
	defer tx.Rollback(ctx)

	// Target tenant invitation se milega, request body se nahi.
	var tenantID int64
	err = tx.QueryRow(ctx, `
		SELECT tenant_id
		FROM tenant_invitations
		WHERE token_hash = $1
	`, tokenHash).Scan(&tenantID)

	if errors.Is(err, pgx.ErrNoRows) {
		return Membership{}, ErrInvalid
	}
	if err != nil {
		return Membership{}, err
	}

	// Lock order: tenant -> invitation -> membership.
	var status string
	err = tx.QueryRow(ctx, `
		SELECT status
		FROM tenants
		WHERE id = $1
		FOR UPDATE
	`, tenantID).Scan(&status)

	if errors.Is(err, pgx.ErrNoRows) ||
		(err == nil && status != "active") {
		return Membership{}, ErrInvalid
	}
	if err != nil {
		return Membership{}, err
	}

	var inv Invitation
	err = tx.QueryRow(ctx, `
		SELECT id, tenant_id, email, expires_at,
		       accepted_at, revoked_at
		FROM tenant_invitations
		WHERE token_hash = $1
		FOR UPDATE
	`, tokenHash).Scan(
		&inv.ID,
		&inv.TenantID,
		&inv.Email,
		&inv.ExpiresAt,
		&inv.AcceptedAt,
		&inv.RevokedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return Membership{}, ErrInvalid
	}
	if err != nil {
		return Membership{}, err
	}

	// Lock wait ke baad bhi invitation expire ho sakti hai.
	if current := time.Now(); current.After(now) {
		now = current
	}

	if err := validateInvitation(inv, email, now); err != nil {
		return Membership{}, err
	}

	// Invite bhejne wale ki membership abhi bhi active honi chahiye.
	var inviterActive bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM tenant_invitations i
			JOIN tenant_users tu
			  ON tu.tenant_id = i.tenant_id
			 AND tu.user_id = i.invited_by
			WHERE i.id = $1
			  AND tu.deleted_at IS NULL
		)
	`, inv.ID).Scan(&inviterActive)
	if err != nil {
		return Membership{}, err
	}
	if !inviterActive {
		return Membership{}, ErrInvalid
	}

	// Existing global user ki membership create ya restore hogi.
	tag, err := tx.Exec(ctx, `
		INSERT INTO tenant_users (tenant_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (tenant_id, user_id)
		DO UPDATE
		SET deleted_at = NULL,
		    display_name = NULL
		WHERE tenant_users.deleted_at IS NOT NULL
	`, tenantID, userID)
	if err != nil {
		return Membership{}, err
	}
	if tag.RowsAffected() == 0 {
		return Membership{}, ErrAlreadyMember
	}

	// Rejoin par old roles restore nahi honge.
	_, err = tx.Exec(ctx, `
		DELETE FROM user_roles
		WHERE tenant_id = $1 AND user_id = $2
	`, tenantID, userID)
	if err != nil {
		return Membership{}, err
	}

	// Old refresh sessions bhi restore nahi hongi.
	_, err = tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE tenant_id = $1
		  AND user_id = $2
		  AND revoked_at IS NULL
	`, tenantID, userID)
	if err != nil {
		return Membership{}, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE tenant_invitations
		SET accepted_at = $1
		WHERE id = $2
	`, now, inv.ID)
	if err != nil {
		return Membership{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs (
			tenant_id, actor_id, action,
			target_type, target_id, metadata
		)
		VALUES
			(
				$1, $2, 'invitation:accept', 'invitation', $3,
				jsonb_build_object('user_id', $2::bigint)
			),
			(
				$1, $2, 'user:join', 'user', $2,
				jsonb_build_object('invitation_id', $3::bigint)
			)
	`, tenantID, userID, inv.ID)
	if err != nil {
		return Membership{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Membership{}, err
	}

	return Membership{
		TenantID: tenantID,
		UserID:   userID,
	}, nil
}
