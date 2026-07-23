package duwdoctor

import (
	"testing"
	"time"
)

func TestDoctorState_RoundTrip(t *testing.T) {
	// Arrange
	now := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	in := DoctorState{ConsecutiveMismatches: 2, LastEscalationAt: now, LastReason: "monitor_missed_transition"}
	// Act
	raw, err := in.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	out, err := ParseState(raw)
	if err != nil {
		t.Fatalf("ParseState: %v", err)
	}
	// Assert
	if out.ConsecutiveMismatches != 2 || out.LastReason != "monitor_missed_transition" || !out.LastEscalationAt.Equal(now) {
		t.Fatalf("round-trip mismatch: %+v", out)
	}
}

func TestParseState_Empty(t *testing.T) {
	for _, raw := range []string{"", "{}"} {
		s, err := ParseState(raw)
		if err != nil {
			t.Fatalf("ParseState(%q): %v", raw, err)
		}
		if s.ConsecutiveMismatches != 0 || !s.LastEscalationAt.IsZero() {
			t.Fatalf("ParseState(%q) = %+v, want zero", raw, s)
		}
	}
}

func TestInCooldown(t *testing.T) {
	now := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	cd := 60 * time.Minute
	if (DoctorState{}).InCooldown(now, cd) {
		t.Fatal("zero LastEscalationAt must not be in cooldown")
	}
	within := DoctorState{LastEscalationAt: now.Add(-30 * time.Minute)}
	if !within.InCooldown(now, cd) {
		t.Fatal("30m < 60m must be in cooldown")
	}
	after := DoctorState{LastEscalationAt: now.Add(-90 * time.Minute)}
	if after.InCooldown(now, cd) {
		t.Fatal("90m > 60m must not be in cooldown")
	}
}
