package queuemonitor

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/uladzk/duw-queue-monitor/internal/logger"

	"github.com/google/go-cmp/cmp"
)

type mockNotifier struct {
	shouldFail        bool
	called            bool
	lastSentStatus    *Queue
	sendMessageCalled bool
	lastSentChatID    string
	lastSentMessage   string
}

func (f *mockNotifier) SendGeneralQueueStatusUpdateNotification(broadcastChannelName, queueName string, active bool, enabled bool, actualTicket string, numberOfTicketsLeft int) error {
	f.called = true

	if f.shouldFail {
		return fmt.Errorf("failed to send notification")
	}

	f.lastSentStatus = &Queue{
		Name:        queueName,
		Active:      active,
		Enabled:     enabled,
		TicketValue: actualTicket,
		TicketsLeft: numberOfTicketsLeft,
	}
	return nil
}

func (f *mockNotifier) SendMessage(ctx context.Context, chatID, text string) error {
	f.sendMessageCalled = true
	f.lastSentChatID = chatID
	f.lastSentMessage = text

	if f.shouldFail {
		return fmt.Errorf("failed to send message")
	}

	return nil
}

type mockDailyStatsRepo struct {
	saveCalled                bool
	lastQueueID               int
	lastQueueName             string
	lastDate                  time.Time
	lastTotalTicketsAvailable int
	lastTakenTickets          int
	shouldFail                bool
}

func (m *mockDailyStatsRepo) SaveDailyStats(ctx context.Context, queueID int, queueName string, date time.Time, totalTicketsAvailable int, takenTickets int) error {
	m.saveCalled = true
	m.lastQueueID = queueID
	m.lastQueueName = queueName
	m.lastDate = date
	m.lastTotalTicketsAvailable = totalTicketsAvailable
	m.lastTakenTickets = takenTickets
	if m.shouldFail {
		return fmt.Errorf("mock stats save failed")
	}
	return nil
}

type mockDateTimeProvider struct {
	fixedTime time.Time
}

func (m *mockDateTimeProvider) Now() time.Time {
	return m.fixedTime
}

func deriveStateName(active, enabled bool) string {
	if !active {
		return "Inactive"
	}
	if enabled {
		return "ActiveEnabled"
	}
	return "ActiveDisabled"
}

func TestCheckAndProcessStatus_WhenStateIsNotInitialized_CorrectlyHandlesStateTransition(t *testing.T) {
	// Arrange
	const queueName = "Odbior karty"
	testConditions := []struct {
		name                     string
		newState                 Queue
		notificationShouldBeSent bool
		expectedNotification     *Queue
	}{
		{
			"Condition 1: \"queue is not active, state was not initialized, queue becomes active.\" Expected: \"notification should not be sent.\"",
			Queue{Name: queueName, Active: false, Enabled: false, TicketValue: "", TicketsLeft: 0},
			false,
			nil,
		},
		{
			"Condition 2: \"queue is active, state was not initialized.\" Expected: \"notification should be sent.\"",
			Queue{Name: queueName, Active: true, Enabled: true, TicketValue: "K123", TicketsLeft: 10},
			true,
			&Queue{Name: queueName, Active: true, Enabled: true, TicketValue: "K123", TicketsLeft: 10},
		},
	}

	for _, tc := range testConditions {
		t.Run(tc.name, func(t *testing.T) {

			mockDuwApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{
					"result": {
						"Wrocław": [{
							"id": 24,
							"name": "%v",
							"ticket_value": "%v",
							"tickets_left": %v,
							"active": %v,
							"enabled": %v
						}]
					}
				}`, queueName,
					tc.newState.TicketValue,
					tc.newState.TicketsLeft,
					tc.newState.Active,
					tc.newState.Enabled)
			}))
			defer mockDuwApi.Close()

			cfg := &Config{
				QueueMonitor: QueueMonitorConfig{
					StatusApiUrl:              mockDuwApi.URL,
					StatusCheckTimeoutMs:      4000,
					StatusCheckMaxAttempts:    3,
					StatusCheckAttemptDelayMs: 500,
					StatusMonitoredQueueId:    24,
					StatusMonitoredQueueCity:  "Wrocław",
				},
			}

			logger := logger.NewLogger(&logger.Config{
				Level: "error"})
			collector := NewStatusCollector(&cfg.QueueMonitor, &http.Client{}, logger)

			notifier := &mockNotifier{}

			sut := NewQueueMonitor(cfg, logger, collector, notifier, nil, &mockDateTimeProvider{fixedTime: time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)})
			expectedFinalState := &MonitorState{
				StateName:           deriveStateName(tc.newState.Active, tc.newState.Enabled),
				QueueActive:         tc.newState.Active,
				QueueEnabled:        tc.newState.Enabled,
				LastTicketProcessed: tc.newState.TicketValue,
				TicketsLeft:         tc.newState.TicketsLeft,
			}

			// Act
			err := sut.CheckAndProcessStatus(context.Background())

			// Assert
			if err != nil {
				t.Fatalf("Expected successful execution, but execution returned error: %v", err)
			}

			if notifier.sendMessageCalled != tc.notificationShouldBeSent {
				t.Errorf("Expected notification sending: %v, but it was: %v", tc.notificationShouldBeSent, notifier.sendMessageCalled)
			}

			if stateDiff := cmp.Diff(sut.GetState(), expectedFinalState); stateDiff != "" {
				t.Errorf("State mismatch between currently set state of monitor and latest state (-want +got):\n%s", stateDiff)
			}
		})
	}
}

func TestCheckAndProcessStatus_WhenStateIsInitialized_CorrectlyHandlesStrateTransition(t *testing.T) {
	// Arrange
	const queueName = "Odbior karty"
	testConditions := []struct {
		name                     string
		isStateInitialized       bool
		initialState             MonitorState
		newState                 Queue
		notificationShouldBeSent bool
		expectedNotification     *Queue
	}{
		{
			"Condition 1: \"queue was active, state was initialized, no changes.\" Expected: \"notification shoud NOT be sent.\"",
			true,
			MonitorState{QueueActive: true, QueueEnabled: true, TicketsLeft: 10, LastTicketProcessed: "K123"},
			Queue{Name: queueName, Active: true, Enabled: true, TicketValue: "K123", TicketsLeft: 10},
			false,
			nil,
		},
		{
			"Condition 2: \"queue was active and disabled, state was initialized, queue becomes not active.\" Expected: \"notification should be sent.\"",
			true,
			MonitorState{StateName: "ActiveDisabled", QueueActive: true, QueueEnabled: false, TicketsLeft: 0, LastTicketProcessed: "K123"},
			Queue{Name: queueName, Active: false, Enabled: false, TicketValue: "K123", TicketsLeft: 0},
			true,
			&Queue{Name: queueName, Active: false, Enabled: false, TicketValue: "K123", TicketsLeft: 0},
		},
		{
			"Condition 3: \"queue was active, state was initialized, queue remains active, status becomes not enabled.\" Expected: \"notification should be sent.\"",
			true,
			MonitorState{QueueActive: true, QueueEnabled: true, TicketsLeft: 10, LastTicketProcessed: "K123"},
			Queue{Name: queueName, Active: true, Enabled: false, TicketValue: "K123", TicketsLeft: 0},
			true,
			&Queue{Name: queueName, Active: true, Enabled: false, TicketValue: "K123", TicketsLeft: 0},
		},
		{
			"Condition 4: \"queue was active, state was initialized, queue remains active and enabled, ticket left changed.\" Expected: \"notification should be sent.\"",
			true,
			MonitorState{QueueActive: true, QueueEnabled: true, TicketsLeft: 10, LastTicketProcessed: "K123"},
			Queue{Name: queueName, Active: true, Enabled: true, TicketValue: "K123", TicketsLeft: 5},
			true,
			&Queue{Name: queueName, Active: true, Enabled: true, TicketValue: "K123", TicketsLeft: 5},
		},
		{
			"Condition 5: \"queue was active, state was initialized, queue remains active and enabled, only ticket value changed.\" Expected: \"notification should NOT be sent.\"",
			true,
			MonitorState{QueueActive: true, QueueEnabled: true, TicketsLeft: 10, LastTicketProcessed: "K123"},
			Queue{Name: queueName, Active: true, Enabled: true, TicketValue: "K456", TicketsLeft: 10},
			false,
			nil,
		},
		{
			"Condition 6: \"queue was active, state was initialized, queue remains active and enabled, ticket value is empty and not changed.\" Expected: \"notification should NOT be sent.\"",
			true,
			MonitorState{QueueActive: true, QueueEnabled: true, TicketsLeft: 100, LastTicketProcessed: ""},
			Queue{Name: queueName, Active: true, Enabled: true, TicketsLeft: 100, TicketValue: ""},
			false,
			nil,
		},
		{
			"Condition 7: \"queue was active and enabled, state was initialized, queue becomes inactive.\" Expected: \"notification should be sent.\"",
			true,
			MonitorState{StateName: "ActiveEnabled", QueueActive: true, QueueEnabled: true, TicketsLeft: 10, LastTicketProcessed: "K123"},
			Queue{Name: queueName, Active: false, Enabled: false, TicketValue: "", TicketsLeft: 0},
			true,
			&Queue{Name: queueName, Active: false, Enabled: false, TicketValue: "", TicketsLeft: 0},
		},
	}

	for _, tc := range testConditions {
		t.Run(tc.name, func(t *testing.T) {

			mockDuwApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{
					"result": {
						"Wrocław": [{
							"id": 24,
							"name": "%v",
							"ticket_value": "%v",
							"tickets_left": %v,
							"active": %v,
							"enabled": %v
						}]
					}
				}`, queueName,
					tc.newState.TicketValue,
					tc.newState.TicketsLeft,
					tc.newState.Active,
					tc.newState.Enabled)
			}))
			defer mockDuwApi.Close()

			cfg := &Config{
				QueueMonitor: QueueMonitorConfig{
					StatusApiUrl:              mockDuwApi.URL,
					StatusCheckTimeoutMs:      4000,
					StatusCheckMaxAttempts:    3,
					StatusCheckAttemptDelayMs: 500,
					StatusMonitoredQueueId:    24,
					StatusMonitoredQueueCity:  "Wrocław",
				},
			}

			logger := logger.NewLogger(&logger.Config{Level: "error"})
			collector := NewStatusCollector(&cfg.QueueMonitor, &http.Client{}, logger)

			notifier := &mockNotifier{}

			sut := NewQueueMonitor(cfg, logger, collector, notifier, nil, &mockDateTimeProvider{fixedTime: time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)})
			sut.Init(&tc.initialState)
			expectedFinalState := &MonitorState{
				StateName:           deriveStateName(tc.newState.Active, tc.newState.Enabled),
				QueueActive:         tc.newState.Active,
				QueueEnabled:        tc.newState.Enabled,
				TicketsLeft:         tc.newState.TicketsLeft,
				LastTicketProcessed: tc.newState.TicketValue,
			}

			// Act
			err := sut.CheckAndProcessStatus(context.Background())

			// Assert
			if err != nil {
				t.Fatalf("Expected successful execution, but execution returned error: %v", err)
			}

			if notifier.sendMessageCalled != tc.notificationShouldBeSent {
				t.Errorf("Expected notification sending: %v, but it was: %v", tc.notificationShouldBeSent, notifier.sendMessageCalled)
			}

			if diffState := cmp.Diff(sut.GetState(), expectedFinalState); diffState != "" {
				t.Errorf("State mismatch between currently set state of monitor and latest state (-want +got):\n%s", diffState)
			}
		})
	}
}

func TestCheckAndProcessStatus_WhenCollectingQueueStatusFailed_DoesNotPushNotificationAndReturnsError(t *testing.T) {
	// Arrange
	mockDuwApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))

	defer mockDuwApi.Close()

	cfg := &Config{
		QueueMonitor: QueueMonitorConfig{
			StatusApiUrl:              mockDuwApi.URL,
			StatusCheckTimeoutMs:      4000,
			StatusCheckMaxAttempts:    3,
			StatusCheckAttemptDelayMs: 500,
			StatusMonitoredQueueId:    24,
			StatusMonitoredQueueCity:  "Wrocław",
		},
	}

	logger := logger.NewLogger(&logger.Config{
		Level: "error"})
	collector := NewStatusCollector(&cfg.QueueMonitor, &http.Client{}, logger)

	notifier := &mockNotifier{shouldFail: true}

	sut := NewQueueMonitor(cfg, logger, collector, notifier, nil, &mockDateTimeProvider{fixedTime: time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)})
	sut.Init(&MonitorState{
		QueueActive:         true,
		QueueEnabled:        true,
		TicketsLeft:         10,
		LastTicketProcessed: "K123",
	})

	// Act
	err := sut.CheckAndProcessStatus(context.Background())

	// Assert
	if err == nil {
		t.Fatal("Expected error to be returned, but there is no one.", err)
	}

	if notifier.sendMessageCalled {
		t.Errorf("Expected no notification to be sent, but there was one %+v", notifier.lastSentStatus)
	}
}

func TestCheckAndProcessStatus_WhenPushNotificationFailed_ReturnsError(t *testing.T) {
	// Arrange
	mockDuwApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{
			"result": {
				"Wrocław": [{
					"id": 24,
					"name": "Odbior karty",
					"ticket_value": "K123",
					"tickets_left": 0,
					"active": true,
					"enabled": true
				}]
			}
		}`)
	}))
	defer mockDuwApi.Close()

	cfg := &Config{
		QueueMonitor: QueueMonitorConfig{
			StatusApiUrl:              mockDuwApi.URL,
			StatusCheckTimeoutMs:      4000,
			StatusCheckMaxAttempts:    3,
			StatusCheckAttemptDelayMs: 500,
			StatusMonitoredQueueId:    24,
			StatusMonitoredQueueCity:  "Wrocław",
		},
	}

	logger := logger.NewLogger(&logger.Config{
		Level: "error"})
	collector := NewStatusCollector(&cfg.QueueMonitor, &http.Client{}, logger)

	notifier := &mockNotifier{shouldFail: true}

	sut := NewQueueMonitor(cfg, logger, collector, notifier, nil, &mockDateTimeProvider{fixedTime: time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)})
	sut.Init(&MonitorState{
		QueueActive:  true,
		QueueEnabled: true,
		TicketsLeft:  10,
	})

	// Act
	err := sut.CheckAndProcessStatus(context.Background())

	// Assert
	if err == nil {
		t.Fatal("Expected error to be returned, but there is no one.", err)
	}

	if !notifier.sendMessageCalled {
		t.Error("Expected notification to be sent, but it wasn't")
	}
}

func TestCheckAndProcessStatus_WhenApiReturnsNegativeTicketsLeft_ClampsToZeroAndProceeds(t *testing.T) {
	// Arrange
	mockDuwApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{
			"result": {
				"Wrocław": [{
					"id": 24,
					"name": "Odbior karty",
					"ticket_value": "K123",
					"tickets_left": -5,
					"active": true,
					"enabled": true
				}]
			}
		}`)
	}))
	defer mockDuwApi.Close()

	cfg := &Config{
		QueueMonitor: QueueMonitorConfig{
			StatusApiUrl:              mockDuwApi.URL,
			StatusCheckTimeoutMs:      4000,
			StatusCheckMaxAttempts:    1,
			StatusCheckAttemptDelayMs: 100,
			StatusMonitoredQueueId:    24,
			StatusMonitoredQueueCity:  "Wrocław",
		},
	}

	logger := logger.NewLogger(&logger.Config{Level: "error"})
	collector := NewStatusCollector(&cfg.QueueMonitor, &http.Client{}, logger)
	notifier := &mockNotifier{}

	sut := NewQueueMonitor(cfg, logger, collector, notifier, nil, &mockDateTimeProvider{fixedTime: time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)})

	// Act
	err := sut.CheckAndProcessStatus(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("Expected no error for negative TicketsLeft, but got: %v", err)
	}

	if got := sut.GetState().TicketsLeft; got != 0 {
		t.Errorf("Expected negative TicketsLeft to be clamped to 0, but got %d", got)
	}

	if strings.Contains(notifier.lastSentMessage, "-5") {
		t.Errorf("Expected clamped value in notification, but raw negative leaked: %q", notifier.lastSentMessage)
	}
}

func TestCheckAndProcessStatus_MessageFormat_CorrectlyFormatsMessages(t *testing.T) {
	// Arrange
	testConditions := []struct {
		name                string
		queueActive         bool
		queueEnabled        bool
		queueName           string
		actualTicket        string
		numberOfTicketsLeft int
		expectedMessage     string
		expectedChatID      string
		initialState        *MonitorState
	}{
		{
			"Available queue with ticket",
			true,
			true,
			"test-queue",
			"K80",
			10,
			"🔔 Kolejka <b>test-queue</b> jest teraz dostępna!\n🎟️ Ostatni przywołany bilet: <b>K80</b>\n🧾 Pozostało biletów: <b>10</b>",
			"@test-channel",
			nil,
		},
		{
			"Unavailable queue",
			true,
			false,
			"test-queue",
			"K80",
			10,
			"💤 Kolejka <b>test-queue</b> jest obecnie niedostępna (na razie nie ma wolnych biletów).",
			"@test-channel",
			nil,
		},
		{
			"Available queue without ticket",
			true,
			true,
			"Odbiór karty",
			"",
			5,
			"🔔 Kolejka <b>Odbiór karty</b> jest teraz dostępna!\n🧾 Pozostało biletów: <b>5</b>",
			"@test-channel",
			nil,
		},
		{
			"Inactive queue",
			false,
			false,
			"test-queue",
			"",
			0,
			"🌙 Kolejka <b>test-queue</b> jest nieaktywna — prawdopodobnie koniec godzin pracy DUW.",
			"@test-channel",
			&MonitorState{StateName: "ActiveEnabled", QueueActive: true, QueueEnabled: true, TicketsLeft: 10},
		},
	}

	for _, tc := range testConditions {
		t.Run(tc.name, func(t *testing.T) {
			mockDuwApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{
					"result": {
						"Wrocław": [{
							"id": 24,
							"name": "%v",
							"ticket_value": "%v",
							"tickets_left": %v,
							"active": %v,
							"enabled": %v
						}]
					}
				}`, tc.queueName,
					tc.actualTicket,
					tc.numberOfTicketsLeft,
					tc.queueActive,
					tc.queueEnabled)
			}))
			defer mockDuwApi.Close()

			cfg := &Config{
				BroadcastChannelName: "test-channel",
				QueueMonitor: QueueMonitorConfig{
					StatusApiUrl:              mockDuwApi.URL,
					StatusCheckTimeoutMs:      4000,
					StatusCheckMaxAttempts:    3,
					StatusCheckAttemptDelayMs: 500,
					StatusMonitoredQueueId:    24,
					StatusMonitoredQueueCity:  "Wrocław",
				},
			}

			logger := logger.NewLogger(&logger.Config{Level: "error"})
			collector := NewStatusCollector(&cfg.QueueMonitor, &http.Client{}, logger)
			notifier := &mockNotifier{}
			sut := NewQueueMonitor(cfg, logger, collector, notifier, nil, &mockDateTimeProvider{fixedTime: time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)})
			if tc.initialState != nil {
				sut.Init(tc.initialState)
			}

			// Act
			err := sut.CheckAndProcessStatus(context.Background())

			// Assert
			if err != nil {
				t.Fatalf("Expected successful execution, but execution returned error: %v", err)
			}

			if !notifier.sendMessageCalled {
				t.Error("Expected SendMessage to be called, but it wasn't")
			}

			if notifier.lastSentChatID != tc.expectedChatID {
				t.Errorf("Expected chat ID to be '%s', but got '%s'", tc.expectedChatID, notifier.lastSentChatID)
			}

			if notifier.lastSentMessage != tc.expectedMessage {
				t.Errorf("Expected message to be:\n'%s'\nbut got:\n'%s'", tc.expectedMessage, notifier.lastSentMessage)
			}
		})
	}
}

func TestCheckAndProcessStatus_WhenTransitionToInactiveWithStatsRepo_SavesDailyStats(t *testing.T) {
	// Arrange
	testConditions := []struct {
		name         string
		initialState MonitorState
	}{
		{
			"Condition 1: \"queue was active and enabled, queue becomes inactive, stats repo provided.\" Expected: \"daily stats should be saved.\"",
			MonitorState{StateName: "ActiveEnabled", QueueActive: true, QueueEnabled: true, TicketsLeft: 10, LastTicketProcessed: "K123"},
		},
		{
			"Condition 2: \"queue was active and disabled, queue becomes inactive, stats repo provided.\" Expected: \"daily stats should be saved.\"",
			MonitorState{StateName: "ActiveDisabled", QueueActive: true, QueueEnabled: false, TicketsLeft: 0, LastTicketProcessed: "K123"},
		},
	}

	for _, tc := range testConditions {
		t.Run(tc.name, func(t *testing.T) {
			const queueName = "Odbior karty"
			mockDuwApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{
					"result": {
						"Wrocław": [{
							"id": 24,
							"name": "%v",
							"ticket_value": "",
							"tickets_left": 0,
							"active": false,
							"enabled": false,
							"max_tickets": 180,
							"registered_tickets": 50
						}]
					}
				}`, queueName)
			}))
			defer mockDuwApi.Close()

			cfg := &Config{
				BroadcastChannelName: "test-channel",
				QueueMonitor: QueueMonitorConfig{
					StatusApiUrl:              mockDuwApi.URL,
					StatusCheckTimeoutMs:      4000,
					StatusCheckMaxAttempts:    3,
					StatusCheckAttemptDelayMs: 500,
					StatusMonitoredQueueId:    24,
					StatusMonitoredQueueCity:  "Wrocław",
				},
			}

			logger := logger.NewLogger(&logger.Config{Level: "error"})
			collector := NewStatusCollector(&cfg.QueueMonitor, &http.Client{}, logger)
			notifier := &mockNotifier{}
			statsRepo := &mockDailyStatsRepo{}
			sut := NewQueueMonitor(cfg, logger, collector, notifier, statsRepo, &mockDateTimeProvider{fixedTime: time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)})
			sut.Init(&tc.initialState)

			// Act
			err := sut.CheckAndProcessStatus(context.Background())

			// Assert
			if err != nil {
				t.Fatalf("Expected successful execution, but execution returned error: %v", err)
			}

			if !statsRepo.saveCalled {
				t.Error("Expected daily stats to be saved on transition to Inactive, but SaveDailyStats was not called")
			}

			if statsRepo.lastQueueID != 24 {
				t.Errorf("Expected queueID 24, got %d", statsRepo.lastQueueID)
			}

			if statsRepo.lastTotalTicketsAvailable != 180 {
				t.Errorf("Expected totalTicketsAvailable 180, got %d", statsRepo.lastTotalTicketsAvailable)
			}

			if statsRepo.lastTakenTickets != 50 {
				t.Errorf("Expected takenTickets 50, got %d", statsRepo.lastTakenTickets)
			}

			expectedDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
			if statsRepo.lastDate != expectedDate {
				t.Errorf("Expected date %v, got %v", expectedDate, statsRepo.lastDate)
			}
		})
	}
}

func TestCheckAndProcessStatus_WhenNoTransitionToInactive_DoesNotSaveDailyStats(t *testing.T) {
	// Arrange
	mockDuwApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{
			"result": {
				"Wrocław": [{
					"id": 24,
					"name": "Odbior karty",
					"ticket_value": "K123",
					"tickets_left": 5,
					"active": true,
					"enabled": true,
					"max_tickets": 180,
					"registered_tickets": 50
				}]
			}
		}`)
	}))
	defer mockDuwApi.Close()

	cfg := &Config{
		BroadcastChannelName: "test-channel",
		QueueMonitor: QueueMonitorConfig{
			StatusApiUrl:              mockDuwApi.URL,
			StatusCheckTimeoutMs:      4000,
			StatusCheckMaxAttempts:    3,
			StatusCheckAttemptDelayMs: 500,
			StatusMonitoredQueueId:    24,
			StatusMonitoredQueueCity:  "Wrocław",
		},
	}

	logger := logger.NewLogger(&logger.Config{Level: "error"})
	collector := NewStatusCollector(&cfg.QueueMonitor, &http.Client{}, logger)
	notifier := &mockNotifier{}
	statsRepo := &mockDailyStatsRepo{}
	sut := NewQueueMonitor(cfg, logger, collector, notifier, statsRepo, &mockDateTimeProvider{fixedTime: time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)})
	sut.Init(&MonitorState{StateName: "ActiveEnabled", QueueActive: true, QueueEnabled: true, TicketsLeft: 10})

	// Act
	err := sut.CheckAndProcessStatus(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("Expected successful execution, but execution returned error: %v", err)
	}

	if statsRepo.saveCalled {
		t.Error("Expected daily stats NOT to be saved when no transition to Inactive, but SaveDailyStats was called")
	}
}

func TestCheckAndProcessStatus_WhenStatsRepoIsNil_DoesNotPanicOnInactiveTransition(t *testing.T) {
	// Arrange
	mockDuwApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{
			"result": {
				"Wrocław": [{
					"id": 24,
					"name": "Odbior karty",
					"ticket_value": "",
					"tickets_left": 0,
					"active": false,
					"enabled": false
				}]
			}
		}`)
	}))
	defer mockDuwApi.Close()

	cfg := &Config{
		BroadcastChannelName: "test-channel",
		QueueMonitor: QueueMonitorConfig{
			StatusApiUrl:              mockDuwApi.URL,
			StatusCheckTimeoutMs:      4000,
			StatusCheckMaxAttempts:    3,
			StatusCheckAttemptDelayMs: 500,
			StatusMonitoredQueueId:    24,
			StatusMonitoredQueueCity:  "Wrocław",
		},
	}

	logger := logger.NewLogger(&logger.Config{Level: "error"})
	collector := NewStatusCollector(&cfg.QueueMonitor, &http.Client{}, logger)
	notifier := &mockNotifier{}
	sut := NewQueueMonitor(cfg, logger, collector, notifier, nil, &mockDateTimeProvider{fixedTime: time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)})
	sut.Init(&MonitorState{StateName: "ActiveEnabled", QueueActive: true, QueueEnabled: true, TicketsLeft: 10})

	// Act
	err := sut.CheckAndProcessStatus(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("Expected successful execution with nil statsRepo, but execution returned error: %v", err)
	}
}

func TestCheckAndProcessStatus_WhenStatsRepoFails_DoesNotReturnError(t *testing.T) {
	// Arrange
	mockDuwApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{
			"result": {
				"Wrocław": [{
					"id": 24,
					"name": "Odbior karty",
					"ticket_value": "",
					"tickets_left": 0,
					"active": false,
					"enabled": false,
					"max_tickets": 180,
					"registered_tickets": 50
				}]
			}
		}`)
	}))
	defer mockDuwApi.Close()

	cfg := &Config{
		BroadcastChannelName: "test-channel",
		QueueMonitor: QueueMonitorConfig{
			StatusApiUrl:              mockDuwApi.URL,
			StatusCheckTimeoutMs:      4000,
			StatusCheckMaxAttempts:    3,
			StatusCheckAttemptDelayMs: 500,
			StatusMonitoredQueueId:    24,
			StatusMonitoredQueueCity:  "Wrocław",
		},
	}

	logger := logger.NewLogger(&logger.Config{Level: "error"})
	collector := NewStatusCollector(&cfg.QueueMonitor, &http.Client{}, logger)
	notifier := &mockNotifier{}
	statsRepo := &mockDailyStatsRepo{shouldFail: true}
	sut := NewQueueMonitor(cfg, logger, collector, notifier, statsRepo, &mockDateTimeProvider{fixedTime: time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)})
	sut.Init(&MonitorState{StateName: "ActiveEnabled", QueueActive: true, QueueEnabled: true, TicketsLeft: 10})

	// Act
	err := sut.CheckAndProcessStatus(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("Expected successful execution even when statsRepo fails, but got error: %v", err)
	}

	if !statsRepo.saveCalled {
		t.Error("Expected SaveDailyStats to be called, but it wasn't")
	}
}
