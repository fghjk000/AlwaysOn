package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/alwayson/server/config"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}

	cfg := config.Load()
	dbURL := "pgx5://" + cfg.DBUser + ":" + cfg.DBPassword + "@" + cfg.DBHost + ":" + cfg.DBPort + "/" + cfg.DBName + "?sslmode=disable"

	migrationsPath := findMigrationsPath()

	m, err := migrate.New("file://"+migrationsPath, dbURL)
	if err != nil {
		log.Fatalf("migrate 초기화 실패: %v", err)
	}
	defer m.Close()

	switch direction {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate up 실패: %v", err)
		}
		log.Println("Migration up 완료")
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate down 실패: %v", err)
		}
		log.Println("Migration down 완료")
	default:
		log.Fatalf("알 수 없는 direction: %s (up 또는 down 사용)", direction)
	}
}

func findMigrationsPath() string {
	candidates := []string{
		"../../migrations",
		"../migrations",
		"migrations",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}
	log.Fatal("migrations 디렉토리를 찾을 수 없습니다")
	return ""
}
