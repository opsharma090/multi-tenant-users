package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Log struct {
	ID         int64           `json:"id"`
	ActorID    *int64          `json:"actor_id"`
	Action     string          `json:"action"`
	TargetType string          `json:"target_type"`
	TargetID   *int64          `json:"target_id"`
	Metadata   json.RawMessage `json:"metadata"`
	CreatedAt  time.Time       `json:"created_at"`
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
	action string,
	limit, offset int,
) ([]Log, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, actor_id, action, target_type,
		       target_id, metadata, created_at
		FROM audit_logs
		WHERE tenant_id = $1
		  AND ($2 = '' OR action = $2)
		ORDER BY created_at DESC, id DESC
		LIMIT $3 OFFSET $4
	`, tenantID, action, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Log, 0)

	for rows.Next() {
		var item Log
		if err := rows.Scan(
			&item.ID,
			&item.ActorID,
			&item.Action,
			&item.TargetType,
			&item.TargetID,
			&item.Metadata,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}

		result = append(result, item)
	}

	return result, rows.Err()
}
