package repository

import (
	"context"
	"time"

	"github.com/alwayson/server/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MetricRepo struct{ pool *pgxpool.Pool }

func NewMetricRepo(pool *pgxpool.Pool) *MetricRepo {
	return &MetricRepo{pool: pool}
}

func (r *MetricRepo) Insert(ctx context.Context, m *model.Metric) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO metrics (time, server_id, cpu, memory, disk, net_in, net_out)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		m.Time, m.ServerID, m.CPU, m.Memory, m.Disk, m.NetIn, m.NetOut,
	)
	return err
}

func (r *MetricRepo) GetRecent(ctx context.Context, serverID string, since time.Time) ([]model.Metric, error) {
	sql := `
		SELECT time, server_id, cpu, memory, disk, net_in, net_out
		FROM metrics
		WHERE server_id = $1 AND time >= $2
		ORDER BY time ASC`
	rows, err := r.pool.Query(ctx, sql, serverID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []model.Metric
	for rows.Next() {
		var m model.Metric
		if err := rows.Scan(&m.Time, &m.ServerID, &m.CPU, &m.Memory, &m.Disk, &m.NetIn, &m.NetOut); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

func (r *MetricRepo) GetLatest(ctx context.Context, serverID string) (*model.Metric, error) {
	sql := `
		SELECT time, server_id, cpu, memory, disk, net_in, net_out
		FROM metrics WHERE server_id = $1
		ORDER BY time DESC LIMIT 1`
	var m model.Metric
	err := r.pool.QueryRow(ctx, sql, serverID).Scan(
		&m.Time, &m.ServerID, &m.CPU, &m.Memory, &m.Disk, &m.NetIn, &m.NetOut,
	)
	return &m, err
}
