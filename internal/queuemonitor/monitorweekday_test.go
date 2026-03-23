package queuemonitor

import (
	"context"
	"testing"
	"time"
	"github.com/uladzk/duw-queue-monitor/internal/logger"
)

type MockTimeProvider struct {
	time string
}

type MockedQueueMonitor struct {
	statusCheckCalled bool
	initCalled        bool
	getStateCalled    bool
}

func (m *MockedQueueMonitor) Init(initState *MonitorState) {
	m.initCalled = true
}

func (m *MockedQueueMonitor) GetState() *MonitorState {
	m.getStateCalled = true
	return &MonitorState{QueueActive: true} // Return a dummy state for testing
}

func (m *MockedQueueMonitor) CheckAndProcessStatus(ctx context.Context) error {
	m.statusCheckCalled = true
	return nil
}

func NewMockTimeProvider(time string) *MockTimeProvider {
	return &MockTimeProvider{
		time: time,
	}
}

func (m *MockTimeProvider) Now() time.Time {
	t, _ := time.Parse(time.RFC3339, m.time)
	return t
}

func TestCheckAndProcessStatus_Always_RunsCheckDependingOnCurrentDateTime(t *testing.T) {
	// Arrange
	tests := []struct {
		name     string
		time     *MockTimeProvider
		expected bool
	}{
		{
			name:     "Saturday 10:00 in Poland",
			time:     NewMockTimeProvider("2025-04-05T10:00:00+02:00"),
			expected: false,
		},
		{
			name:     "Sunday 10:00 in Poland",
			time:     NewMockTimeProvider("2025-04-06T10:00:00+02:00"),
			expected: false,
		},
		{
			name:     "Wednesday 05:59 in Poland (03:59 UTC)",
			time:     NewMockTimeProvider("2025-04-02T05:59:00+02:00"),
			expected: false,
		},
		{
			name:     "Wednesday 06:59 in Poland (04:59 UTC)",
			time:     NewMockTimeProvider("2025-04-02T06:59:00+02:00"),
			expected: false,
		},
		{
			name:     "Wednesday 07:00 in Poland (05:00 UTC)",
			time:     NewMockTimeProvider("2025-04-02T07:00:00+02:00"),
			expected: true,
		},
		{
			name:     "Monday 08:00 in Poland",
			time:     NewMockTimeProvider("2025-04-07T08:00:00+02:00"),
			expected: true,
		},
		{
			name:     "Friday 17:59 in Poland",
			time:     NewMockTimeProvider("2025-04-04T17:59:00+02:00"),
			expected: true,
		},
		{
			name:     "Monday 20:30 in Poland",
			time:     NewMockTimeProvider("2025-04-07T20:30:00+02:00"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mm := &MockedQueueMonitor{}
			cfg := &Config{WorkingHourStartUTC: 5, WorkingHourEndUTC: 18}
			logger := logger.NewLogger(&logger.Config{Level: "error"})
			wm := NewWeekdayQueueMonitor(mm, cfg, tt.time, logger)

			// Act
			_ = wm.CheckAndProcessStatus(context.Background())

			// Assert
			if mm.statusCheckCalled != tt.expected {
				t.Errorf("Status check called: expected %v, got %v", tt.expected, mm.statusCheckCalled)
			}
		})
	}
}
