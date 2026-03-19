package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"github.com/uladzk/duw-queue-monitor/internal/logger"
)

func TestSendMessage_WhenRequestSuccessful_SendsMessageToTelegramApiWithCorrectFormat(t *testing.T) {
	// Arrange
	testBotToken := "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ"
	testChatID := "123456789"
	testMessage := "Test message"

	mockTelegramApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, fmt.Sprintf("Expected HTTP POST but got %v", r.Method), http.StatusInternalServerError)
			return
		}

		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, fmt.Sprintf("Expected Content-Type to be 'application/json' but got '%s'", r.Header.Get("Content-Type")), http.StatusInternalServerError)
			return
		}

		if r.URL.Path != fmt.Sprintf("/bot%v/sendMessage", testBotToken) {
			http.Error(w, fmt.Sprintf("Expected URL to be '/bot%v/sendMessage' but got '%s'", testBotToken, r.URL.Path), http.StatusInternalServerError)
			return
		}

		var message SendMessageChannelRequest
		if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
			http.Error(w, fmt.Sprintf("Failed to decode request body: %v", err), http.StatusInternalServerError)
			return
		}

		if message.ChatID != testChatID {
			http.Error(w, fmt.Sprintf("Expected chat_id to be '%s' but got '%s'", testChatID, message.ChatID), http.StatusInternalServerError)
			return
		}

		if message.ParseMode != "HTML" {
			http.Error(w, fmt.Sprintf("Expected parse_mode to be 'HTML' but got '%s'", message.ParseMode), http.StatusInternalServerError)
			return
		}

		if message.Text != testMessage {
			http.Error(w, fmt.Sprintf("Expected text to be '%s' but got '%s'", testMessage, message.Text), http.StatusInternalServerError)
			return
		}

		fmt.Fprintln(w, `{"status": 200}`)
	}))

	defer mockTelegramApi.Close()

	cfg := &TelegramConfig{
		BaseApiUrl:            mockTelegramApi.URL,
		BotToken:              testBotToken,
		MaxRetryAttempts:      1,
		RetryDelayMs:          100,
		RequestTimeoutSeconds: 2,
	}

	logger := logger.NewLogger(&logger.Config{
		Level: "error"})

	sut := NewTelegramNotifier(cfg, logger, &http.Client{})

	// Act
	err := sut.SendMessage(context.Background(), testChatID, testMessage)

	// Assert
	if err != nil {
		t.Fatalf("Expected successful message sending, but got error: \"%v\"", err)
	}
}

func TestSendMessage_WhenApiReturnsError_ReturnsError(t *testing.T) {
	// Arrange
	testBotToken := "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ"
	testChatID := "123456789"
	testMessage := "Test message"

	mockTelegramApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Bad Request", http.StatusBadRequest)
	}))

	defer mockTelegramApi.Close()

	cfg := &TelegramConfig{
		BaseApiUrl:            mockTelegramApi.URL,
		BotToken:              testBotToken,
		MaxRetryAttempts:      1,
		RetryDelayMs:          100,
		RequestTimeoutSeconds: 2,
	}

	logger := logger.NewLogger(&logger.Config{
		Level: "error"})

	sut := NewTelegramNotifier(cfg, logger, &http.Client{})

	// Act
	err := sut.SendMessage(context.Background(), testChatID, testMessage)

	// Assert
	if err == nil {
		t.Fatalf("Expected error when API returns bad request, but got nil")
	}
}
