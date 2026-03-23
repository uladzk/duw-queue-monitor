package queuemonitor

import (
	"context"
	"time"
	"github.com/uladzk/duw-queue-monitor/internal/logger"
)

// WeekdayQueueMonitor is a wrapper around the DefaultQueueMonitor that disables queue monitoring on weekends and outside working hours (05:00 - 18:00 UTC).
// It uses a DateTimeProvider to get the current time, allowing for easier testing and mocking
// Note: I don't like this idea, but DUW API returns queue active and available during weekends, so this is the easiest way to avoid unnecessary notifications to users.
type WeekdayQueueMonitor struct {
	defaultMonitor QueueMonitor
	timeProvider   DateTimeProvider
	log            *logger.Logger
}

const (
	workingHourStart = 5  // 05:00 UTC
	workingHourEnd   = 18 // 18:00 UTC
)

type DateTimeProvider interface {
	Now() time.Time
}

func NewWeekdayQueueMonitor(defaultMonitor QueueMonitor, timeProvider DateTimeProvider, log *logger.Logger) *WeekdayQueueMonitor {
	return &WeekdayQueueMonitor{
		defaultMonitor: defaultMonitor,
		log:            log,
		timeProvider:   timeProvider,
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
		w.log.Debug("Queue monitoring is disabled on weekends and outside working hours (05:00 - 18:00 UTC), skipping status check")
		return nil
	}

	return w.defaultMonitor.CheckAndProcessStatus(ctx)
}

func (w *WeekdayQueueMonitor) isDuwOffTime() bool {
	now := w.timeProvider.Now().UTC()

	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return true
	}

	if now.Hour() < workingHourStart || now.Hour() >= workingHourEnd {
		return true
	}

	return false
}
