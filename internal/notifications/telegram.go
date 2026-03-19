package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"github.com/uladzk/duw-queue-monitor/internal/logger"

	"github.com/avast/retry-go/v4"
)

type TelegramNotifier struct {
	cfg        *TelegramConfig
	log        *logger.Logger
	httpClient *http.Client
}

func NewTelegramNotifier(cfg *TelegramConfig, log *logger.Logger, httpClient *http.Client) *TelegramNotifier {
	return &TelegramNotifier{
		cfg:        cfg,
		log:        log,
		httpClient: httpClient,
	}
}

type SendMessageChannelRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"` // needed to correctly format the message in Telegram
}

func (s *TelegramNotifier) SendMessage(ctx context.Context, chatID, text string) error {
	botApiFullUrl := fmt.Sprintf("%s/bot%s/sendMessage", s.cfg.BaseApiUrl, s.cfg.BotToken)

	reqBody := SendMessageChannelRequest{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "HTML",
	}

	return s.sendMessageWithRetries(ctx, botApiFullUrl, reqBody)
}

func (s *TelegramNotifier) sendMessageWithRetries(ctx context.Context, url string, reqBody SendMessageChannelRequest) error {
	requestTimeout := time.Duration(s.cfg.RequestTimeoutSeconds) * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	retryDelay := time.Duration(s.cfg.RetryDelayMs) * time.Millisecond

	return retry.Do(
		func() error {
			b, err := json.Marshal(reqBody)
			if err != nil {
				return fmt.Errorf("failed to marshal request body when sending message to TelegramApi: %w", err)
			}

			req, err := http.NewRequestWithContext(timeoutCtx, "POST", url, bytes.NewBuffer(b))
			if err != nil {
				return fmt.Errorf("failed to create HTTP request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := s.httpClient.Do(req)
			if err != nil {
				return fmt.Errorf("failed to send message to TelegramApi: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				respTxt, err := io.ReadAll(resp.Body)
				if err != nil {
					return fmt.Errorf("failed to read response body when sending message to TelegramApi. got unsuccessful status code: %d", resp.StatusCode)
				}

				return fmt.Errorf("sending message to TelegramApi failed. got unsuccessful status code: %d, api response: \"%s\"", resp.StatusCode, respTxt)
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
