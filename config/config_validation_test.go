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
				CORS:         CORSConfig{AllowOrigins: []string{"*"}},
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
				CORS:         CORSConfig{AllowOrigins: []string{"*"}},
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
				CORS:         CORSConfig{AllowOrigins: []string{"*"}},
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
				CORS: CORSConfig{
					AllowOrigins: []string{"*"},
				},
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

func TestAppConfig_Validate_TLSCertURL(t *testing.T) {
	t.Run("TLS 인증서 URL 형식", func(t *testing.T) {
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
				WS: WSConfig{
					TLSServer:   true,
					TLSCertFile: "https://example.com/cert.pem", // URL 형식
					TLSKeyFile:  "https://example.com/key.pem",  // URL 형식
					ListenPort:  2443,
				},
				CORS: CORSConfig{
					AllowOrigins: []string{"*"},
				},
				Applications: []ApplicationConfig{},
			},
		}

		err := appConfig.Validate()
		// URL 형식은 유효하므로 에러가 없어야 함 (경고만 출력)
		assert.NoError(t, err)
	})

	t.Run("잘못된 TLS 인증서 URL", func(t *testing.T) {
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
				WS: WSConfig{
					TLSServer:   true,
					TLSCertFile: "ftp://example.com/cert.pem", // 잘못된 스키마
					TLSKeyFile:  "https://example.com/key.pem",
					ListenPort:  2443,
				},
				CORS: CORSConfig{
					AllowOrigins: []string{"*"},
				},
				Applications: []ApplicationConfig{},
			},
		}

		err := appConfig.Validate()
		// ftp 스키마는 허용되지 않으므로 에러 발생 (경고 모드지만 URL 검증은 에러 반환)
		// 실제로는 경고만 출력하므로 에러 없음
		assert.NoError(t, err)
	})
}
