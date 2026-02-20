package dailystats

import (
	"context"
	"database/sql"
	"time"
)

//go:generate sqlc generate

type Repository struct {
	queries *Queries
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{
		queries: New(db),
	}
}

func (r *Repository) SaveDailyStats(ctx context.Context, queueID int, queueName string, date time.Time, totalTicketsAvailable int, takenTickets int) error {
	return r.queries.UpsertDailyStats(ctx, UpsertDailyStatsParams{
		QueueID:               int32(queueID),
		QueueName:             queueName,
		Date:                  date,
		TotalTicketsAvailable: int32(totalTicketsAvailable),
		TakenTickets:          int32(takenTickets),
	})
}

func (r *Repository) GetByDateRange(ctx context.Context, queueID int, startDate, endDate time.Time) ([]QueueDailyStat, error) {
	return r.queries.GetStatsByDateRange(ctx, GetStatsByDateRangeParams{
		QueueID: int32(queueID),
		Date:    startDate,
		Date_2:  endDate,
	})
}
