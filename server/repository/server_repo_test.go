package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/alwayson/server/config"
	"github.com/alwayson/server/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testServerRepo(t *testing.T) *repository.ServerRepo {
	t.Helper()
	cfg := config.Load()
	pool := repository.NewPool(cfg)
	t.Cleanup(func() { pool.Close() })
	return repository.NewServerRepo(pool)
}

func TestServerRepo_UpsertAndGet(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("INTEGRATION 환경변수 필요 (DB 연결 필요)")
	}
	repo := testServerRepo(t)
	ctx := context.Background()

	server, err := repo.Upsert(ctx, "test-host-01", "Test Server 01")
	require.NoError(t, err)
	assert.NotEmpty(t, server.ID)
	assert.Equal(t, "test-host-01", server.Host)
	assert.Equal(t, "Test Server 01", server.Name)

	got, err := repo.GetByHost(ctx, "test-host-01")
	require.NoError(t, err)
	assert.Equal(t, server.ID, got.ID)

	t.Cleanup(func() { _ = repo.Delete(ctx, server.ID) })
}

func TestServerRepo_GetAll(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("INTEGRATION 환경변수 필요 (DB 연결 필요)")
	}
	repo := testServerRepo(t)
	ctx := context.Background()

	server, err := repo.Upsert(ctx, "test-host-all", "Test All")
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo.Delete(ctx, server.ID) })

	servers, err := repo.GetAll(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(servers), 1)
}
