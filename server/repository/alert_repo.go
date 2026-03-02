package repository

import (
	"context"
	"time"

	"github.com/alwayson/server/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AlertRepo struct{ pool *pgxpool.Pool }

func NewAlertRepo(pool *pgxpool.Pool) *AlertRepo {
	return &AlertRepo{pool: pool}
}

func (r *AlertRepo) Insert(ctx context.Context, a *model.Alert) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO alerts (server_id, level, metric, value, message, created_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())`,
		a.ServerID, a.Level, a.Metric, a.Value, a.Message,
	)
	return err
}

func (r *AlertRepo) GetAll(ctx context.Context, limit int) ([]model.Alert, error) {
	sql := `
		SELECT id, server_id, level, metric, value, message, created_at, resolved_at
		FROM alerts ORDER BY created_at DESC LIMIT $1`
	rows, err := r.pool.Query(ctx, sql, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []model.Alert
	for rows.Next() {
		var a model.Alert
		if err := rows.Scan(&a.ID, &a.ServerID, &a.Level, &a.Metric, &a.Value,
			&a.Message, &a.CreatedAt, &a.ResolvedAt); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func (r *AlertRepo) ResolveByServer(ctx context.Context, serverID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE alerts SET resolved_at = $1
		 WHERE server_id = $2 AND resolved_at IS NULL`,
		time.Now(), serverID,
	)
	return err
}
