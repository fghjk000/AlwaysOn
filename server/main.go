package main

import (
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

	handlers := &api.Handlers{
		Metric:  api.NewMetricHandler(metricSvc),
		ServerH: api.NewServerHandler(serverRepo, metricRepo, thresholdRepo),
		AlertH:  api.NewAlertHandler(alertRepo),
	}

	r := api.NewRouter(handlers)
	log.Printf("AlwaysOn Server 시작 - 포트 :%s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("서버 시작 실패: %v", err)
	}
}
