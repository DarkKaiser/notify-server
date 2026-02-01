package fetcher

import (
	"container/list"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// @@@@@
var (
	// defaultTransport 전역에서 공유할 HTTP Transport 싱글톤
	//
	// 프록시나 특수 설정이 필요 없는 일반적인 HTTP 요청에 사용됩니다.
	// 연결 풀링과 Keep-Alive를 통해 성능을 최적화합니다.
	//
	// 주요 설정:
	//   - MaxIdleConns: 100개 (전역 유휴 연결 제한)
	//   - MaxIdleConnsPerHost: 100개 (호스트당 유휴 연결 제한)
	//   - IdleConnTimeout: 90초 (유휴 연결 유지 시간)
	//   - TLSHandshakeTimeout: 10초 (HTTPS 협상 제한)
	//   - DialContext: 30초 연결 타임아웃, 30초 Keep-Alive
	defaultTransport = &http.Transport{
		MaxIdleConns:        DefaultMaxIdleConns,
		MaxIdleConnsPerHost: DefaultMaxIdleConns,
		IdleConnTimeout:     DefaultIdleConnTimeout,
		TLSHandshakeTimeout: DefaultTLSHandshakeTimeout,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	// transportCache 프록시 및 타임아웃 설정별로 Transport를 캐싱합니다.
	//
	// 캐싱 전략:
	//   - 키: transportKey (프록시 URL, 타임아웃, 연결 풀 설정 등)
	//   - 값: *list.Element (LRU 리스트의 요소, Transport 포함)
	//   - 퇴출 정책: LRU (Least Recently Used)
	//   - 최대 크기: maxTransportCacheSize (기본 100개)
	//
	// 목적:
	//   - 동일한 설정의 요청들이 Transport를 재사용하여 성능 향상
	//   - 각 Transport는 자체 연결 풀을 유지하므로 과도한 생성 방지
	transportCache = make(map[transportKey]*list.Element)
	transportList  = list.New() // LRU 순서 관리용 이중 연결 리스트
	transportMu    sync.RWMutex // transportCache와 transportList 동시성 제어

	// maxTransportCacheSize 현재 캐싱할 최대 Transport 개수
	//
	// 런타임에 동적으로 변경 가능하며, LRU 정책으로 관리됩니다.
	// 초과 시 가장 오래 사용되지 않은 Transport가 제거됩니다.
	maxTransportCacheSize = DefaultMaxTransportCacheSize
)

// @@@@@
// transportKey Transport 캐시를 위한 키 구조체
//
// 동일한 설정을 가진 요청들은 같은 Transport를 재사용하여 성능을 최적화합니다.
// 프록시 URL, 타임아웃, 연결 풀 설정 등이 다르면 별도의 Transport를 생성합니다.
type transportKey struct {
	proxyURL            string        // 프록시 서버 URL (빈 문자열이면 프록시 미사용)
	headerTimeout       time.Duration // 응답 헤더 대기 타임아웃
	maxIdleConns        int           // 전역 최대 유휴 연결 수
	idleConnTimeout     time.Duration // 유휴 연결 타임아웃
	tlsHandshakeTimeout time.Duration // TLS 핸드셰이크 타임아웃
	maxConnsPerHost     int           // 호스트당 최대 연결 수 (0이면 무제한)
}

// @@@@@
// transportCacheEntry Transport 캐시 항목
//
// LRU 리스트에 저장되는 실제 데이터 구조입니다.
// Lazy LRU Update 최적화를 위해 접근 카운터를 포함합니다.
type transportCacheEntry struct {
	key         transportKey    // 캐시 키 (프록시 URL, 타임아웃 등)
	transport   *http.Transport // 실제 HTTP Transport 객체
	accessCount atomic.Int64    // Lazy LRU Update를 위한 접근 카운터 (10회마다 LRU 업데이트)
}

// @@@@@
// createTransport 설정에 따라 새로운 http.Transport를 생성합니다.
//
// 이 함수는 기본 Transport를 복제한 후 사용자 설정을 적용하여 커스터마이징된 Transport를 생성합니다.
// 프록시, 타임아웃, 연결 풀 설정 등을 유연하게 조정할 수 있습니다.
//
// 매개변수:
//   - base: 복제할 기본 Transport (nil이면 defaultTransport 사용)
//   - proxyURL: 프록시 서버 URL (빈 문자열이면 프록시 미사용)
//   - headerTimeout: 응답 헤더 대기 타임아웃 (0이면 기본값 유지)
//   - maxIdle: 전역 최대 유휴 연결 수 (음수면 무제한, 0이면 기본값 유지)
//   - idleTimeout: 유휴 연결 타임아웃 (0이면 기본값 유지)
//   - tlsTimeout: TLS 핸드셰이크 타임아웃 (0이면 기본값 유지)
//   - maxConns: 호스트당 최대 연결 수 (0이면 기본값 유지)
//
// 반환값:
//   - *http.Transport: 설정이 적용된 새 Transport
//   - error: 프록시 URL 파싱 실패 시 에러 (비밀번호는 마스킹됨)
//
// 보안:
//   - 프록시 URL 파싱 실패 시 에러 메시지에서 비밀번호를 마스킹하여 로그 노출 방지
func createTransport(base *http.Transport, proxyURL string, headerTimeout time.Duration, maxIdle int, idleTimeout, tlsTimeout time.Duration, maxConns int) (*http.Transport, error) {
	// [1단계] 기본 Transport 복제
	var newTr *http.Transport
	if base != nil {
		newTr = base.Clone()
	} else {
		newTr = defaultTransport.Clone() // 전역 기본 Transport 사용
	}

	// [2단계] 프록시 설정
	if proxyURL != "" {
		parsedURL, err := url.Parse(proxyURL)
		if err != nil {
			// URL 파싱 실패 시 안전하게 마스킹 처리하여 반환
			// 원본 err를 직접 노출하면 비밀번호가 포함될 수 있으므로 마스킹된 결과만 포함
			redacted := redactRawURL(proxyURL)
			return nil, fmt.Errorf("invalid proxy URL: %s", redacted)
		}
		newTr.Proxy = http.ProxyURL(parsedURL)
	}

	// [3단계] 타임아웃 설정
	if headerTimeout > 0 {
		newTr.ResponseHeaderTimeout = headerTimeout
	}

	// [4단계] 연결 풀 설정
	if maxIdle >= 0 {
		// 전역 및 호스트당 유휴 연결 수를 동일하게 설정
		newTr.MaxIdleConns = maxIdle
		newTr.MaxIdleConnsPerHost = maxIdle
	}
	if idleTimeout > 0 {
		newTr.IdleConnTimeout = idleTimeout
	}

	// [5단계] TLS 설정
	if tlsTimeout > 0 {
		newTr.TLSHandshakeTimeout = tlsTimeout
	}

	// [6단계] 호스트당 최대 연결 수 설정
	if maxConns > 0 {
		newTr.MaxConnsPerHost = maxConns
	}

	return newTr, nil
}

// @@@@@
// getSharedTransport 설정에 맞는 캐싱된 Transport를 반환하거나 새로 생성합니다.
//
// 이 함수는 Transport 재사용을 통해 성능을 최적화하는 핵심 로직입니다.
// 동일한 설정(프록시, 타임아웃 등)을 가진 요청들은 같은 Transport를 공유하여
// 연결 풀링의 효율을 극대화하고 메모리 사용량을 줄입니다.
//
// 최적화 기법:
//  1. Lazy LRU Update: 10번 접근마다 한 번씩만 LRU 업데이트 (Write Lock 획득 빈도 90% 감소)
//  2. Double-Check Locking: 경합 상황에서 중복 생성 방지
//  3. 스마트 퇴출: 프록시 설정된 항목을 우선 제거 (일반 요청 성능 보호)
//
// 매개변수:
//   - proxyURL: 프록시 서버 URL (빈 문자열이면 프록시 미사용)
//   - headerTimeout: 응답 헤더 대기 타임아웃
//   - maxIdle: 전역 최대 유휴 연결 수
//   - idleTimeout: 유휴 연결 타임아웃
//   - tlsTimeout: TLS 핸드셰이크 타임아웃
//   - maxConns: 호스트당 최대 연결 수
//
// 반환값:
//   - *http.Transport: 재사용 가능한 Transport 객체
//   - error: Transport 생성 실패 시 에러
func getSharedTransport(proxyURL string, headerTimeout time.Duration, maxIdle int, idleTimeout, tlsTimeout time.Duration, maxConns int) (*http.Transport, error) {
	// [1단계] 캐시 키 생성
	key := transportKey{
		proxyURL:            proxyURL,
		headerTimeout:       headerTimeout,
		maxIdleConns:        maxIdle,
		idleConnTimeout:     idleTimeout,
		tlsHandshakeTimeout: tlsTimeout,
		maxConnsPerHost:     maxConns,
	}

	// [2단계] 캐시 조회 (Read Lock)
	transportMu.RLock()
	el, ok := transportCache[key]
	if ok {
		entry := el.Value.(*transportCacheEntry)
		tr := entry.transport

		// [Lazy LRU Update 최적화]
		// 매번 LRU 업데이트를 하면 Write Lock 획득으로 인한 경합이 심함
		// 10번 접근마다 한 번씩만 업데이트하여 동시성 성능을 크게 개선
		accessCount := entry.accessCount.Add(1)

		// 10번째 접근마다 LRU 업데이트 수행 (이미 최신이면 스킵)
		if accessCount%10 == 0 && transportList.Front() != el {
			transportMu.RUnlock()
			transportMu.Lock()
			// Double-check: Lock 전환 사이에 다른 고루틴이 변경했을 수 있음
			if el, ok = transportCache[key]; ok {
				transportList.MoveToFront(el) // LRU 리스트 앞으로 이동 (최근 사용)
			}
			transportMu.Unlock()
		} else {
			transportMu.RUnlock()
		}

		return tr, nil
	}
	transportMu.RUnlock()

	// [3단계] 캐시 미스 - 새로운 Transport 생성
	newTr, err := createTransport(nil, proxyURL, headerTimeout, maxIdle, idleTimeout, tlsTimeout, maxConns)
	if err != nil {
		return nil, err
	}

	// [4단계] 캐시에 추가 (Write Lock)
	transportMu.Lock()
	defer transportMu.Unlock()

	// [Double-Check Locking]
	// Lock 획득 사이에 다른 고루틴이 이미 생성했을 수 있음
	if el, ok := transportCache[key]; ok {
		// 경합에서 패배: 현재 고루틴이 생성한 newTr은 사용되지 않으므로 정리
		newTr.CloseIdleConnections()
		transportList.MoveToFront(el)
		return el.Value.(*transportCacheEntry).transport, nil
	}

	// [5단계] 캐시 크기 제한 확인 및 퇴출
	if transportList.Len() >= maxTransportCacheSize {
		// [스마트 퇴출 전략]
		// 프록시 설정된 항목을 우선 제거하여 일반 요청의 성능 보호
		// 뒤에서부터 최대 10개 항목을 순회하며 프록시 항목 탐색
		var evictEl *list.Element
		curr := transportList.Back()
		for i := 0; i < 10 && curr != nil; i++ {
			if curr.Value.(*transportCacheEntry).key.proxyURL != "" {
				evictEl = curr // 프록시 항목 발견
				break
			}
			curr = curr.Prev()
		}

		// 프록시 항목을 못 찾았으면 가장 오래된 항목(Back)을 강제 제거
		if evictEl == nil {
			evictEl = transportList.Back()
		}

		// 선택된 항목 제거 및 리소스 정리
		entry := evictEl.Value.(*transportCacheEntry)
		entry.transport.CloseIdleConnections() // 유휴 연결 닫기
		delete(transportCache, entry.key)
		transportList.Remove(evictEl)
	}

	// [6단계] 새 Transport를 캐시에 추가
	el = transportList.PushFront(&transportCacheEntry{
		key:       key,
		transport: newTr,
	})
	transportCache[key] = el
	return newTr, nil
}

// @@@@@
// setMaxTransportCacheSize Transport 캐시의 최대 크기를 설정합니다.
//
// 이 함수는 런타임에 캐시 크기를 동적으로 조정할 수 있게 합니다.
// 크기를 줄이면 즉시 LRU 정책에 따라 오래된 Transport를 제거하여 메모리를 확보합니다.
//
// 매개변수:
//   - size: 새로운 최대 캐시 크기 (0 이하이면 자동으로 1로 설정)
//
// 동작:
//  1. 크기 검증: 0 이하이면 최소값 1로 보정
//  2. 즉시 정리: 현재 캐시 크기가 새로운 제한을 초과하면 LRU 순서대로 제거
//  3. 리소스 정리: 제거되는 Transport의 유휴 연결을 모두 닫음
//
// 주의사항:
//   - 이 함수는 Write Lock을 획득하므로 동시 호출 시 블로킹될 수 있음
//   - 제거되는 Transport의 연결은 즉시 닫히지만, 진행 중인 요청은 영향받지 않음
func setMaxTransportCacheSize(size int) {
	if size <= 0 {
		size = 1 // 최소 1개는 캐싱 허용
	}
	transportMu.Lock()
	defer transportMu.Unlock()

	maxTransportCacheSize = size

	// 캐시 크기가 제한을 초과하는 경우 즉시 정리
	// LRU 순서대로 가장 오래 사용되지 않은 항목부터 제거
	for transportList.Len() > size {
		evictEl := transportList.Back() // 가장 오래된 항목 (LRU)
		if evictEl == nil {
			break
		}
		entry := evictEl.Value.(*transportCacheEntry)
		entry.transport.CloseIdleConnections() // 유휴 연결 정리
		delete(transportCache, entry.key)
		transportList.Remove(evictEl)
	}
}
