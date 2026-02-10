package statsreporting

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/UladzK/duw-queue-monitor/internal/dailystats"
	"github.com/UladzK/duw-queue-monitor/internal/logger"
)

type mockStatsReader struct {
	result        []dailystats.QueueDailyStat
	shouldFail    bool
	lastQueueID   int
	lastStartDate time.Time
	lastEndDate   time.Time
}

func (m *mockStatsReader) GetByDateRange(ctx context.Context, queueID int, startDate, endDate time.Time) ([]dailystats.QueueDailyStat, error) {
	m.lastQueueID = queueID
	m.lastStartDate = startDate
	m.lastEndDate = endDate
	if m.shouldFail {
		return nil, fmt.Errorf("mock stats reader failed")
	}
	return m.result, nil
}

type mockMessageSender struct {
	sendMessageCalled bool
	lastSentChatID    string
	lastSentMessage   string
	shouldFail        bool
}

func (m *mockMessageSender) SendMessage(ctx context.Context, chatID, text string) error {
	m.sendMessageCalled = true
	m.lastSentChatID = chatID
	m.lastSentMessage = text
	if m.shouldFail {
		return fmt.Errorf("mock message sender failed")
	}
	return nil
}

type mockDateTimeProvider struct {
	fixedTime time.Time
}

func (m *mockDateTimeProvider) Now() time.Time {
	return m.fixedTime
}

func newTestReporter(statsReader StatsReader, sender MessageSender, timeProvider DateTimeProvider) *Reporter {
	cfg := &StatsReportingConfig{
		QueueID:     24,
		QueueName:   "Odbiór karty pobytu",
		Timezone:    "Europe/Warsaw",
		ChannelName: "test-channel",
	}
	log := logger.NewLogger(&logger.Config{Level: "error"})
	return NewReporter(cfg, log, statsReader, sender, timeProvider)
}

func TestSendReport_WhenDailyPeriod_SendsCorrectMessage(t *testing.T) {
	// Arrange
	// "Now" is Wednesday 2025-06-18 10:00 Warsaw → today is 2025-06-18
	testConditions := []struct {
		name            string
		stats           []dailystats.QueueDailyStat
		expectedMessage string
	}{
		{
			"Condition 1: \"report date has stats data.\" Expected: \"daily report with tickets served and issued should be sent.\"",
			[]dailystats.QueueDailyStat{
				{QueueID: 24, Date: time.Date(2025, 6, 18, 0, 0, 0, 0, time.UTC), TicketsServed: 42, RegisteredTickets: 50},
			},
			"📊 Kolejka <b>Odbiór karty pobytu</b> — podsumowanie dnia:\nObsłużonych klientów: <b>42</b>\nWydanych biletów: <b>50</b>",
		},
		{
			"Condition 2: \"report date has no stats data.\" Expected: \"no data daily report should be sent.\"",
			[]dailystats.QueueDailyStat{},
			"📊 Kolejka <b>Odbiór karty pobytu</b> — podsumowanie dnia:\nBrak danych za ten dzień.",
		},
	}

	for _, tc := range testConditions {
		t.Run(tc.name, func(t *testing.T) {
			statsReader := &mockStatsReader{result: tc.stats}
			sender := &mockMessageSender{}
			timeProvider := &mockDateTimeProvider{fixedTime: time.Date(2025, 6, 18, 10, 0, 0, 0, time.UTC)}
			sut := newTestReporter(statsReader, sender, timeProvider)

			// Act
			err := sut.SendReport(context.Background(), "daily")

			// Assert
			if err != nil {
				t.Fatalf("Expected successful execution, but execution returned error: %v", err)
			}

			if !sender.sendMessageCalled {
				t.Error("Expected SendMessage to be called, but it wasn't")
			}

			if sender.lastSentChatID != "@test-channel" {
				t.Errorf("Expected chat ID '@test-channel', got '%s'", sender.lastSentChatID)
			}

			if sender.lastSentMessage != tc.expectedMessage {
				t.Errorf("Expected message:\n'%s'\nbut got:\n'%s'", tc.expectedMessage, sender.lastSentMessage)
			}

			expectedDate := time.Date(2025, 6, 18, 0, 0, 0, 0, time.UTC)
			if statsReader.lastStartDate != expectedDate {
				t.Errorf("Expected start date %v, got %v", expectedDate, statsReader.lastStartDate)
			}
		})
	}
}

func TestSendReport_WhenWeeklyPeriod_SendsCorrectMessage(t *testing.T) {
	// Arrange
	// "Now" is Wednesday 2025-06-18 10:00 Warsaw → current week is Mon 2025-06-16 to Wed 2025-06-18
	testConditions := []struct {
		name            string
		stats           []dailystats.QueueDailyStat
		expectedMessage string
	}{
		{
			"Condition 1: \"current week has stats data.\" Expected: \"weekly report with per-day breakdown and totals should be sent.\"",
			[]dailystats.QueueDailyStat{
				{QueueID: 24, Date: time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC), TicketsServed: 40, RegisteredTickets: 45},
				{QueueID: 24, Date: time.Date(2025, 6, 17, 0, 0, 0, 0, time.UTC), TicketsServed: 38, RegisteredTickets: 42},
				{QueueID: 24, Date: time.Date(2025, 6, 18, 0, 0, 0, 0, time.UTC), TicketsServed: 35, RegisteredTickets: 40},
			},
			"📊 Kolejka <b>Odbiór karty pobytu</b> — podsumowanie tygodnia:\n" +
				"• Pon 16.06 — obsłużono <b>40</b>, wydano <b>45</b> biletów\n" +
				"• Wt 17.06 — obsłużono <b>38</b>, wydano <b>42</b> biletów\n" +
				"• Śr 18.06 — obsłużono <b>35</b>, wydano <b>40</b> biletów\n" +
				"Razem obsłużono: <b>113</b> klientów\nRazem wydano: <b>127</b> biletów",
		},
		{
			"Condition 2: \"current week has no stats data.\" Expected: \"no data weekly report should be sent.\"",
			[]dailystats.QueueDailyStat{},
			"📊 Kolejka <b>Odbiór karty pobytu</b> — podsumowanie tygodnia:\nBrak danych za ten tydzień.",
		},
	}

	for _, tc := range testConditions {
		t.Run(tc.name, func(t *testing.T) {
			statsReader := &mockStatsReader{result: tc.stats}
			sender := &mockMessageSender{}
			timeProvider := &mockDateTimeProvider{fixedTime: time.Date(2025, 6, 18, 10, 0, 0, 0, time.UTC)}
			sut := newTestReporter(statsReader, sender, timeProvider)

			// Act
			err := sut.SendReport(context.Background(), "weekly")

			// Assert
			if err != nil {
				t.Fatalf("Expected successful execution, but execution returned error: %v", err)
			}

			if !sender.sendMessageCalled {
				t.Error("Expected SendMessage to be called, but it wasn't")
			}

			if sender.lastSentMessage != tc.expectedMessage {
				t.Errorf("Expected message:\n'%s'\nbut got:\n'%s'", tc.expectedMessage, sender.lastSentMessage)
			}

			expectedStart := time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC)
			expectedEnd := time.Date(2025, 6, 18, 0, 0, 0, 0, time.UTC)
			if statsReader.lastStartDate != expectedStart {
				t.Errorf("Expected start date %v, got %v", expectedStart, statsReader.lastStartDate)
			}
			if statsReader.lastEndDate != expectedEnd {
				t.Errorf("Expected end date %v, got %v", expectedEnd, statsReader.lastEndDate)
			}
		})
	}
}

func TestSendReport_WhenMonthlyPeriod_SendsCorrectMessage(t *testing.T) {
	// Arrange
	// "Now" is 2025-06-18 → current month is June 2025 (01–18)
	testConditions := []struct {
		name            string
		stats           []dailystats.QueueDailyStat
		expectedMessage string
	}{
		{
			"Condition 1: \"current month has stats data.\" Expected: \"monthly report with totals should be sent.\"",
			[]dailystats.QueueDailyStat{
				{QueueID: 24, Date: time.Date(2025, 6, 5, 0, 0, 0, 0, time.UTC), TicketsServed: 100, RegisteredTickets: 120},
				{QueueID: 24, Date: time.Date(2025, 6, 12, 0, 0, 0, 0, time.UTC), TicketsServed: 150, RegisteredTickets: 170},
			},
			"📊 Kolejka <b>Odbiór karty pobytu</b> — podsumowanie miesiąca:\nŁączna liczba obsłużonych klientów: <b>250</b>\nŁączna liczba wydanych biletów: <b>290</b>",
		},
		{
			"Condition 2: \"current month has no stats data.\" Expected: \"no data monthly report should be sent.\"",
			[]dailystats.QueueDailyStat{},
			"📊 Kolejka <b>Odbiór karty pobytu</b> — podsumowanie miesiąca:\nBrak danych za ten miesiąc.",
		},
	}

	for _, tc := range testConditions {
		t.Run(tc.name, func(t *testing.T) {
			statsReader := &mockStatsReader{result: tc.stats}
			sender := &mockMessageSender{}
			timeProvider := &mockDateTimeProvider{fixedTime: time.Date(2025, 6, 18, 10, 0, 0, 0, time.UTC)}
			sut := newTestReporter(statsReader, sender, timeProvider)

			// Act
			err := sut.SendReport(context.Background(), "monthly")

			// Assert
			if err != nil {
				t.Fatalf("Expected successful execution, but execution returned error: %v", err)
			}

			if !sender.sendMessageCalled {
				t.Error("Expected SendMessage to be called, but it wasn't")
			}

			if sender.lastSentMessage != tc.expectedMessage {
				t.Errorf("Expected message:\n'%s'\nbut got:\n'%s'", tc.expectedMessage, sender.lastSentMessage)
			}

			expectedStart := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
			expectedEnd := time.Date(2025, 6, 18, 0, 0, 0, 0, time.UTC)
			if statsReader.lastStartDate != expectedStart {
				t.Errorf("Expected start date %v, got %v", expectedStart, statsReader.lastStartDate)
			}
			if statsReader.lastEndDate != expectedEnd {
				t.Errorf("Expected end date %v, got %v", expectedEnd, statsReader.lastEndDate)
			}
		})
	}
}

func TestSendReport_WhenInvalidPeriod_ReturnsError(t *testing.T) {
	// Arrange
	statsReader := &mockStatsReader{}
	sender := &mockMessageSender{}
	timeProvider := &mockDateTimeProvider{fixedTime: time.Date(2025, 6, 18, 10, 0, 0, 0, time.UTC)}
	sut := newTestReporter(statsReader, sender, timeProvider)

	// Act
	err := sut.SendReport(context.Background(), "yearly")

	// Assert
	if err == nil {
		t.Fatal("Expected error for invalid period, but got nil")
	}

	if sender.sendMessageCalled {
		t.Error("Expected SendMessage NOT to be called for invalid period, but it was")
	}
}

func TestSendReport_WhenStatsReaderFails_ReturnsError(t *testing.T) {
	// Arrange
	statsReader := &mockStatsReader{shouldFail: true}
	sender := &mockMessageSender{}
	timeProvider := &mockDateTimeProvider{fixedTime: time.Date(2025, 6, 18, 10, 0, 0, 0, time.UTC)}
	sut := newTestReporter(statsReader, sender, timeProvider)

	// Act
	err := sut.SendReport(context.Background(), "daily")

	// Assert
	if err == nil {
		t.Fatal("Expected error when stats reader fails, but got nil")
	}

	if sender.sendMessageCalled {
		t.Error("Expected SendMessage NOT to be called when stats reader fails, but it was")
	}
}

func TestSendReport_WhenMessageSenderFails_ReturnsError(t *testing.T) {
	// Arrange
	statsReader := &mockStatsReader{result: []dailystats.QueueDailyStat{
		{QueueID: 24, Date: time.Date(2025, 6, 18, 0, 0, 0, 0, time.UTC), TicketsServed: 42, RegisteredTickets: 50},
	}}
	sender := &mockMessageSender{shouldFail: true}
	timeProvider := &mockDateTimeProvider{fixedTime: time.Date(2025, 6, 18, 10, 0, 0, 0, time.UTC)}
	sut := newTestReporter(statsReader, sender, timeProvider)

	// Act
	err := sut.SendReport(context.Background(), "daily")

	// Assert
	if err == nil {
		t.Fatal("Expected error when message sender fails, but got nil")
	}

	if !sender.sendMessageCalled {
		t.Error("Expected SendMessage to be called, but it wasn't")
	}
}
