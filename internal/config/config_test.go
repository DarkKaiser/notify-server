package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Helper Functions
// =============================================================================

// createTempConfigFile creates a temporary file with the given content.
func createTempConfigFile(t *testing.T, content string) string {
	t.Helper()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

// minimalValidJSON returns a minimal valid JSON configuration string.
func minimalValidJSON() string {
	return `{
		"debug": true,
		"notifiers": {
			"default_notifier_id": "telegram1",
			"telegrams": [
				{"id": "telegram1", "bot_token": "token", "chat_id": 123}
			]
		},
		"tasks": [],
		"notify_api": {
			"ws": {"listen_port": 8080},
			"cors": {"allow_origins": ["*"]},
			"applications": []
		}
	}`
}

// =============================================================================
// Tests: LoadWithFile (Integration of Loading, Defaults, and Validation)
// =============================================================================

func TestLoadWithFile(t *testing.T) {
	// t.Setenv 사용으로 인해 Parallel 실행 불가

	t.Run("Success Scenarios", func(t *testing.T) {
		tests := []struct {
			name           string
			fileContent    string
			envVars        map[string]string
			validateConfig func(*testing.T, *AppConfig)
		}{
			{
				name:        "Load Valid File with Defaults",
				fileContent: minimalValidJSON(),
				validateConfig: func(t *testing.T, cfg *AppConfig) {
					assert.True(t, cfg.Debug)
					assert.Equal(t, DefaultMaxRetries, cfg.HTTPRetry.MaxRetries, "Default MaxRetries should be applied")
					assert.Equal(t, DefaultRetryDelay, cfg.HTTPRetry.RetryDelay, "Default RetryDelay should be applied")
				},
			},
			{
				name: "Override Defaults via JSON",
				fileContent: `{
					"http_retry": {"max_retries": 5, "retry_delay": "5s"},
					"notifiers": {
						"default_notifier_id": "t1",
						"telegrams": [{"id": "t1", "bot_token": "a", "chat_id": 1}]
					},
					"notify_api": { "ws": {"listen_port": 9090}, "cors": {"allow_origins": ["*"]} }
				}`,
				validateConfig: func(t *testing.T, cfg *AppConfig) {
					assert.Equal(t, 5, cfg.HTTPRetry.MaxRetries)
					assert.Equal(t, "5s", cfg.HTTPRetry.RetryDelay)
				},
			},
			{
				name:        "Override via Environment Variables",
				fileContent: minimalValidJSON(),
				envVars: map[string]string{
					"NOTIFY_DEBUG":                   "false",
					"NOTIFY_HTTP_RETRY__MAX_RETRIES": "99",
				},
				validateConfig: func(t *testing.T, cfg *AppConfig) {
					assert.False(t, cfg.Debug)
					assert.Equal(t, 99, cfg.HTTPRetry.MaxRetries)
				},
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				// Set Environment Variables
				for k, v := range tt.envVars {
					t.Setenv(k, v)
				}

				path := createTempConfigFile(t, tt.fileContent)
				cfg, err := LoadWithFile(path)

				require.NoError(t, err)
				require.NotNil(t, cfg)
				if tt.validateConfig != nil {
					tt.validateConfig(t, cfg)
				}
			})
		}
	})

	t.Run("Failure Scenarios", func(t *testing.T) {
		tests := []struct {
			name          string
			filename      string
			fileContent   string // used if filename is empty (auto-create temp file)
			expectedError interface{}
			errorContains string
		}{
			{
				name:          "File Not Found",
				filename:      "non_existent_config.json",
				expectedError: apperrors.System,
				errorContains: "설정 파일을 찾을 수 없습니다",
			},
			{
				name:          "Invalid JSON Syntax",
				fileContent:   `{ invalid json`,
				expectedError: apperrors.InvalidInput, // or System depending on parser
				errorContains: "설정 파일 로드 중 오류",
			},
			{
				name: "Validation Failure (Missing Required Field)",
				fileContent: `{
					"notifiers": {} 
				}`, // Missing 'default_notifier_id' etc.
				expectedError: apperrors.InvalidInput,
				errorContains: "유효성 검증에 실패했습니다",
			},
			{
				name: "Strict Decoding (Unknown Field)",
				fileContent: `{
					"unknown_field_123": "value",
					"notifiers": { "default_notifier_id": "t1", "telegrams": [] }
				}`,
				expectedError: apperrors.System, // Decode failure usually System or InvalidInput
				errorContains: "구조체로 변환하는데 실패했습니다",
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				path := tt.filename
				if path == "" {
					path = createTempConfigFile(t, tt.fileContent)
				}

				cfg, err := LoadWithFile(path)

				assert.Error(t, err)
				assert.Nil(t, cfg)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}

				// Check custom error type if possible
				var appErr *apperrors.AppError
				if errors.As(err, &appErr) {
					switch expected := tt.expectedError.(type) {
					case apperrors.ErrorType:
						assert.Equal(t, expected, appErr.Type)
					case error:
						assert.True(t, errors.Is(err, expected))
					}
				}
			})
		}
	})
}

// =============================================================================
// Tests: AppConfig.validate() (Domain Business Logic)
// =============================================================================

func TestAppConfig_Validate_Logic(t *testing.T) {
	t.Parallel()

	// Helper to create a valid config and modify it
	baseConfig := func() *AppConfig {
		return &AppConfig{
			HTTPRetry: HTTPRetryConfig{MaxRetries: 3, RetryDelay: "1s"},
			Notifiers: NotifierConfig{
				DefaultNotifierID: "t1",
				Telegrams:         []TelegramConfig{{ID: "t1", BotToken: "b", ChatID: 1}},
			},
			NotifyAPI: NotifyAPIConfig{
				WS:           WSConfig{ListenPort: 8080},
				CORS:         CORSConfig{AllowOrigins: []string{"*"}},
				Applications: []ApplicationConfig{},
			},
		}
	}

	t.Run("HTTP Retry Validation", func(t *testing.T) {
		cfg := baseConfig()
		cfg.HTTPRetry.RetryDelay = "invalid"

		err := cfg.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP 재시도 대기 시간")
	})

	t.Run("NotifyAPI Integrity (Missing Default Notifier)", func(t *testing.T) {
		cfg := baseConfig()
		cfg.NotifyAPI.Applications = []ApplicationConfig{
			{ID: "app1", AppKey: "key", DefaultNotifierID: "missing_notifier"},
		}

		err := cfg.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "참조하는 기본 NotifierID('missing_notifier')가 정의되지 않았습니다")
	})

	t.Run("Task Scheduler Cron Validation", func(t *testing.T) {
		cfg := baseConfig()
		cfg.Tasks = []TaskConfig{{
			ID: "task1",
			Commands: []CommandConfig{{
				ID: "cmd1", DefaultNotifierID: "t1",
				Scheduler: struct {
					Runnable bool   `json:"runnable"`
					TimeSpec string `json:"time_spec"`
				}{Runnable: true, TimeSpec: "invalid_cron"},
			}},
		}}

		err := cfg.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "스케줄러(TimeSpec) 설정이 유효하지 않습니다")
	})
}

// =============================================================================
// Tests: VerifyRecommendations
// =============================================================================

func TestVerifyRecommendations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		wsPort           int
		expectedWarnings []string
	}{
		{
			name:             "Safe Port",
			wsPort:           8080,
			expectedWarnings: nil,
		},
		{
			name:             "Privileged Port",
			wsPort:           80,
			expectedWarnings: []string{"시스템 예약 포트"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &AppConfig{
				NotifyAPI: NotifyAPIConfig{
					WS: WSConfig{ListenPort: tt.wsPort},
				},
			}

			warnings := cfg.VerifyRecommendations()

			if len(tt.expectedWarnings) == 0 {
				assert.Empty(t, warnings)
			} else {
				assert.NotEmpty(t, warnings)
				for _, expected := range tt.expectedWarnings {
					found := false
					for _, w := range warnings {
						if strings.Contains(w, expected) {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected warning not found: %s", expected)
				}
			}
		})
	}
}

// =============================================================================
// Tests: Default Values
// =============================================================================

func TestAppConstants(t *testing.T) {
	assert.Equal(t, "notify-server", AppName)
	assert.Equal(t, "notify-server.json", DefaultFilename)
	assert.Equal(t, 3, DefaultMaxRetries)
	assert.Equal(t, "2s", DefaultRetryDelay)
}
