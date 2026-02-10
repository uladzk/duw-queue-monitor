package statsreporting

import "github.com/UladzK/duw-queue-monitor/internal/notifications"

type Config struct {
	StatsReporting       StatsReportingConfig
	NotificationTelegram notifications.TelegramConfig
}

type StatsReportingConfig struct {
	PostgresConString string `env:"STATS_POSTGRES_CONNECTION_STRING,required"`
	QueueID           int    `env:"STATS_QUEUE_ID" envDefault:"24"`
	QueueName         string `env:"STATS_QUEUE_NAME" envDefault:"Odbiór karty pobytu"`
	ChannelName       string `env:"NOTIFICATION_TELEGRAM_BROADCAST_CHANNEL_NAME,required"`
}
