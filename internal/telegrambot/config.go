package telegrambot

import "github.com/uladzk/duw-queue-monitor/internal/notifications"

type Config struct {
	FeedbackChatID       string `env:"NOTIFICATION_TELEGRAM_FEEDBACK_CHAT_ID,required"`
	NotificationTelegram notifications.TelegramConfig
}
