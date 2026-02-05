package fetcher

import (
	"net/http"
	"time"
)

const (
	// @@@@@
	// defaultTimeout HTTP 클라이언트의 전체 요청 타임아웃
	defaultTimeout = 30 * time.Second

	// @@@@@
	// defaultTLSHandshakeTimeout TLS 핸드셰이크 타임아웃
	defaultTLSHandshakeTimeout = 10 * time.Second

	// @@@@@
	// defaultIdleConnTimeout 유휴 연결이 닫히기 전 유지되는 타임아웃
	defaultIdleConnTimeout = 90 * time.Second

	// @@@@@
	// defaultMaxIdleConns 전체 유휴(Idle) 연결의 최대 개수
	defaultMaxIdleConns = 100

	// defaultMaxTransportCacheSize Transport 재사용 캐시의 최대 개수입니다.
	defaultMaxTransportCacheSize = 100

	// defaultMaxRedirects HTTP 클라이언트의 최대 리다이렉트 횟수입니다.
	defaultMaxRedirects = 10
)

// HTTPFetcher 기본 HTTP 클라이언트 미들웨어입니다.
//
// 주요 기능:
//   - 타임아웃 관리: 전체 요청 타임아웃, 헤더 응답 타임아웃, TLS 핸드셰이크 타임아웃
//   - 연결 풀링: 유휴 연결 재사용, 호스트별 연결 수 제한
//   - 프록시 지원: HTTP/HTTPS 프록시 서버 설정
//   - Transport 캐싱: 동일한 설정의 요청들이 Transport를 공유하여 성능 최적화
//   - User-Agent 관리: 기본 User-Agent 설정 및 요청별 커스터마이징
type HTTPFetcher struct {
	// ========================================
	// 핵심 컴포넌트
	// ========================================

	// client 실제 HTTP 요청을 수행하는 클라이언트입니다.
	client *http.Client

	// ========================================
	// 초기화 상태
	// ========================================

	// initErr 초기화 중 발생한 에러입니다.
	// NewHTTPFetcher 실행 시 옵션 검증이나 Transport 설정 과정에서 에러가 발생하면 여기에 저장됩니다.
	// Do 메서드 호출 시 이 값이 nil이 아니면 즉시 반환하여 잘못된 설정으로 요청을 시도하는 것을 방지합니다.
	initErr error

	// ========================================
	// 타임아웃(Timeout) 설정
	// ========================================

	// @@@@@
	// tlsHandshakeTimeout TLS 핸드셰이크 타임아웃입니다.
	// HTTPS 연결 시 SSL/TLS 협상에 허용되는 최대 시간입니다.
	tlsHandshakeTimeout time.Duration

	// @@@@@
	// responseHeaderTimeout HTTP 응답 헤더 대기 타임아웃입니다.
	// 요청 전송 후 서버로부터 응답 헤더를 받을 때까지 허용되는 최대 시간입니다.
	// 본문(Body) 데이터 수신 시간은 포함되지 않습니다.
	responseHeaderTimeout time.Duration

	// @@@@@
	// idleConnTimeout 유휴 연결이 닫히기 전 유지되는 타임아웃입니다.
	// 연결 풀에서 사용되지 않는 연결이 닫히기 전까지 유지되는 최대 시간입니다.
	idleConnTimeout time.Duration

	// ========================================
	// 네트워크 라우팅
	// ========================================

	// @@@@@
	// proxyURL 프록시 서버 주소입니다.
	// 빈 문자열이면 기본 설정(환경 변수 HTTP_PROXY 등)을 따릅니다.
	// 형식: "http://host:port" 또는 "https://user:pass@host:port"
	proxyURL string

	// ========================================
	// 연결 풀(Connection Pool) 설정
	// ========================================

	// @@@@@
	// maxIdleConns 전체 유휴(Idle) 연결의 최대 개수입니다.
	// 모든 호스트에 대해 유지할 수 있는 유휴 연결의 최대 개수를 제한합니다.
	// 0이면 무제한입니다.
	maxIdleConns int

	// @@@@@
	// maxConnsPerHost 호스트(도메인)당 최대 연결 개수입니다.
	// 동일한 호스트에 대해 동시에 유지할 수 있는 최대 연결 개수를 제한합니다.
	// 0이면 무제한입니다.
	maxConnsPerHost int

	// @@@@@
	// maxIdleConnsPerHost 호스트(도메인)당 최대 유휴(Idle) 연결 개수입니다.
	// 0이면 기본값(2)을 사용하거나 maxIdleConns를 따릅니다.
	maxIdleConnsPerHost int

	// ========================================
	// 성능 최적화 (Performance Optimization)
	// ========================================

	// @@@@@
	// disableTransportCache Transport 캐시 사용 여부입니다.
	// true이면 매번 새로운 Transport를 생성하고, false이면 동일한 설정의 Transport를 재사용합니다.
	disableTransportCache bool

	// ========================================
	// 요청 헤더 설정
	// ========================================

	// defaultUA 기본 User-Agent 문자열입니다.
	// 요청 헤더에 User-Agent가 설정되지 않은 경우 이 값이 자동으로 사용됩니다.
	defaultUA string
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ Fetcher = (*HTTPFetcher)(nil)

// NewHTTPFetcher 새로운 HTTPFetcher 인스턴스를 생성합니다.
//
// 이 함수는 Functional Options 패턴을 사용하여 유연한 설정을 지원합니다.
// 기본값으로 초기화된 후 제공된 옵션들을 순차적으로 적용합니다.
//
// 매개변수:
//   - opts: 설정 옵션들 (가변 인자)
//
// 반환값:
//   - *HTTPFetcher: 설정이 적용된 HTTPFetcher 인스턴스
//
// 주의사항:
//   - 초기화 중 에러 발생 시 initErr 필드에 저장되며, Do 호출 시 반환됨!
//   - Transport 캐싱은 성능 최적화를 위해 기본적으로 활성화됨!
func NewHTTPFetcher(opts ...Option) *HTTPFetcher {
	f := &HTTPFetcher{
		client: &http.Client{
			Timeout: defaultTimeout,

			// 연결 풀 공유 (여러 HTTPFetcher가 연결 풀을 공유하여 성능 최적화)
			Transport: defaultTransport,

			// 리다이렉트 정책: 최대 10회까지 허용하며 Referer 헤더를 자동 설정
			CheckRedirect: newCheckRedirectPolicy(defaultMaxRedirects),
		},

		tlsHandshakeTimeout: DefaultTLSHandshakeTimeout,
		idleConnTimeout:     DefaultIdleConnTimeout,

		maxIdleConns: DefaultMaxIdleConns,

		defaultUA: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}

	// 설정 옵션 적용 (일부 옵션은 검증 실패 시 initErr을 설정할 수 있음)
	for _, opt := range opts {
		opt(f)
	}

	// Transport 설정: 캐시 정책에 따라 공유 Transport 재사용 또는 새로운 Transport 생성
	if f.initErr == nil {
		f.initErr = f.setupTransport()
	}

	return f
}

// Do HTTP 요청을 수행합니다.
//
// 이 메서드는 표준 http.Client.Do와 유사하지만, 다음과 같은 추가 기능을 제공합니다:
//
//  1. 초기화 에러 확인: NewHTTPFetcher에서 발생한 에러가 있으면 즉시 반환
//  2. 요청 객체 복제: 원본 요청 객체를 보호하기 위해 복제본 사용
//  3. 기본 헤더 자동 추가: Accept, Accept-Language, User-Agent
//
// 매개변수:
//   - req: 처리할 HTTP 요청
//
// 반환값:
//   - HTTP 응답 객체 (성공 시)
//   - 에러 (요청 처리 중 발생한 에러)
//
// 주의사항:
//   - 원본 요청 객체는 수정되지 않음 (복제본 사용)
//   - 요청 객체에 이미 헤더가 설정되어 있으면 덮어쓰지 않음
//   - 반환된 응답 객체의 Body는 호출자가 반드시 닫아야 함
func (h *HTTPFetcher) Do(req *http.Request) (*http.Response, error) {
	// 초기화 에러 조기 반환
	// NewHTTPFetcher에서 Transport 설정 실패 등의 에러가 발생했다면 여기서 즉시 반환
	if h.initErr != nil {
		return nil, h.initErr
	}

	// 요청 객체 복제: 원본 요청 객체 보호
	// 헤더 수정이 호출자의 원본 요청 객체에 영향을 주지 않도록 복제본 사용
	// Context는 원본과 동일하게 유지하여 취소/타임아웃 전파 보장
	clonedReq := req.Clone(req.Context())

	// 기본 HTTP 헤더 설정: 실제 브라우저처럼 동작하도록 표준 헤더 자동 추가
	if clonedReq.Header.Get("Accept") == "" {
		// 다양한 MIME 타입 지원 명시 (HTML, XML, 이미지 등)
		clonedReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	}
	if clonedReq.Header.Get("Accept-Language") == "" {
		// 한국어 우선, 영어 대체 (q값으로 우선순위 지정)
		clonedReq.Header.Set("Accept-Language", "ko-KR,ko;q=0.9,en-US;q=0.8,en;q=0.7")
	}

	// User-Agent 헤더 설정: 봇 차단 회피 및 실제 브라우저 모방
	if clonedReq.Header.Get("User-Agent") == "" && h.defaultUA != "" {
		clonedReq.Header.Set("User-Agent", h.defaultUA)
	}

	// HTTP 요청 실행: 설정된 Transport, 타임아웃, 리다이렉트 정책 적용
	return h.client.Do(clonedReq)
}

// Close HTTPFetcher가 사용 중인 네트워크 리소스를 정리합니다.
//
// 이 메서드는 HTTPFetcher가 더 이상 필요하지 않을 때 호출하여 메모리 누수를 방지합니다.
// 하지만 모든 리소스를 무조건 정리하는 것이 아니라, 안전하게 정리할 수 있는 것만 선별적으로 처리합니다.
//
// 왜 선별적으로 정리할까요?
//
//	Transport는 여러 HTTPFetcher가 공유할 수 있는 리소스입니다.
//	만약 다른 곳에서 사용 중인 Transport를 닫아버리면, 그곳의 네트워크 연결이 끊어지는 문제가 발생합니다.
//	따라서 이 메서드는 "내가 독점적으로 사용하는 리소스"만 정리합니다.
//
// 동작 방식:
//
//  1. 전역 기본 Transport (defaultTransport)
//     - 정리하지 않음
//     - 이유: 애플리케이션 전체에서 공유하는 싱글톤 리소스이므로, 닫으면 다른 모든 클라이언트의 연결이 끊어집니다.
//
//  2. 공유 Transport (캐시에 등록된 Transport)
//     - 정리하지 않음
//     - 이유: 동일한 설정을 가진 다른 HTTPFetcher들이 함께 사용 중일 수 있으므로, 닫으면 다른 인스턴스에 영향을 줍니다.
//
//  3. 격리된 Transport (DisableTransportCache 옵션으로 생성된 전용 Transport)
//     - 정리함 (CloseIdleConnections 호출)
//     - 이유: 이 HTTPFetcher만 사용하는 독립적인 리소스이므로, 안전하게 유휴 연결을 닫을 수 있습니다.
//
// 반환값:
//   - error: 항상 nil (인터페이스 호환성을 위해 유지)
//
// 주의사항:
//   - Close 호출 후 Do 메서드를 다시 호출하는 것은 권장하지 않습니다.
//   - 공유 리소스는 Go의 가비지 컬렉터(GC)가 자동으로 관리하므로, 수동으로 정리할 필요가 없습니다.
func (h *HTTPFetcher) Close() error {
	// 1. 기본 검증
	// 클라이언트나 Transport가 없으면 정리할 것이 없으므로 즉시 반환합니다.
	if h.client == nil || h.client.Transport == nil {
		return nil
	}

	// 2. 전역 기본 Transport 확인
	if h.client.Transport == defaultTransport {
		return nil
	}

	// 3. 사용자 정의 Transport 처리
	// *http.Transport 타입인 경우에만 정리 가능 여부를 판단합니다.
	// (다른 타입의 RoundTripper는 정리 방법을 알 수 없으므로 무시합니다)
	if tr, ok := h.client.Transport.(*http.Transport); ok {
		// 3-1. 격리된 Transport만 정리
		// 격리된 Transport인 경우에만 리소스를 독점하고 있다고 확신할 수 있으므로 정리합니다.
		//
		// ⚠️ 주의: 공유 Transport는 절대 닫으면 안 됩니다!
		//    공유 모드(disableTransportCache=false)에서는 캐시에서 퇴출되었다고 하더라도
		//    다른 Fetcher가 여전히 참조하고 있을 수 있으므로 GC에 맡겨야 합니다.
		if h.disableTransportCache {
			tr.CloseIdleConnections()
		}
	}

	return nil
}

// normalizeMaxRedirects 최대 리다이렉트 횟수를 정규화합니다.
//
// 정규화 규칙:
//   - 음수: 유효하지 않은 값으로 간주하여 기본값(defaultMaxRedirects)으로 보정
//   - 0: 리다이렉트를 허용하지 않음 (그대로 반환)
//   - 양수: 지정된 횟수만큼 리다이렉트 허용 (그대로 반환)
func normalizeMaxRedirects(maxRedirects int) int {
	if maxRedirects < 0 {
		return defaultMaxRedirects
	}

	return maxRedirects
}

// @@@@@
// transport 현재 HTTPFetcher가 사용 중인 Transport(http.RoundTripper)를 반환합니다.
//
// 이 메서드는 내부 상태 검증 및 디버깅을 위한 진단(Diagnostic) 목적으로 설계되었습니다.
// 반환된 인터페이스를 실제 구현체(예: *http.Transport)로 타입 단언(Type Assertion)하여
// 타임아웃, 프록시, 커넥션 풀 등의 세부 설정을 확인할 수 있습니다.
//
// 주요 활용:
//   - 단위 테스트(Unit Test)에서 설정 값 검증
//   - 런타임 구성(Configuration) 상태 모니터링
//   - Mock RoundTripper 주입 여부 확인
//
// 예제:
//
//	if tr, ok := f.transport().(*http.Transport); ok {
//	    // *http.Transport의 세부 필드 접근 가능
//	    fmt.Printf("MaxIdleConns: %d\n", tr.MaxIdleConns)
//	}
func (h *HTTPFetcher) transport() http.RoundTripper {
	if h.client == nil {
		return nil
	}

	return h.client.Transport
}

// newCheckRedirectPolicy HTTP 리다이렉트 처리를 위한 정책 함수를 생성합니다.
//
// # 목적
//
// HTTP 클라이언트가 3xx 리다이렉트 응답을 받았을 때 어떻게 처리할지 결정하는 함수를 생성합니다.
// 무한 리다이렉트 루프를 방지하고, Referer 헤더를 안전하게 설정하여 사이트 차단을 우회하면서도
// 보안을 유지합니다.
//
// # 적용되는 보안 정책
//
// 1. **리다이렉트 횟수 제한**: 지정된 최대 횟수를 초과하면 리다이렉트를 중단하고 마지막 응답을 반환
// 2. **Referer 헤더 자동 설정**: 이전 요청의 URL을 Referer로 설정하여 일부 사이트의 차단을 방지
// 3. **HTTPS → HTTP 다운그레이드 방지**: 보안 수준이 낮아지는 경우 Referer 전송을 차단 (RFC 7231 준수)
// 4. **인증 정보 제거**: Referer 헤더에서 사용자 자격 증명(ID/Password)과 민감한 쿼리 파라미터를 마스킹
//
// 매개변수:
//   - maxRedirects: 허용할 최대 리다이렉트 횟수 (0이면 리다이렉트 비활성화)
//
// 반환값:
//   - http.Client.CheckRedirect에 할당할 수 있는 정책 함수
func newCheckRedirectPolicy(maxRedirects int) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return http.ErrUseLastResponse
		}

		// 리다이렉트 시 이전 요청의 URL을 Referer로 설정하여 사이트 차단 방지
		if len(via) > 0 {
			prevReq := via[len(via)-1]
			if prevReq != nil && prevReq.URL != nil {
				// [보안 강화 1] HTTPS -> HTTP 다운그레이드 시 Referer 전송 방지
				if prevReq.URL.Scheme == "https" && req.URL.Scheme != "https" {
					// 보안 수준이 낮아지므로 Referer를 설정하지 않음
				} else {
					// [보안 강화 2] Referer 헤더 설정 시 사용자 자격 증명(ID/Password) 제거
					referer := redactRefererURL(prevReq.URL)
					req.Header.Set("Referer", referer)
				}
			}
		}

		return nil
	}
}
