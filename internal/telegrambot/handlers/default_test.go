package handlers

import (
	"context"
	"github.com/uladzk/duw-queue-monitor/internal/logger"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/google/go-cmp/cmp"
)

type mockHandlerRegistry struct {
	commands []models.BotCommand
}

func (m *mockHandlerRegistry) GetAvailableCommands() []models.BotCommand {
	return m.commands
}

type mockReplyRegistry struct {
	handlers map[string]ReplyHandler
}

func (m *mockReplyRegistry) RegisterReplyHandler(handler ReplyHandler) {
	for _, pattern := range handler.GetReplyPatterns() {
		m.handlers[pattern] = handler
	}
}

func (m *mockReplyRegistry) FindHandler(replyText string) ReplyHandler {
	return m.handlers[replyText]
}

func TestBuildMenuMessage_WithMultipleCommands_BuildsCorrectMenuMessage(t *testing.T) {
	// Arrange
	mockHandlerRegistry := &mockHandlerRegistry{
		commands: []models.BotCommand{
			{Command: "feedback", Description: "Send feedback"},
			{Command: "status", Description: "Check status"},
			{Command: "help", Description: "Show help"},
		},
	}

	expectedMessage := "Witaj!\n\n<b>Dostępne komendy</b>\n/feedback - Send feedback\n/status - Check status\n/help - Show help\n\nUżyj /start aby zobaczyć to menu ponownie\n"

	// Act
	success := buildMenuMessage(mockHandlerRegistry)

	// Assert
	if success != expectedMessage {
		t.Errorf("Expected menu message:\n%s\nGot:\n%s", expectedMessage, success)
	}
}

func TestHandleReplyMessage_WhenUpdateIsNil_DoesNotPanicAndReturnsFalse(t *testing.T) {
	// Arrange
	logger := logger.NewLogger(&logger.Config{Level: "error"})
	mockHandlerRegistry := &mockHandlerRegistry{commands: []models.BotCommand{}}
	mockReplyRegistry := &mockReplyRegistry{handlers: make(map[string]ReplyHandler)}

	sut := NewDefaultHandler(logger, mockReplyRegistry, mockHandlerRegistry)

	// Act
	success := sut.handleReplyMessage(nil, nil, nil)

	// Assert
	if success {
		t.Error("Expected handleReplyMessage to return false when update is nil")
	}
}

func TestHandleReplyMessage_WhenMessageIsNil_DoesNotPanicAndReturnsFalse(t *testing.T) {
	// Arrange
	logger := logger.NewLogger(&logger.Config{Level: "error"})
	mockHandlerRegistry := &mockHandlerRegistry{commands: []models.BotCommand{}}
	mockReplyRegistry := &mockReplyRegistry{handlers: make(map[string]ReplyHandler)}

	sut := NewDefaultHandler(logger, mockReplyRegistry, mockHandlerRegistry)
	update := &models.Update{Message: nil}

	// Act
	success := sut.handleReplyMessage(nil, nil, update)

	// Assert
	if success {
		t.Error("Expected handleReplyMessage to return false when message is nil")
	}
}

func TestHandleReplyMessage_WhenReplyToMessageIsNil_DoesNotPanicAndReturnsFalse(t *testing.T) {
	// Arrange
	logger := logger.NewLogger(&logger.Config{Level: "error"})
	mockHandlerRegistry := &mockHandlerRegistry{commands: []models.BotCommand{}}
	mockReplyRegistry := &mockReplyRegistry{handlers: make(map[string]ReplyHandler)}

	sut := NewDefaultHandler(logger, mockReplyRegistry, mockHandlerRegistry)
	update := &models.Update{
		Message: &models.Message{
			ReplyToMessage: nil,
		},
	}

	// Act
	success := sut.handleReplyMessage(nil, nil, update)

	// Assert
	if success {
		t.Error("Expected handleReplyMessage to return false when reply to message is nil")
	}
}

func TestDefaultHandler_HandleReplyMessage_WhenReplyToMessageTextIsEmpty_ReturnsFalse(t *testing.T) {
	// Arrange
	logger := logger.NewLogger(&logger.Config{Level: "error"})
	mockHandlerRegistry := &mockHandlerRegistry{commands: []models.BotCommand{}}
	mockReplyRegistry := &mockReplyRegistry{handlers: make(map[string]ReplyHandler)}

	sut := NewDefaultHandler(logger, mockReplyRegistry, mockHandlerRegistry)
	update := &models.Update{
		Message: &models.Message{
			ReplyToMessage: &models.Message{
				Text: "",
			},
		},
	}

	// Act
	success := sut.handleReplyMessage(nil, nil, update)

	// Assert
	if success != false {
		t.Error("Expected handleReplyMessage to return false when reply to message text is empty")
	}
}

func TestHandleReplyMessage_WhenNoHandlerFound_DoesNotPanicAndReturnsFalse(t *testing.T) {
	// Arrange
	logger := logger.NewLogger(&logger.Config{Level: "error"})
	mockHandlerRegistry := &mockHandlerRegistry{commands: []models.BotCommand{}}
	mockReplyRegistry := &mockReplyRegistry{handlers: make(map[string]ReplyHandler)}

	sut := NewDefaultHandler(logger, mockReplyRegistry, mockHandlerRegistry)
	update := &models.Update{
		Message: &models.Message{
			ReplyToMessage: &models.Message{
				Text: "unknown pattern",
			},
		},
	}

	// Act
	success := sut.handleReplyMessage(nil, nil, update)

	// Assert
	if success {
		t.Error("Expected handleReplyMessage to return false when no handler found")
	}
}

type mockReplyHandler struct {
	patterns     []string
	handleCalled bool
	lastUpdate   *models.Update
}

func (m *mockReplyHandler) GetReplyPatterns() []string {
	return m.patterns
}

func (m *mockReplyHandler) HandleReply(ctx context.Context, b *bot.Bot, update *models.Update) {
	m.handleCalled = true
	m.lastUpdate = update
}

func TestHandleReplyMessage_WhenHandlerFound_CallsHandlerAndReturnsTrue(t *testing.T) {
	// Arrange
	logger := logger.NewLogger(&logger.Config{Level: "error"})
	mockHandlerRegistry := &mockHandlerRegistry{commands: []models.BotCommand{}}
	mockReplyRegistry := &mockReplyRegistry{handlers: make(map[string]ReplyHandler)}

	replyHandler := &mockReplyHandler{patterns: []string{"test reply pattern"}}
	mockReplyRegistry.RegisterReplyHandler(replyHandler)

	sut := NewDefaultHandler(logger, mockReplyRegistry, mockHandlerRegistry)
	update := &models.Update{
		Message: &models.Message{
			ReplyToMessage: &models.Message{
				Text: "test reply pattern",
			},
		},
	}

	// Act
	success := sut.handleReplyMessage(nil, nil, update)

	// Assert
	if !success {
		t.Error("Expected handleReplyMessage to return true when handler is found and called")
	}

	if !replyHandler.handleCalled {
		t.Error("Expected reply handler to be called")
	}

	if diff := cmp.Diff(replyHandler.lastUpdate, update); diff != "" {
		t.Errorf("Reply handler called with wrong update (-want +got):\n%s", diff)
	}
}
