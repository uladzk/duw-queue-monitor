package duwdoctor

import (
	"fmt"
	"time"
)

// CheckInput is the full set of observations a single checker run feeds to Evaluate.
type CheckInput struct {
	Now             time.Time
	Zone            Zone
	SchemaValid     bool  // DUW response decoded and queue found
	Expected        State // from DUW
	Observed        State // from channel
	ObservedOK      bool  // a status post was found
	MaxIdenticalRun int   // duplicate-flood signal
	Prev            DoctorState
	DebounceK       int
	Cooldown        time.Duration
	FloodThreshold  int
}

// CheckResult is the verdict plus the next state to persist.
type CheckResult struct {
	Escalate bool
	Reason   string
	Next     DoctorState
}

// Evaluate is the pure decision core: compare expected vs observed with debounce and
// cooldown, plus an independent duplicate-flood branch. No I/O.
func Evaluate(in CheckInput) CheckResult {
	next := in.Prev

	// Duplicate-flood — independent of the state comparison (state is correct during a flood).
	if in.FloodThreshold > 0 && in.MaxIdenticalRun >= in.FloodThreshold {
		if in.Prev.InCooldown(in.Now, in.Cooldown) {
			return CheckResult{Escalate: false, Reason: "duplicate_flood (cooldown)", Next: next}
		}
		next.ConsecutiveMismatches = 0
		next.LastEscalationAt = in.Now
		next.LastReason = fmt.Sprintf("duplicate_flood: %d identical posts", in.MaxIdenticalRun)
		return CheckResult{Escalate: true, Reason: next.LastReason, Next: next}
	}

	// Monitor off by design -> never a fault.
	if in.Zone == ZoneOutOfWindow {
		next.ConsecutiveMismatches = 0
		return CheckResult{Escalate: false, Reason: "out_of_window", Next: next}
	}

	// DUW unreachable / contract changed.
	if !in.SchemaValid {
		next.ConsecutiveMismatches++
		if next.ConsecutiveMismatches >= in.DebounceK && !in.Prev.InCooldown(in.Now, in.Cooldown) {
			next.LastEscalationAt = in.Now
			next.LastReason = "duw_contract_drift"
			return CheckResult{Escalate: true, Reason: next.LastReason, Next: next}
		}
		return CheckResult{Escalate: false, Reason: "duw_contract_drift (debouncing)", Next: next}
	}

	if in.ObservedOK && in.Expected == in.Observed {
		next.ConsecutiveMismatches = 0
		return CheckResult{Escalate: false, Reason: "ok", Next: next}
	}

	// Mismatch. Padding band: boundary grace — neither accrue nor escalate.
	if in.Zone == ZonePadding {
		return CheckResult{Escalate: false, Reason: "padding_mismatch", Next: next}
	}

	next.ConsecutiveMismatches++
	if next.ConsecutiveMismatches >= in.DebounceK && !in.Prev.InCooldown(in.Now, in.Cooldown) {
		next.LastEscalationAt = in.Now
		next.LastReason = fmt.Sprintf("monitor_missed_transition: expected=%s observed=%s", in.Expected, observedLabel(in))
		return CheckResult{Escalate: true, Reason: next.LastReason, Next: next}
	}
	return CheckResult{Escalate: false, Reason: "mismatch (debouncing)", Next: next}
}

func observedLabel(in CheckInput) string {
	if !in.ObservedOK {
		return "none"
	}
	return string(in.Observed)
}
