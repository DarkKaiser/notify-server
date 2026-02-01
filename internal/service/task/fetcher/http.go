package fetcher

import (
	"fmt"
	"net/http"
	"time"
)

// @@@@@
const (
	// DefaultTimeout HTTP 클라이언트의 기본 타임아웃 (30초)
	// 요청 시작부터 응답 완료까지의 전체 시간 제한
	DefaultTimeout = 30 * time.Second

	// DefaultMaxIdleConns 전역 최대 유휴 연결 수 (100개)
	// 모든 호스트에 대한 유휴 연결의 총합 제한
	// 값이 클수록 연결 재사용률이 높아지지만 메모리 사용량 증가
	DefaultMaxIdleConns = 100

	// DefaultIdleConnTimeout 유휴 연결 타임아웃 (90초)
	// 사용되지 않는 연결을 유지할 최대 시간
	// 이 시간이 지나면 연결이 자동으로 닫힘
	DefaultIdleConnTimeout = 90 * time.Second

	// DefaultTLSHandshakeTimeout TLS 핸드셰이크 타임아웃 (10초)
	// HTTPS 연결 시 SSL/TLS 협상에 허용되는 최대 시간
	DefaultTLSHandshakeTimeout = 10 * time.Second

	// DefaultMaxTransportCacheSize 캐싱할 최대 Transport 개수 기본값 (100개)
	// LRU 정책으로 관리되며, 초과 시 가장 오래된 Transport 제거
	DefaultMaxTransportCacheSize = 100
)

// @@@@@
// HTTPFetcher 기본 HTTP 클라이언트 구현체입니다.
//
// 주요 기능:
//   - 타임아웃 관리: 전체 요청 타임아웃, 헤더 응답 타임아웃, TLS 핸드셰이크 타임아웃
//   - 연결 풀링: 유휴 연결 재사용, 호스트별 연결 수 제한
//   - 프록시 지원: HTTP/HTTPS 프록시 서버 설정
//   - Transport 캐싱: 동일한 설정의 요청들이 Transport를 공유하여 성능 최적화
//   - User-Agent 관리: 기본 User-Agent 설정 및 요청별 커스터마이징
//
// 사용 예시:
//
//	fetcher := NewHTTPFetcher(
//	    WithTimeout(30*time.Second),
//	    WithProxy("http://proxy.example.com:8080"),
//	    WithMaxIdleConns(100),
//	)
type HTTPFetcher struct {
	client              *http.Client  // 실제 HTTP 요청을 수행하는 클라이언트
	defaultUA           string        // 기본 User-Agent 문자열
	proxyURL            string        // 프록시 서버 URL (빈 문자열이면 프록시 미사용)
	headerTimeout       time.Duration // 응답 헤더 대기 타임아웃
	maxIdleConns        int           // 전역 최대 유휴 연결 수
	idleConnTimeout     time.Duration // 유휴 연결 타임아웃
	tlsHandshakeTimeout time.Duration // TLS 핸드셰이크 타임아웃
	maxConnsPerHost     int           // 호스트당 최대 연결 수
	disableCache        bool          // Transport 캐시 비활성화 여부
	initErr             error         // 초기화 중 발생한 에러 (있으면 Do 호출 시 반환)
}

// @@@@@
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
// 기본 설정:
//   - Timeout: 30초
//   - Transport: 공유 defaultTransport (연결 풀링 활성화)
//   - MaxRedirects: 10회 (Referer 헤더 자동 설정)
//   - User-Agent: Chrome 120 (Windows 10)
//   - MaxIdleConns: 100개
//   - IdleConnTimeout: 90초
//   - TLSHandshakeTimeout: 10초
//
// 초기화 과정:
//  1. 기본값으로 HTTPFetcher 생성
//  2. 제공된 옵션들 순차 적용
//  3. Transport 설정 (필요 시 캐시에서 가져오거나 새로 생성)
//
// 주의사항:
//   - 초기화 중 에러 발생 시 initErr 필드에 저장되며, Do 호출 시 반환됨
//   - Transport 캐싱은 성능 최적화를 위해 기본적으로 활성화됨
func NewHTTPFetcher(opts ...Option) *HTTPFetcher {
	f := &HTTPFetcher{
		client: &http.Client{
			Timeout:   DefaultTimeout,
			Transport: defaultTransport, // 기본적으로 공유 Transport 사용 (연결 풀링)
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 { // 기본 최대 10회 리다이렉트
					return http.ErrUseLastResponse
				}
				// 리다이렉트 시 이전 요청의 URL을 Referer로 설정하여 사이트 차단 방지
				if len(via) > 0 {
					req.Header.Set("Referer", via[len(via)-1].URL.String())
				}
				return nil
			},
		},
		defaultUA:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		maxIdleConns:        DefaultMaxIdleConns,
		idleConnTimeout:     DefaultIdleConnTimeout,
		tlsHandshakeTimeout: DefaultTLSHandshakeTimeout,
	}

	// [1단계] 옵션 적용
	for _, opt := range opts {
		opt(f)
	}

	// [2단계] Transport 설정 (필요 시 캐시에서 가져오거나 새로 생성)
	if f.initErr == nil {
		f.initErr = f.configureTransport()
	}

	return f
}

// @@@@@
// Do 커스텀 HTTP 요청을 실행합니다.
//
// 이 메서드는 표준 http.Client.Do와 유사하지만, 다음과 같은 추가 기능을 제공합니다:
//  1. 초기화 에러 확인: NewHTTPFetcher에서 발생한 에러가 있으면 즉시 반환
//  2. 요청 복제: 원본 요청을 보호하기 위해 복제본 사용 (방어적 프로그래밍)
//  3. 기본 헤더 설정: Accept, Accept-Language, User-Agent 자동 설정
//
// 매개변수:
//   - req: 실행할 HTTP 요청 (*http.Request)
//
// 반환값:
//   - *http.Response: HTTP 응답 (성공 시)
//   - error: 초기화 에러 또는 요청 실행 실패 시 에러
//
// 기본 헤더:
//   - Accept: "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"
//   - Accept-Language: "ko-KR,ko;q=0.9,en-US;q=0.8,en;q=0.7"
//   - User-Agent: 설정된 기본 User-Agent (없으면 설정 안 함)
//
// 주의사항:
//   - 원본 요청은 수정되지 않음 (복제본 사용)
//   - 요청에 이미 헤더가 설정되어 있으면 덮어쓰지 않음
//   - 반환된 응답의 Body는 호출자가 반드시 닫아야 함
func (h *HTTPFetcher) Do(req *http.Request) (*http.Response, error) {
	// [1단계] 초기화 에러 확인
	if h.initErr != nil {
		return nil, h.initErr
	}

	// [2단계] 원본 요청을 오염시키지 않기 위해 복제본 생성
	// 방어적 프로그래밍: 원본 요청을 수정하면 호출자에게 예상치 못한 부작용 발생 가능
	clonedReq := req.Clone(req.Context())

	// [3단계] 기본 헤더 설정 (요청에 이미 설정되어 있으면 스킵)
	// Accept 헤더: 브라우저처럼 보이도록 다양한 MIME 타입 지원 명시
	if clonedReq.Header.Get("Accept") == "" {
		clonedReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	}
	// Accept-Language 헤더: 한국어 우선, 영어 대체
	if clonedReq.Header.Get("Accept-Language") == "" {
		clonedReq.Header.Set("Accept-Language", "ko-KR,ko;q=0.9,en-US;q=0.8,en;q=0.7")
	}

	// [4단계] User-Agent 설정 (기본값이 있고, 요청에 설정되지 않은 경우)
	if clonedReq.Header.Get("User-Agent") == "" && h.defaultUA != "" {
		clonedReq.Header.Set("User-Agent", h.defaultUA)
	}

	// [5단계] HTTP 요청 실행
	return h.client.Do(clonedReq)
}

// @@@@@
// Close HTTPFetcher가 사용 중인 리소스(유휴 커넥션 등)를 명시적으로 정리합니다.
//
// 이 메서드는 Transport 캐싱 전략에 따라 다르게 동작합니다:
//  1. 공유 Transport (기본): 닫지 않음 (다른 HTTPFetcher와 공유 중)
//  2. 캐시된 Transport: 닫지 않음 (getSharedTransport를 통해 공유 중)
//  3. 전용 Transport: 닫음 (DisableTransportCache 또는 외부 주입 후 복제)
//
// 반환값:
//   - error: 항상 nil (현재 구현에서는 에러 발생하지 않음)
//
// 주의사항:
//   - 공유 Transport를 닫으면 다른 HTTPFetcher의 연결도 끊김
//   - 전용 Transport만 안전하게 닫을 수 있음
//   - Close 후에는 Do 메서드를 호출하지 말 것 (새 연결 생성됨)
//
// 사용 예시:
//
//	fetcher := NewHTTPFetcher(WithDisableTransportCache(true))
//	defer fetcher.Close() // 전용 Transport이므로 안전하게 닫힘
func (h *HTTPFetcher) Close() error {
	if h.client == nil || h.client.Transport == nil {
		return nil
	}

	// [검사 1] 기본 공유 Transport인 경우: 닫지 않음
	// defaultTransport는 전역 싱글톤이므로 다른 인스턴스와 공유됨
	if h.client.Transport == defaultTransport {
		return nil
	}

	// [검사 2] 캐시된 Transport인 경우: 공유 여부 확인 후 결정
	// DisableTransportCache가 true여서 전용으로 생성된 경우나
	// 외부에서 주입되어 Clone된 경우에는 닫아야 함
	if tr, ok := h.client.Transport.(*http.Transport); ok {
		// 공유 캐시에 존재하는지 확인
		isShared := false
		if !h.disableCache {
			transportMu.RLock()
			for _, el := range transportCache {
				if el.Value.(*transportCacheEntry).transport == tr {
					isShared = true
					break
				}
			}
			transportMu.RUnlock()
		}

		// 공유되지 않은 전용 Transport만 닫음
		if !isShared {
			tr.CloseIdleConnections()
		}
	}

	return nil
}

// @@@@@
// configureTransport Transport 설정이 필요한 경우 적절한 Transport를 구성합니다.
//
// 이 함수는 HTTPFetcher의 설정에 따라 Transport를 선택하거나 생성합니다.
// 기본 설정이면 공유 Transport를 사용하고, 특수 설정이 필요하면 캐시에서 가져오거나 새로 생성합니다.
//
// 처리 흐름:
//  1. 특수 설정 필요 여부 확인 (프록시, 타임아웃 등)
//  2. 기본 Transport인 경우: setupDefaultTransport 호출
//  3. 외부 주입 Transport인 경우: setupCustomTransport 호출
//  4. 기타 RoundTripper: 에러 반환
//
// 반환값:
//   - error: Transport 설정 실패 시 에러
//
// 주의사항:
//   - *http.Transport가 아닌 RoundTripper는 설정 적용 불가
func (f *HTTPFetcher) configureTransport() error {
	// [검사 1] Transport 설정이 필요한지 확인
	if !f.needsSpecialTransport() {
		// 기본 설정 그대로 사용 (공유 defaultTransport)
		return nil
	}

	// [검사 2] 기본 Transport인 경우
	if f.client.Transport == defaultTransport {
		return f.setupDefaultTransport()
	}

	// [검사 3] 외부 주입 Transport인 경우 (*http.Transport 타입 확인)
	if tr, ok := f.client.Transport.(*http.Transport); ok {
		return f.setupCustomTransport(tr)
	}

	// [검사 4] RoundTripper가 *http.Transport가 아닌 경우 (모의 객체, 커스텀 구현체 등)
	return fmt.Errorf("fetcher: cannot apply special settings to non-http.Transport")
}

// @@@@@
// needsSpecialTransport 특별한 Transport 설정이 필요한지 확인합니다.
//
// 다음 중 하나라도 해당하면 특수 설정이 필요합니다:
//   - 프록시 설정
//   - 응답 헤더 타임아웃 설정
//   - 기본값과 다른 유휴 연결 수
//   - 기본값과 다른 유휴 연결 타임아웃
//   - 기본값과 다른 TLS 핸드셰이크 타임아웃
//   - 호스트당 최대 연결 수 설정
//
// 반환값:
//   - bool: 특수 설정 필요 여부
func (f *HTTPFetcher) needsSpecialTransport() bool {
	return f.proxyURL != "" || f.headerTimeout > 0 ||
		f.maxIdleConns != DefaultMaxIdleConns ||
		f.idleConnTimeout != DefaultIdleConnTimeout ||
		f.tlsHandshakeTimeout != DefaultTLSHandshakeTimeout ||
		f.maxConnsPerHost > 0
}

// @@@@@
// setupDefaultTransport 기본 Transport를 설정합니다.
//
// DisableTransportCache 옵션에 따라 두 가지 방식으로 동작합니다:
//  1. 캐시 활성화 (기본): 공유 캐시에서 Transport 가져오기 (성능 최적화)
//  2. 캐시 비활성화: 전용 Transport 생성 (격리된 연결 풀)
//
// 캐시 사용 시 장점:
//   - 동일한 설정의 HTTPFetcher들이 Transport를 공유하여 연결 재사용률 극대화
//   - 메모리 사용량 감소
//
// 캐시 미사용 시 장점:
//   - 독립적인 연결 풀 유지 (다른 HTTPFetcher와 격리)
//   - 테스트 환경에서 유용
//
// 반환값:
//   - error: Transport 생성 또는 가져오기 실패 시 에러
func (f *HTTPFetcher) setupDefaultTransport() error {
	if f.disableCache {
		// [캐시 우회] 전용 Transport 생성
		tr, err := createTransport(nil, f.proxyURL, f.headerTimeout,
			f.maxIdleConns, f.idleConnTimeout, f.tlsHandshakeTimeout, f.maxConnsPerHost)
		if err != nil {
			return fmt.Errorf("fetcher: %w", err)
		}
		f.client.Transport = tr
		return nil
	}

	// [캐시 사용] 공유 캐시에서 Transport 가져오기 (연결 재사용 극대화)
	tr, err := getSharedTransport(f.proxyURL, f.headerTimeout,
		f.maxIdleConns, f.idleConnTimeout, f.tlsHandshakeTimeout, f.maxConnsPerHost)
	if err != nil {
		return fmt.Errorf("fetcher: %w", err)
	}
	f.client.Transport = tr
	return nil
}

// @@@@@
// setupCustomTransport 외부에서 주입된 Transport를 설정합니다.
//
// WithTransport 옵션으로 제공된 Transport를 처리합니다.
// 설정 변경이 필요한 경우에만 복제(Clone)하여 적용하고, 그렇지 않으면 원본을 그대로 사용합니다.
//
// 처리 방식:
//  1. 설정 변경 필요 여부 확인 (shouldCloneTransport)
//  2. 변경 불필요: 원본 Transport 사용 (커넥션 풀 공유)
//  3. 변경 필요: 복제 후 설정 적용 (원본 보호)
//
// 매개변수:
//   - tr: 외부에서 주입된 *http.Transport
//
// 반환값:
//   - error: Transport 복제 또는 설정 실패 시 에러
//
// 주의사항:
//   - 원본 Transport를 직접 수정하지 않고 복제하여 사용 (방어적 프로그래밍)
func (f *HTTPFetcher) setupCustomTransport(tr *http.Transport) error {
	if !f.shouldCloneTransport(tr) {
		// 기본값 그대로이므로 원본 사용 (커넥션 풀 공유)
		return nil
	}

	// 설정 변경이 필요하므로 복제하여 적용 (원본 보호)
	cloned, err := createTransport(tr, f.proxyURL, f.headerTimeout,
		f.maxIdleConns, f.idleConnTimeout, f.tlsHandshakeTimeout, f.maxConnsPerHost)
	if err != nil {
		return fmt.Errorf("fetcher: %w", err)
	}
	f.client.Transport = cloned
	return nil
}

// @@@@@
// shouldCloneTransport Transport를 복제해야 하는지 확인합니다.
//
// 다음 중 하나라도 해당하면 복제가 필요합니다:
//   - 프록시 설정이 있는 경우
//   - 응답 헤더 타임아웃이 다른 경우
//   - 최대 유휴 연결 수가 다른 경우
//   - 유휴 연결 타임아웃이 다른 경우
//   - TLS 핸드셰이크 타임아웃이 다른 경우
//   - 호스트당 최대 연결 수가 다른 경우
//
// 매개변수:
//   - tr: 확인할 *http.Transport
//
// 반환값:
//   - bool: 복제 필요 여부
//
// 목적:
//   - 불필요한 복제를 방지하여 메모리 사용량 최소화
//   - 원본 Transport의 연결 풀을 최대한 재사용
func (f *HTTPFetcher) shouldCloneTransport(tr *http.Transport) bool {
	return f.proxyURL != "" ||
		(f.headerTimeout > 0 && tr.ResponseHeaderTimeout != f.headerTimeout) ||
		(f.maxIdleConns >= 0 && tr.MaxIdleConns != f.maxIdleConns) ||
		(f.idleConnTimeout != DefaultIdleConnTimeout && tr.IdleConnTimeout != f.idleConnTimeout) ||
		(f.tlsHandshakeTimeout != DefaultTLSHandshakeTimeout && tr.TLSHandshakeTimeout != f.tlsHandshakeTimeout) ||
		(f.maxConnsPerHost > 0 && tr.MaxConnsPerHost != f.maxConnsPerHost)
}

// @@@@@
// GetTransport 현재 사용 중인 http.RoundTripper를 반환합니다.
//
// 이 메서드는 주로 테스트 및 검증 용도로 사용됩니다.
// Transport 설정이 올바르게 적용되었는지 확인하거나, 모의 객체(mock)를 주입했는지 검증할 때 유용합니다.
//
// 반환값:
//   - http.RoundTripper: 현재 사용 중인 Transport (nil일 수 있음)
//
// 사용 예시:
//
//	if tr, ok := fetcher.GetTransport().(*http.Transport); ok {
//	    fmt.Println("MaxIdleConns:", tr.MaxIdleConns)
//	}
func (h *HTTPFetcher) GetTransport() http.RoundTripper {
	if h.client == nil {
		return nil
	}
	return h.client.Transport
}
