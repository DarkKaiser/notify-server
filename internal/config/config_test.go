package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/go-viper/mapstructure/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Unit Tests: Helper Functions
// =============================================================================

func TestNormalizeEnvKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simle Key",
			input:    "NOTIFY_DEBUG",
			expected: "debug",
		},
		{
			name:     "Nested Key (Double Underscore)",
			input:    "NOTIFY_HTTP_RETRY__MAX_RETRIES",
			expected: "http_retry.max_retries",
		},
		{
			name:     "Deeply Nested Key",
			input:    "NOTIFY_NOTIFY_API__CORS__ALLOW_ORIGINS",
			expected: "notify_api.cors.allow_origins",
		},
		{
			name:     "No Prefix",
			input:    "DEBUG",
			expected: "debug", // Prefix removal is just TrimPrefix, rest is Lowercased
		},
		{
			name:     "Mixed Case",
			input:    "NOTIFY_Mixed_Case__Key",
			expected: "mixed_case.key",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeEnvKey(tt.input)
			assert.Equal(t, tt.expected, got, "Input: %s", tt.input)
		})
	}
}

func TestNewDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := newDefaultConfig()

	assert.False(t, cfg.Debug, "Default Debug check")
	assert.Equal(t, DefaultMaxRetries, cfg.HTTPRetry.MaxRetries, "Default MaxRetries check")
	assert.Equal(t, DefaultRetryDelay, cfg.HTTPRetry.RetryDelay, "Default RetryDelay check")
	assert.Empty(t, cfg.Notifier.Telegrams, "Default Notifiers check")
	assert.Empty(t, cfg.Tasks, "Default Tasks check")
}

// =============================================================================
// Unit Tests: Duration Parsing (Mapstructure)
// =============================================================================

func TestDurationParsing(t *testing.T) {
	t.Parallel()

	type DurationConfig struct {
		Delay time.Duration `mapstructure:"delay"`
	}

	tests := []struct {
		name     string
		input    interface{}
		expected time.Duration
	}{
		{
			name:     "String Seconds",
			input:    "10s",
			expected: 10 * time.Second,
		},
		{
			name:     "String Milliseconds",
			input:    "500ms",
			expected: 500 * time.Millisecond,
		},
		{
			name:     "String Minutes",
			input:    "1m",
			expected: 1 * time.Minute,
		},
		{
			name:     "Integer (Nanoseconds Default)",
			input:    1000,
			expected: 1000 * time.Nanosecond,
		},
		{
			name:     "Explicit Zero",
			input:    0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inputMap := map[string]interface{}{
				"delay": tt.input,
			}

			var result DurationConfig
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				WeaklyTypedInput: true,
				DecodeHook: mapstructure.ComposeDecodeHookFunc(
					mapstructure.StringToTimeDurationHookFunc(),
				),
				Result: &result,
			})
			require.NoError(t, err)

			err = decoder.Decode(inputMap)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, result.Delay)
		})
	}
}

// =============================================================================
// Integration Tests: Load Function
// =============================================================================

func TestLoad_Integration(t *testing.T) {
	// Note: We do NOT use t.Parallel() here because we are manipulating environment variables.
	// Parallel execution could cause race conditions or interference between tests.

	// Helper to create temp config file
	createConfigFile := func(t *testing.T, content string) string {
		t.Helper()
		dir := t.TempDir()
		path := filepath.Join(dir, "config.json")
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
		return path
	}

	// Minimally valid JSON to pass validation (requires at least one notifier, default_notifier_id, and CORS)
	minimalValidJSON := `{
		"notifier": {
			"default_notifier_id": "test-bot",
			"telegrams": [{ "id": "test-bot", "bot_token": "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", "chat_id": 12345 }]
		},
		"notify_api": {
			"cors": { "allow_origins": ["*"] }
		}
	}`

	tests := []struct {
		name          string
		fileContent   string // If empty, file is not created (simulating file not found if filename passed)
		envVars       map[string]string
		useInvalidDir bool // If true, force file load error
		wantErr       bool
		errType       apperrors.ErrorType
		errMsg        string
		wantWarnings  bool
		validate      func(*testing.T, *AppConfig)
	}{
		{
			name:        "Success: Valid Defaults with Minimal Config",
			fileContent: minimalValidJSON,
			validate: func(t *testing.T, c *AppConfig) {
				assert.Equal(t, DefaultMaxRetries, c.HTTPRetry.MaxRetries)
				assert.False(t, c.Debug) // Default is false
				assert.Equal(t, DefaultListenPort, c.NotifyAPI.WS.ListenPort)
			},
		},
		{
			name: "Success: File Overrides Defaults",
			fileContent: `{ 
				"debug": true, 
				"http_retry": { "max_retries": 10 },
				"notifier": {
					"default_notifier_id": "file-bot",
					"telegrams": [{ "id": "file-bot", "bot_token": "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", "chat_id": 1 }]
				},
				"notify_api": {
					"cors": { "allow_origins": ["*"] }
				}
			}`,
			validate: func(t *testing.T, c *AppConfig) {
				assert.True(t, c.Debug)
				assert.Equal(t, 10, c.HTTPRetry.MaxRetries)
				assert.Equal(t, DefaultRetryDelay, c.HTTPRetry.RetryDelay) // Preserved default
			},
		},
		{
			name:        "Success: Env Overrides File",
			fileContent: minimalValidJSON,
			envVars: map[string]string{
				"NOTIFY_DEBUG":                   "true",
				"NOTIFY_HTTP_RETRY__MAX_RETRIES": "99",
			},
			validate: func(t *testing.T, c *AppConfig) {
				assert.True(t, c.Debug, "Env should override File")
				assert.Equal(t, 99, c.HTTPRetry.MaxRetries, "Env should override File")
			},
		},
		{
			name:        "Success: Partial Config Merges",
			fileContent: `{ "notifier": { "default_notifier_id": "custom", "telegrams": [{ "id": "custom", "bot_token": "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", "chat_id": 1 }] }, "notify_api": { "cors": { "allow_origins": ["*"] } } }`,
			validate: func(t *testing.T, c *AppConfig) {
				assert.Equal(t, "custom", c.Notifier.DefaultNotifierID)
				assert.False(t, c.Debug, "Missing fields should retain defaults")
			},
		},
		{
			name:        "Success: Nested Env Override",
			fileContent: minimalValidJSON,
			envVars: map[string]string{
				"NOTIFY_NOTIFY_API__WS__LISTEN_PORT": "9090",
			},
			validate: func(t *testing.T, c *AppConfig) {
				assert.Equal(t, 9090, c.NotifyAPI.WS.ListenPort)
			},
		},
		{
			name:        "Success: Duration Parsing (String)",
			fileContent: minimalValidJSON,
			envVars: map[string]string{
				"NOTIFY_HTTP_RETRY__RETRY_DELAY": "5m",
			},
			validate: func(t *testing.T, c *AppConfig) {
				assert.Equal(t, 5*time.Minute, c.HTTPRetry.RetryDelay)
			},
		},
		{
			name:        "Success: Warnings Generated (Well-known Port)",
			fileContent: minimalValidJSON,
			envVars: map[string]string{
				"NOTIFY_NOTIFY_API__WS__LISTEN_PORT": "80",
			},
			wantWarnings: true,
			validate: func(t *testing.T, c *AppConfig) {
				assert.Equal(t, 80, c.NotifyAPI.WS.ListenPort)
			},
		},
		{
			name:          "Failure: File Not Found",
			fileContent:   "",   // Will trigger file not found if we pass a non-existent path
			useInvalidDir: true, // Marker to pass invalid path
			wantErr:       true,
			errType:       apperrors.System,
			errMsg:        "설정 파일을 찾을 수 없습니다",
		},
		{
			name:        "Failure: Malformed JSON",
			fileContent: `{ "debug": tr__ue }`, // Invalid JSON
			wantErr:     true,
			errType:     apperrors.InvalidInput,
			errMsg:      "설정 파일 로드 중 오류가 발생하였습니다",
		},
		{
			name:        "Failure: Validation Error (Logic)",
			fileContent: `{ "notify_api": { "ws": { "listen_port": -1 } }, "notifier": { "default_notifier_id": "test-bot", "telegrams": [{ "id": "test-bot", "bot_token": "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", "chat_id": 12345 }] } }`,
			wantErr:     true,
			errType:     apperrors.InvalidInput,
			errMsg:      "유효성 검증에 실패하였습니다",
		},
		{
			name:        "Failure: Type Mismatch in Env",
			fileContent: minimalValidJSON,
			envVars: map[string]string{
				"NOTIFY_HTTP_RETRY__MAX_RETRIES": "invalid-number",
			},
			// Koanf/mapstructure with WeaklyTypedInput might zero-value it or error depending on parser.
			// Let's verify actual behavior. Likely an unmarshal error or zero value.
			// After testing, mapstructure often returns error for incompatible types even with WeaklyTypedInput.
			wantErr: true,
			errMsg:  "설정값을 구조체에 매핑하지 못했습니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Setup Env
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// 2. Setup File
			var filePath string
			if tt.useInvalidDir {
				filePath = "non-existent-config.json"
			} else {
				filePath = createConfigFile(t, tt.fileContent)
			}

			// 3. Execution
			cfg, warnings, err := LoadWithFile(filePath)

			// 4. Assertion
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				if tt.errType != apperrors.Unknown {
					var appErr *apperrors.AppError
					if errors.As(err, &appErr) {
						assert.True(t, apperrors.Is(err, tt.errType))
					}
				}
				assert.Nil(t, cfg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				if tt.validate != nil {
					tt.validate(t, cfg)
				}

				// Check warnings if expected
				if tt.wantWarnings {
					assert.NotEmpty(t, warnings, "Expected warnings but got empty")
				}
			}
		})
	}
}

// =============================================================================
// Unit Tests: Secret Masking
// =============================================================================

func TestConfig_StringMasking(t *testing.T) {
	t.Parallel()

	t.Run("TelegramConfig Masking", func(t *testing.T) {
		cfg := TelegramConfig{
			ID:       "test-bot",
			BotToken: "123456789:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
			ChatID:   12345,
		}
		str := cfg.String()
		// strutil.Mask (length > 12): Prefix(4) + *** + Suffix(4)
		// 1234...ew11
		assert.Contains(t, str, "BotToken:1234***ew11")
		assert.NotContains(t, str, "ABC-DEF")
	})

	t.Run("ApplicationConfig Masking", func(t *testing.T) {
		cfg := ApplicationConfig{
			ID:     "test-app",
			AppKey: "secret-key-value",
		}
		str := cfg.String()
		// strutil.Mask (length > 12): Prefix(4) + *** + Suffix(4)
		// secr...alue
		assert.Contains(t, str, "AppKey:secr***alue")
		assert.NotContains(t, str, "et-key-v")
	})

	t.Run("Short Secret Masking", func(t *testing.T) {
		cfg := ApplicationConfig{
			ID:     "short-secret",
			AppKey: "12",
		}
		str := cfg.String()
		// strutil.Mask (length <= 3): ***
		assert.Contains(t, str, "AppKey:***")
		assert.NotContains(t, str, "12")
	})
}
