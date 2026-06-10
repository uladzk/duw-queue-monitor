package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/uladzk/duw-queue-monitor/internal/logger"
	"github.com/uladzk/duw-queue-monitor/internal/notifications"
	"github.com/uladzk/duw-queue-monitor/internal/telegrambot"

	"github.com/caarlos0/env/v11"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var log *logger.Logger

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var err error
	log, err = buildLogger()
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}

	log.Info("Building bot with handlers...")
	b, handlerRegistry, err := buildBotWithHandlers()
	if err != nil {
		panic(err)
	}

	log.Info("Configuring Telegram bot profile...")
	if err := setProfile(ctx, b, handlerRegistry); err != nil {
		panic("failed to set bot profile: " + err.Error())
	}
	log.Info("Bot profile set successfully")

	log.Info("Starting Telegram bot...")
	go b.Start(ctx)
	log.Info("Telegram bot started. Waiting for shutdown signal...")

	<-ctx.Done()

	log.Info("Received shutdown signal, stopping Telegram bot...")

	log.Info("Telegram bot stopped")
}

func setProfile(ctx context.Context, b *bot.Bot, registry *telegrambot.HandlerRegistry) error {
	profile := telegrambot.NewProfile(b, registry, log)
	if err := profile.SetProfile(ctx); err != nil {
		return err
	}
	return nil
}

func buildBotWithHandlers() (*bot.Bot, *telegrambot.HandlerRegistry, error) {
	var cfg telegrambot.Config
	if err := env.Parse(&cfg); err != nil {
		return nil, nil, err
	}

	// Use a closure to break the circular dependency:
	// bot needs default handler → handler registry needs notifier → notifier needs bot
	var handlerRegistry *telegrambot.HandlerRegistry

	opts := []bot.Option{
		bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, update *models.Update) {
			handlerRegistry.GetDefaultHandler()(ctx, b, update)
		}),
	}

	b, err := bot.New(cfg.NotificationTelegram.BotToken, opts...)
	if err != nil {
		return nil, nil, err
	}

	telegramNotifier := notifications.NewTelegramNotifier(&cfg.NotificationTelegram, b, log)
	handlerRegistry = telegrambot.NewHandlerRegistry(log, telegramNotifier, cfg.FeedbackChatID)

	handlerRegistry.RegisterAllHandlers(b)

	return b, handlerRegistry, nil
}

func buildLogger() (*logger.Logger, error) {
	var cfg logger.Config
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}
	return logger.NewLogger(&cfg), nil
}
