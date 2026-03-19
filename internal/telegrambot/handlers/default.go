package handlers

import (
	"context"
	"fmt"
	"strings"
	"github.com/uladzk/duw-queue-monitor/internal/logger"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	menuTemplate = "Witaj!\n\n<b>Dostępne komendy</b>\n%s\n\nUżyj /start aby zobaczyć to menu ponownie\n"
)

func buildMenuMessage(handlerRegistry HandlerRegistry) string {
	commands := handlerRegistry.GetAvailableCommands()
	commandStrings := make([]string, 0, len(commands))

	for _, cmd := range commands {
		commandStrings = append(commandStrings, fmt.Sprintf("/%s - %s", cmd.Command, cmd.Description))
	}

	commandsText := strings.Join(commandStrings, "\n")
	menuMessage := fmt.Sprintf(menuTemplate, commandsText)
	return menuMessage
}

type HandlerRegistry interface {
	GetAvailableCommands() []models.BotCommand
}

type DefaultHandler struct {
	replyRegistry   ReplyRegistry
	log             *logger.Logger
	handlerRegistry HandlerRegistry
	menuMessage     string
}

func NewDefaultHandler(log *logger.Logger, replyRegistry ReplyRegistry, handlerRegistry HandlerRegistry) *DefaultHandler {
	return &DefaultHandler{
		log:             log,
		replyRegistry:   replyRegistry,
		handlerRegistry: handlerRegistry,
		menuMessage:     buildMenuMessage(handlerRegistry),
	}
}

func (d *DefaultHandler) Register(b *bot.Bot, replyRegistry ReplyRegistry) {
	// Default handler does not need to register any commands, it's added automatically
	// it simply handles all updates that do not match any specific command
}

func (d *DefaultHandler) HandleUpdate(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil {
		d.log.Warn("Received empty update, skipping")
		return
	}

	d.log.Debug(fmt.Sprintf("Received message from chat ID %d: %s", update.Message.Chat.ID, update.Message.Text))

	if d.handleReplyMessage(ctx, b, update) {
		return
	}

	d.sendDefaultMenu(ctx, b, update.Message.Chat.ID)
}

func (d *DefaultHandler) handleReplyMessage(ctx context.Context, b *bot.Bot, update *models.Update) bool {
	// sometimes updates contain nil message. they are handled specially to avoid panics
	if update == nil || update.Message == nil || update.Message.ReplyToMessage == nil || update.Message.ReplyToMessage.Text == "" {
		return false
	}

	handler := d.replyRegistry.FindHandler(update.Message.ReplyToMessage.Text)
	if handler == nil {
		d.log.Warn("No handler found for reply: " + update.Message.ReplyToMessage.Text)
		return false
	}

	handler.HandleReply(ctx, b, update)
	return true
}

func (d *DefaultHandler) sendDefaultMenu(ctx context.Context, b *bot.Bot, chatID int64) {
	if b == nil {
		return
	}

	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      d.menuMessage,
		ParseMode: models.ParseModeHTML,
	})

	if err != nil {
		d.log.Error("Failed to send menu message: ", err)
	} else {
		d.log.Debug("Menu message sent to user: " + msg.Text)
	}
}
