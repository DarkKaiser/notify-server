package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Test Helpers
// =============================================================================

// createBaseValidConfig는 검증 테스트용 기본 유효한 설정을 생성합니다.
// ConfigBuilder 패턴을 활용하여 간결하게 구성합니다.
func createBaseValidConfig() *AppConfig {
	return NewConfigBuilder().Build()
}

// =============================================================================
// Validation Tests
// =============================================================================

// TestAppConfig_Validate_TableDriven은 AppConfig의 다양한 검증 시나리오를 테스트합니다.
//
// 검증 항목:
//   - HTTP Retry 설정 검증 (Duration 형식)
//   - Scheduler Cron 표현식 검증
//   - NotifyAPI 포트 및 TLS 설정 검증
//   - 중복 ID 검증 (Notifier, Task, Command, Application)
//   - 참조 무결성 검증 (존재하지 않는 NotifierID)
//   - 필수 필드 검증 (AppKey)
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

		// =================================================================
		// HTTP Retry Validation
		// =================================================================
		{
			name: "Invalid HTTP Retry Duration",
			modifyConfig: func(c *AppConfig) {
				c.HTTPRetry.RetryDelay = "invalid"
			},
			shouldError:   true,
			errorContains: "HTTP Retry",
		},
		{
			name: "Zero MaxRetries (Valid)",
			modifyConfig: func(c *AppConfig) {
				c.HTTPRetry.MaxRetries = 0
			},
			shouldError: false,
		},
		{
			name: "Negative MaxRetries (Valid - Treated as 0)",
			modifyConfig: func(c *AppConfig) {
				c.HTTPRetry.MaxRetries = -1
			},
			shouldError: false,
		},

		// =================================================================
		// Scheduler Validation
		// =================================================================
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
			errorContains: "Scheduler",
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
		{
			name: "Scheduler Disabled (No Validation)",
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
								}{Runnable: false, TimeSpec: "invalid"},
							},
						},
					},
				}
			},
			shouldError: false,
		},

		// =================================================================
		// NotifyAPI - WS Validation
		// =================================================================
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
			name: "Port 0 (Invalid)",
			modifyConfig: func(c *AppConfig) {
				c.NotifyAPI.WS.ListenPort = 0
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
			errorContains: "인증서 파일 경로(TLSCertFile)",
		},
		{
			name: "TLS Enabled but Missing Key",
			modifyConfig: func(c *AppConfig) {
				c.NotifyAPI.WS.TLSServer = true
				c.NotifyAPI.WS.TLSCertFile = "cert.pem"
				c.NotifyAPI.WS.TLSKeyFile = "" // Missing
			},
			shouldError:   true,
			errorContains: "키 파일 경로(TLSKeyFile)",
		},

		// =================================================================
		// Duplicate ID Validation
		// =================================================================
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

		// =================================================================
		// Reference Integrity Validation
		// =================================================================
		{
			name: "Missing Default Notifier ID",
			modifyConfig: func(c *AppConfig) {
				c.Notifiers.DefaultNotifierID = "non-existent"
			},
			shouldError:   true,
			errorContains: "존재하지 않습니다",
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
			name: "Application uses unknown Notifier ID",
			modifyConfig: func(c *AppConfig) {
				c.NotifyAPI.Applications = []ApplicationConfig{
					{ID: "app1", AppKey: "k1", DefaultNotifierID: "unknown"},
				}
			},
			shouldError:   true,
			errorContains: "존재하지 않습니다",
		},

		// =================================================================
		// Required Field Validation
		// =================================================================
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
		{
			name: "Application AppKey with Whitespace Only",
			modifyConfig: func(c *AppConfig) {
				c.NotifyAPI.Applications = []ApplicationConfig{
					{ID: "app1", AppKey: "   ", DefaultNotifierID: "telegram1"},
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
