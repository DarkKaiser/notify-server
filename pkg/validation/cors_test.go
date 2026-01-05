package validation_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/pkg/validation"
	"github.com/stretchr/testify/assert"
)

// TestValidateCORSOrigin_Comprehensive 는 CORS Origin 검증의 모든 측면을 테스트합니다.
//
// 테스트 전략:
//  1. [Standard] 표준 웹 오리진 (Scheme + Host + [Port])
//  2. [Security] 보안 취약점 방지 (Null origin, 스크립트 삽입 시도 등)
//  3. [Network] 다양한 네트워크 주소 포맷 (IPv4, IPv6)
//  4. [RFC Rules] RFC 6454 및 URL 스펙 준수 여부
func TestValidateCORSOrigin(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		origin        string
		isValid       bool
		errorContains string
	}

	testGroups := []struct {
		groupName string
		cases     []testCase
	}{
		{
			groupName: "1. Wildcard & Basic Domains",
			cases: []testCase{
				{name: "Wildcard (Allow All)", origin: "*", isValid: true},
				{name: "Simple HTTP", origin: "http://example.com", isValid: true},
				{name: "Simple HTTPS", origin: "https://example.com", isValid: true},
				{name: "Subdomain", origin: "https://api.ver1.example.com", isValid: true},
				{name: "Hyphenated Domain", origin: "https://my-cool-app.com", isValid: true},
			},
		},
		{
			groupName: "2. Port Handling",
			cases: []testCase{
				{name: "Standard HTTP Port", origin: "http://example.com:80", isValid: true},
				{name: "Standard HTTPS Port", origin: "https://example.com:443", isValid: true},
				{name: "Custom Port", origin: "http://localhost:8080", isValid: true},
				{name: "High Port", origin: "http://example.com:65535", isValid: true},
				{name: "Port 0 (Invalid)", origin: "http://example.com:0", isValid: false, errorContains: "유효한 포트 범위"},
				{name: "Port Too High", origin: "http://example.com:65536", isValid: false, errorContains: "유효한 포트 범위"},
				{name: "Non-numeric Port", origin: "http://example.com:abc", isValid: false, errorContains: "유효한 URL 형식이 아닙니다"},
			},
		},
		{
			groupName: "3. IP Address Support",
			cases: []testCase{
				{name: "IPv4 Localhost", origin: "http://127.0.0.1", isValid: true},
				{name: "IPv4 Private", origin: "http://192.168.1.100", isValid: true},
				{name: "IPv4 with Port", origin: "http://10.0.0.1:3000", isValid: true},
				{name: "IPv6 Loopback", origin: "http://[::1]", isValid: true},
				{name: "IPv6 Full", origin: "http://[2001:0db8:85a3:0000:0000:8a2e:0370:7334]", isValid: true},
				{name: "IPv6 with Port", origin: "https://[::1]:9090", isValid: true},
			},
		},
		{
			groupName: "4. Internal & Special Hostnames",
			cases: []testCase{
				{name: "Localhost", origin: "http://localhost", isValid: true},
				{name: "Internal Service DNS", origin: "http://notify-backend", isValid: true},              // K8s service name style
				{name: "Punycode (International Domain)", origin: "https://xn--b60b52j.com", isValid: true}, // "테스트.com"
			},
		},
		{
			groupName: "5. Invalid Formats & Constraints",
			cases: []testCase{
				{name: "Empty String", origin: "", isValid: false, errorContains: "비어있을 수 없습니다"},
				{name: "Whitespace Only", origin: "   ", isValid: false, errorContains: "비어있을 수 없습니다"},
				{name: "Trailing Slash", origin: "https://example.com/", isValid: false, errorContains: "경로 구분자"},
				{name: "With Explicit Path", origin: "https://example.com/api", isValid: false, errorContains: "경로(Path)"},
				{name: "With Query Params", origin: "https://example.com?foo=bar", isValid: false, errorContains: "쿼리 파라미터"},
				{name: "With Fragment", origin: "https://example.com#section", isValid: false, errorContains: "URL Fragment"},
				{name: "With Credentials", origin: "https://user:pass@example.com", isValid: false, errorContains: "자격 증명"},
			},
		},
		{
			groupName: "6. Scheme & Protocol Validation",
			cases: []testCase{
				{name: "Mixed Case HTTP", origin: "HTTP://example.com", isValid: true},   // Should be normalized
				{name: "Mixed Case HTTPS", origin: "HtTpS://example.com", isValid: true}, // Should be normalized
				{name: "FTP Scheme (Unsupported)", origin: "ftp://example.com", isValid: false, errorContains: "http' 또는 'https'만 허용"},
				{name: "File Scheme (Unsupported)", origin: "file:///etc/passwd", isValid: false, errorContains: "http' 또는 'https'만 허용"},
				{name: "Missing Scheme", origin: "example.com", isValid: false, errorContains: "허용됩니다"}, // URL parse error or scheme check
				{name: "Scheme Relative", origin: "//example.com", isValid: false, errorContains: "허용됩니다"},
			},
		},
	}

	for _, tg := range testGroups {
		tg := tg // Capture for closure
		t.Run(tg.groupName, func(t *testing.T) {
			t.Parallel()
			for _, tc := range tg.cases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					err := validation.ValidateCORSOrigin(tc.origin)
					if tc.isValid {
						assert.NoError(t, err, "Origin should be valid: %s", tc.origin)
					} else {
						assert.Error(t, err, "Origin should be invalid: %s", tc.origin)
						if tc.errorContains != "" {
							assert.Contains(t, err.Error(), tc.errorContains)
						}
					}
				})
			}
		})
	}
}

// TestValidateHostname_EdgeCases 는 호스트명 검증의 엣지 케이스를 집중적으로 확인합니다.
func TestValidateHostname_EdgeCases(t *testing.T) {
	t.Parallel()

	// 253자 (Max Valid Length) 생성
	// 'a' * 63 + '.' ...
	// 63자 레이블 4개(252자) + 점 3개 = 255자 -> 너무 김.
	// 60자 * 4 = 240 + 점 3개 = 243. OK.
	longLabel := strings.Repeat("a", 63)
	validLongHost := longLabel + "." + longLabel + "." + longLabel + ".com" // ~190 chars

	// 254자 (Invalid Length)
	tooLongHost := strings.Repeat("a.", 127) + "com" // 256+ chars

	tests := []struct {
		name          string
		host          string
		isValid       bool
		errorContains string
	}{
		// Valid Edge Cases
		{name: "Max Length Label (63 chars)", host: longLabel + ".com", isValid: true},
		{name: "Valid Long Hostname", host: validLongHost, isValid: true},
		{name: "Single Label (Intranet)", host: "localhost", isValid: true},
		{name: "Punycode Domain", host: "xn--b60b52j.com", isValid: true},
		{name: "Numeric Label (Valid for non-TLD)", host: "123.example.com", isValid: true},

		// Invalid Edge Cases
		{name: "Empty Host", host: "", isValid: false, errorContains: "빈 레이블"},
		{name: "Label Too Long (>63)", host: strings.Repeat("a", 64) + ".com", isValid: false, errorContains: "63자를 초과"},
		{name: "Host Too Long (>253)", host: tooLongHost, isValid: false, errorContains: "253자를 초과"},
		{name: "Starts with Hyphen", host: "-example.com", isValid: false, errorContains: "하이픈"},
		{name: "Ends with Hyphen", host: "example-.com", isValid: false, errorContains: "하이픈"},
		{name: "Starts with Dot", host: ".example.com", isValid: false, errorContains: "빈 레이블"},
		{name: "Double Dot", host: "example..com", isValid: false, errorContains: "빈 레이블"},
		{name: "Invalid Characters", host: "ex_ample.com", isValid: false, errorContains: "영문, 숫자, 하이픈"},
		{name: "Numeric TLD", host: "example.123", isValid: false, errorContains: "최상위 도메인"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validation.ValidateHostname(tc.host)
			if tc.isValid {
				assert.NoError(t, err, "Hostname should be valid: %s", tc.host)
			} else {
				assert.Error(t, err, "Hostname should be invalid: %s", tc.host)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			}
		})
	}
}

// TestValidatePort_Boundaries 는 포트 검증의 경계값을 테스트합니다.
func TestValidatePort_Boundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		port    int
		isValid bool
	}{
		{-1, false},
		{0, false},
		{1, true}, // Min Valid
		{80, true},
		{443, true},
		{65535, true}, // Max Valid
		{65536, false},
		{99999, false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("Port %d", tc.port), func(t *testing.T) {
			t.Parallel()
			err := validation.ValidatePort(tc.port)
			if tc.isValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
