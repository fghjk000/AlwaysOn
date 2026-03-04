//go:build integration

package repository_test

import (
	"context"
	"testing"

	"github.com/alwayson/server/model"
	"github.com/alwayson/server/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthCheckRepo_CRUD(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewHealthCheckRepo(pool)
	serverRepo := repository.NewServerRepo(pool)
	ctx := context.Background()

	server, err := serverRepo.Upsert(ctx, "hc-test-host", "HC Test")
	require.NoError(t, err)

	// Insert
	cfg := &model.HealthCheckConfig{
		ServerID: server.ID, Name: "Nginx",
		Type: "http", Target: "http://localhost/health",
		ExpectedStatus: 200, IntervalSec: 30, Enabled: true,
	}
	err = repo.Insert(ctx, cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.ID)

	// ListByServer
	list, err := repo.ListByServer(ctx, server.ID)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "Nginx", list[0].Name)

	// ListEnabled (all enabled)
	all, err := repo.ListEnabled(ctx)
	require.NoError(t, err)
	found := false
	for _, c := range all {
		if c.ID == list[0].ID {
			found = true
		}
	}
	assert.True(t, found)

	// Delete
	err = repo.Delete(ctx, list[0].ID)
	require.NoError(t, err)

	list2, err := repo.ListByServer(ctx, server.ID)
	require.NoError(t, err)
	assert.Len(t, list2, 0)
}
