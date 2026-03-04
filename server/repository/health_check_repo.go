package repository

import (
	"context"

	"github.com/alwayson/server/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type HealthCheckRepo struct{ pool *pgxpool.Pool }

func NewHealthCheckRepo(pool *pgxpool.Pool) *HealthCheckRepo {
	return &HealthCheckRepo{pool: pool}
}

func (r *HealthCheckRepo) Insert(ctx context.Context, cfg *model.HealthCheckConfig) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO health_check_configs (server_id, name, type, target, expected_status, interval_sec, enabled)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at`,
		cfg.ServerID, cfg.Name, cfg.Type, cfg.Target,
		cfg.ExpectedStatus, cfg.IntervalSec, cfg.Enabled,
	).Scan(&cfg.ID, &cfg.CreatedAt)
}

func (r *HealthCheckRepo) ListByServer(ctx context.Context, serverID string) ([]model.HealthCheckConfig, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, server_id, name, type, target, expected_status, interval_sec, enabled, created_at
		 FROM health_check_configs WHERE server_id = $1 ORDER BY created_at`,
		serverID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanHealthChecks(rows)
}

func (r *HealthCheckRepo) ListEnabled(ctx context.Context) ([]model.HealthCheckConfig, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT hc.id, hc.server_id, hc.name, hc.type, hc.target,
		        hc.expected_status, hc.interval_sec, hc.enabled, hc.created_at
		 FROM health_check_configs hc
		 JOIN servers s ON s.id = hc.server_id
		 WHERE hc.enabled = true AND s.status != 'down'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanHealthChecks(rows)
}

func (r *HealthCheckRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM health_check_configs WHERE id = $1`, id)
	return err
}

func scanHealthChecks(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]model.HealthCheckConfig, error) {
	var list []model.HealthCheckConfig
	for rows.Next() {
		var c model.HealthCheckConfig
		if err := rows.Scan(&c.ID, &c.ServerID, &c.Name, &c.Type, &c.Target,
			&c.ExpectedStatus, &c.IntervalSec, &c.Enabled, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}
