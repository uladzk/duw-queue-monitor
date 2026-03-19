package telegrambot

import (
	"context"
	"github.com/uladzk/duw-queue-monitor/internal/logger"
	"github.com/uladzk/duw-queue-monitor/internal/notifications"
	"github.com/uladzk/duw-queue-monitor/internal/telegrambot/handlers"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Handler interface {
	Register(b *bot.Bot, replyRegistry handlers.ReplyRegistry)
	HandleUpdate(ctx context.Context, b *bot.Bot, update *models.Update)
}

type ReplyHandlerRegistry struct {
	patternToHandler map[string]handlers.ReplyHandler
}

func NewReplyHandlerRegistry() *ReplyHandlerRegistry {
	return &ReplyHandlerRegistry{
		patternToHandler: make(map[string]handlers.ReplyHandler),
	}
}

func (r *ReplyHandlerRegistry) RegisterReplyHandler(handler handlers.ReplyHandler) {
	for _, pattern := range handler.GetReplyPatterns() {
		r.patternToHandler[pattern] = handler
	}
}

func (r *ReplyHandlerRegistry) FindHandler(replyText string) handlers.ReplyHandler {
	return r.patternToHandler[replyText]
}

type HandlerRegistry struct {
	logger           *logger.Logger
	replyRegistry    *ReplyHandlerRegistry
	handlersMap      map[string]Handler
	telegramNotifier *notifications.TelegramNotifier
	adminChatID      string
}

func NewHandlerRegistry(log *logger.Logger, telegramNotifier *notifications.TelegramNotifier, adminChatID string) *HandlerRegistry {
	handlersMap := map[string]Handler{
		"feedback": handlers.NewFeedbackHandler(log, telegramNotifier, adminChatID),
	}

	return &HandlerRegistry{
		logger:           log,
		replyRegistry:    NewReplyHandlerRegistry(),
		handlersMap:      handlersMap,
		telegramNotifier: telegramNotifier,
		adminChatID:      adminChatID,
	}
}

func (hr *HandlerRegistry) GetDefaultHandler() func(context.Context, *bot.Bot, *models.Update) {
	return handlers.NewDefaultHandler(hr.logger, hr.replyRegistry, hr).HandleUpdate
}

func (hr *HandlerRegistry) RegisterAllHandlers(b *bot.Bot) {
	for _, handler := range hr.handlersMap {
		handler.Register(b, hr.replyRegistry)
	}
}

func (hr *HandlerRegistry) GetAvailableCommands() []models.BotCommand {
	commands := make([]models.BotCommand, 0, len(hr.handlersMap))
	for commandName := range hr.handlersMap {
		commands = append(commands, models.BotCommand{
			Command:     commandName,
			Description: commandName,
		})
	}
	return commands
}
