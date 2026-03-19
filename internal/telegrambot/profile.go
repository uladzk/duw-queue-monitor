package telegrambot

import (
	"context"
	"fmt"
	"github.com/uladzk/duw-queue-monitor/internal/logger"

	"github.com/go-telegram/bot"
)

type Profile struct {
	bot      *bot.Bot
	registry *HandlerRegistry
	logger   *logger.Logger
}

func NewProfile(b *bot.Bot, registry *HandlerRegistry, logger *logger.Logger) *Profile {
	return &Profile{bot: b, registry: registry, logger: logger}
}

func (p *Profile) SetProfile(ctx context.Context) error {
	if _, err := p.bot.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands: p.registry.GetAvailableCommands(),
	}); err != nil {
		p.logger.Error("Failed to set bot commands: ", err)

		return fmt.Errorf("failed to set bot commands: %w", err)
	}

	return nil
}
