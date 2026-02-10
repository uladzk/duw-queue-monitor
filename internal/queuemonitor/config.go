package queuemonitor

import "github.com/UladzK/duw-queue-monitor/internal/notifications"

type Config struct {
	StatusCheckInternalSeconds int    `env:"STATUS_CHECK_INTERVAL_SECONDS" envDefault:"10"`
	BroadcastChannelName       string `env:"NOTIFICATION_TELEGRAM_BROADCAST_CHANNEL_NAME,required"`
	FFDailyStatsEnabled        bool   `env:"FF_DAILY_STATS_ENABLED" envDefault:"false"`
	QueueMonitor               QueueMonitorConfig
	NotificationTelegram       notifications.TelegramConfig
}

type QueueMonitorConfig struct {
	StatusMonitoredQueueId    int    `env:"STATUS_MONITORED_QUEUE_ID" envDefault:"24"`
	StatusMonitoredQueueCity  string `env:"STATUS_MONITORED_QUEUE_CITY" envDefault:"Wrocław"`
	StatusApiUrl              string `env:"STATUS_API_URL" envDefault:"https://rezerwacje.duw.pl/status_kolejek/query.php?status="`
	StatusCheckTimeoutMs      uint   `env:"STATUS_CHECK_TIMEOUT_MS" envDefault:"4000"`
	StatusCheckMaxAttempts    uint   `env:"STATUS_CHECK_MAX_ATTEMPTS" envDefault:"3"`
	StatusCheckAttemptDelayMs uint   `env:"STATUS_CHECK_ATTEMPT_DELAY_MS" envDefault:"500"`
	HttpClientTimeoutSeconds  int    `env:"MONITOR_HTTP_CLIENT_TIMEOUT_SECONDS" envDefault:"5"`
	RedisConString            string `env:"STATE_REDIS_CONNECTION_STRING,required"`
	StateTtlSeconds           int    `env:"STATE_TTL_SECONDS" envDefault:"60"`
	PostgresConString         string `env:"STATS_POSTGRES_CONNECTION_STRING"`
}
