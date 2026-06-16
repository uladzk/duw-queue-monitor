package duwdoctor

import (
	"testing"
	"time"
)

func TestEvaluate(t *testing.T) {
	now := time.Date(2026, 6, 8, 14, 0, 0, 0, time.UTC) // Monday, in-window
	const k = 3
	const cd = 60 * time.Minute
	base := func() CheckInput {
		return CheckInput{
			Now: now, Zone: ZoneInWindow, SchemaValid: true,
			Expected: StateActiveEnabled, Observed: StateActiveEnabled, ObservedOK: true,
			MaxIdenticalRun: 1, DebounceK: k, Cooldown: cd, FloodThreshold: 5,
		}
	}
	t.Run("out-of-window resets", func(t *testing.T) {
		in := base()
		in.Zone = ZoneOutOfWindow
		in.Observed = StateInactive // mismatch, but monitor is off
		in.Prev = DoctorState{ConsecutiveMismatches: 2}
		r := Evaluate(in)
		if r.Escalate || r.Next.ConsecutiveMismatches != 0 {
			t.Fatalf("out-of-window: %+v", r)
		}
	})
	t.Run("match resets", func(t *testing.T) {
		in := base()
		in.Prev = DoctorState{ConsecutiveMismatches: 2}
		r := Evaluate(in)
		if r.Escalate || r.Next.ConsecutiveMismatches != 0 {
			t.Fatalf("match: %+v", r)
		}
	})
	t.Run("06-08 freeze signature escalates at K", func(t *testing.T) {
		// DUW closed (Inactive) but channel still shows ActiveEnabled; K-1 already accrued.
		in := base()
		in.Expected, in.Observed = StateInactive, StateActiveEnabled
		in.Prev = DoctorState{ConsecutiveMismatches: k - 1}
		r := Evaluate(in)
		if !r.Escalate || r.Reason == "" || r.Next.LastEscalationAt.IsZero() {
			t.Fatalf("expected escalation, got %+v", r)
		}
	})
	t.Run("single blip below K does not escalate", func(t *testing.T) {
		in := base()
		in.Expected, in.Observed = StateInactive, StateActiveEnabled
		in.Prev = DoctorState{ConsecutiveMismatches: 0}
		r := Evaluate(in)
		if r.Escalate || r.Next.ConsecutiveMismatches != 1 {
			t.Fatalf("single blip: %+v", r)
		}
	})
	t.Run("padding mismatch does not accrue", func(t *testing.T) {
		in := base()
		in.Zone = ZonePadding
		in.Expected, in.Observed = StateActiveEnabled, StateInactive
		in.Prev = DoctorState{ConsecutiveMismatches: 1}
		r := Evaluate(in)
		if r.Escalate || r.Next.ConsecutiveMismatches != 1 {
			t.Fatalf("padding: %+v", r)
		}
	})
	t.Run("cooldown suppresses escalation", func(t *testing.T) {
		in := base()
		in.Expected, in.Observed = StateInactive, StateActiveEnabled
		in.Prev = DoctorState{ConsecutiveMismatches: k - 1, LastEscalationAt: now.Add(-10 * time.Minute)}
		r := Evaluate(in)
		if r.Escalate {
			t.Fatalf("cooldown should suppress, got %+v", r)
		}
	})
	t.Run("schema invalid debounces to contract drift", func(t *testing.T) {
		in := base()
		in.SchemaValid = false
		in.Prev = DoctorState{ConsecutiveMismatches: k - 1}
		r := Evaluate(in)
		if !r.Escalate || r.Reason != "duw_contract_drift" {
			t.Fatalf("contract drift: %+v", r)
		}
	})
	t.Run("flood escalates regardless of state match", func(t *testing.T) {
		in := base() // state matches
		in.MaxIdenticalRun = 22
		r := Evaluate(in)
		if !r.Escalate || r.Next.LastEscalationAt.IsZero() {
			t.Fatalf("flood: %+v", r)
		}
	})
	t.Run("flood in cooldown does not escalate", func(t *testing.T) {
		in := base()
		in.MaxIdenticalRun = 22
		in.Prev = DoctorState{LastEscalationAt: now.Add(-5 * time.Minute)}
		r := Evaluate(in)
		if r.Escalate {
			t.Fatalf("flood cooldown: %+v", r)
		}
	})
}
