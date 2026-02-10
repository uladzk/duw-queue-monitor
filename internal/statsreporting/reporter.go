package statsreporting

import (
	"context"
	"fmt"
	"time"

	"github.com/UladzK/duw-queue-monitor/internal/logger"
)

type Reporter struct {
	cfg          *StatsReportingConfig
	log          *logger.Logger
	statsReader  StatsReader
	sender       MessageSender
	timeProvider DateTimeProvider
}

func NewReporter(cfg *StatsReportingConfig, log *logger.Logger, statsReader StatsReader, sender MessageSender, timeProvider DateTimeProvider) *Reporter {
	return &Reporter{
		cfg:          cfg,
		log:          log,
		statsReader:  statsReader,
		sender:       sender,
		timeProvider: timeProvider,
	}
}

func (r *Reporter) SendReport(ctx context.Context, period string) error {
	switch period {
	case "daily":
		return r.sendDailyReport(ctx)
	case "weekly":
		return r.sendWeeklyReport(ctx)
	case "monthly":
		return r.sendMonthlyReport(ctx)
	default:
		return fmt.Errorf("invalid report period: %q, expected: daily, weekly, monthly", period)
	}
}

func (r *Reporter) sendDailyReport(ctx context.Context) error {
	loc := r.loadTimezone()
	yesterday := r.timeProvider.Now().In(loc).AddDate(0, 0, -1)
	start := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, time.UTC)

	stats, err := r.statsReader.GetByDateRange(ctx, r.cfg.QueueID, start, start)
	if err != nil {
		return fmt.Errorf("failed to fetch daily stats: %w", err)
	}

	msg := buildDailyMsg(r.cfg.QueueName, stats)
	r.log.Info("Sending daily report", "date", start.Format("2006-01-02"))
	return sendReport(ctx, r.sender, r.cfg.ChannelName, msg)
}

func (r *Reporter) sendWeeklyReport(ctx context.Context) error {
	loc := r.loadTimezone()
	now := r.timeProvider.Now().In(loc)

	// Previous week: Monday to Sunday
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	lastMonday := now.AddDate(0, 0, -int(weekday)-6)
	lastSunday := lastMonday.AddDate(0, 0, 6)

	start := time.Date(lastMonday.Year(), lastMonday.Month(), lastMonday.Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(lastSunday.Year(), lastSunday.Month(), lastSunday.Day(), 0, 0, 0, 0, time.UTC)

	stats, err := r.statsReader.GetByDateRange(ctx, r.cfg.QueueID, start, end)
	if err != nil {
		return fmt.Errorf("failed to fetch weekly stats: %w", err)
	}

	msg := buildWeeklyMsg(r.cfg.QueueName, stats)
	r.log.Info("Sending weekly report", "from", start.Format("2006-01-02"), "to", end.Format("2006-01-02"))
	return sendReport(ctx, r.sender, r.cfg.ChannelName, msg)
}

func (r *Reporter) sendMonthlyReport(ctx context.Context) error {
	loc := r.loadTimezone()
	now := r.timeProvider.Now().In(loc)

	// Previous month
	firstOfCurrentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	firstOfPrevMonth := firstOfCurrentMonth.AddDate(0, -1, 0)
	lastOfPrevMonth := firstOfCurrentMonth.AddDate(0, 0, -1)

	start := time.Date(firstOfPrevMonth.Year(), firstOfPrevMonth.Month(), firstOfPrevMonth.Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(lastOfPrevMonth.Year(), lastOfPrevMonth.Month(), lastOfPrevMonth.Day(), 0, 0, 0, 0, time.UTC)

	stats, err := r.statsReader.GetByDateRange(ctx, r.cfg.QueueID, start, end)
	if err != nil {
		return fmt.Errorf("failed to fetch monthly stats: %w", err)
	}

	msg := buildMonthlyMsg(r.cfg.QueueName, stats)
	r.log.Info("Sending monthly report", "month", firstOfPrevMonth.Format("2006-01"))
	return sendReport(ctx, r.sender, r.cfg.ChannelName, msg)
}

func (r *Reporter) loadTimezone() *time.Location {
	loc, err := time.LoadLocation(r.cfg.Timezone)
	if err != nil {
		r.log.Warn("Failed to load timezone, falling back to UTC", "timezone", r.cfg.Timezone)
		return time.UTC
	}
	return loc
}
