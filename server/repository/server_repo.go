package repository

import (
	"context"
	"time"

	"github.com/alwayson/server/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ServerRepo struct{ pool *pgxpool.Pool }

func NewServerRepo(pool *pgxpool.Pool) *ServerRepo {
	return &ServerRepo{pool: pool}
}

func (r *ServerRepo) Upsert(ctx context.Context, host, name string) (*model.Server, error) {
	sql := `
		INSERT INTO servers (name, host, status, last_seen)
		VALUES ($1, $2, 'normal', NOW())
		ON CONFLICT (host) DO UPDATE SET last_seen = NOW()
		RETURNING id, name, host, status, last_seen`
	var s model.Server
	err := r.pool.QueryRow(ctx, sql, name, host).Scan(
		&s.ID, &s.Name, &s.Host, &s.Status, &s.LastSeen,
	)
	return &s, err
}

func (r *ServerRepo) GetByHost(ctx context.Context, host string) (*model.Server, error) {
	sql := `SELECT id, name, host, status, last_seen FROM servers WHERE host = $1`
	var s model.Server
	err := r.pool.QueryRow(ctx, sql, host).Scan(
		&s.ID, &s.Name, &s.Host, &s.Status, &s.LastSeen,
	)
	return &s, err
}

func (r *ServerRepo) GetAll(ctx context.Context) ([]model.Server, error) {
	sql := `SELECT id, name, host, status, last_seen FROM servers ORDER BY name`
	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []model.Server
	for rows.Next() {
		var s model.Server
		if err := rows.Scan(&s.ID, &s.Name, &s.Host, &s.Status, &s.LastSeen); err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}
	return servers, nil
}

func (r *ServerRepo) UpdateStatus(ctx context.Context, id string, status model.ServerStatus) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE servers SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (r *ServerRepo) UpdateLastSeen(ctx context.Context, id string, t time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE servers SET last_seen = $1 WHERE id = $2`, t, id)
	return err
}

func (r *ServerRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM servers WHERE id = $1`, id)
	return err
}
