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

func (r *Repository) SaveDailyStats(ctx context.Context, queueID int, queueName string, date time.Time, ticketsServed int, registeredTickets int) error {
	return r.queries.UpsertDailyStats(ctx, UpsertDailyStatsParams{
		QueueID:           int32(queueID),
		QueueName:         queueName,
		Date:              date,
		TicketsServed:     int32(ticketsServed),
		RegisteredTickets: int32(registeredTickets),
	})
}

func (r *Repository) GetByDate(ctx context.Context, queueID int, date time.Time) (*QueueDailyStat, error) {
	result, err := r.queries.GetDailyStatsByDate(ctx, GetDailyStatsByDateParams{
		QueueID: int32(queueID),
		Date:    date,
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *Repository) GetByDateRange(ctx context.Context, queueID int, startDate, endDate time.Time) ([]QueueDailyStat, error) {
	return r.queries.GetStatsByDateRange(ctx, GetStatsByDateRangeParams{
		QueueID: int32(queueID),
		Date:    startDate,
		Date_2:  endDate,
	})
}
