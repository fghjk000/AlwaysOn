package repository

import (
	"context"

	"github.com/alwayson/server/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ThresholdRepo struct{ pool *pgxpool.Pool }

func NewThresholdRepo(pool *pgxpool.Pool) *ThresholdRepo {
	return &ThresholdRepo{pool: pool}
}

func (r *ThresholdRepo) Get(ctx context.Context, serverID string) (*model.Threshold, error) {
	sql := `
		SELECT server_id, cpu_warning, cpu_critical, mem_warning, mem_critical,
		       disk_warning, disk_critical
		FROM thresholds WHERE server_id = $1`
	var t model.Threshold
	err := r.pool.QueryRow(ctx, sql, serverID).Scan(
		&t.ServerID, &t.CPUWarning, &t.CPUCritical,
		&t.MemWarning, &t.MemCritical,
		&t.DiskWarning, &t.DiskCritical,
	)
	if err != nil {
		// 커스텀 설정 없으면 기본값 반환
		d := model.DefaultThreshold(serverID)
		return &d, nil
	}
	return &t, nil
}

func (r *ThresholdRepo) Upsert(ctx context.Context, t *model.Threshold) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO thresholds (server_id, cpu_warning, cpu_critical, mem_warning, mem_critical, disk_warning, disk_critical)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (server_id) DO UPDATE SET
		   cpu_warning=$2, cpu_critical=$3,
		   mem_warning=$4, mem_critical=$5,
		   disk_warning=$6, disk_critical=$7`,
		t.ServerID, t.CPUWarning, t.CPUCritical,
		t.MemWarning, t.MemCritical,
		t.DiskWarning, t.DiskCritical,
	)
	return err
}
