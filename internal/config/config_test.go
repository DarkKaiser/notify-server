package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Unit Tests: Configuration Logic & Helpers
// =============================================================================

func TestNormalizeEnvKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"NOTIFY_DEBUG", "debug"},
		{"NOTIFY_HTTP_RETRY__MAX_RETRIES", "http_retry.max_retries"},
		{"NOTIFY_NOTIFY_API__CORS__ALLOW_ORIGINS", "notify_api.cors.allow_origins"},
		{"NOTIFY_DEBUG", "debug"}, // This line was added based on the instruction's content
		{"DEBUG", "debug"},        // Prefix가 없어도 동작은 하지만, 실제 호출부는 prefix를 제거하고 넘길 수도 있음 (현재 구현상 TrimPrefix는 한번만 동작)
		{"NOTIFY_DEBUG", "debug"},
		{"NOTIFY_Mixed_Case__Key", "mixed_case.key"},
	}

	for _, tt := range tests {
		got := normalizeEnvKey(tt.input)
		assert.Equal(t, tt.expected, got, "Input: %s", tt.input)
	}
}

func TestNewDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := newDefaultConfig()

	// Assert Default Values
	assert.True(t, cfg.Debug, "Default debug should be true")
	assert.Equal(t, DefaultMaxRetries, cfg.HTTPRetry.MaxRetries)
	assert.Equal(t, DefaultRetryDelay, cfg.HTTPRetry.RetryDelay)
	assert.Empty(t, cfg.Notifiers.Telegrams)
	assert.Empty(t, cfg.Tasks)
}

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

// =============================================================================
// Integration Tests: Load Logic
// =============================================================================

func TestLoad_Integration(t *testing.T) {
	// 통합 테스트는 Env 설정 등으로 인해 병렬 실행 시 간섭이 발생할 수 있으므로 t.Parallel() 사용에 주의해야 합니다.
	// t.Setenv는 해당 테스트 범위 내에서만 Env를 변경하므로 안전하지만,
	// 다른 병렬 테스트가 Env를 읽는다면 문제가 될 수 있습니다.
	// 여기서는 순차 실행을 보장하기 위해 t.Parallel()을 생략하거나 주의해서 사용합니다.

	createTempConfig := func(t *testing.T, content string) string {
		t.Helper()
		dir := t.TempDir()
		path := filepath.Join(dir, "config.json")
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
		return path
	}

	t.Run("Priority: Env > File > Defaults", func(t *testing.T) {
		// 1. File Config (Overrides Defaults)
		jsonContent := `{
			"http_retry": {"max_retries": 10},
			"notifiers": {
				"default_notifier_id": "file-tele",
				"telegrams": [{"id": "file-tele", "bot_token": "123456789:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", "chat_id": 1}]
			},
			"notify_api": { "ws": {"listen_port": 9000}, "cors": {"allow_origins": ["*"]}, "applications": [] }
		}`
		path := createTempConfig(t, jsonContent)

		// 2. Env Config (Overrides File)
		t.Setenv("NOTIFY_HTTP_RETRY__MAX_RETRIES", "50")

		// 3. Load
		cfg, err := LoadWithFile(path)
		require.NoError(t, err)

		// 4. Verification
		assert.Equal(t, 50, cfg.HTTPRetry.MaxRetries, "Environment variable should take precedence over file")
		assert.Equal(t, 9000, cfg.NotifyAPI.WS.ListenPort, "File config should take precedence over defaults")
		assert.True(t, cfg.Debug, "Default value should persist if not overridden")
	})

	t.Run("Error: File Not Found", func(t *testing.T) {
		cfg, err := LoadWithFile("non-existent-config.json")
		require.Error(t, err)
		assert.Nil(t, cfg)

		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			assert.Equal(t, apperrors.System, appErr.Type)
			assert.Contains(t, err.Error(), "설정 파일을 찾을 수 없습니다")
		}
	})

	t.Run("Error: Malformed JSON", func(t *testing.T) {
		path := createTempConfig(t, "{ invalid_json: ... }")
		cfg, err := LoadWithFile(path)
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "설정 파일 로드 중 오류")
	})

	t.Run("Error: Unknown Fields (Strict Unmarshal)", func(t *testing.T) {
		jsonContent := `{
			"unknown_field": "hacking",
			"debug": true
		}`
		path := createTempConfig(t, jsonContent)
		cfg, err := LoadWithFile(path)
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "구조체로 변환하는데 실패했습니다")
	})

	t.Run("Error: Validation Failure After Load", func(t *testing.T) {
		// Valid JSON structure but invalid logic (e.g., negative port)
		// Validation fails fast, so we need minimally valid config for previous steps (Notifiers)
		jsonContent := `{
			"notifiers": {
				"default_notifier_id": "test-notifier",
				"telegrams": [{"id": "test-notifier", "bot_token": "123456789:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", "chat_id": 12345}]
			},
			"notify_api": { "ws": {"listen_port": -1}, "cors": {"allow_origins": ["*"]}, "applications": [] }
		}`
		path := createTempConfig(t, jsonContent)
		cfg, err := LoadWithFile(path)
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "웹 서버 포트(listen_port)는 1에서 65535 사이의 값이어야 합니다")
	})
}
