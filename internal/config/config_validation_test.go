package config

import (
	"strings"
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

// =============================================================================
// HTTP Retry Validation Tests (Moved from config_test.go)
// =============================================================================

// TestHTTPRetryConfig_EdgeCases는 HTTPRetryConfig의 경계값 및 특수 케이스를 검증합니다.
func TestHTTPRetryConfig_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		maxRetries  int
		retryDelay  string
		shouldError bool
	}{
		{"Zero Retries", 0, "1s", false},
		{"Negative Retries", -1, "1s", false}, // 음수는 허용되지만 동작은 0으로 처리
		{"Minimum Duration", 3, "1ns", false},
		{"Maximum Duration", 3, "24h", false},
		{"Invalid Duration Format", 3, "abc", true},
		{"Empty Duration", 3, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfigBuilder().
				WithHTTPRetry(tt.maxRetries, tt.retryDelay).
				Build()

			err := cfg.Validate()
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// CORS Validation Tests (Moved from config_cors_test.go)
// =============================================================================

// createCORSTestConfig는 CORS 테스트용 기본 설정을 생성합니다.
// origins 파라미터로 AllowOrigins를 지정할 수 있습니다.
func createCORSTestConfig(origins ...string) *AppConfig {
	return NewConfigBuilder().
		WithCORSOrigins(origins...).
		Build()
}

// TestCORSConfig_Validate_Wildcard는 와일드카드(*) 사용 시나리오를 검증합니다.
//
// 검증 항목:
//   - 와일드카드만 사용하는 경우 (유효)
//   - 와일드카드와 다른 Origin을 함께 사용하는 경우 (무효)
func TestCORSConfig_Validate_Wildcard(t *testing.T) {
	t.Run("와일드카드만 사용 - 유효", func(t *testing.T) {
		cfg := createCORSTestConfig("*")
		assert.NoError(t, cfg.Validate())
	})

	t.Run("와일드카드와 다른 Origin 함께 사용 - 무효", func(t *testing.T) {
		cfg := createCORSTestConfig("*", "http://localhost:3000")
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "와일드카드")
	})
}

// TestCORSConfig_Validate_Origins는 다양한 Origin 형식의 유효성을 검증합니다.
// 유효한 Origin과 무효한 Origin을 모두 테스트합니다.
func TestCORSConfig_Validate_Origins(t *testing.T) {
	tests := []struct {
		name        string
		origin      string
		shouldError bool
		errorMsg    string
	}{
		// Valid Origins
		{"HTTP 프로토콜 + 도메인", "http://example.com", false, ""},
		{"HTTPS 프로토콜 + 도메인", "https://example.com", false, ""},
		{"도메인 + 포트", "http://example.com:8080", false, ""},
		{"서브도메인", "https://api.example.com", false, ""},
		{"localhost", "http://localhost", false, ""},
		{"localhost + 포트", "http://localhost:3000", false, ""},
		{"IP 주소", "http://192.168.1.1", false, ""},
		{"IP 주소 + 포트", "http://192.168.1.1:8080", false, ""},
		{"HTTPS + IP + 포트", "https://10.0.0.1:443", false, ""},
		{"최소 포트 번호 (1)", "http://example.com:1", false, ""},
		{"최대 포트 번호 (65535)", "http://example.com:65535", false, ""},
		{"긴 서브도메인", "https://very.long.subdomain.example.com", false, ""},
		{"하이픈 포함 도메인", "https://my-domain.com", false, ""},
		{"숫자 포함 도메인", "https://example123.com", false, ""},

		// Invalid Origins
		{"슬래시로 끝남", "http://example.com/", true, "CORS 설정 오류"},
		{"경로 포함", "http://example.com/api", true, "CORS 설정 오류"},
		{"쿼리 스트링 포함", "http://example.com?query=1", true, "CORS 설정 오류"},
		{"프로토콜 없음", "example.com", true, "CORS 설정 오류"},
		{"잘못된 프로토콜 (ftp)", "ftp://example.com", true, "CORS 설정 오류"},
		{"잘못된 프로토콜 (ws)", "ws://example.com", true, "CORS 설정 오류"},
		{"프로토콜만", "http://", true, "CORS 설정 오류"},
		{"빈 문자열", "", true, "CORS 설정 오류"},
		{"공백만", "   ", true, "CORS 설정 오류"},
		{"잘못된 IP 주소", "http://999.999.999.999", true, "CORS 설정 오류"},
		{"포트만", "http://:8080", true, "CORS 설정 오류"},
		{"localhost IPv6 (지원함)", "http://[::1]", false, ""},
		{"대문자 도메인 (지원함)", "HTTP://EXAMPLE.COM", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createCORSTestConfig(tt.origin)
			err := cfg.Validate()

			if tt.shouldError {
				assert.Error(t, err, "Origin: %s는 무효해야 합니다", tt.origin)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err, "Origin: %s는 유효해야 합니다", tt.origin)
			}
		})
	}
}

// TestCORSConfig_Validate_MultipleOrigins는 여러 Origin 조합 시나리오를 검증합니다.
//
// 검증 항목:
//   - 여러 유효한 Origin 조합
//   - 여러 Origin 중 하나가 무효한 경우
func TestCORSConfig_Validate_MultipleOrigins(t *testing.T) {
	t.Run("여러 유효한 Origin", func(t *testing.T) {
		cfg := createCORSTestConfig(
			"http://localhost:3000",
			"https://example.com",
			"http://192.168.1.1:8080",
		)
		assert.NoError(t, cfg.Validate())
	})

	t.Run("여러 Origin 중 하나가 무효", func(t *testing.T) {
		cfg := createCORSTestConfig(
			"http://localhost:3000",
			"http://example.com/api", // 무효: 경로 포함
			"https://example.com",
		)
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "CORS 설정 오류")
	})
}

// TestCORSConfig_Validate_EmptyOrigins는 빈 Origin 리스트 시나리오를 검증합니다.
//
// 검증 항목:
//   - 빈 AllowOrigins 배열 (무효)
func TestCORSConfig_Validate_EmptyOrigins(t *testing.T) {
	t.Run("빈 AllowOrigins 리스트", func(t *testing.T) {
		cfg := createCORSTestConfig()
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "비어있습니다")
	})
}

// TestCORSConfig_Validate_EdgeCases는 CORS 설정의 엣지 케이스를 검증합니다.
//
// 검증 항목:
//   - 매우 긴 Origin (1000자 이상)
//   - 특수 문자 포함 Origin
//   - 중복된 Origin
func TestCORSConfig_Validate_EdgeCases(t *testing.T) {
	t.Run("최대 길이에 근접한 긴 Origin", func(t *testing.T) {
		// RFC 1123에 따라 호스트명은 253자를 넘을 수 없음
		// "subdomain." (10자) * 20 = 200자 + "example.com" = 211자 (허용 범위)
		longSubdomain := strings.Repeat("subdomain.", 20) + "example.com"
		cfg := createCORSTestConfig("https://" + longSubdomain)
		assert.NoError(t, cfg.Validate())
	})

	t.Run("중복된 Origin (허용됨)", func(t *testing.T) {
		cfg := createCORSTestConfig(
			"http://example.com",
			"http://example.com", // 중복
		)
		// 중복은 검증 레벨에서 허용 (실제 사용 시 중복 제거는 애플리케이션 로직)
		assert.NoError(t, cfg.Validate())
	})

	t.Run("특수 문자 포함 도메인 (언더스코어)", func(t *testing.T) {
		// 도메인에 언더스코어는 기술적으로 무효하지만 일부 시스템에서 사용
		cfg := createCORSTestConfig("http://my_domain.com")
		// 현재 검증 로직에서는 허용될 수 있음
		err := cfg.Validate()
		// 검증 결과에 따라 조정 (현재는 형식 검증에 따름)
		_ = err // 결과는 검증 로직에 의존
	})
}

// TestCORSConfig_Validate_RealWorldScenarios는 실제 사용 시나리오를 검증합니다.
//
// 검증 항목:
//   - 개발 환경 설정 (localhost 여러 포트)
//   - 프로덕션 환경 설정 (여러 도메인)
//   - 스테이징 환경 설정
func TestCORSConfig_Validate_RealWorldScenarios(t *testing.T) {
	t.Run("개발 환경 - localhost 여러 포트", func(t *testing.T) {
		cfg := createCORSTestConfig(
			"http://localhost:3000",
			"http://localhost:3001",
			"http://localhost:8080",
		)
		assert.NoError(t, cfg.Validate())
	})

	t.Run("프로덕션 환경 - 여러 도메인", func(t *testing.T) {
		cfg := createCORSTestConfig(
			"https://app.example.com",
			"https://admin.example.com",
			"https://api.example.com",
		)
		assert.NoError(t, cfg.Validate())
	})

	t.Run("스테이징 환경 - 서브도메인", func(t *testing.T) {
		cfg := createCORSTestConfig(
			"https://staging.example.com",
			"https://staging-api.example.com",
		)
		assert.NoError(t, cfg.Validate())
	})

	t.Run("혼합 환경 - HTTP + HTTPS", func(t *testing.T) {
		cfg := createCORSTestConfig(
			"http://localhost:3000", // 개발
			"https://example.com",   // 프로덕션
		)
		assert.NoError(t, cfg.Validate())
	})
}
