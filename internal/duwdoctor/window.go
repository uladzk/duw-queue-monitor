package duwdoctor

import "time"

// Zone classifies a moment relative to the monitor's working window.
type Zone int

const (
	ZoneOutOfWindow Zone = iota
	ZonePadding
	ZoneInWindow
)

// ZoneAt classifies now (compared in UTC) against the monitor's true window
// (weekdays, [startHour, endHour) UTC) with ±padMinutes of boundary grace.
// Mirrors WeekdayQueueMonitor.isDuwOffTime semantics, plus the padding band.
func ZoneAt(now time.Time, startHour, endHour, padMinutes int) Zone {
	u := now.UTC()
	if u.Weekday() == time.Saturday || u.Weekday() == time.Sunday {
		return ZoneOutOfWindow
	}
	start := time.Date(u.Year(), u.Month(), u.Day(), startHour, 0, 0, 0, time.UTC)
	end := time.Date(u.Year(), u.Month(), u.Day(), endHour, 0, 0, 0, time.UTC)
	if !u.Before(start) && u.Before(end) {
		return ZoneInWindow
	}
	pad := time.Duration(padMinutes) * time.Minute
	inLeadPad := !u.Before(start.Add(-pad)) && u.Before(start)
	inTrailPad := !u.Before(end) && u.Before(end.Add(pad))
	if inLeadPad || inTrailPad {
		return ZonePadding
	}
	return ZoneOutOfWindow
}
