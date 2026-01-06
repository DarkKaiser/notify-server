package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Unit Tests: Validation Logic (AppConfig.validate)
// =============================================================================

func TestAppConfig_Validate_TableDriven(t *testing.T) {
	t.Parallel()

	// 1. Base Valid Configuration Factory
	baseConfig := func() *AppConfig {
		return &AppConfig{
			Debug: true,
			HTTPRetry: HTTPRetryConfig{
				MaxRetries: 3,
				RetryDelay: 1 * time.Second,
			},
			Notifiers: NotifierConfig{
				DefaultNotifierID: "telegram-1",
				Telegrams: []TelegramConfig{
					{ID: "telegram-1", BotToken: "123456789:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", ChatID: 12345},
				},
			},
			Tasks: []TaskConfig{
				{
					ID: "task-1",
					Commands: []CommandConfig{
						{
							ID:                "cmd-1",
							DefaultNotifierID: "telegram-1",
							Scheduler:         SchedulerConfig{Runnable: true, TimeSpec: "@daily"},
						},
					},
				},
			},
			NotifyAPI: NotifyAPIConfig{
				WS:           WSConfig{ListenPort: 8080},
				CORS:         CORSConfig{AllowOrigins: []string{"*"}},
				Applications: []ApplicationConfig{{ID: "app-1", AppKey: "secret-key", DefaultNotifierID: "telegram-1"}},
			},
		}
	}

	tests := []struct {
		name        string
		modifier    func(*AppConfig) // Config을 망가뜨리는 함수
		expectError bool
		errorMsg    string
	}{
		// Happy Path
		{
			name:        "Valid Configuration",
			modifier:    func(c *AppConfig) {},
			expectError: false,
		},
		// HTTP Retry
		{
			name:        "HTTPRetry: Invalid Delay (Zero)",
			modifier:    func(c *AppConfig) { c.HTTPRetry.RetryDelay = 0 },
			expectError: true,
			errorMsg:    "HTTP 재시도 대기 시간",
		},
		// Notifiers
		{
			name:        "Notifier: Default ID Not Found",
			modifier:    func(c *AppConfig) { c.Notifiers.DefaultNotifierID = "invalid-id" },
			expectError: true,
			errorMsg:    "기본 NotifierID('invalid-id')가 정의된 Notifier 목록에 존재하지 않습니다",
		},
		{
			name: "Notifier: Duplicate ID",
			modifier: func(c *AppConfig) {
				c.Notifiers.Telegrams = append(c.Notifiers.Telegrams, TelegramConfig{
					ID: "telegram-1", BotToken: "987654321:XYZ-DEF1234ghIkl-zyx57W2v1u123ew11", ChatID: 999,
				})
			},
			expectError: true,
			errorMsg:    "중복된 Notifier ID가 존재합니다",
		},
		{
			name: "Notifier: Invalid Bot Token",
			modifier: func(c *AppConfig) {
				c.Notifiers.Telegrams[0].BotToken = "invalid-token"
			},
			expectError: true,
			errorMsg:    "bot_token", // JSON tag name is used in error message
		},
		// Tasks
		{
			name: "Task: Duplicate ID",
			modifier: func(c *AppConfig) {
				c.Tasks = append(c.Tasks, TaskConfig{ID: "task-1"})
			},
			expectError: true,
			errorMsg:    "중복된 Task ID가 존재합니다",
		},
		{
			name: "Task Command: Duplicate ID",
			modifier: func(c *AppConfig) {
				c.Tasks[0].Commands = append(c.Tasks[0].Commands, CommandConfig{ID: "cmd-1"})
			},
			expectError: true,
			errorMsg:    "중복된 Command ID가 존재합니다",
		},
		{
			name: "Task Command: Unknown Notifier Ref",
			modifier: func(c *AppConfig) {
				c.Tasks[0].Commands[0].DefaultNotifierID = "unknown"
			},
			expectError: true,
			errorMsg:    "참조하는 NotifierID('unknown')가 정의되지 않았습니다",
		},
		{
			name: "Task Command: Invalid Cron Spec",
			modifier: func(c *AppConfig) {
				c.Tasks[0].Commands[0].Scheduler.TimeSpec = "invalid-cron"
			},
			expectError: true,
			errorMsg:    "스케줄러(TimeSpec) 설정이 유효하지 않습니다",
		},
		// Notify API
		{
			name: "API: Duplicate Application ID",
			modifier: func(c *AppConfig) {
				c.NotifyAPI.Applications = append(c.NotifyAPI.Applications, ApplicationConfig{ID: "app-1"})
			},
			expectError: true,
			errorMsg:    "중복된 Application ID가 존재합니다",
		},
		{
			name: "API: App Missing AppKey",
			modifier: func(c *AppConfig) {
				c.NotifyAPI.Applications[0].AppKey = ""
			},
			expectError: true,
			errorMsg:    "API 키(APP_KEY)가 설정되지 않았습니다",
		},
		{
			name: "API: App Unknown Notifier Ref",
			modifier: func(c *AppConfig) {
				c.NotifyAPI.Applications[0].DefaultNotifierID = "unknown"
			},
			expectError: true,
			errorMsg:    "참조하는 기본 NotifierID('unknown')가 정의되지 않았습니다",
		},
		// TLS Validation
		{
			name: "WS: TLS Enabled but Missing Cert",
			modifier: func(c *AppConfig) {
				c.NotifyAPI.WS.TLSServer = true
				c.NotifyAPI.WS.TLSCertFile = ""
			},
			expectError: true,
			errorMsg:    "TLS 서버 활성화 시 인증서 파일 경로(tls_cert_file)는 필수입니다",
		},
		{
			name: "WS: TLS Cert File Not Found",
			modifier: func(c *AppConfig) {
				c.NotifyAPI.WS.TLSServer = true
				c.NotifyAPI.WS.TLSCertFile = "non-existent.pem"
				c.NotifyAPI.WS.TLSKeyFile = "non-existent.key" // Key file check might be skipped if Cert fails first, but strictly speaking both are checked
			},
			expectError: true,
			errorMsg:    "지정된 TLS 인증서 파일(tls_cert_file)을 찾을 수 없습니다",
		},
		// CORS
		{
			name: "CORS: Empty Origins",
			modifier: func(c *AppConfig) {
				c.NotifyAPI.CORS.AllowOrigins = []string{}
			},
			expectError: true,
			errorMsg:    "CORS 허용 도메인(allow_origins) 목록이 비어있습니다",
		},
		{
			name: "CORS: Wildcard Mixed with Others",
			modifier: func(c *AppConfig) {
				c.NotifyAPI.CORS.AllowOrigins = []string{"*", "https://google.com"}
			},
			expectError: true,
			errorMsg:    "와일드카드(*)는 다른 도메인과 함께 사용할 수 없습니다",
		},
		{
			name: "CORS: Invalid Origin Format",
			modifier: func(c *AppConfig) {
				c.NotifyAPI.CORS.AllowOrigins = []string{"ht tp://bad-url"}
			},
			expectError: true,
			errorMsg:    "CORS Origin 형식이 올바르지 않습니다",
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := baseConfig()
			tt.modifier(cfg)

			err := cfg.validate(newValidator())

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestVerifyRecommendations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		port       int
		expectWarn bool
		warnMsg    string
	}{
		{"Safe Port", 8080, false, ""},
		{"Privileged Port (HTTP)", 80, true, "시스템 예약 포트"},
		{"Privileged Port (HTTPS)", 443, true, "시스템 예약 포트"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &AppConfig{NotifyAPI: NotifyAPIConfig{WS: WSConfig{ListenPort: tt.port}}}
			warnings := cfg.VerifyRecommendations()

			if tt.expectWarn {
				assert.NotEmpty(t, warnings)
				assert.Contains(t, warnings[0], tt.warnMsg)
			} else {
				assert.Empty(t, warnings)
			}
		})
	}
}
