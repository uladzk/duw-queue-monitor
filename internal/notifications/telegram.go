package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/uladzk/duw-queue-monitor/internal/logger"

	"github.com/avast/retry-go/v4"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type TelegramNotifier struct {
	cfg *TelegramConfig
	bot *bot.Bot
	log *logger.Logger
}

func NewTelegramNotifier(cfg *TelegramConfig, b *bot.Bot, log *logger.Logger) *TelegramNotifier {
	return &TelegramNotifier{
		cfg: cfg,
		bot: b,
		log: log,
	}
}

func (s *TelegramNotifier) SendMessage(ctx context.Context, chatID, text string) error {
	requestTimeout := time.Duration(s.cfg.RequestTimeoutSeconds) * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	retryDelay := time.Duration(s.cfg.RetryDelayMs) * time.Millisecond

	return retry.Do(
		func() error {
			_, err := s.bot.SendMessage(timeoutCtx, &bot.SendMessageParams{
				ChatID:    chatID,
				Text:      text,
				ParseMode: models.ParseModeHTML,
			})
			if err != nil {
				return fmt.Errorf("failed to send message via Telegram bot: %w", err)
			}

			s.log.Info("Message sent successfully to TelegramApi.")
			return nil
		},
		retry.Attempts(s.cfg.MaxRetryAttempts),
		retry.Delay(retryDelay),
		retry.DelayType(retry.FixedDelay),
		retry.Context(timeoutCtx),
	)
}
