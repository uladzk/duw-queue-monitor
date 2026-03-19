package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/uladzk/duw-queue-monitor/internal/dailystats"
	"github.com/uladzk/duw-queue-monitor/internal/logger"
	"github.com/uladzk/duw-queue-monitor/internal/notifications"
	"github.com/uladzk/duw-queue-monitor/internal/queuemonitor"

	"github.com/caarlos0/env/v11"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log, err := buildLogger()
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	runner, cleanup, err := buildRunner(log)
	if err != nil {
		return fmt.Errorf("failed to initialize runner: %w", err)
	}
	defer cleanup()

	log.Info("Starting queue monitor...")

	done := make(chan bool, 1)
	go runner.Run(ctx, done)

	log.Info("Queue monitor started. Waiting for shutdown signal...")
	<-ctx.Done()
	log.Info("Received shutdown signal, waiting for status collector to stop...")
	cancel()
	<-done

	log.Info("Queue monitor stopped")

	return nil
}

func buildLogger() (*logger.Logger, error) {
	var cfg logger.Config
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	return logger.NewLogger(&cfg), nil
}

func buildRunner(log *logger.Logger) (*queuemonitor.Runner, func(), error) {
	var cfg queuemonitor.Config
	if err := env.Parse(&cfg); err != nil {
		return nil, nil, err
	}

	httpClient := &http.Client{
		Timeout: time.Duration(cfg.QueueMonitor.HttpClientTimeoutSeconds) * time.Second,
	}

	opt, err := redis.ParseURL(cfg.QueueMonitor.RedisConString)
	if err != nil {
		return nil, nil, err
	}
	redisClient := redis.NewClient(opt)

	stateRepo := queuemonitor.NewMonitorStateRepository(redisClient, cfg.QueueMonitor.StateTtlSeconds)
	collector := queuemonitor.NewStatusCollector(&cfg.QueueMonitor, httpClient, log)
	notifier := buildNotifier(&cfg, log, httpClient)

	cleanup := func() {}
	var statsRepo queuemonitor.DailyStatsRepository
	if cfg.FFDailyStatsEnabled {
		db, err := sql.Open("postgres", cfg.QueueMonitor.PostgresConString)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open postgres connection: %w", err)
		}
		if err := db.Ping(); err != nil {
			return nil, nil, fmt.Errorf("failed to ping postgres: %w", err)
		}
		log.Info("Daily stats feature enabled, connected to PostgreSQL")
		statsRepo = dailystats.NewRepository(db)
		cleanup = func() { db.Close() }
	}

	timeProvider := queuemonitor.NewSystemDateTimeProvider()
	monitor := queuemonitor.NewQueueMonitor(&cfg, log, collector, notifier, statsRepo, timeProvider)
	weekdayMonitor := queuemonitor.NewWeekdayQueueMonitor(monitor, timeProvider, log)

	runner := queuemonitor.NewRunner(&cfg, log, weekdayMonitor, stateRepo)
	return runner, cleanup, nil
}

func buildNotifier(cfg *queuemonitor.Config, log *logger.Logger, httpClient *http.Client) queuemonitor.Notifier {
	return notifications.NewTelegramNotifier(&cfg.NotificationTelegram, log, httpClient)
}
