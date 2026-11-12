package user

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(
	ctx context.Context,
	tenantID int64,
	search string,
	limit, offset int,
) ([]User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, COALESCE(tu.display_name, u.name), u.email, u.created_at
		FROM users u
		JOIN tenant_users tu ON tu.user_id = u.id
		WHERE tu.tenant_id = $1
		  AND tu.deleted_at IS NULL
		  AND (
		    $2 = ''
		    OR COALESCE(tu.display_name, u.name) ILIKE '%' || $2 || '%'
		    OR u.email ILIKE '%' || $2 || '%'
		  )
		ORDER BY u.id
		LIMIT $3 OFFSET $4
	`, tenantID, search, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]User, 0)

	for rows.Next() {
		var u User
		if err := rows.Scan(
			&u.ID, &u.Name, &u.Email, &u.CreatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, rows.Err()
}

func (r *Repository) Get(
	ctx context.Context,
	tenantID, userID int64,
) (User, error) {
	var u User

	err := r.db.QueryRow(ctx, `
		SELECT u.id, COALESCE(tu.display_name, u.name), u.email, u.created_at
		FROM users u
		JOIN tenant_users tu ON tu.user_id = u.id
		WHERE tu.tenant_id = $1
		  AND u.id = $2
		  AND tu.deleted_at IS NULL
	`, tenantID, userID).Scan(
		&u.ID, &u.Name, &u.Email, &u.CreatedAt,
	)

	return u, err
}
