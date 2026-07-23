package telegrambot

import (
	"fmt"
	"github.com/uladzk/duw-queue-monitor/internal/logger"
	"github.com/uladzk/duw-queue-monitor/internal/notifications"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/go-telegram/bot"
)

func createTestHandlerRegistry() *HandlerRegistry {
	logger := logger.NewLogger(&logger.Config{Level: "error"})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"ok":true}`)
	}))

	cfg := &notifications.TelegramConfig{
		BaseApiUrl: server.URL,
		BotToken:   "test-token",
	}

	telegramNotifier := notifications.NewTelegramNotifier(cfg, logger, &http.Client{})

	return NewHandlerRegistry(logger, telegramNotifier, "admin123")
}

func TestHandlerRegistry_RegisterAllHandlers_FullFunctionality(t *testing.T) {
	// Arrange
	registry := createTestHandlerRegistry()

	var capturedRegistrations []string
	mockBotServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRegistrations = append(capturedRegistrations, r.URL.Path)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"ok":true,"result":{"message_id":1}}`)
	}))
	defer mockBotServer.Close()

	testBot, err := bot.New("test-token", bot.WithServerURL(mockBotServer.URL))
	if err != nil {
		t.Fatalf("Failed to create test bot: %v", err)
	}

	// Act
	registry.RegisterAllHandlers(testBot)

	// Assert
	defaultHandler := registry.GetDefaultHandler()
	if defaultHandler == nil {
		t.Error("Expected default handler to be available")
	}

	if registry.replyRegistry == nil {
		t.Error("Expected reply registry to be initialized")
	}

	feedbackReplyText := "Aby wysłać opinię, proszę odpowiedz na tę wiadomość swoją opinią:"
	feedbackReplyHandler := registry.replyRegistry.FindHandler(feedbackReplyText)
	if feedbackReplyHandler == nil {
		t.Error("Expected feedback reply handler to be registered in reply registry")
	}

	handler := registry.replyRegistry.FindHandler(feedbackReplyText)
	if handler == nil {
		t.Error("Expected to find feedback reply handler")
	} else {
		patterns := handler.GetReplyPatterns()
		if len(patterns) == 0 {
			t.Error("Expected handler to have reply patterns")
		}

		if !slices.Contains(patterns, feedbackReplyText) {
			t.Errorf("Expected handler to contain pattern '%s'", feedbackReplyText)
		}
	}
}
