package duwdoctor

import (
	"testing"
	"time"
)

func TestZoneAt(t *testing.T) {
	// Arrange
	const start, end, pad = 5, 17, 30
	mk := func(s string) time.Time {
		ts, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatalf("bad time %q: %v", s, err)
		}
		return ts
	}
	cases := []struct {
		name string
		now  string
		want Zone
	}{
		{"saturday", "2026-06-13T10:00:00Z", ZoneOutOfWindow},
		{"sunday", "2026-06-14T10:00:00Z", ZoneOutOfWindow},
		{"mon-0420-before-pad", "2026-06-15T04:20:00Z", ZoneOutOfWindow},
		{"mon-0440-pad", "2026-06-15T04:40:00Z", ZonePadding},
		{"mon-0500-in", "2026-06-15T05:00:00Z", ZoneInWindow},
		{"mon-1659-in", "2026-06-15T16:59:00Z", ZoneInWindow},
		{"mon-1715-pad", "2026-06-15T17:15:00Z", ZonePadding},
		{"mon-1740-out", "2026-06-15T17:40:00Z", ZoneOutOfWindow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := ZoneAt(mk(tc.now), start, end, pad)
			// Assert
			if got != tc.want {
				t.Fatalf("ZoneAt(%s) = %v, want %v", tc.now, got, tc.want)
			}
		})
	}
}
