package fetcher

import (
	"net/http"
	"net/url"
	"strings"
)

// redactHeaders HTTP 응답 헤더에서 민감한 정보를 마스킹하여 안전한 복사본을 반환합니다.
//
// # 목적
//
// 로깅이나 에러 메시지에 HTTP 헤더를 포함할 때, 인증 토큰이나 쿠키 같은 민감한 정보가
// 노출되지 않도록 보호합니다. 원본 헤더는 변경하지 않고 복사본을 생성하여 마스킹합니다.
//
// # 마스킹 대상 헤더
//
//   - Authorization: Bearer 토큰, Basic 인증 정보 등
//   - Proxy-Authorization: 프록시 인증 정보
//   - Cookie: 세션 쿠키 등 클라이언트 측 인증 정보
//   - Set-Cookie: 서버가 설정하는 쿠키 정보
//
// # 동작 방식
//
// 1. 입력 헤더를 Clone()하여 원본을 보호
// 2. 민감한 헤더 목록을 순회하며 값이 존재하면 "***"로 치환
// 3. 마스킹된 복사본 반환
//
// 매개변수:
//   - h: 마스킹할 HTTP 헤더 (nil 허용)
//
// 반환값:
//   - 민감한 정보가 마스킹된 헤더 복사본 (입력이 nil이면 nil 반환)
func redactHeaders(h http.Header) http.Header {
	if h == nil {
		return nil
	}

	masked := h.Clone()

	sensitive := []string{"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie"}
	for _, key := range sensitive {
		if masked.Get(key) != "" {
			masked.Set(key, "***")
		}
	}

	return masked
}

// redactURL URL에서 민감한 정보를 마스킹하여 안전한 문자열로 반환합니다.
//
// # 목적
//
// 로깅이나 에러 메시지에 URL을 포함할 때, 비밀번호, API 키, 토큰, 프록시 인증 정보 등의 민감한 정보가
// 노출되지 않도록 보호합니다. URL의 구조는 유지하면서 민감한 값만 마스킹합니다.
//
// # 마스킹 대상
//
// 1. **사용자 인증 정보**: `https://user:password@example.com` → `https://user:xxxxx@example.com`
// 2. **쿼리 파라미터 값**: `?token=secret&key=value` → `?token=xxxxx&key=xxxxx`
//
// # 동작 방식
//
// 1. url.Redacted()를 호출하여 사용자 비밀번호 부분을 먼저 마스킹
// 2. 쿼리 파라미터가 있으면 모든 파라미터 값을 "xxxxx"로 치환
// 3. Fragment(#)는 그대로 유지
//
// # 사용 예시
//
//	u, _ := url.Parse("https://admin:secret@api.example.com/v1/users?token=abc123&id=456")
//	safe := redactURL(u)
//	// 결과: "https://admin:xxxxx@api.example.com/v1/users?token=xxxxx&id=xxxxx"
//
// 매개변수:
//   - u: 마스킹할 URL (nil 허용)
//
// 반환값:
//   - 민감한 정보가 마스킹된 URL 문자열 (입력이 nil이면 빈 문자열 반환)
//
// 주의사항:
//   - 원본 URL 객체는 변경되지 않습니다 (불변성 보장)
//   - 파싱 실패 시 기본 마스킹 결과(Redacted())를 반환합니다
func redactURL(u *url.URL) string {
	if u == nil {
		return ""
	}

	// 1. 기본 Redacted() 호출 (user:password 마스킹)
	// u.Redacted()는 내부적으로 u를 복제하여 처리하므로 원본 u는 안전함
	redactedStr := u.Redacted()

	// 2. 쿼리 파라미터가 없으면 그대로 반환
	if u.RawQuery == "" {
		return redactedStr
	}

	// 3. 쿼리 파라미터 값 마스킹
	parsedURL, err := url.Parse(redactedStr)
	if err != nil {
		// 파싱 실패 시 기본 마스킹 결과 반환
		return redactedStr
	}

	query := parsedURL.Query()
	for key := range query {
		query.Set(key, "xxxxx")
	}

	parsedURL.RawQuery = query.Encode()

	return parsedURL.String()
}

// redactRawURL URL 문자열에서 민감한 정보를 마스킹하여 안전한 문자열로 반환합니다.
//
// # 목적
//
// 로깅이나 에러 메시지에 URL을 포함할 때, 비밀번호, API 키, 토큰, 프록시 인증 정보 등의 민감한 정보가
// 노출되지 않도록 보호합니다. URL의 구조는 유지하면서 민감한 값만 마스킹합니다.
//
// # 처리 전략
//
// 1. **정상 파싱 가능한 경우**: `redactURL` 함수를 통해 표준적인 마스킹 수행
//
// 2. **파싱 실패 시 폴백(Fallback)**: 문자열 조작으로 최선의 마스킹 수행
//   - 비표준 형식의 URL(스킴 없는 프록시 주소 등)도 안전하게 처리
//   - @ 기호를 기준으로 인증 정보 부분을 감지하여 마스킹
//
// # 사용 예시
//
//	redactRawURL("http://admin:secret@api.example.com/v1?token=abc123")
//	// 결과: "http://admin:xxxxx@api.example.com/v1?token=xxxxx"
//
//	redactRawURL("user:pass@proxy.internal.com:8080")  // 스킴 없는 프록시 주소
//	// 결과: "xxxxx:xxxxx@proxy.internal.com:8080"
//
//	redactRawURL("https://example.com/public")  // 인증 정보 없음
//	// 결과: "https://example.com/public" (변경 없음)
//
// 매개변수:
//   - rawURL: 마스킹할 URL 문자열
//
// 반환값:
//   - 민감한 정보가 마스킹된 URL 문자열
//
// 주의사항:
//   - 파싱 불가능한 URL도 최선의 노력으로 마스킹을 시도합니다
//   - 원본 URL 문자열은 변경되지 않습니다 (불변성 보장)
func redactRawURL(rawURL string) string {
	u, err := url.Parse(rawURL)

	// 파싱에 실패했거나, 파싱은 됐지만 인증 정보(User)가 없는 경우(예: 스킴 없는 URL)에도
	// 원본에 '@'가 있다면 잠재적인 인증 정보로 간주하여 폴백 로직을 수행합니다.
	if err != nil || (u.User == nil && strings.Contains(rawURL, "@")) {
		// [파싱 실패 또는 불완전 파싱 시 폴백 전략]
		// 비표준 형식의 URL(예: 스킴 없는 프록시 주소)도 안전하게 처리하기 위해
		// 문자열 조작을 통해 최소한의 마스킹을 수행합니다.

		// @ 기호를 찾아 인증 정보 존재 여부 확인
		if authSplitIdx := strings.LastIndex(rawURL, "@"); authSplitIdx != -1 {
			// 스킴(http://, https:// 등)이 있는지 확인
			if schemeSplitIdx := strings.Index(rawURL, "://"); schemeSplitIdx != -1 && authSplitIdx > schemeSplitIdx {
				// 스킴이 있는 경우
				// 스킴 부분은 유지하고, @ 앞의 인증 정보만 마스킹 ("http://user:pass@host" → "http://xxxxx:xxxxx@host")
				return rawURL[:schemeSplitIdx+3] + "xxxxx:xxxxx" + rawURL[authSplitIdx:]
			}

			// 스킴이 없는 경우
			// 프록시 설정 등에서 스킴 없이 입력되는 경우 처리 ("user:pass@host" → "xxxxx:xxxxx@host")
			return "xxxxx:xxxxx" + rawURL[authSplitIdx:]
		}

		// @ 기호가 없으면 인증 정보가 없는 것으로 간주하여 원본 그대로 반환
		return rawURL
	}

	return redactURL(u)
}
