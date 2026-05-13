package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/uladzk/duw-queue-monitor/internal/logger"
	"github.com/uladzk/duw-queue-monitor/internal/notifications"

	"github.com/go-telegram/bot"
)

func createMockTelegramNotifier(shouldFail bool) *notifications.TelegramNotifier {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if shouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, `{"ok":false,"description":"Server error"}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"ok":true,"result":{"message_id":1,"chat":{"id":1},"text":"ok"}}`)
	}))

	cfg := &notifications.TelegramConfig{
		BotToken:              "test-token",
		MaxRetryAttempts:      1,
		RetryDelayMs:          100,
		RequestTimeoutSeconds: 2,
	}

	b, _ := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	log := logger.NewLogger(&logger.Config{Level: "error"})
	return notifications.NewTelegramNotifier(cfg, b, log)
}

func TestFeedbackHandler_GetReplyPatterns_ReturnsCorrectPattern(t *testing.T) {
	// Arrange
	logger := logger.NewLogger(&logger.Config{Level: "error"})
	mockNotifier := createMockTelegramNotifier(false)
	adminChatID := "admin123"

	sut := NewFeedbackHandler(logger, mockNotifier, adminChatID)

	// Act
	patterns := sut.GetReplyPatterns()

	// Assert
	if len(patterns) != 1 || patterns[0] != feedbackReplyText {
		t.Errorf("Expected patterns [%s], got %v", feedbackReplyText, patterns)
	}
}

func TestFeedbackHandler_HandleReply_WhenCalled_ProcessesUserFeedbackCorrectly(t *testing.T) {
	// Arrange
	log := logger.NewLogger(&logger.Config{Level: "error"})

	var capturedAdminRequest *http.Request
	adminServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAdminRequest = r
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"ok":true,"result":{"message_id":1,"chat":{"id":1},"text":"ok"}}`)
	}))
	defer adminServer.Close()

	cfg := &notifications.TelegramConfig{
		BotToken:              "test-token",
		MaxRetryAttempts:      1,
		RetryDelayMs:          100,
		RequestTimeoutSeconds: 2,
	}

	b, _ := bot.New("test-token", bot.WithServerURL(adminServer.URL), bot.WithSkipGetMe())
	mockNotifier := notifications.NewTelegramNotifier(cfg, b, log)

	adminChatID := "admin123"
	feedbackText := "This is user feedback about the bot"

	// Act
	adminMessage := fmt.Sprintf(feedbackAdminTemplate, feedbackText)
	err := mockNotifier.SendMessage(context.Background(), adminChatID, adminMessage)

	// Assert
	if err != nil {
		t.Errorf("Expected no error when sending admin notification, got: %v", err)
	}

	if capturedAdminRequest == nil {
		t.Error("Expected admin notification to be sent")
	}

	if capturedAdminRequest != nil && capturedAdminRequest.Method != "POST" {
		t.Errorf("Expected POST request for admin notification, got %s", capturedAdminRequest.Method)
	}

	expectedAdminMessage := "💬 <b>Nowa opinia od użytkownika</b>\n\n📝 Treść:\nThis is user feedback about the bot"
	actualAdminMessage := fmt.Sprintf(feedbackAdminTemplate, feedbackText)
	if actualAdminMessage != expectedAdminMessage {
		t.Errorf("Expected admin message:\n%s\nGot:\n%s", expectedAdminMessage, actualAdminMessage)
	}
}

func TestFeedbackHandler_HandleReply_WhenAdminNotificationFails_HandlesError(t *testing.T) {
	// Arrange
	mockNotifier := createMockTelegramNotifier(true) // This will fail
	adminChatID := "admin123"

	feedbackText := "This is user feedback"
	adminMessage := fmt.Sprintf(feedbackAdminTemplate, feedbackText)

	// Act
	err := mockNotifier.SendMessage(context.Background(), adminChatID, adminMessage)

	// Assert
	if err == nil {
		t.Error("Expected error when admin notification fails")
	}
}

func TestFeedbackHandler_HandleUpdate_MessageFormat_VerifyCorrectTextsAndFormats(t *testing.T) {
	testCases := []struct {
		name           string
		expectedText   string
		actualConstant string
	}{
		{
			"Info message text",
			"Możesz wysłać swoją opinię na temat działania bota. Twoja wiadomość będzie anonimowa i nie będzie publikowana.",
			feedbackInfoText,
		},
		{
			"Reply prompt text",
			"Aby wysłać opinię, proszę odpowiedz na tę wiadomość swoją opinią:",
			feedbackReplyText,
		},
		{
			"Thank you text",
			"Dziękujemy za Twoją opinię! Twoja wiadomość została wysłana do nas.",
			thankYouText,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.actualConstant != tc.expectedText {
				t.Errorf("Expected text: %s, got: %s", tc.expectedText, tc.actualConstant)
			}
		})
	}
}

func TestFeedbackHandler_AdminMessageTemplate_FormatsCorrectly(t *testing.T) {
	// Arrange
	testCases := []struct {
		name            string
		feedbackText    string
		expectedMessage string
	}{
		{
			"Simple feedback",
			"Great bot!",
			"💬 <b>Nowa opinia od użytkownika</b>\n\n📝 Treść:\nGreat bot!",
		},
		{
			"Multi-line feedback",
			"This is line 1\nThis is line 2",
			"💬 <b>Nowa opinia od użytkownika</b>\n\n📝 Treść:\nThis is line 1\nThis is line 2",
		},
		{
			"Feedback with special characters",
			"Bot works well! 👍 <good>",
			"💬 <b>Nowa opinia od użytkownika</b>\n\n📝 Treść:\nBot works well! 👍 <good>",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			actualMessage := fmt.Sprintf(feedbackAdminTemplate, tc.feedbackText)

			// Assert
			if actualMessage != tc.expectedMessage {
				t.Errorf("Expected admin message:\n%s\nGot:\n%s", tc.expectedMessage, actualMessage)
			}
		})
	}
}
