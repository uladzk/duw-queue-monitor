package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/UladzK/duw-queue-monitor/internal/dailystats"
	"github.com/UladzK/duw-queue-monitor/internal/logger"
	"github.com/UladzK/duw-queue-monitor/internal/notifications"
	"github.com/UladzK/duw-queue-monitor/internal/statsreporting"

	"github.com/caarlos0/env/v11"
	_ "github.com/lib/pq"
)

func main() {
	period := flag.String("period", "", "Report period: daily, weekly, monthly")
	flag.Parse()

	if err := run(*period); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(period string) error {
	if period == "" {
		return fmt.Errorf("--period flag is required (daily, weekly, monthly)")
	}

	log, err := buildLogger()
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	var cfg statsreporting.Config
	if err := env.Parse(&cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	db, err := sql.Open("postgres", cfg.StatsReporting.PostgresConString)
	if err != nil {
		return fmt.Errorf("failed to open postgres connection: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping postgres: %w", err)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	sender := notifications.NewTelegramNotifier(&cfg.NotificationTelegram, log, httpClient)
	statsReader := dailystats.NewRepository(db)
	timeProvider := statsreporting.NewSystemDateTimeProvider()

	reporter := statsreporting.NewReporter(&cfg.StatsReporting, log, statsReader, sender, timeProvider)

	chatID := fmt.Sprintf("@%s", cfg.StatsReporting.ChannelName)
	log.Info("Sending stats report", "period", period, "chatID", chatID)
	if err := reporter.SendReport(context.Background(), period, chatID); err != nil {
		return fmt.Errorf("failed to send report: %w", err)
	}

	log.Info("Report sent successfully")
	return nil
}

func buildLogger() (*logger.Logger, error) {
	var cfg logger.Config
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}
	return logger.NewLogger(&cfg), nil
}
