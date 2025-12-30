package validation

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidateCORSOrigin은 CORS Origin 유효성 검사 로직을 검증합니다.
//
// 배경:
//
//	Origin 헤더는 브라우저 보안의 핵심 요소이며, 모호함 없이 정확하게 검증되어야 합니다.
//	이 테스트는 표준(RFC 6454), 네트워크 스펙(IPv4/IPv6), 호스트명 규칙(RFC 1123)을 포괄합니다.
//
// 검증 범위:
//  1. [Valid] 표준 스키마(http/https), 도메인, IP(v4/v6), Localhost, Wildcard
//  2. [Invalid] 포맷 위반 (Path, Query, Fragment, UserInfo 포함)
//  3. [Invalid] 스키마 위반 (ftp, file 등)
//  4. [Invalid] 호스트명 규칙 위반 (특수문자, 하이픈 위치, 길이 제한)
//  5. [Invalid] 포트 범위 위반
func TestValidateCORSOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		group         string // 테스트 그룹 (리포팅 용도)
		name          string // 테스트 케이스 명
		origin        string // 입력 Origin
		wantErr       bool   // 에러 발생 여부
		errorContains string // 포함되어야 할 에러 메시지 (옵션)
	}{
		// =================================================================
		// 1. Valid Cases
		// =================================================================
		{group: "Valid", name: "Wildcard", origin: "*"},
		{group: "Valid", name: "HTTP Domain", origin: "http://example.com"},
		{group: "Valid", name: "HTTPS Domain", origin: "https://example.com"},
		{group: "Valid", name: "Subdomain", origin: "https://api.dev.example.com"},
		{group: "Valid", name: "Localhost", origin: "http://localhost"},
		{group: "Valid", name: "Localhost with Port", origin: "http://localhost:3000"},
		{group: "Valid", name: "IPv4", origin: "http://192.168.0.1"},
		{group: "Valid", name: "IPv4 with Port", origin: "https://10.0.0.1:8443"},
		{group: "Valid", name: "IPv6 Loopback", origin: "http://[::1]"},
		{group: "Valid", name: "IPv6 Full", origin: "https://[2001:db8::1]"},
		{group: "Valid", name: "IPv6 with Port", origin: "http://[::1]:8080"},
		{group: "Valid", name: "Internal Hostname", origin: "http://backend"},
		{group: "Valid", name: "Mixed Case Scheme", origin: "HTTP://example.com"}, // url.Parse가 스킴을 소문자로 정규화함

		// =================================================================
		// 2. Input Validation (Empty, Space)
		// =================================================================
		{group: "Input", name: "Empty String", origin: "", wantErr: true, errorContains: "비어있을 수 없습니다"},
		{group: "Input", name: "Whitespace Only", origin: "   ", wantErr: true, errorContains: "비어있을 수 없습니다"},

		// =================================================================
		// 3. Scheme Validation
		// =================================================================
		{group: "Scheme", name: "Unsupported Scheme (FTP)", origin: "ftp://example.com", wantErr: true, errorContains: "허용됩니다"},
		{group: "Scheme", name: "Missing Scheme", origin: "example.com", wantErr: true, errorContains: "허용됩니다"},
		{group: "Scheme", name: "Scheme Relative", origin: "//example.com", wantErr: true, errorContains: "허용됩니다"},
		{group: "Scheme", name: "Just Scheme", origin: "https://", wantErr: true}, // url.Parse 에러 또는 Host 누락

		// =================================================================
		// 4. Format Constraints (Pure Origin Only)
		// =================================================================
		{group: "Format", name: "Trailing Slash", origin: "https://example.com/", wantErr: true, errorContains: "경로 구분자"},
		{group: "Format", name: "Deep Path", origin: "https://example.com/api/v1", wantErr: true, errorContains: "경로(Path)"},
		{group: "Format", name: "Query Parameter", origin: "https://example.com?q=1", wantErr: true, errorContains: "쿼리 파라미터"},
		{group: "Format", name: "Fragment", origin: "https://example.com#home", wantErr: true, errorContains: "URL Fragment"},
		{group: "Format", name: "UserInfo", origin: "https://user:pass@example.com", wantErr: true, errorContains: "사용자 자격 증명"},

		// =================================================================
		// 5. Host & Port Validation
		// =================================================================
		{group: "HostPort", name: "Invalid Port (Letters)", origin: "http://example.com:abc", wantErr: true},
		{group: "HostPort", name: "Invalid Port (Zero)", origin: "http://example.com:0", wantErr: true, errorContains: "유효한 포트 범위"}, // 1-65535
		{group: "HostPort", name: "Invalid Port (Too Large)", origin: "http://example.com:70000", wantErr: true, errorContains: "유효한 포트 범위"},
		{group: "HostPort", name: "Invalid Domain (Spaces)", origin: "http://exa mple.com", wantErr: true},

		// =================================================================
		// 6. RFC 1123 Hostname Rules
		// =================================================================
		{
			group:         "RFC1123",
			name:          "Invalid Char (Underscore)",
			origin:        "http://ex_ample.com",
			wantErr:       true,
			errorContains: "영문, 숫자, 하이픈(-)으로만 구성",
		},
		{
			group:         "RFC1123",
			name:          "Empty Label (Start Dot)",
			origin:        "http://.example.com",
			wantErr:       true,
			errorContains: "빈 레이블",
		},
		{
			group:         "RFC1123",
			name:          "Empty Label (Double Dot)",
			origin:        "http://example..com",
			wantErr:       true,
			errorContains: "빈 레이블",
		},
		{
			group:         "RFC1123",
			name:          "Hyphen at Start",
			origin:        "http://-example.com",
			wantErr:       true,
			errorContains: "하이픈(-)으로 시작하거나 끝날 수 없습니다",
		},
		{
			group:         "RFC1123",
			name:          "Hyphen at End",
			origin:        "http://example-.com",
			wantErr:       true,
			errorContains: "하이픈(-)으로 시작하거나 끝날 수 없습니다",
		},
		{
			group:         "RFC1123",
			name:          "Label Too Long (>63)",
			origin:        "http://" + strings.Repeat("a", 64) + ".com",
			wantErr:       true,
			errorContains: "63자를 초과할 수 없습니다",
		},
		{
			group:   "RFC1123",
			name:    "Max Label Length (63) - Valid",
			origin:  "http://" + strings.Repeat("a", 63) + ".com",
			wantErr: false,
		},
		{
			group:         "RFC1123",
			name:          "Total Host Too Long (>253)",
			origin:        "http://" + strings.Repeat("a.", 130) + "com", // Roughly 260 chars
			wantErr:       true,
			errorContains: "전체 길이는 253자를 초과할 수 없습니다",
		},
		{
			group:         "RFC1123",
			name:          "Invalid TLD (Numeric)",
			origin:        "http://example.123",
			wantErr:       true,
			errorContains: "최상위 도메인(TLD)은 숫자로만 구성될 수 없습니다",
		},
	}

	for _, tt := range tests {
		tt := tt // Capture for closure
		t.Run(fmt.Sprintf("[%s] %s", tt.group, tt.name), func(t *testing.T) {
			t.Parallel()

			err := ValidateCORSOrigin(tt.origin)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains, "에러 메시지 불일치")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
