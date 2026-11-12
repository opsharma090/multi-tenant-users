package role

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrConflict = errors.New("role name already exists")

type Role struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(
	ctx context.Context,
	tenantID, actorID int64,
	name string,
) (Role, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Role{}, err
	}
	defer tx.Rollback(ctx)

	var result Role

	err = tx.QueryRow(ctx, `
		INSERT INTO roles (tenant_id, name)
		VALUES ($1, $2)
		RETURNING id, name, created_at
	`, tenantID, name).Scan(
		&result.ID, &result.Name, &result.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Role{}, ErrConflict
		}
		return Role{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs
			(tenant_id, actor_id, action, target_type, target_id)
		VALUES ($1, $2, 'role:create', 'role', $3)
	`, tenantID, actorID, result.ID)
	if err != nil {
		return Role{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Role{}, err
	}

	return result, nil
}

func (r *Repository) List(
	ctx context.Context,
	tenantID int64,
	search string,
	limit, offset int,
) ([]Role, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, created_at
		FROM roles
		WHERE tenant_id = $1
		  AND ($2 = '' OR name ILIKE '%' || $2 || '%')
		ORDER BY id
		LIMIT $3 OFFSET $4
	`, tenantID, search, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Role, 0)

	for rows.Next() {
		var item Role
		if err := rows.Scan(
			&item.ID, &item.Name, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, rows.Err()
}
