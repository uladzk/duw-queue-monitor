package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/uladzk/duw-queue-monitor/internal/logger"

	"github.com/go-telegram/bot"
)

func TestSendMessage_WhenRequestSuccessful_SendsMessageToTelegramApiWithCorrectFormat(t *testing.T) {
	// Arrange
	testBotToken := "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ"
	testChatID := "123456789"
	testMessage := "Test message"

	var capturedBody map[string]interface{}
	mockTelegramApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, fmt.Sprintf("Expected HTTP POST but got %v", r.Method), http.StatusInternalServerError)
			return
		}

		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			http.Error(w, fmt.Sprintf("Failed to decode request body: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"ok":true,"result":{"message_id":1,"chat":{"id":123456789},"text":"Test message"}}`)
	}))
	defer mockTelegramApi.Close()

	cfg := &TelegramConfig{
		BotToken:              testBotToken,
		MaxRetryAttempts:      1,
		RetryDelayMs:          100,
		RequestTimeoutSeconds: 2,
	}

	b, err := bot.New(testBotToken, bot.WithServerURL(mockTelegramApi.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("Failed to create bot: %v", err)
	}

	log := logger.NewLogger(&logger.Config{Level: "error"})
	sut := NewTelegramNotifier(cfg, b, log)

	// Act
	err = sut.SendMessage(context.Background(), testChatID, testMessage)

	// Assert
	if err != nil {
		t.Fatalf("Expected successful message sending, but got error: \"%v\"", err)
	}

	if capturedBody["chat_id"] != testChatID {
		t.Errorf("Expected chat_id '%s', got '%v'", testChatID, capturedBody["chat_id"])
	}

	if capturedBody["text"] != testMessage {
		t.Errorf("Expected text '%s', got '%v'", testMessage, capturedBody["text"])
	}

	if capturedBody["parse_mode"] != "HTML" {
		t.Errorf("Expected parse_mode 'HTML', got '%v'", capturedBody["parse_mode"])
	}
}

func TestSendMessage_WhenApiReturnsError_ReturnsError(t *testing.T) {
	// Arrange
	testBotToken := "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ"
	testChatID := "123456789"
	testMessage := "Test message"

	mockTelegramApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, `{"ok":false,"description":"Bad Request"}`)
	}))
	defer mockTelegramApi.Close()

	cfg := &TelegramConfig{
		BotToken:              testBotToken,
		MaxRetryAttempts:      1,
		RetryDelayMs:          100,
		RequestTimeoutSeconds: 2,
	}

	b, err := bot.New(testBotToken, bot.WithServerURL(mockTelegramApi.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("Failed to create bot: %v", err)
	}

	log := logger.NewLogger(&logger.Config{Level: "error"})
	sut := NewTelegramNotifier(cfg, b, log)

	// Act
	err = sut.SendMessage(context.Background(), testChatID, testMessage)

	// Assert
	if err == nil {
		t.Fatalf("Expected error when API returns bad request, but got nil")
	}
}
