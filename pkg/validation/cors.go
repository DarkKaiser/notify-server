package validation

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ValidateCORSOrigin 주어진 문자열이 유효한 CORS(Cross-Origin Resource Sharing) Origin 표준을 준수하는지 검증합니다.
//
// 이 함수는 'Scheme://Host[:Port]' 형식을 엄격하게 요구하며, 와일드카드('*')를 지원합니다.
//
// 검증 규칙:
//   - 특수 값: '*' (모든 출처 허용)는 유효합니다.
//   - 스키마: 'http' 또는 'https'만 허용됩니다.
//   - 호스트: 도메인명, 로컬호스트(localhost), IPv4 또는 IPv6 주소여야 합니다.
//
// 제약 사항 (다음 요소 포함 시 유효하지 않음):
//   - 경로 (Path) 및 후행 슬래시 ('/')
//   - 쿼리 스트링 (Query String)
//   - URL 프래그먼트/해시 (Fragment)
func ValidateCORSOrigin(origin string) error {
	trimmedOrigin := strings.TrimSpace(origin)
	if trimmedOrigin == "*" {
		return nil
	}

	if trimmedOrigin == "" {
		return fmt.Errorf("CORS Origin은 비어있을 수 없습니다")
	}

	if strings.HasSuffix(trimmedOrigin, "/") {
		return fmt.Errorf("CORS Origin 포맷 오류: 경로 구분자('/')로 끝날 수 없습니다 (input=%q)", trimmedOrigin)
	}

	parsedURL, err := url.Parse(trimmedOrigin)
	if err != nil {
		return fmt.Errorf("CORS Origin 파싱 실패: 유효한 URL 형식이 아닙니다 (input=%q): %w", trimmedOrigin, err)
	}

	// 1. Scheme 검증
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("CORS Origin 스키마 오류: 'http' 또는 'https'만 허용됩니다 (input=%q)", trimmedOrigin)
	}

	// 2. 구성 요소(Path, Query, Fragment, UserInfo) 검증
	if parsedURL.Path != "" && parsedURL.Path != "/" {
		return fmt.Errorf("CORS Origin 포맷 오류: 경로(Path)를 포함할 수 없습니다 (input=%q)", trimmedOrigin)
	}

	if parsedURL.RawQuery != "" {
		return fmt.Errorf("CORS Origin 포맷 오류: 쿼리 파라미터를 포함할 수 없습니다 (input=%q)", trimmedOrigin)
	}

	if parsedURL.Fragment != "" {
		return fmt.Errorf("CORS Origin 포맷 오류: URL Fragment(#)를 포함할 수 없습니다 (input=%q)", trimmedOrigin)
	}

	if parsedURL.User != nil {
		return fmt.Errorf("CORS Origin 포맷 오류: 보안 정책상 사용자 자격 증명(UserInfo)을 포함할 수 없습니다 (input=%q)", trimmedOrigin)
	}

	// 3. Port 검증 (포트가 명시된 경우)
	if portStr := parsedURL.Port(); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("CORS Origin 포트 오류: 포트 번호가 유효하지 않습니다 (input=%q, port=%s)", trimmedOrigin, portStr)
		}
		if err := ValidatePort(port); err != nil {
			return fmt.Errorf("CORS Origin 포트 오류: %w (input=%q)", err, trimmedOrigin)
		}
	}

	// 4. Host 검증
	host := parsedURL.Hostname()
	if host == "" {
		return fmt.Errorf("CORS Origin 포맷 오류: 호스트(Host) 정보가 누락되었습니다 (input=%q)", trimmedOrigin)
	}
	if err := ValidateHostname(host); err != nil {
		return fmt.Errorf("CORS Origin 호스트 유효성 검증 실패: %w", err)
	}

	return nil
}

// ValidatePort 포트 번호가 유효한 범위(1-65535) 내에 있는지 검증합니다.
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("유효한 포트 범위(1-65535)가 아닙니다 (port=%d)", port)
	}
	return nil
}

// ValidateHostname 호스트명이 RFC 1123 표준을 준수하는지, 또는 IP 주소/로컬호스트인지 검증합니다.
//
// 규칙:
//   - localhost 허용
//   - 유효한 IPv4 및 IPv6 주소 허용
//   - 도메인명은 RFC 1123 규칙을 따름 (최대 253자, 레이블당 63자, 영문/숫자/하이픈)
func ValidateHostname(host string) error {
	// 1. localhost 체크
	if host == "localhost" {
		return nil
	}

	// 2. IP 주소 체크 (IPv4, IPv6)
	if ip := net.ParseIP(host); ip != nil {
		return nil
	}

	/*
		3. RFC 1123 도메인/호스트명 형식 검증
		규칙:
		- 전체 길이: 최대 253자
		- 레이블(Label): 점(.)으로 구분된 각 부분
			- 길이: 1 ~ 63자
			- 문자: 영문, 숫자, 하이픈(-)만 허용
			- 시작과 끝: 반드시 영문 또는 숫자 (하이픈으로 시작/끝 불가)
	*/

	if len(host) > 253 {
		return fmt.Errorf("호스트명 전체 길이는 253자를 초과할 수 없습니다 (len=%d)", len(host))
	}

	labels := strings.Split(host, ".")
	for _, label := range labels {
		if len(label) == 0 {
			return fmt.Errorf("호스트명에 빈 레이블(연속된 점 등)이 포함되어 있습니다 (host=%q)", host)
		}
		if len(label) > 63 {
			return fmt.Errorf("각 레이블은 63자를 초과할 수 없습니다 (label=%q)", label)
		}

		// 시작과 끝 문자는 하이픈이 아니어야 함
		if label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("레이블은 하이픈(-)으로 시작하거나 끝날 수 없습니다 (label=%q)", label)
		}

		// 정규식이나 복잡한 파싱 대신 직접 순회하며 검증 (성능 및 명확성)
		for _, r := range label {
			// 허용 문자: 영문( 대소문자), 숫자, 하이픈
			isValidChar := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-'
			if !isValidChar {
				return fmt.Errorf("호스트명은 영문, 숫자, 하이픈(-)으로만 구성되어야 합니다 (invalid_char=%q, host=%q)", r, host)
			}
		}
	}

	// 4. TLD(Top-Level Domain) 숫자 검증
	// RFC 1123에 따라 TLD(마지막 레이블)는 숫자로만 구성될 수 없습니다.
	if len(labels) > 0 {
		lastLabel := labels[len(labels)-1]

		isAllNumeric := true
		for _, r := range lastLabel {
			if r < '0' || r > '9' {
				isAllNumeric = false
				break
			}
		}
		if isAllNumeric {
			return fmt.Errorf("최상위 도메인(TLD)은 숫자로만 구성될 수 없습니다 (tld=%q)", lastLabel)
		}
	}

	return nil
}
