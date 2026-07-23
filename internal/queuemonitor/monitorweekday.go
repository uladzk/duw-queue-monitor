package queuemonitor

import (
	"context"
	"fmt"
	"github.com/uladzk/duw-queue-monitor/internal/logger"
	"time"
)

// WeekdayQueueMonitor is a wrapper around the DefaultQueueMonitor that disables queue monitoring on weekends and outside configured working hours (UTC).
// It uses a DateTimeProvider to get the current time, allowing for easier testing and mocking
// Note: I don't like this idea, but DUW API returns queue active and available during weekends, so this is the easiest way to avoid unnecessary notifications to users.
type WeekdayQueueMonitor struct {
	defaultMonitor   QueueMonitor
	timeProvider     DateTimeProvider
	log              *logger.Logger
	workingHourStart int
	workingHourEnd   int
}

type DateTimeProvider interface {
	Now() time.Time
}

func NewWeekdayQueueMonitor(defaultMonitor QueueMonitor, cfg *Config, timeProvider DateTimeProvider, log *logger.Logger) *WeekdayQueueMonitor {
	return &WeekdayQueueMonitor{
		defaultMonitor:   defaultMonitor,
		log:              log,
		timeProvider:     timeProvider,
		workingHourStart: cfg.WorkingHourStartUTC,
		workingHourEnd:   cfg.WorkingHourEndUTC,
	}
}

func (w *WeekdayQueueMonitor) Init(initState *MonitorState) {
	w.defaultMonitor.Init(initState)
}

func (w *WeekdayQueueMonitor) GetState() *MonitorState {
	return w.defaultMonitor.GetState()
}

func (w *WeekdayQueueMonitor) CheckAndProcessStatus(ctx context.Context) error {
	if w.isDuwOffTime() {
		w.log.Debug(fmt.Sprintf("Queue monitoring is disabled on weekends and outside working hours (%02d:00 - %02d:00 UTC), skipping status check", w.workingHourStart, w.workingHourEnd))
		return nil
	}

	return w.defaultMonitor.CheckAndProcessStatus(ctx)
}

func (w *WeekdayQueueMonitor) isDuwOffTime() bool {
	now := w.timeProvider.Now().UTC()

	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return true
	}

	if now.Hour() < w.workingHourStart || now.Hour() >= w.workingHourEnd {
		return true
	}

	return false
}
