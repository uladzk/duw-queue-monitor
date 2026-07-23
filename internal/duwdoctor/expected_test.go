package duwdoctor

import (
	"testing"

	"github.com/uladzk/duw-queue-monitor/internal/queuemonitor"
)

func TestExpectedFromQueue(t *testing.T) {
	// Arrange
	cases := []struct {
		name string
		q    queuemonitor.Queue
		want State
	}{
		{"inactive", queuemonitor.Queue{Active: false, Enabled: true}, StateInactive},
		{"active-disabled", queuemonitor.Queue{Active: true, Enabled: false}, StateActiveDisabled},
		{"active-enabled", queuemonitor.Queue{Active: true, Enabled: true}, StateActiveEnabled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Act / Assert
			if got := ExpectedFromQueue(tc.q); got != tc.want {
				t.Fatalf("ExpectedFromQueue(%+v) = %v, want %v", tc.q, got, tc.want)
			}
		})
	}
}
