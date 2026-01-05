package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Helper Functions
// =============================================================================

// createBaseValidConfig는 검증 테스트용 기본 유효한 설정을 생성합니다.
// 모든 필수 필드가 채워져 있어, 특정 필드만 변경하여 테스트하기 적합합니다.
func createBaseValidConfig() *AppConfig {
	return NewConfigBuilder().Build()
}

// =============================================================================
// Unit Tests: Custom Validators & Infrastructure
// =============================================================================

// TestValidate_Infrastructure_JSONTagName은 검증 실패 시 구조체 필드명 대신
// 'json' 태그에 정의된 이름이 에러 메시지에 반환되는지 확인합니다.
func TestValidate_Infrastructure_JSONTagName(t *testing.T) {
	t.Parallel()

	type TestStruct struct {
		RequiredField string `json:"required_field" validate:"required"`
		OmitField     string `json:"omit_field,omitempty" validate:"required"`
		NoTagField    string `validate:"required"`
		DashTagField  string `json:"-" validate:"required"`
	}

	tests := []struct {
		name          string
		input         TestStruct
		expectedValid bool
		errorContains string
	}{
		{
			name:          "Required Field Missing",
			input:         TestStruct{},
			expectedValid: false,
			errorContains: "required_field", // json tag name
		},
		{
			name:          "Omit Option Handling",
			input:         TestStruct{},
			expectedValid: false,
			errorContains: "omit_field", // json tag name without option
		},
		{
			name:          "No JSON Tag",
			input:         TestStruct{},
			expectedValid: false,
			errorContains: "NoTagField", // fallback to field name
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validate.Struct(tt.input)
			if tt.expectedValid {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			}
		})
	}
}

// TestValidate_Unit_CORSOrigin은 `validateCORSOrigin` 커스텀 밸리데이터 함수 자체를
// 독립적으로 검증합니다. 스키마, 포트, 특수 문자 등 다양한 Edge Case를 다룹니다.
func TestValidate_Unit_CORSOrigin(t *testing.T) {
	t.Parallel()

	type CORSStruct struct {
		Origin string `validate:"cors_origin"`
	}

	tests := []struct {
		name   string
		origin string
		valid  bool
	}{
		// Valid cases
		{"Wildcard", "*", true},
		{"HTTP Localhost", "http://localhost", true},
		{"HTTPS Example", "https://example.com", true},
		{"HTTP with Port", "http://localhost:8080", true},
		{"Subdomain", "https://api.example.com", true},
		{"IP Address", "http://127.0.0.1", true},
		{"IP with Port", "http://192.168.0.1:3000", true},
		{"Hyphenated Domain", "https://my-site.com", true},

		// Invalid cases
		{"Missing Scheme", "example.com", false},
		{"Unsupported Scheme (FTP)", "ftp://example.com", false},
		{"Empty String", "", false},
		{"Just Scheme", "http://", false},
		{"Leading Whitespace", " https://example.com", false},
		{"Trailing Slash", "https://example.com/", false},
		{"Path Included", "https://example.com/api", false},
		{"Query String Included", "https://example.com?q=1", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validate.Struct(CORSStruct{Origin: tt.origin})
			if tt.valid {
				assert.NoError(t, err, "Origin '%s' should be valid", tt.origin)
			} else {
				assert.Error(t, err, "Origin '%s' should be invalid", tt.origin)
			}
		})
	}
}

// =============================================================================
// Integration Tests: AppConfig Validation
// =============================================================================

// TestAppConfig_Validate_HTTPRetry verifies validation logic for HTTP Retry configuration.
func TestAppConfig_Validate_HTTPRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		retryDelay  string
		maxRetries  int
		shouldError bool
	}{
		{"Valid Config", "2s", 3, false},
		{"Zero MaxRetries", "1s", 0, false},
		{"Negative MaxRetries", "1s", -1, false}, // Treated as valid (semantics handled by usage)
		{"Invalid Duration", "invalid", 3, true},
		{"Empty Duration", "", 3, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := createBaseValidConfig()
			cfg.HTTPRetry.RetryDelay = tt.retryDelay
			cfg.HTTPRetry.MaxRetries = tt.maxRetries

			err := cfg.Validate()
			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "HTTP Retry")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestAppConfig_Validate_Scheduler verifies validation logic for Task schedulers.
func TestAppConfig_Validate_Scheduler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		runnable      bool
		timeSpec      string
		shouldError   bool
		errorContains string
	}{
		{"Valid Cron", true, "0 */5 * * * *", false, ""},
		{"Invalid Cron", true, "invalid-cron", true, "Scheduler"},
		{"Disabled Scheduler (Invalid Cron Ignored)", false, "invalid-cron", false, ""}, // Not validated if runnable=false
		{"Disabled Scheduler (Empty Cron)", false, "", false, ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := createBaseValidConfig()
			// Setup a task with specific scheduler config
			cfg.Tasks = []TaskConfig{{
				ID: "task1",
				Commands: []CommandConfig{{
					ID:                "cmd1",
					DefaultNotifierID: cfg.Notifiers.DefaultNotifierID,
					Scheduler: struct {
						Runnable bool   `json:"runnable"`
						TimeSpec string `json:"time_spec"`
					}{Runnable: tt.runnable, TimeSpec: tt.timeSpec},
				}},
			}}

			err := cfg.Validate()
			if tt.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestAppConfig_Validate_Integrity verifies reference integrity between components.
// E.g., Tasks referencing non-existent Notifiers.
func TestAppConfig_Validate_Integrity(t *testing.T) {
	t.Parallel()

	t.Run("Missing Default Notifier ID", func(t *testing.T) {
		cfg := createBaseValidConfig()
		cfg.Notifiers.DefaultNotifierID = "non-existent-id"
		assert.ErrorContains(t, cfg.Validate(), "존재하지 않습니다")
	})

	t.Run("Task referencing unknown Notifier", func(t *testing.T) {
		cfg := createBaseValidConfig()
		cfg.Tasks = []TaskConfig{{
			ID: "task1",
			Commands: []CommandConfig{{
				ID:                "cmd1",
				DefaultNotifierID: "unknown-notifier",
			}},
		}}
		assert.ErrorContains(t, cfg.Validate(), "존재하지 않습니다")
	})

	t.Run("Application referencing unknown Notifier", func(t *testing.T) {
		cfg := createBaseValidConfig()
		cfg.NotifyAPI.Applications = []ApplicationConfig{{
			ID: "app1", AppKey: "key", DefaultNotifierID: "unknown-notifier",
		}}
		assert.ErrorContains(t, cfg.Validate(), "존재하지 않습니다")
	})
}

// TestAppConfig_Validate_Uniqueness verifies duplicate ID detection.
func TestAppConfig_Validate_Uniqueness(t *testing.T) {
	t.Parallel()

	t.Run("Duplicate Notifier IDs", func(t *testing.T) {
		cfg := createBaseValidConfig()
		cfg.Notifiers.Telegrams = append(cfg.Notifiers.Telegrams,
			TelegramConfig{ID: "dup", BotToken: "t", ChatID: 1},
			TelegramConfig{ID: "dup", BotToken: "t", ChatID: 2},
		)
		// Fix default notifier to point to something valid if needed, though uniqueness checks first
		assert.ErrorContains(t, cfg.Validate(), "중복되었습니다")
	})

	t.Run("Duplicate Task IDs", func(t *testing.T) {
		cfg := createBaseValidConfig()
		cfg.Tasks = []TaskConfig{
			{ID: "dup"}, {ID: "dup"},
		}
		assert.ErrorContains(t, cfg.Validate(), "중복되었습니다")
	})

	t.Run("Duplicate Command IDs", func(t *testing.T) {
		cfg := createBaseValidConfig()
		cfg.Tasks = []TaskConfig{{
			ID: "task1",
			Commands: []CommandConfig{
				{ID: "dup", DefaultNotifierID: "telegram1"},
				{ID: "dup", DefaultNotifierID: "telegram1"},
			},
		}}
		assert.ErrorContains(t, cfg.Validate(), "중복되었습니다")
	})
}

// TestAppConfig_Validate_NotifyAPI verifies NotifyAPI configuration rules.
func TestAppConfig_Validate_NotifyAPI(t *testing.T) {
	t.Parallel()

	t.Run("Valid Configuration", func(t *testing.T) {
		cfg := createBaseValidConfig()
		assert.NoError(t, cfg.Validate())
	})

	t.Run("Invalid Listen Port", func(t *testing.T) {
		invalidPorts := []int{-1, 0, 70000}
		for _, port := range invalidPorts {
			cfg := createBaseValidConfig()
			cfg.NotifyAPI.WS.ListenPort = port
			assert.ErrorContains(t, cfg.Validate(), "포트 설정이 올바르지 않습니다")
		}
	})

	t.Run("AppKey Validation", func(t *testing.T) {
		cfg := createBaseValidConfig()
		cfg.NotifyAPI.Applications = []ApplicationConfig{{
			ID: "app1", AppKey: "", DefaultNotifierID: "telegram1",
		}}
		assert.ErrorContains(t, cfg.Validate(), "APP_KEY")
	})

	t.Run("TLS Configuration", func(t *testing.T) {
		base := createBaseValidConfig()
		base.NotifyAPI.WS.TLSServer = true

		t.Run("Missing Cert File", func(t *testing.T) {
			cfg := *base                         // copy
			cfg.NotifyAPI.WS = base.NotifyAPI.WS // copy struct
			cfg.NotifyAPI.WS.TLSCertFile = ""
			cfg.NotifyAPI.WS.TLSKeyFile = "key.pem"
			assert.ErrorContains(t, cfg.Validate(), "인증서 파일 경로(TLSCertFile)는 필수입니다")
		})

		t.Run("Missing Key File", func(t *testing.T) {
			// Create a dummy cert file to satisfy TLSCertFile validation
			validCert := filepath.Join(t.TempDir(), "valid_cert.pem")
			_ = os.WriteFile(validCert, []byte("cert"), 0644)

			cfg := *base
			cfg.NotifyAPI.WS = base.NotifyAPI.WS
			cfg.NotifyAPI.WS.TLSCertFile = validCert
			cfg.NotifyAPI.WS.TLSKeyFile = ""
			assert.ErrorContains(t, cfg.Validate(), "키 파일 경로(TLSKeyFile)는 필수입니다")
		})

		t.Run("File Not Found", func(t *testing.T) {
			cfg := *base
			cfg.NotifyAPI.WS = base.NotifyAPI.WS
			cfg.NotifyAPI.WS.TLSCertFile = "nonexistent.pem"
			cfg.NotifyAPI.WS.TLSKeyFile = "nonexistent.pem"
			assert.ErrorContains(t, cfg.Validate(), "존재하지 않거나 유효하지 않습니다")
		})

		t.Run("Valid Files", func(t *testing.T) {
			// Create dummy files
			tempDir := t.TempDir()
			certPath := filepath.Join(tempDir, "cert.pem")
			keyPath := filepath.Join(tempDir, "key.pem")
			_ = os.WriteFile(certPath, []byte("cert"), 0644)
			_ = os.WriteFile(keyPath, []byte("key"), 0644)

			cfg := *base
			cfg.NotifyAPI.WS = base.NotifyAPI.WS
			cfg.NotifyAPI.WS.TLSCertFile = certPath
			cfg.NotifyAPI.WS.TLSKeyFile = keyPath
			// 만약 TLS 파일 내용까지 검증하는 로직이 없다면 통과
			// (현재 구현상 파일 존재 여부만 file 태그로 검사하므로 통과 예상)
			assert.NoError(t, cfg.Validate())
		})
	})
}

// TestCORSConfig_Scenarios tests real-world usage patterns for CORS.
func TestCORSConfig_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		origins     []string
		shouldError bool
	}{
		{
			name:        "Development (Localhost)",
			origins:     []string{"http://localhost:3000", "http://localhost:8080"},
			shouldError: false,
		},
		{
			name:        "Typescript/React Frontend",
			origins:     []string{"https://myapp.com", "https://admin.myapp.com"},
			shouldError: false,
		},
		{
			name:        "Invalid Mixed with Valid",
			origins:     []string{"https://valid.com", "invalid-domain"},
			shouldError: true,
		},
		{
			name:        "Empty Origin List",
			origins:     []string{},
			shouldError: true,
		},
		{
			name:        "Wildcard Mixed (Invalid)",
			origins:     []string{"*", "https://other.com"},
			shouldError: true,
		},
		{
			name:        "Wildcard Only (Valid)",
			origins:     []string{"*"},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := NewConfigBuilder().WithCORSOrigins(tt.origins...).Build()
			err := cfg.Validate()
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
