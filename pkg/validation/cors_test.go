package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// CORS Origin Validation Tests
// =============================================================================

// TestValidateCORSOrigin은 CORS Origin 유효성 검사를 검증합니다.
//
// 검증 항목:
//   - 기본 유효성: Wildcard(*), 표준 URL
//   - 네트워크 레이어: IP 주소, 로컬호스트, 다양한 포트
//   - 제약 사항: 경로 금지, 쿼리 금지, Trailing Slash 금지
//   - 포맷 정밀 검증: 스키마(http/https), 호스트 형식
//   - 입력값 검증: 빈 문자열, 공백 처리
func TestValidateCORSOrigin(t *testing.T) {
	tests := []struct {
		name          string // 테스트 케이스 명
		origin        string // 입력 Origin
		wantErr       bool   // 에러 발생 여부
		errorContains string // 포함되어야 할 에러 메시지 (옵션)
	}{
		// =================================================================
		// Valid Cases
		// =================================================================
		{
			name:    "Wildcard",
			origin:  "*",
			wantErr: false,
		},
		{
			name:    "Standard HTTP Domain",
			origin:  "http://example.com",
			wantErr: false,
		},
		{
			name:    "Standard HTTPS Domain",
			origin:  "https://example.com",
			wantErr: false,
		},
		{
			name:    "Subdomain",
			origin:  "https://api.dev.example.com",
			wantErr: false,
		},
		{
			name:    "Localhost",
			origin:  "http://localhost",
			wantErr: false,
		},
		{
			name:    "Localhost with Port",
			origin:  "http://localhost:3000",
			wantErr: false,
		},
		{
			name:    "IP Address (IPv4)",
			origin:  "http://192.168.0.1",
			wantErr: false,
		},
		{
			name:    "IP Address with Port",
			origin:  "https://10.0.0.1:8443",
			wantErr: false,
		},

		// =================================================================
		// Invalid Cases - Input Validation
		// =================================================================
		{
			name:          "Empty String",
			origin:        "",
			wantErr:       true,
			errorContains: "비어있을 수 없습니다",
		},
		{
			name:          "Whitespace Only",
			origin:        "   ",
			wantErr:       true,
			errorContains: "비어있을 수 없습니다",
		},

		// =================================================================
		// Invalid Cases - Format Constraints
		// =================================================================
		{
			name:          "Trailing Slash",
			origin:        "https://example.com/",
			wantErr:       true,
			errorContains: "경로 구분자('/')로 끝날 수 없습니다",
		},
		{
			name:          "Included Path",
			origin:        "https://example.com/api",
			wantErr:       true,
			errorContains: "경로(Path)를 포함할 수 없습니다",
		},
		{
			name:          "Included Query",
			origin:        "https://example.com?q=test",
			wantErr:       true,
			errorContains: "쿼리 파라미터를 포함할 수 없습니다",
		},
		{
			name:          "Included Fragment",
			origin:        "https://example.com#header",
			wantErr:       true,
			errorContains: "URL Fragment(#)를 포함할 수 없습니다",
		},

		// =================================================================
		// Invalid Cases - Scheme Validation
		// =================================================================
		{
			name:          "Unsupported Scheme (FTP)",
			origin:        "ftp://example.com",
			wantErr:       true,
			errorContains: "'http' 또는 'https'만 허용됩니다",
		},
		{
			name:          "Missing Scheme",
			origin:        "example.com",
			wantErr:       true,
			errorContains: "'http' 또는 'https'만 허용됩니다", // 스키마 검증에서 감지
		},
		{
			name:          "Scheme Relative",
			origin:        "//example.com",
			wantErr:       true,
			errorContains: "'http' 또는 'https'만 허용됩니다", // 스키마 검증에서 감지
		},
		{
			name:    "Just Scheme",
			origin:  "https://",
			wantErr: true,
			// errorContains: "유효한 URL 형식이 아닙니다", // 환경에 따라 에러 메시지가 다를 수 있어 생략
		},

		// =================================================================
		// Invalid Cases - Host/Port Format
		// =================================================================
		{
			name:          "Invalid Port (Letters)",
			origin:        "http://example.com:abc",
			wantErr:       true,
			errorContains: "유효한 URL 형식이 아닙니다", // url.Parse에서 에러
		},
		{
			name:          "Invalid Domain (Spaces)",
			origin:        "http://exa mple.com",
			wantErr:       true,
			errorContains: "유효한 URL 형식이 아닙니다", // url.Parse에서 에러
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCORSOrigin(tt.origin)
			if tt.wantErr {
				if assert.Error(t, err) {
					if tt.errorContains != "" {
						assert.Contains(t, err.Error(), tt.errorContains, "에러 메시지에 예상된 문구가 포함되어야 합니다")
					}
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
