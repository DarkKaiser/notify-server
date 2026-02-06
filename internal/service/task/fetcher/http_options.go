package fetcher

import (
	"net/http"
	"time"
)

// Option HTTPFetcher의 설정을 변경하기 위한 함수 타입입니다.
//
// Functional Options 패턴을 사용하여 선택적 매개변수를 유연하게 설정할 수 있습니다.
// 각 Option 함수는 HTTPFetcher의 특정 필드를 수정합니다.
type Option func(*HTTPFetcher)

// WithTimeout HTTP 요청 전체에 대한 타임아웃을 설정합니다.
//
// 이 타임아웃은 요청 시작부터 응답 완료까지의 전체 시간을 제한합니다.
// DNS 조회, 연결, TLS 핸드셰이크, 응답 헤더, 응답 본문 읽기 등 모든 단계를 포함합니다.
//
// 매개변수:
//   - timeout: 요청 전체에 대한 타임아웃 (예: 30*time.Second)
//
// 주의사항:
//   - 0 또는 음수 값을 설정하면 타임아웃이 비활성화됨 (무한 대기)
func WithTimeout(timeout time.Duration) Option {
	return func(h *HTTPFetcher) {
		h.client.Timeout = timeout
	}
}

// WithTLSHandshakeTimeout TLS 핸드셰이크 타임아웃을 설정합니다.
//
// HTTPS 연결 시 SSL/TLS 협상에 허용되는 최대 시간입니다.
// 네트워크가 느리거나 서버 부하가 높을 때 타임아웃이 발생할 수 있습니다.
//
// 매개변수:
//   - timeout: TLS 핸드셰이크 타임아웃 (기본값: 10초)
//
// 권장값:
//   - 일반적으로 5~10초 권장
//   - 모바일 네트워크나 느린 환경에서는 15~20초 고려
func WithTLSHandshakeTimeout(timeout time.Duration) Option {
	return func(h *HTTPFetcher) {
		h.tlsHandshakeTimeout = timeout
	}
}

// WithResponseHeaderTimeout HTTP 응답 헤더 대기 타임아웃을 설정합니다.
//
// 이 타임아웃은 요청을 보낸 후 응답 헤더를 받을 때까지의 시간을 제한합니다.
// 응답 본문 읽기 시간은 포함되지 않으므로, 전체 타임아웃(WithTimeout)과 함께 사용하세요.
//
// 매개변수:
//   - timeout: 응답 헤더 대기 타임아웃 (예: 10*time.Second)
//
// 사용 시나리오:
//   - 서버가 연결은 수락했지만 응답을 보내지 않는 경우 감지
//   - 느린 서버로부터 빠르게 타임아웃하여 재시도
func WithResponseHeaderTimeout(timeout time.Duration) Option {
	return func(h *HTTPFetcher) {
		h.responseHeaderTimeout = timeout
	}
}

// WithIdleConnTimeout 유휴 연결이 닫히기 전 유지되는 타임아웃을 설정합니다.
//
// 사용되지 않는 연결을 유지할 최대 시간입니다.
// 이 시간이 지나면 연결이 자동으로 닫히고 풀에서 제거됩니다.
//
// 매개변수:
//   - timeout: 유휴 연결이 닫히기 전 유지되는 타임아웃 (기본값: 90초)
//
// 권장값:
//   - 일반적으로 30~90초 권장
//   - 너무 짧으면 연결 재사용률 감소
//   - 너무 길면 서버 측에서 먼저 연결을 끊을 수 있음
func WithIdleConnTimeout(timeout time.Duration) Option {
	return func(h *HTTPFetcher) {
		h.idleConnTimeout = timeout
	}
}

// WithProxy HTTP 클라이언트에 프록시 서버를 설정합니다.
//
// 모든 HTTP/HTTPS 요청이 지정된 프록시 서버를 통해 전송됩니다.
// 프록시 서버 URL 형식: "http://proxy.example.com:8080" 또는 "http://user:pass@proxy.example.com:8080"
//
// 매개변수:
//   - proxyURL: 프록시 서버 주소 (빈 문자열이면 기본 설정(환경 변수 HTTP_PROXY 등)을 따름)
//
// 주의사항:
//   - 잘못된 URL 형식은 초기화 시 에러 발생
//   - 프록시 서버 인증 정보(비밀번호)는 로그에 마스킹되어 출력됨
func WithProxy(proxyURL string) Option {
	return func(h *HTTPFetcher) {
		h.proxyURL = proxyURL
	}
}

// WithMaxIdleConns 전체 유휴 연결의 최대 개수를 설정합니다.
//
// 매개변수:
//   - max: 전체 유휴 연결의 최대 개수
//     · 0: 무제한
//     · 양수: 지정된 개수로 제한
//     · 음수: 기본값으로 보정
func WithMaxIdleConns(max int) Option {
	// 전체 유휴 연결의 최대 개수를 정규화합니다.
	max = normalizeMaxIdleConns(max)

	return func(h *HTTPFetcher) {
		h.maxIdleConns = max
	}
}

// WithMaxIdleConnsPerHost 호스트당 유휴 연결의 최대 개수를 설정합니다.
//
// 매개변수:
//   - max: 호스트당 유휴 연결의 최대 개수
//     · 0: net/http가 기본값 2로 해석
//     · 양수: 지정된 개수로 제한
//     · 음수: 기본값으로 보정
func WithMaxIdleConnsPerHost(max int) Option {
	// 호스트당 유휴 연결의 최대 개수를 정규화합니다.
	max = normalizeMaxIdleConnsPerHost(max)

	return func(h *HTTPFetcher) {
		h.maxIdleConnsPerHost = max
	}
}

// WithMaxConnsPerHost 호스트당 최대 연결 개수를 설정합니다.
//
// 매개변수:
//   - max: 호스트당 최대 연결 개수
//     · 0: 무제한
//     · 양수: 지정된 개수로 제한
//     · 음수: 기본값으로 보정
func WithMaxConnsPerHost(max int) Option {
	// 호스트당 최대 연결 개수를 정규화합니다.
	max = normalizeMaxConnsPerHost(max)

	return func(h *HTTPFetcher) {
		h.maxConnsPerHost = max
	}
}

// WithUserAgent 기본 User-Agent를 설정합니다.
//
// 이 옵션으로 설정한 User-Agent는 요청 헤더에 User-Agent가 없을 때만 자동으로 추가됩니다.
// 요청 헤더에 이미 User-Agent가 설정되어 있으면 그 값이 우선적으로 사용됩니다.
//
// 매개변수:
//   - ua: User-Agent 문자열 (예: "MyBot/1.0", "Mozilla/5.0 ...")
func WithUserAgent(ua string) Option {
	return func(h *HTTPFetcher) {
		h.defaultUA = ua
	}
}

// WithMaxRedirects HTTP 클라이언트의 최대 리다이렉트 횟수를 설정합니다.
//
// 기본적으로 Go HTTP 클라이언트는 최대 10번까지 리다이렉트를 따라갑니다.
// 이 옵션으로 제한을 변경할 수 있으며, 리다이렉트 시 Referer 헤더를 자동으로 설정합니다.
//
// 매개변수:
//   - max: 최대 리다이렉트 횟수
//     · 0: 리다이렉트 허용 안 함
//     · 양수: 지정된 횟수만큼 리다이렉트 허용
//     · 음수: 기본값으로 보정
func WithMaxRedirects(max int) Option {
	// 최대 리다이렉트 횟수를 정규화합니다.
	max = normalizeMaxRedirects(max)

	return func(h *HTTPFetcher) {
		h.client.CheckRedirect = newCheckRedirectPolicy(max)
	}
}

// WithTransport HTTP 클라이언트의 Transport를 직접 설정합니다.
//
// 이 옵션을 사용하면 Transport 캐싱이 비활성화되고 제공된 Transport가 그대로 사용됩니다.
// 고급 설정(커스텀 Dialer, TLS 설정 등)이 필요한 경우에만 사용하세요.
//
// 매개변수:
//   - transport: 사용할 http.RoundTripper 구현체 (일반적으로 *http.Transport)
//
// 주의사항:
//   - 이 옵션을 사용하면 다른 Transport 관련 옵션(WithMaxIdleConns 등)이 무시됨
//   - Transport 캐싱이 비활성화되므로 성능이 저하될 수 있음
func WithTransport(transport http.RoundTripper) Option {
	return func(h *HTTPFetcher) {
		h.client.Transport = transport
	}
}

// WithCookieJar HTTP 클라이언트에 쿠키 관리자(CookieJar)를 설정합니다.
//
// CookieJar를 설정하면 HTTP 응답의 Set-Cookie 헤더를 자동으로 저장하고,
// 동일한 도메인에 대한 후속 요청에 쿠키를 자동으로 포함합니다.
//
// 매개변수:
//   - jar: http.CookieJar 구현체 (예: cookiejar.New(nil))
//
// 사용 예시:
//
//	jar, _ := cookiejar.New(nil)
//	fetcher := NewHTTPFetcher(WithCookieJar(jar))
func WithCookieJar(jar http.CookieJar) Option {
	return func(h *HTTPFetcher) {
		h.client.Jar = jar
	}
}
