package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// helper to create a base valid config
func createBaseValidConfig() *AppConfig {
	return &AppConfig{
		HTTPRetry: HTTPRetryConfig{MaxRetries: 3, RetryDelay: "1s"},
		Notifiers: NotifierConfig{
			DefaultNotifierID: "telegram1",
			Telegrams: []TelegramConfig{
				{ID: "telegram1", BotToken: "token", ChatID: 123},
			},
		},
		Tasks: []TaskConfig{},
		NotifyAPI: NotifyAPIConfig{
			WS:           WSConfig{ListenPort: 8080},
			CORS:         CORSConfig{AllowOrigins: []string{"*"}},
			Applications: []ApplicationConfig{},
		},
	}
}

func TestAppConfig_Validate_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		modifyConfig  func(*AppConfig) // Function to modify base valid config
		shouldError   bool
		errorContains string
	}{
		{
			name:         "Valid Config",
			modifyConfig: func(c *AppConfig) {},
			shouldError:  false,
		},
		// HTTP Retry
		{
			name: "Invalid HTTP Retry Duration",
			modifyConfig: func(c *AppConfig) {
				c.HTTPRetry.RetryDelay = "invalid"
			},
			shouldError:   true,
			errorContains: "HTTP Retry",
		},
		// Scheduler
		{
			name: "Invalid Task Cron Expression",
			modifyConfig: func(c *AppConfig) {
				c.Tasks = []TaskConfig{
					{
						ID:    "task1",
						Title: "Task 1",
						Commands: []CommandConfig{
							{
								ID:                "cmd1",
								Title:             "Cmd 1",
								DefaultNotifierID: "telegram1",
								Scheduler: struct {
									Runnable bool   `json:"runnable"`
									TimeSpec string `json:"time_spec"`
								}{Runnable: true, TimeSpec: "invalid cron"},
							},
						},
					},
				}
			},
			shouldError:   true,
			errorContains: "Scheduler", // Validation logic adds this context
		},
		{
			name: "Valid Task Cron Expression",
			modifyConfig: func(c *AppConfig) {
				c.Tasks = []TaskConfig{
					{
						ID:    "task1",
						Title: "Task 1",
						Commands: []CommandConfig{
							{
								ID:                "cmd1",
								Title:             "Cmd 1",
								DefaultNotifierID: "telegram1",
								Scheduler: struct {
									Runnable bool   `json:"runnable"`
									TimeSpec string `json:"time_spec"`
								}{Runnable: true, TimeSpec: "0 */5 * * * *"},
							},
						},
					},
				}
			},
			shouldError: false,
		},
		// NotifyAPI - WS
		{
			name: "Invalid Listen Port (Too High)",
			modifyConfig: func(c *AppConfig) {
				c.NotifyAPI.WS.ListenPort = 70000
			},
			shouldError:   true,
			errorContains: "포트",
		},
		{
			name: "Invalid Listen Port (Too Low)",
			modifyConfig: func(c *AppConfig) {
				c.NotifyAPI.WS.ListenPort = -1
			},
			shouldError:   true,
			errorContains: "포트",
		},
		{
			name: "TLS Enabled but Missing Cert",
			modifyConfig: func(c *AppConfig) {
				c.NotifyAPI.WS.TLSServer = true
				c.NotifyAPI.WS.TLSCertFile = "" // Missing
				c.NotifyAPI.WS.TLSKeyFile = "key.pem"
			},
			shouldError:   true,
			errorContains: "Cert 파일 경로",
		},
		{
			name: "TLS Valid URL Cert",
			modifyConfig: func(c *AppConfig) {
				c.NotifyAPI.WS.TLSServer = true
				c.NotifyAPI.WS.TLSCertFile = "https://example.com/cert"
				c.NotifyAPI.WS.TLSKeyFile = "https://example.com/key"
			},
			shouldError: false,
		},
		// Logic Errors (Duplicates, Missing IDs)
		{
			name: "Duplicate Notifier ID",
			modifyConfig: func(c *AppConfig) {
				c.Notifiers.Telegrams = append(c.Notifiers.Telegrams, TelegramConfig{
					ID: "telegram1", BotToken: "dup", ChatID: 123,
				})
			},
			shouldError:   true,
			errorContains: "중복되었습니다",
		},
		{
			name: "Missing Default Notifier ID",
			modifyConfig: func(c *AppConfig) {
				c.Notifiers.DefaultNotifierID = "non-existent"
			},
			shouldError:   true,
			errorContains: "존재하지 않습니다",
		},
		{
			name: "Duplicate Task ID",
			modifyConfig: func(c *AppConfig) {
				c.Tasks = []TaskConfig{
					{ID: "dup"}, {ID: "dup"},
				}
			},
			shouldError:   true,
			errorContains: "중복되었습니다",
		},
		{
			name: "Duplicate Command ID within Task",
			modifyConfig: func(c *AppConfig) {
				c.Tasks = []TaskConfig{
					{
						ID: "task1",
						Commands: []CommandConfig{
							{ID: "dup", DefaultNotifierID: "telegram1"},
							{ID: "dup", DefaultNotifierID: "telegram1"},
						},
					},
				}
			},
			shouldError:   true,
			errorContains: "중복되었습니다",
		},
		{
			name: "Command uses unknown Notifier ID",
			modifyConfig: func(c *AppConfig) {
				c.Tasks = []TaskConfig{
					{
						ID: "task1",
						Commands: []CommandConfig{
							{ID: "cmd1", DefaultNotifierID: "unknown"},
						},
					},
				}
			},
			shouldError:   true,
			errorContains: "존재하지 않습니다",
		},
		{
			name: "Duplicate Application ID",
			modifyConfig: func(c *AppConfig) {
				c.NotifyAPI.Applications = []ApplicationConfig{
					{ID: "app1", AppKey: "k1", DefaultNotifierID: "telegram1"},
					{ID: "app1", AppKey: "k2", DefaultNotifierID: "telegram1"},
				}
			},
			shouldError:   true,
			errorContains: "중복되었습니다",
		},
		{
			name: "Application uses unknown Notifier ID",
			modifyConfig: func(c *AppConfig) {
				c.NotifyAPI.Applications = []ApplicationConfig{
					{ID: "app1", AppKey: "k1", DefaultNotifierID: "unknown"},
				}
			},
			shouldError:   true,
			errorContains: "존재하지 않습니다",
		},
		{
			name: "Application Missing AppKey",
			modifyConfig: func(c *AppConfig) {
				c.NotifyAPI.Applications = []ApplicationConfig{
					{ID: "app1", AppKey: "", DefaultNotifierID: "telegram1"},
				}
			},
			shouldError:   true,
			errorContains: "APP_KEY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createBaseValidConfig()
			tt.modifyConfig(cfg)

			err := cfg.Validate()
			if tt.shouldError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
