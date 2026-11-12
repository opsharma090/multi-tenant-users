package tenant

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("tenant not found")

type Tenant struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Code   string `json:"code"`
	Status string `json:"status"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Get(ctx context.Context, tenantID int64) (Tenant, error) {
	var t Tenant

	err := r.db.QueryRow(ctx, `
		SELECT id, name, code, status
		FROM tenants WHERE id = $1
	`, tenantID).Scan(&t.ID, &t.Name, &t.Code, &t.Status)

	if errors.Is(err, pgx.ErrNoRows) {
		return Tenant{}, ErrNotFound
	}
	return t, err
}

func (r *Repository) Update(
	ctx context.Context,
	tenantID, actorID int64,
	name, status string,
) (Tenant, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Tenant{}, err
	}
	defer tx.Rollback(ctx)

	var t Tenant
	err = tx.QueryRow(ctx, `
		UPDATE tenants
		SET name = $1, status = $2
		WHERE id = $3
		RETURNING id, name, code, status
	`, name, status, tenantID).Scan(
		&t.ID, &t.Name, &t.Code, &t.Status,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return Tenant{}, ErrNotFound
	}
	if err != nil {
		return Tenant{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs
			(tenant_id, actor_id, action, target_type, target_id, metadata)
		VALUES (
			$1, $2, 'tenant:update', 'tenant', $1,
			jsonb_build_object('name', $3::text, 'status', $4::text)
		)
	`, tenantID, actorID, name, status)
	if err != nil {
		return Tenant{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Tenant{}, err
	}
	return t, nil
}
