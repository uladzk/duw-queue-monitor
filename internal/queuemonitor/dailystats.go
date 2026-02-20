package queuemonitor

import (
	"context"
	"time"
)

// DailyStatsRepository defines the interface for persisting daily queue statistics.
type DailyStatsRepository interface {
	SaveDailyStats(ctx context.Context, queueID int, queueName string, date time.Time, totalTicketsAvailable int, takenTickets int) error
}
