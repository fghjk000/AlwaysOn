package main

import (
	"context"
	"log"

	"github.com/alwayson/server/api"
	"github.com/alwayson/server/config"
	"github.com/alwayson/server/repository"
	"github.com/alwayson/server/service"
	"github.com/alwayson/server/worker"
)

func main() {
	cfg := config.Load()
	pool := repository.NewPool(cfg)
	defer pool.Close()

	serverRepo := repository.NewServerRepo(pool)
	metricRepo := repository.NewMetricRepo(pool)
	alertRepo := repository.NewAlertRepo(pool)
	thresholdRepo := repository.NewThresholdRepo(pool)

	notifier := service.NewSlackNotifier(cfg.SlackWebhookURL)
	alertWorker := worker.NewAlertWorker(serverRepo, alertRepo, thresholdRepo, notifier)
	metricSvc := service.NewMetricService(serverRepo, metricRepo, alertWorker)

	// Down 감지 워커 시작
	downDetector := worker.NewDownDetector(serverRepo, alertRepo, notifier)
	downDetector.Start(context.Background())
	log.Println("DownDetector 시작 (15초 간격)")

	// HealthCheck 워커 시작
	healthCheckRepo := repository.NewHealthCheckRepo(pool)
	healthCheckWorker := worker.NewHealthCheckWorker(healthCheckRepo, alertRepo, notifier)
	healthCheckWorker.Start(context.Background())
	log.Println("HealthCheckWorker 시작 (30초 간격)")

	handlers := &api.Handlers{
		Metric:       api.NewMetricHandler(metricSvc),
		ServerH:      api.NewServerHandler(serverRepo, metricRepo, thresholdRepo),
		AlertH:       api.NewAlertHandler(alertRepo),
		HealthCheckH: api.NewHealthCheckHandler(healthCheckRepo),
	}

	r := api.NewRouter(handlers)
	log.Printf("AlwaysOn Server 시작 - 포트 :%s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("서버 시작 실패: %v", err)
	}
}
