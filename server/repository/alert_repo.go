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

// CanAlert: key의 마지막 알림 시각이 cooldown보다 오래됐으면 true를 반환하고 시각을 갱신한다.
// 동시성 안전을 위해 단일 UPSERT로 처리한다.
func (r *AlertRepo) CanAlert(ctx context.Context, key string, cooldown time.Duration) (bool, error) {
	cutoff := time.Now().Add(-cooldown)
	result, err := r.pool.Exec(ctx, `
		INSERT INTO alert_cooldowns (key, alerted_at)
		VALUES ($1, NOW())
		ON CONFLICT (key) DO UPDATE
		  SET alerted_at = NOW()
		  WHERE alert_cooldowns.alerted_at < $2
	`, key, cutoff)
	if err != nil {
		return false, err
	}
	// RowsAffected == 1이면 INSERT 또는 UPDATE 성공 → 알림 허용
	return result.RowsAffected() == 1, nil
}
