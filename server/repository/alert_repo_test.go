//go:build integration

package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/alwayson/server/config"
	"github.com/alwayson/server/model"
	"github.com/alwayson/server/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("INTEGRATION 환경변수 필요 (DB 연결 필요)")
	}
	cfg := config.Load()
	pool := repository.NewPool(cfg)
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestAlertRepo_ResolveByServerAndMetric(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewAlertRepo(pool)
	ctx := context.Background()

	// 서버 upsert
	serverRepo := repository.NewServerRepo(pool)
	server, err := serverRepo.Upsert(ctx, "test-hc-host", "Test HC")
	require.NoError(t, err)

	// 알림 2개 삽입 (다른 metric)
	err = repo.Insert(ctx, &model.Alert{
		ServerID: server.ID, Level: model.LevelCritical,
		Metric: "health_check:Nginx", Value: 0, Message: "fail",
	})
	require.NoError(t, err)
	err = repo.Insert(ctx, &model.Alert{
		ServerID: server.ID, Level: model.LevelCritical,
		Metric: "process:mysql", Value: 0, Message: "fail",
	})
	require.NoError(t, err)

	// Nginx 헬스체크 알림만 resolve
	affected, err := repo.ResolveByServerAndMetric(ctx, server.ID, "health_check:Nginx")
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	// mysql 프로세스 알림은 여전히 열려있어야 함
	alerts, err := repo.GetAll(ctx, 10)
	require.NoError(t, err)
	open := 0
	for _, a := range alerts {
		if a.ServerID == server.ID && a.ResolvedAt == nil {
			open++
		}
	}
	assert.Equal(t, 1, open)
}
