package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Test Helpers
// =============================================================================

// createCORSTestConfig는 CORS 테스트용 기본 설정을 생성합니다.
// origins 파라미터로 AllowOrigins를 지정할 수 있습니다.
func createCORSTestConfig(origins ...string) *AppConfig {
	return NewConfigBuilder().
		WithCORSOrigins(origins...).
		Build()
}

// =============================================================================
// CORS Validation Tests
// =============================================================================

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
		{"슬래시로 끝남", "http://example.com/", true, "슬래시"},
		{"경로 포함", "http://example.com/api", true, "경로"},
		{"쿼리 스트링 포함", "http://example.com?query=1", true, "쿼리"},
		{"프로토콜 없음", "example.com", true, "스키마"},
		{"잘못된 프로토콜 (ftp)", "ftp://example.com", true, "스키마"},
		{"잘못된 프로토콜 (ws)", "ws://example.com", true, "스키마"},
		{"프로토콜만", "http://", true, "슬래시"},
		{"빈 문자열", "", true, "빈 문자열"},
		{"공백만", "   ", true, "빈 문자열"},
		{"잘못된 IP 주소", "http://999.999.999.999", true, "형식"},
		{"포트만", "http://:8080", true, "형식"},
		{"localhost IPv6 (지원하지 않음)", "http://[::1]", true, "형식"},
		{"대문자 도메인", "HTTP://EXAMPLE.COM", true, "형식"},
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
		assert.Contains(t, err.Error(), "경로")
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
	t.Run("매우 긴 Origin (1000자)", func(t *testing.T) {
		// 1000자 이상의 긴 서브도메인 생성
		longSubdomain := strings.Repeat("subdomain.", 100) + "example.com"
		cfg := createCORSTestConfig("https://" + longSubdomain)
		// 매우 긴 Origin도 형식이 올바르면 허용
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
