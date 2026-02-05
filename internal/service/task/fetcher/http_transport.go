package fetcher

import (
	"container/list"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// defaultTransport 전역 기본 Transport입니다.
	//
	// 이 Transport는 싱글톤(Singleton)으로 동작하며, 프록시나 특수 설정이 필요 없는
	// 일반적인 HTTP 요청에 사용됩니다.
	//
	// 주요 설정:
	//   - 연결 생성: 30초 타임아웃, 30초 Keep-Alive
	//   - TLS 핸드셰이크: 10초 타임아웃
	//   - 연결 풀: 최대 100개 유휴 연결 (전체 및 호스트당)
	//   - 유휴 연결 유지: 90초
	//
	// 이를 통해 연결 재사용과 Keep-Alive로 성능을 최적화합니다.
	defaultTransport = &http.Transport{
		// 0. 프록시 설정 (Proxy)
		// 환경 변수(HTTP_PROXY, HTTPS_PROXY)를 따르도록 설정합니다.
		Proxy: http.ProxyFromEnvironment,

		// 1. 연결 생성 (Dialing)
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,

		// 2. 보안 연결 (TLS)
		TLSHandshakeTimeout: DefaultTLSHandshakeTimeout,

		// 3. 연결 풀 관리 (Connection Pool)
		MaxIdleConns:        DefaultMaxIdleConns,
		MaxIdleConnsPerHost: DefaultMaxIdleConns,
		IdleConnTimeout:     DefaultIdleConnTimeout,
	}

	// transportCache 설정별로 Transport를 캐싱하는 저장소입니다.
	//
	// 동일한 설정(프록시, 타임아웃 등)을 가진 Fetcher들이 Transport를 공유하여
	// 불필요한 객체 생성을 방지하고 연결 풀을 효율적으로 재사용합니다.
	//
	// 캐시 구조:
	//   - 키: transportCacheKey (프록시 URL, 타임아웃, 연결 풀 설정)
	//   - 값: *list.Element (LRU 리스트의 노드, transportCacheEntry 포함)
	//
	// 관리 정책:
	//   - LRU(Least Recently Used): 오래 사용되지 않은 항목부터 제거
	//   - 최대 크기: maxTransportCacheSize (기본 100개)
	transportCache = make(map[transportCacheKey]*list.Element)

	// transportCacheList LRU 캐시의 사용 순서를 관리하는 이중 연결 리스트입니다.
	//
	// 최근에 사용된 항목은 리스트 앞쪽으로 이동하고,
	// 캐시가 가득 차면 리스트 뒤쪽(가장 오래된 항목)부터 제거됩니다.
	transportCacheList = list.New()

	// transportCacheMu transportCache와 transportCacheList의 동시성을 제어하는 뮤텍스입니다.
	transportCacheMu sync.RWMutex
)

// transportCacheKey Transport 캐시를 위한 식별자입니다.
//
// 이 구조체는 Transport를 캐시할 때 사용하는 키로, 동일한 설정을 가진 Fetcher들이
// 같은 Transport를 공유할 수 있도록 합니다.
//
// 역할:
//   - Transport 재사용 판단: 모든 필드가 일치하면 기존 Transport를 재사용합니다.
//   - 리소스 최적화: 불필요한 Transport 생성을 방지하여 메모리와 연결 풀을 절약합니다.
type transportCacheKey struct {
	proxyURL string // 프록시 URL (빈 문자열이면 기본 설정(환경 변수 HTTP_PROXY 등)을 따름)

	// 연결 풀 관련 설정 (개수 제한)
	maxIdleConns    int // 전체 유휴(Idle) 연결의 최대 개수
	maxConnsPerHost int // 호스트(도메인)당 최대 연결 개수 (0이면 무제한)

	// 타임아웃 관련 설정 (시간 제한)
	idleConnTimeout       time.Duration // 유휴 연결이 닫히기 전 유지되는 타임아웃
	tlsHandshakeTimeout   time.Duration // TLS 핸드셰이크 타임아웃
	responseHeaderTimeout time.Duration // HTTP 응답 헤더 대기 타임아웃
}

// transportCacheEntry Transport 캐시에 저장되는 항목입니다.
//
// LRU 캐시에 저장되는 항목으로, Transport 객체와 접근 빈도를 함께 관리합니다.
// 접근 빈도를 추적하여 자주 사용되는 Transport는 캐시에 유지하고,
// 오랫동안 사용되지 않은 Transport는 자동으로 제거됩니다.
//
// 최적화 전략:
//   - Lazy LRU Update: 매번 LRU 위치를 갱신하지 않고, 10회 접근마다 한 번씩 갱신합니다.
//   - 이를 통해 Lock 경합을 줄이고 캐시 성능을 향상시킵니다.
type transportCacheEntry struct {
	key         transportCacheKey
	transport   *http.Transport // 실제 HTTP Transport 객체
	accessCount atomic.Int64    // Lazy LRU Update를 위한 접근 카운터 (10회마다 LRU 업데이트)
}

// newTransport 사용자 설정에 맞춰 새로운 Transport를 생성합니다.
//
// 이 함수는 제공된 Transport를 복제한 후 사용자 설정을 적용하여
// 격리된(Isolated) Transport를 생성합니다.
//
// 처리 흐름:
//  1. 제공된 Transport 복제 (base 또는 defaultTransport)
//  2. 프록시 설정 적용 (URL 파싱 및 검증)
//  3. 연결 풀 설정 적용 (최대 연결 수, 호스트당 연결 수)
//  4. 타임아웃 설정 적용 (응답 헤더, 유휴 연결, TLS)
//
// 매개변수:
//   - base: 복제할 Transport (nil이면 defaultTransport 사용)
//   - key: Transport 설정을 담은 키 객체
//
// 반환값:
//   - *http.Transport: 설정이 적용된 새 Transport
//   - error: 프록시 URL 파싱 실패 시 에러 (비밀번호는 마스킹됨)
//
// 보안:
//   - 프록시 URL 파싱 실패 시 에러 메시지에서 비밀번호를 마스킹하여 로그 노출 방지
func newTransport(base *http.Transport, key transportCacheKey) (*http.Transport, error) {
	// 1단계: 제공된 Transport 복제
	var newTr *http.Transport
	if base != nil {
		newTr = base.Clone()
	} else {
		newTr = defaultTransport.Clone()
	}

	// 2단계: 프록시 설정 적용
	if key.proxyURL != "" {
		proxyURL, err := url.Parse(key.proxyURL)
		if err != nil {
			// URL 파싱 실패 시, URL에서 민감한 정보를 마스킹하여 안전한 문자열로 변환합니다.
			redactedURL := redactRawURL(key.proxyURL)

			return nil, newErrInvalidProxyURL(redactedURL)
		}

		newTr.Proxy = http.ProxyURL(proxyURL)
	}

	// 3단계: 연결 풀 설정 적용
	if key.maxIdleConns >= 0 {
		newTr.MaxIdleConns = key.maxIdleConns
		newTr.MaxIdleConnsPerHost = key.maxIdleConns
	}
	if key.maxConnsPerHost > 0 {
		newTr.MaxConnsPerHost = key.maxConnsPerHost
	}

	// 4단계: 타임아웃 설정 적용
	if key.idleConnTimeout > 0 {
		newTr.IdleConnTimeout = key.idleConnTimeout
	}
	if key.tlsHandshakeTimeout > 0 {
		newTr.TLSHandshakeTimeout = key.tlsHandshakeTimeout
	}
	if key.responseHeaderTimeout > 0 {
		newTr.ResponseHeaderTimeout = key.responseHeaderTimeout
	}

	return newTr, nil
}

// getSharedTransport 설정에 맞는 공유 Transport를 반환하거나 새로 생성합니다.
//
// 이 함수는 Transport 재사용을 통해 성능을 최적화하는 핵심 로직입니다.
// 동일한 설정(프록시, 타임아웃 등)을 가진 Fetcher들이 같은 Transport를 공유하여
// 연결 풀링의 효율을 극대화하고 메모리 사용량을 줄입니다.
//
// 최적화 기법:
//  1. Lazy LRU Update: 10번 접근마다 한 번씩만 LRU 업데이트 (Write Lock 경합 90% 감소)
//  2. Double-Check Locking: 경합 상황에서 중복 생성 방지
//  3. 스마트 퇴출: 프록시 설정된 항목을 우선 제거 (일반 요청 성능 보호)
//
// 매개변수:
//   - key: Transport 설정을 담은 키 객체
//
// 반환값:
//   - *http.Transport: 재사용 가능한 Transport 객체
//   - error: Transport 생성 실패 시 에러
func getSharedTransport(key transportCacheKey) (*http.Transport, error) {
	// 1단계: 캐시 조회
	// 읽기 잠금(RLock)을 사용하여 여러 고루틴이 동시에 캐시를 조회할 수 있도록 합니다.
	// 이는 읽기 작업이 빈번한 캐시 조회 성능을 최적화합니다.
	transportCacheMu.RLock()
	el, ok := transportCache[key]
	if ok {
		// 캐시 히트(Cache Hit): 요청한 설정과 일치하는 Transport를 발견했습니다.
		entry := el.Value.(*transportCacheEntry)
		tr := entry.transport

		// Lazy LRU Update (지연된 LRU 갱신) 최적화:
		// 매번 접근할 때마다 LRU 리스트를 갱신하면 쓰기 잠금(Lock)이 필요하여 경합(Contention)이 발생합니다.
		// 이를 방지하기 위해 10번 접근할 때마다 한 번씩만 리스트 위치를 갱신하여 동시성 성능을 극대화합니다.
		accessCount := entry.accessCount.Add(1)

		// 10번째 접근이면서, 해당 항목이 이미 리스트의 맨 앞에 있지 않은 경우에만 갱신을 시도합니다.
		if accessCount%10 == 0 && transportCacheList.Front() != el {
			// 읽기 잠금을 해제하고 쓰기 잠금을 획득하여 리스트 변경을 준비합니다.
			transportCacheMu.RUnlock()
			transportCacheMu.Lock()

			// 이중 확인:
			// 잠금을 교체하는 짧은 순간에 다른 고루틴이 해당 항목을 제거하거나 변경했을 수 있습니다.
			// 데이터의 일관성을 보장하기 위해 항목의 존재 여부를 다시 확인합니다.
			if el, ok = transportCache[key]; ok {
				// 항목을 리스트의 맨 앞으로 이동시켜 '최근 사용됨'으로 표시합니다.
				transportCacheList.MoveToFront(el)

				transportCacheMu.Unlock()

				return el.Value.(*transportCacheEntry).transport, nil
			}

			// 경합 패배: 잠금 교체 중에 항목이 제거되었습니다.
			// 기존에 조회한 Transport(tr)는 이미 닫혔거나 유효하지 않을 수 있으므로 반환하지 않습니다.
			// 대신, 아래(2단계)의 새로운 Transport 생성 로직으로 넘어갑니다.
			transportCacheMu.Unlock()
		} else {
			// 아직 10번째 접근이 아니거나, 이미 리스트의 맨 앞에 있다면 읽기 잠금만 해제하고 즉시 반환합니다.
			transportCacheMu.RUnlock()

			return tr, nil
		}
	} else {
		// 캐시 미스(Cache Miss): 캐시 조회 실패 시, 읽기 잠금을 해제하고 생성 단계로 진행합니다.
		transportCacheMu.RUnlock()
	}

	// 2단계: 새로운 Transport 생성
	// 캐시에 없는 경우, 요청된 설정에 맞춰 새로운 Transport 인스턴스를 무조건 생성합니다.
	// (이 단계는 잠금 없이 수행되어 다른 고루틴을 차단하지 않습니다)
	newTr, err := newTransport(nil, key)
	if err != nil {
		return nil, err
	}

	// 3단계: 캐시에 등록 (Write Lock 활용)
	// 생성된 Transport를 공유 캐시에 등록하기 위해 쓰기 잠금을 획득합니다.
	transportCacheMu.Lock()
	defer transportCacheMu.Unlock()

	// 생성 후 재확인:
	// Transport를 생성하는 동안(2단계), 다른 고루틴이 동일한 설정으로 먼저 캐시에 등록했을 수 있습니다.
	// 중복 생성을 방지하고 리소스를 절약하기 위해 캐시를 다시 한 번 확인합니다.
	if el, ok := transportCache[key]; ok {
		// 경합 패배: 다른 고루틴이 먼저 등록했습니다.
		// 방금 생성한 newTr은 불필요하므로 즉시 정리(Close)합니다.
		newTr.CloseIdleConnections()

		// 먼저 등록된 기존 항목의 LRU 순위를 갱신하고 반환합니다.
		transportCacheList.MoveToFront(el)

		return el.Value.(*transportCacheEntry).transport, nil
	}

	// 4단계: 캐시 용량 관리 (Eviction 정책)
	// 캐시가 최대 크기에 도달한 경우, 오래된 항목을 제거하여 공간을 확보합니다.
	if transportCacheList.Len() >= DefaultMaxTransportCacheSize {
		// '스마트 퇴출(Smart Eviction)' 전략:
		// 프록시를 사용하는 Transport는 리소스를 많이 소모하거나 덜 중요할 가능성이 높다고 가정하여,
		// 일반 연결(Direct)의 성능을 보호하기 위해 프록시 항목을 우선적으로 제거 대상으로 삼습니다.

		var evictEl *list.Element

		// 가장 오래된 항목부터 탐색을 시작해서 최대 10개의 항목을 검사하여 프록시 설정이 있는 항목을 찾습니다.
		curr := transportCacheList.Back()
		for i := 0; i < 10 && curr != nil; i++ {
			if curr.Value.(*transportCacheEntry).key.proxyURL != "" {
				evictEl = curr
				break
			}
			curr = curr.Prev()
		}

		// 프록시 항목을 찾지 못한 경우, 가장 오래된(LRU) 항목을 제거 대상으로 선정합니다.
		if evictEl == nil {
			evictEl = transportCacheList.Back()
		}

		// 선정된 항목을 캐시와 리스트에서 영구적으로 제거하고, 관련 리소스(연결 풀 등)를 정리합니다.
		entry := evictEl.Value.(*transportCacheEntry)
		entry.transport.CloseIdleConnections()
		delete(transportCache, entry.key)
		transportCacheList.Remove(evictEl)
	}

	// 5단계: 최종 등록
	// 새로운 Transport를 리스트의 맨 앞에 추가하여 가장 최근에 사용된 것으로 표시합니다.
	el = transportCacheList.PushFront(&transportCacheEntry{
		key:       key,
		transport: newTr,
	})
	transportCache[key] = el

	return newTr, nil
}

// setupTransport HTTPFetcher의 설정에 맞춰 최적의 Transport를 선택하고 구성합니다.
//
// 이 함수는 HTTPClient 초기화 과정에서 호출되며, 성능과 리소스 효율성을 고려하여 다음과 같이 동작합니다:
//
// 처리 흐름:
//
//  1. 설정 분석: 사용자가 프록시, 타임아웃, 연결 풀 등의 커스텀 설정을 지정했는지 확인합니다.
//
//  2. 기본 동작:
//     - 커스텀 설정이 없다면 기본 Transport(defaultTransport)를 그대로 사용합니다.
//     - 이를 통해 불필요한 객체 생성을 피하고 메모리를 절약합니다.
//
//  3. 사용자 정의 동작:
//     - 커스텀 설정이 있다면 두 가지 방식으로 처리합니다:
//     a) 외부 주입 Transport: `configureTransportFromProvided` 호출 (CoW 전략 적용)
//     b) 내부 생성 Transport: `configureTransportFromOptions` 호출 (캐시 또는 격리 생성)
//
// 제약사항:
//   - `*http.Transport` 타입만 설정 변경이 가능합니다.
//   - 커스텀 `RoundTripper`는 설정 적용 대상에서 제외됩니다.
//
// 반환값:
//   - error: Transport 초기화 실패 시 에러 (예: 잘못된 프록시 URL)
func (f *HTTPFetcher) setupTransport() error {
	// 1단계: 기본 Transport(defaultTransport)를 그대로 사용할 수 있는지 확인합니다.
	if !f.needsCustomTransport() {
		// 모든 설정이 기본값이므로 기본 Transport를 그대로 사용합니다.
		return nil
	}

	// 2단계: 아직 기본 Transport(defaultTransport)를 사용하고 있는 경우...
	if f.client.Transport == defaultTransport {
		// 사용자 설정에 맞춰 새로운 Transport를 구성합니다.
		return f.configureTransportFromOptions()
	}

	// 3단계: 외부에서 주입된 Transport를 사용하고 있는 경우...
	if tr, ok := f.client.Transport.(*http.Transport); ok {
		// 외부에서 주입된 Transport를 기반으로 사용자 설정이 적용된 새로운 Transport를 생성합니다.
		return f.configureTransportFromProvided(tr)
	}

	// 4단계: 커스텀 RoundTripper 감지
	// *http.Transport가 아닌 다른 타입(예: 테스트용 모의 객체)은 설정 변경이 불가능합니다.
	return ErrUnsupportedTransport
}

// needsCustomTransport 기본 Transport(defaultTransport)를 그대로 사용할 수 있는지, 아니면 별도의 구성이 필요한지 판단합니다.
//
// 판단 기준:
//
//	사용자가 다음 중 하나라도 기본값과 다르게 설정했다면 true를 반환합니다:
//	  - Transport 캐시 비활성화 (disableTransportCache)
//	  - 프록시 서버 사용 (proxyURL)
//	  - 연결 풀 크기 조정 (maxIdleConns, maxConnsPerHost)
//	  - 네트워크 타임아웃 변경 (idleConnTimeout, tlsHandshakeTimeout, responseHeaderTimeout)
func (f *HTTPFetcher) needsCustomTransport() bool {
	return f.disableTransportCache ||
		f.proxyURL != "" ||
		f.maxIdleConns != DefaultMaxIdleConns ||
		f.maxConnsPerHost > 0 ||
		f.idleConnTimeout != DefaultIdleConnTimeout ||
		f.tlsHandshakeTimeout != DefaultTLSHandshakeTimeout ||
		f.responseHeaderTimeout > 0
}

// configureTransportFromOptions 사용자 설정을 기반으로 최적의 Transport를 구성합니다.
//
// 이 함수는 사용자가 지정한 프록시, 타임아웃, 연결 풀 설정을 바탕으로
// 가장 효율적인 Transport를 선택하거나 생성합니다.
//
// 처리 흐름:
//
//  1. 설정 키 생성:
//     - 사용자 설정을 transportCacheKey 구조체로 변환합니다.
//
//  2. 운영 모드 선택:
//
//     a) 격리 모드 (DisableTransportCache 활성화):
//     - 다른 Fetcher와 완전히 독립적인 전용 Transport를 생성합니다.
//     - 테스트 환경이나 완벽한 격리가 필요한 경우에 사용됩니다.
//
//     b) 공유 모드 (기본값 - 권장):
//     - 동일한 설정을 가진 Fetcher끼리 Transport를 공유합니다.
//     - TCP 연결을 재사용하여 메모리와 핸드셰이크 비용을 절약합니다.
//
// 반환값:
//   - error: Transport 생성 실패 시 에러 (예: 잘못된 프록시 URL)
func (f *HTTPFetcher) configureTransportFromOptions() error {
	// 1단계: 사용자 설정을 transportCacheKey로 변환합니다.
	key := transportCacheKey{
		proxyURL:              f.proxyURL,
		maxIdleConns:          f.maxIdleConns,
		maxConnsPerHost:       f.maxConnsPerHost,
		idleConnTimeout:       f.idleConnTimeout,
		tlsHandshakeTimeout:   f.tlsHandshakeTimeout,
		responseHeaderTimeout: f.responseHeaderTimeout,
	}

	// 2단계: 운영 모드에 따라 Transport를 생성합니다.
	if f.disableTransportCache {
		// 격리 모드: 이 Fetcher 전용의 독립적인 Transport를 생성합니다.
		tr, err := newTransport(nil, key)
		if err != nil {
			return newErrIsolatedTransportCreateFailed(err)
		}

		// 생성된 Transport를 클라이언트에 설정합니다.
		f.client.Transport = tr

		return nil
	}

	// 공유 모드: 캐시에서 동일한 설정의 Transport를 찾거나 새로 생성합니다.
	// 이를 통해 여러 Fetcher가 연결 풀을 공유하여 리소스를 절약합니다.
	tr, err := getSharedTransport(key)
	if err != nil {
		return newErrSharedTransportCreateFailed(err)
	}

	// 준비된 Transport를 클라이언트에 설정합니다.
	f.client.Transport = tr

	return nil
}

// configureTransportFromProvided 인자로 전달받은 Transport를 기반으로 사용자 설정이 적용된 새로운 Transport를 생성합니다.
//
// 이 함수는 WithTransport 옵션으로 제공된 Transport를 처리하며, 원본 손상을 방지하기 위해
// Copy-on-Write(CoW) 전략을 사용합니다.
//
// 처리 흐름:
//
//  1. 변경 필요성 검사:
//     - 인자로 전달받은 Transport의 설정이 사용자가 요청한 설정과 이미 일치하는지 확인합니다.
//     - 일치한다면 복제 없이 원본을 그대로 사용하여 리소스를 절약합니다.
//
//  2. 안전한 복제 및 적용 (CoW):
//     - 설정 변경이 필요하다면 원본을 복제한 후 사용자 설정을 덧입힙니다.
//     - 이를 통해 원본 Transport를 사용하는 다른 클라이언트에게 영향을 주지 않습니다.
//
// 반환값:
//   - error: Transport 복제 실패 시 에러
func (f *HTTPFetcher) configureTransportFromProvided(tr *http.Transport) error {
	// 1단계: 설정 변경이 필요한지 확인합니다.
	if !f.shouldCloneTransport(tr) {
		// 인자로 전달받은 Transport가 이미 원하는 설정과 일치하므로 그대로 사용합니다.
		// 이를 통해 원본의 연결 풀을 공유하여 리소스를 절약합니다.
		return nil
	}

	// 2단계: 원본을 보호하기 위해 복제본을 생성합니다.
	key := transportCacheKey{
		proxyURL:              f.proxyURL,
		maxIdleConns:          f.maxIdleConns,
		maxConnsPerHost:       f.maxConnsPerHost,
		idleConnTimeout:       f.idleConnTimeout,
		tlsHandshakeTimeout:   f.tlsHandshakeTimeout,
		responseHeaderTimeout: f.responseHeaderTimeout,
	}

	cloned, err := newTransport(tr, key)
	if err != nil {
		return newErrTransportCloneFailed(err)
	}

	// 3단계: 복제된 Transport를 클라이언트에 설정합니다.
	f.client.Transport = cloned

	return nil
}

// shouldCloneTransport 인자로 전달받은 Transport를 복제해야 하는지 판단합니다.
//
// 이 함수는 인자로 전달받은 Transport의 설정과 사용자가 요청한 설정을 비교하여,
// 원본을 그대로 사용할 수 있는지 아니면 복제가 필요한지 결정합니다.
//
// 판단 기준:
//
//	사용자 설정(f.*)과 인자로 전달받은 Transport 설정(tr.*)을 항목별로 비교합니다.
//	단 하나라도 다르다면 true를 반환하여 복제를 수행하도록 합니다.
//
//	비교 항목:
//	  - 프록시 서버 (proxyURL)
//	  - 연결 풀 크기 (maxIdleConns, maxConnsPerHost)
//	  - 네트워크 타임아웃 (idleConnTimeout, tlsHandshakeTimeout, responseHeaderTimeout)
//
// 목적:
//   - 불필요한 복제를 방지하여 메모리 사용량을 최소화합니다.
//   - 가능한 한 원본의 연결 풀을 재사용하여 성능을 최적화합니다.
func (f *HTTPFetcher) shouldCloneTransport(tr *http.Transport) bool {
	return f.proxyURL != "" ||
		(f.maxIdleConns >= 0 && tr.MaxIdleConns != f.maxIdleConns) ||
		(f.maxConnsPerHost > 0 && tr.MaxConnsPerHost != f.maxConnsPerHost) ||
		(f.idleConnTimeout != DefaultIdleConnTimeout && tr.IdleConnTimeout != f.idleConnTimeout) ||
		(f.tlsHandshakeTimeout != DefaultTLSHandshakeTimeout && tr.TLSHandshakeTimeout != f.tlsHandshakeTimeout) ||
		(f.responseHeaderTimeout > 0 && tr.ResponseHeaderTimeout != f.responseHeaderTimeout)
}
