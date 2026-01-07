package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Unit Tests: Helper Functions & Factories
// =============================================================================

// validAppConfig returns a valid AppConfig for testing purposes.
func validAppConfig() *AppConfig {
	return &AppConfig{
		Debug: true,
		HTTPRetry: HTTPRetryConfig{
			MaxRetries: 3,
			RetryDelay: 1 * time.Second,
		},
		Notifier: NotifierConfig{
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
						Notifier:          CommandNotifierConfig{Usable: true},
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

// =============================================================================
// Unit Tests: Individual Config Structs
// =============================================================================

func TestHTTPRetryConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       HTTPRetryConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid",
			input:       HTTPRetryConfig{MaxRetries: 3, RetryDelay: time.Second},
			expectError: false,
		},
		{
			name:        "Invalid Delay (Zero)",
			input:       HTTPRetryConfig{MaxRetries: 3, RetryDelay: 0},
			expectError: true,
			errorMsg:    "HTTP 재시도 대기 시간(retry_delay)은 0보다 커야 합니다",
		},
		{
			name:        "Invalid MaxRetries (Negative)",
			input:       HTTPRetryConfig{MaxRetries: -1, RetryDelay: time.Second},
			expectError: true,
			errorMsg:    "HTTP 최대 재시도 횟수(max_retries)는 0 이상이어야 합니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.input.validate(newValidator())
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNotifierConfig_Validate(t *testing.T) {
	t.Parallel()

	validTele := TelegramConfig{ID: "t1", BotToken: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", ChatID: 12345}

	tests := []struct {
		name        string
		input       NotifierConfig
		notifierIDs []string // For reference check
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid",
			input: NotifierConfig{
				DefaultNotifierID: "t1",
				Telegrams:         []TelegramConfig{validTele},
			},
			notifierIDs: []string{"t1"},
			expectError: false,
		},
		{
			name: "Default ID Not Found in IDs list",
			input: NotifierConfig{
				DefaultNotifierID: "missing",
				Telegrams:         []TelegramConfig{validTele},
			},
			notifierIDs: []string{"t1"},
			expectError: true,
			errorMsg:    "알림 설정 내 기본 알림 채널로 설정된 ID('missing')가 정의되지 않았습니다",
		},
		{
			name: "Duplicate Telegram IDs",
			input: NotifierConfig{
				DefaultNotifierID: "t1",
				Telegrams:         []TelegramConfig{validTele, validTele},
			},
			notifierIDs: []string{"t1"},
			expectError: true,
			errorMsg:    "알림 설정 내에 중복된 알림 채널 ID가 존재합니다",
		},
		{
			name: "Invalid Telegram Config (Bad Token)",
			input: NotifierConfig{
				DefaultNotifierID: "t1",
				Telegrams: []TelegramConfig{
					{ID: "t1", BotToken: "bad-token", ChatID: 123},
				},
			},
			notifierIDs: []string{"t1"},
			expectError: true,
			errorMsg:    "텔레그램 BotToken 형식이 올바르지 않습니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.input.validate(newValidator(), tt.notifierIDs)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWSConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       WSConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid HTTP",
			input:       WSConfig{ListenPort: 8080},
			expectError: false,
		},
		{
			name: "Valid HTTPS",
			// Note: validation tag 'file' checks file existence.
			// We point to existing file for testing purpose.
			input: WSConfig{
				ListenPort:  443,
				TLSServer:   true,
				TLSCertFile: "config_types.go",
				TLSKeyFile:  "config_types.go",
			},
			expectError: false,
		},
		{
			name:        "Port Too Low",
			input:       WSConfig{ListenPort: 0},
			expectError: true,
			errorMsg:    "웹 서비스 포트(listen_port)는 1에서 65535 사이의 값이어야 합니다",
		},
		{
			name:        "Port Too High",
			input:       WSConfig{ListenPort: 70000},
			expectError: true,
			errorMsg:    "웹 서비스 포트(listen_port)는 1에서 65535 사이의 값이어야 합니다",
		},
		{
			name: "TLS Enabled but Missing Cert",
			input: WSConfig{
				ListenPort: 443,
				TLSServer:  true,
				// Cert missing
				TLSKeyFile: "config_types.go",
			},
			expectError: true,
			errorMsg:    "TLS 서버 활성화 시 TLS 인증서 파일 경로(tls_cert_file)는 필수입니다",
		},
		{
			name: "TLS Cert File Not Found",
			input: WSConfig{
				ListenPort:  443,
				TLSServer:   true,
				TLSCertFile: "not-found.pem",
				TLSKeyFile:  "config_types.go",
			},
			expectError: true,
			errorMsg:    "지정된 TLS 인증서 파일(tls_cert_file)을 찾을 수 없습니다",
		},
		{
			name: "TLS Enabled but Missing Key",
			input: WSConfig{
				ListenPort:  443,
				TLSServer:   true,
				TLSCertFile: "config_types.go",
				// Key missing
			},
			expectError: true,
			errorMsg:    "TLS 서버 활성화 시 TLS 키 파일 경로(tls_key_file)는 필수입니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.input.validate(newValidator())
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCORSConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       CORSConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid Wildcard",
			input:       CORSConfig{AllowOrigins: []string{"*"}},
			expectError: false,
		},
		{
			name:        "Valid Mixed",
			input:       CORSConfig{AllowOrigins: []string{"https://a.com", "http://b.com:8080"}},
			expectError: false,
		},
		{
			name:        "Empty Origins",
			input:       CORSConfig{AllowOrigins: []string{}},
			expectError: true,
			errorMsg:    "CORS 허용 도메인(allow_origins) 목록이 비어있습니다",
		},
		{
			name:        "Wildcard Mixed with Others",
			input:       CORSConfig{AllowOrigins: []string{"*", "https://a.com"}},
			expectError: true,
			errorMsg:    "CORS 허용 도메인(allow_origins)에서 와일드카드(*)는 다른 도메인과 함께 사용할 수 없습니다",
		},
		{
			name:        "Invalid Origin Format",
			input:       CORSConfig{AllowOrigins: []string{"just-string"}},
			expectError: true,
			errorMsg:    "CORS Origin 형식이 올바르지 않습니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.input.validate(newValidator())
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// Unit Tests: AppConfig Integration Validation
// =============================================================================

func TestAppConfig_Validate_Integration(t *testing.T) {
	t.Parallel()

	t.Run("Valid Full Configuration", func(t *testing.T) {
		cfg := validAppConfig()
		err := cfg.validate(newValidator())
		assert.NoError(t, err)
	})

	t.Run("Tasks Validation", func(t *testing.T) {
		t.Run("Duplicate Task IDs", func(t *testing.T) {
			cfg := validAppConfig()
			cfg.Tasks = append(cfg.Tasks, TaskConfig{
				ID:       "task-1", // Duplicate
				Commands: []CommandConfig{{ID: "c1"}},
			})
			err := cfg.validate(newValidator())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "작업 설정 내에 중복된 작업(Task) ID가 존재합니다")
		})

		t.Run("Command Referencing Missing Notifier", func(t *testing.T) {
			cfg := validAppConfig()
			cfg.Tasks[0].Commands[0].DefaultNotifierID = "missing-notifier"
			err := cfg.validate(newValidator())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "참조하는 알림 채널(ID: 'missing-notifier')이 정의되지 않았습니다")
		})

		t.Run("Silent Command Should Ignore Missing Notifier", func(t *testing.T) {
			cfg := validAppConfig()
			cfg.Tasks[0].Commands[0].Notifier.Usable = false
			cfg.Tasks[0].Commands[0].DefaultNotifierID = "missing-notifier-but-ok"
			err := cfg.validate(newValidator())
			assert.NoError(t, err) // Should pass
		})

		t.Run("Invalid Cron Spec in Command", func(t *testing.T) {
			cfg := validAppConfig()
			cfg.Tasks[0].Commands[0].Scheduler.TimeSpec = "invalid"
			err := cfg.validate(newValidator())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "스케줄러(TimeSpec) 설정이 유효하지 않습니다")
		})

		t.Run("Empty Commands in Task", func(t *testing.T) {
			cfg := validAppConfig()
			cfg.Tasks[0].Commands = []CommandConfig{}
			err := cfg.validate(newValidator())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "작업(Task)은 최소 1개 이상의 명령(Command)를 포함해야 합니다")
		})

		t.Run("Duplicate Command IDs within Task", func(t *testing.T) {
			cfg := validAppConfig()
			cfg.Tasks[0].Commands = append(cfg.Tasks[0].Commands, CommandConfig{ID: "cmd-1"}) // Duplicate
			err := cfg.validate(newValidator())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "중복된 명령(Command) ID가 존재합니다")
		})
	})

	t.Run("NotifyAPI Validation", func(t *testing.T) {
		t.Run("App Referencing Missing Notifier", func(t *testing.T) {
			cfg := validAppConfig()
			cfg.NotifyAPI.Applications[0].DefaultNotifierID = "missing-notifier"
			err := cfg.validate(newValidator())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "참조하는 알림 채널(ID: 'missing-notifier')이 정의되지 않았습니다")
		})

		t.Run("Duplicate Application IDs", func(t *testing.T) {
			cfg := validAppConfig()
			cfg.NotifyAPI.Applications = append(cfg.NotifyAPI.Applications, ApplicationConfig{ID: "app-1"})
			err := cfg.validate(newValidator())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "알림 API 설정 내에 중복된 애플리케이션(Application) ID가 존재합니다")
		})
	})
}

// =============================================================================
// Unit Tests: Other Methods
// =============================================================================

func TestNotifierConfig_GetIDs(t *testing.T) {
	t.Parallel()
	nc := NotifierConfig{
		Telegrams: []TelegramConfig{{ID: "t1"}, {ID: "t2"}},
	}
	ids := nc.GetIDs()
	assert.ElementsMatch(t, []string{"t1", "t2"}, ids)
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

func TestStringers(t *testing.T) {
	t.Parallel()

	t.Run("TelegramConfig Masking", func(t *testing.T) {
		tc := TelegramConfig{
			ID:       "tg-1",
			BotToken: "123456789:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
			ChatID:   12345,
		}
		str := tc.String()
		assert.Contains(t, str, "tg-1")
		assert.NotContains(t, str, "ABC-DEF")
		assert.Contains(t, str, "***")
		assert.Contains(t, str, "ChatID:12345")
	})

	t.Run("ApplicationConfig Masking", func(t *testing.T) {
		ac := ApplicationConfig{
			ID:     "app-1",
			AppKey: "very-secret-key",
		}
		str := ac.String()
		assert.Contains(t, str, "app-1")
		assert.NotContains(t, str, "very-secret-key")
		assert.Contains(t, str, "***")
	})
}
