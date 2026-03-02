package repository

import (
	"context"
	"log"

	"github.com/alwayson/server/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(cfg *config.Config) *pgxpool.Pool {
	pool, err := pgxpool.New(context.Background(), cfg.DSN())
	if err != nil {
		log.Fatalf("DB 연결 실패: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("DB ping 실패: %v", err)
	}
	return pool
}
