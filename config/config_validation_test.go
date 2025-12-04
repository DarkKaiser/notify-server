package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppConfig_Validate_InvalidDuration(t *testing.T) {
	t.Run("잘못된 HTTP Retry Duration", func(t *testing.T) {
		appConfig := &AppConfig{
			HTTPRetry: HTTPRetryConfig{
				MaxRetries: 3,
				RetryDelay: "2 seconds", // Invalid!
			},
			Notifiers: NotifierConfig{
				DefaultNotifierID: "telegram1",
				Telegrams: []TelegramConfig{
					{ID: "telegram1", BotToken: "token1", ChatID: 123},
				},
			},
			Tasks: []TaskConfig{},
			NotifyAPI: NotifyAPIConfig{
				WS:           WSConfig{TLSServer: false, ListenPort: 2443},
				Applications: []ApplicationConfig{},
			},
		}

		err := appConfig.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP Retry")
	})
}

func TestAppConfig_Validate_InvalidCronExpression(t *testing.T) {
	t.Run("잘못된 Cron 표현식", func(t *testing.T) {
		appConfig := &AppConfig{
			HTTPRetry: HTTPRetryConfig{
				MaxRetries: 3,
				RetryDelay: "2s",
			},
			Notifiers: NotifierConfig{
				DefaultNotifierID: "telegram1",
				Telegrams: []TelegramConfig{
					{ID: "telegram1", BotToken: "token1", ChatID: 123},
				},
			},
			Tasks: []TaskConfig{
				{
					ID:    "test-task",
					Title: "Test Task",
					Commands: []TaskCommandConfig{
						{
							ID:    "test-command",
							Title: "Test Command",
							Scheduler: struct {
								Runnable bool   `json:"runnable"`
								TimeSpec string `json:"time_spec"`
							}{
								Runnable: true,
								TimeSpec: "invalid cron", // Invalid!
							},
							DefaultNotifierID: "telegram1",
						},
					},
				},
			},
			NotifyAPI: NotifyAPIConfig{
				WS:           WSConfig{TLSServer: false, ListenPort: 2443},
				Applications: []ApplicationConfig{},
			},
		}

		err := appConfig.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Scheduler")
		assert.Contains(t, err.Error(), "test-task")
		assert.Contains(t, err.Error(), "test-command")
	})
}

func TestAppConfig_Validate_InvalidPort(t *testing.T) {
	t.Run("잘못된 포트 번호", func(t *testing.T) {
		appConfig := &AppConfig{
			HTTPRetry: HTTPRetryConfig{
				MaxRetries: 3,
				RetryDelay: "2s",
			},
			Notifiers: NotifierConfig{
				DefaultNotifierID: "telegram1",
				Telegrams: []TelegramConfig{
					{ID: "telegram1", BotToken: "token1", ChatID: 123},
				},
			},
			Tasks: []TaskConfig{},
			NotifyAPI: NotifyAPIConfig{
				WS:           WSConfig{TLSServer: false, ListenPort: 70000}, // Invalid!
				Applications: []ApplicationConfig{},
			},
		}

		err := appConfig.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "포트")
	})
}

func TestAppConfig_Validate_ValidCronExpression(t *testing.T) {
	t.Run("유효한 Cron 표현식", func(t *testing.T) {
		appConfig := &AppConfig{
			HTTPRetry: HTTPRetryConfig{
				MaxRetries: 3,
				RetryDelay: "2s",
			},
			Notifiers: NotifierConfig{
				DefaultNotifierID: "telegram1",
				Telegrams: []TelegramConfig{
					{ID: "telegram1", BotToken: "token1", ChatID: 123},
				},
			},
			Tasks: []TaskConfig{
				{
					ID:    "test-task",
					Title: "Test Task",
					Commands: []TaskCommandConfig{
						{
							ID:    "test-command",
							Title: "Test Command",
							Scheduler: struct {
								Runnable bool   `json:"runnable"`
								TimeSpec string `json:"time_spec"`
							}{
								Runnable: true,
								TimeSpec: "0 */5 * * * *", // Valid!
							},
							DefaultNotifierID: "telegram1",
						},
					},
				},
			},
			NotifyAPI: NotifyAPIConfig{
				WS: WSConfig{TLSServer: false, ListenPort: 2443},
				Applications: []ApplicationConfig{
					{
						ID:                "test-app",
						Title:             "Test App",
						DefaultNotifierID: "telegram1",
						AppKey:            "test-key",
					},
				},
			},
		}

		err := appConfig.Validate()
		assert.NoError(t, err)
	})
}
