package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCORSConfig_Validate_Wildcard 와일드카드(*) 관련 테스트
func TestCORSConfig_Validate_Wildcard(t *testing.T) {
	t.Run("와일드카드만 사용 - 유효", func(t *testing.T) {
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
				WS: WSConfig{TLSServer: false, ListenPort: 2443},
				CORS: CORSConfig{
					AllowOrigins: []string{"*"},
				},
				Applications: []ApplicationConfig{},
			},
		}

		err := appConfig.Validate()
		assert.NoError(t, err)
	})

	t.Run("와일드카드와 다른 Origin 함께 사용 - 무효", func(t *testing.T) {
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
				WS: WSConfig{TLSServer: false, ListenPort: 2443},
				CORS: CORSConfig{
					AllowOrigins: []string{"*", "http://localhost:3000"},
				},
				Applications: []ApplicationConfig{},
			},
		}

		err := appConfig.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "와일드카드")
	})
}

// TestCORSConfig_Validate_ValidOrigins 유효한 Origin 형식 테스트
func TestCORSConfig_Validate_ValidOrigins(t *testing.T) {
	validOrigins := []struct {
		name   string
		origin string
	}{
		{"HTTP 프로토콜 + 도메인", "http://example.com"},
		{"HTTPS 프로토콜 + 도메인", "https://example.com"},
		{"도메인 + 포트", "http://example.com:8080"},
		{"서브도메인", "https://api.example.com"},
		{"localhost", "http://localhost"},
		{"localhost + 포트", "http://localhost:3000"},
		{"IP 주소", "http://192.168.1.1"},
		{"IP 주소 + 포트", "http://192.168.1.1:8080"},
		{"HTTPS + IP + 포트", "https://10.0.0.1:443"},
	}

	for _, tc := range validOrigins {
		t.Run(tc.name, func(t *testing.T) {
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
					WS: WSConfig{TLSServer: false, ListenPort: 2443},
					CORS: CORSConfig{
						AllowOrigins: []string{tc.origin},
					},
					Applications: []ApplicationConfig{},
				},
			}

			err := appConfig.Validate()
			assert.NoError(t, err, "Origin: %s는 유효해야 합니다", tc.origin)
		})
	}
}

// TestCORSConfig_Validate_InvalidOrigins 잘못된 Origin 형식 테스트
func TestCORSConfig_Validate_InvalidOrigins(t *testing.T) {
	invalidOrigins := []struct {
		name        string
		origin      string
		expectedErr string
	}{
		{"슬래시로 끝남", "http://example.com/", "슬래시"},
		{"경로 포함", "http://example.com/api", "경로"},
		{"쿼리 스트링 포함", "http://example.com?query=1", "쿼리"},
		{"프로토콜 없음", "example.com", "스키마"},
		{"잘못된 프로토콜 (ftp)", "ftp://example.com", "스키마"},
		{"잘못된 프로토콜 (ws)", "ws://example.com", "스키마"},
		{"프로토콜만", "http://", "슬래시"},
		{"빈 문자열", "", "빈 문자열"},
		{"공백만", "   ", "빈 문자열"},
		{"잘못된 IP 주소", "http://999.999.999.999", "형식"},
		{"포트만", "http://:8080", "형식"},
	}

	for _, tc := range invalidOrigins {
		t.Run(tc.name, func(t *testing.T) {
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
					WS: WSConfig{TLSServer: false, ListenPort: 2443},
					CORS: CORSConfig{
						AllowOrigins: []string{tc.origin},
					},
					Applications: []ApplicationConfig{},
				},
			}

			err := appConfig.Validate()
			assert.Error(t, err, "Origin: %s는 무효해야 합니다", tc.origin)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

// TestCORSConfig_Validate_MultipleOrigins 여러 Origin 조합 테스트
func TestCORSConfig_Validate_MultipleOrigins(t *testing.T) {
	t.Run("여러 유효한 Origin", func(t *testing.T) {
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
				WS: WSConfig{TLSServer: false, ListenPort: 2443},
				CORS: CORSConfig{
					AllowOrigins: []string{
						"http://localhost:3000",
						"https://example.com",
						"http://192.168.1.1:8080",
					},
				},
				Applications: []ApplicationConfig{},
			},
		}

		err := appConfig.Validate()
		assert.NoError(t, err)
	})

	t.Run("여러 Origin 중 하나가 무효", func(t *testing.T) {
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
				WS: WSConfig{TLSServer: false, ListenPort: 2443},
				CORS: CORSConfig{
					AllowOrigins: []string{
						"http://localhost:3000",
						"http://example.com/api", // 무효: 경로 포함
						"https://example.com",
					},
				},
				Applications: []ApplicationConfig{},
			},
		}

		err := appConfig.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "경로")
	})
}

// TestCORSConfig_Validate_EmptyOrigins 빈 Origin 리스트 테스트
func TestCORSConfig_Validate_EmptyOrigins(t *testing.T) {
	t.Run("빈 AllowOrigins 리스트", func(t *testing.T) {
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
				WS: WSConfig{TLSServer: false, ListenPort: 2443},
				CORS: CORSConfig{
					AllowOrigins: []string{},
				},
				Applications: []ApplicationConfig{},
			},
		}

		err := appConfig.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "비어있습니다")
	})
}

// TestCORSConfig_Validate_EdgeCases 경계값 테스트
func TestCORSConfig_Validate_EdgeCases(t *testing.T) {
	edgeCases := []struct {
		name        string
		origin      string
		shouldError bool
		errorMsg    string
	}{
		{"최소 포트 번호 (1)", "http://example.com:1", false, ""},
		{"최대 포트 번호 (65535)", "http://example.com:65535", false, ""},
		{"긴 서브도메인", "https://very.long.subdomain.example.com", false, ""},
		{"하이픈 포함 도메인", "https://my-domain.com", false, ""},
		{"숫자 포함 도메인", "https://example123.com", false, ""},
		{"localhost IPv6 (지원하지 않음)", "http://[::1]", true, "형식"},
		{"대문자 도메인", "HTTP://EXAMPLE.COM", true, "형식"},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
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
					WS: WSConfig{TLSServer: false, ListenPort: 2443},
					CORS: CORSConfig{
						AllowOrigins: []string{tc.origin},
					},
					Applications: []ApplicationConfig{},
				},
			}

			err := appConfig.Validate()
			if tc.shouldError {
				assert.Error(t, err, "Origin: %s는 무효해야 합니다", tc.origin)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				assert.NoError(t, err, "Origin: %s는 유효해야 합니다", tc.origin)
			}
		})
	}
}
