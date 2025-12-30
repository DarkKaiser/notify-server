package validation

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	// urlRegex URL 유효성 검사 정규식
	// 형식: ^https?://[호스트](?::[포트])?(?:/[경로])*$
	//
	// 구성 요소:
	//   - 스키마: http 또는 https (필수)
	//   - 호스트: 다음 중 하나
	//     * 도메인명: 영문자, 숫자, 하이픈, 점으로 구성, 최소 2자 이상의 TLD 필요
	//       예: example.com, api.example.co.kr
	//     * localhost: 로컬 개발 환경 지원
	//     * IPv4 주소: 각 옥텟이 0-255 범위 (예: 192.168.1.1)
	//   - 포트: 선택적, 숫자로 구성 (예: :8080)
	//   - 경로: 선택적, 슬래시로 시작하는 경로 세그먼트 (예: /path/to/resource)
	//
	// 예제:
	//   - https://example.com
	//   - http://localhost:3000
	//   - https://192.168.1.1:8443/api
	urlRegex = regexp.MustCompile(`^https?://(?:[^:@/]+(?::[^@/]+)?@)?(?:[a-zA-Z0-9.-]+(?:\.[a-zA-Z]{2,})+|localhost|(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?))(?::\d+)?(?:[/?#].*)?$`)
)

// ValidateCORSOrigin 주어진 문자열이 유효한 CORS(Cross-Origin Resource Sharing) Origin 표준을 준수하는지 검증합니다.
//
// 이 함수는 'Scheme://Host[:Port]' 형식을 엄격하게 요구하며, 와일드카드('*')를 지원합니다.
//
// 검증 규칙:
//   - 특수 값: '*' (모든 출처 허용)는 유효합니다.
//   - 스키마: 'http' 또는 'https'만 허용됩니다.
//   - 호스트: 도메인명, 로컬호스트(localhost), 또는 유효한 IPv4 주소여야 합니다.
//
// 제약 사항 (다음 요소 포함 시 유효하지 않음):
//   - 경로 (Path) 및 후행 슬래시 ('/')
//   - 쿼리 스트링 (Query String)
//   - URL 프래그먼트/해시 (Fragment)
func ValidateCORSOrigin(origin string) error {
	if origin == "*" {
		return nil
	}

	trimmedOrigin := strings.TrimSpace(origin)
	if trimmedOrigin == "" {
		return fmt.Errorf("CORS Origin은 비어있을 수 없습니다")
	}

	if strings.HasSuffix(origin, "/") {
		return fmt.Errorf("CORS Origin 포맷 오류: 경로 구분자('/')로 끝날 수 없습니다 (input=%q)", origin)
	}

	parsedURL, err := url.Parse(origin)
	if err != nil {
		return fmt.Errorf("CORS Origin 파싱 실패: 유효한 URL 형식이 아닙니다 (input=%q): %w", origin, err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("CORS Origin 스키마 오류: 'http' 또는 'https'만 허용됩니다 (input=%q)", origin)
	}

	if parsedURL.Path != "" && parsedURL.Path != "/" {
		return fmt.Errorf("CORS Origin 포맷 오류: 경로(Path)를 포함할 수 없습니다 (input=%q)", origin)
	}

	if parsedURL.RawQuery != "" {
		return fmt.Errorf("CORS Origin 포맷 오류: 쿼리 파라미터를 포함할 수 없습니다 (input=%q)", origin)
	}

	if parsedURL.Fragment != "" {
		return fmt.Errorf("CORS Origin 포맷 오류: URL Fragment(#)를 포함할 수 없습니다 (input=%q)", origin)
	}

	// 호스트 검증 (정규식 사용)
	if !urlRegex.MatchString(origin) {
		return fmt.Errorf("CORS Origin 형식 불일치: 호스트 또는 포트 형식이 올바르지 않습니다 (input=%q)", origin)
	}

	return nil
}
