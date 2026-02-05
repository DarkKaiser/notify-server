package fetcher

import (
	"time"
)

// Config Fetcher 체인을 구성하기 위한 모든 설정 옵션을 정의하는 구조체입니다.
type Config struct {
	// ========================================
	// 타임아웃(Timeout) 설정
	// ========================================

	// Timeout HTTP 요청 전체에 대한 타임아웃입니다.
	// 연결(Dial), 요청 전송, 응답 수신 등 전체 과정을 포함하는 시간 제한입니다.
	// - 0: 기본값(30초) 적용
	// - 음수: 타임아웃 없음(무한 대기)
	// - 양수: 지정된 시간으로 설정
	Timeout time.Duration

	// TLSHandshakeTimeout TLS 핸드셰이크 타임아웃입니다.
	// HTTPS 연결 시 SSL/TLS 협상에 허용되는 최대 시간입니다.
	// - 0: 기본값(DefaultTLSHandshakeTimeout=10초) 적용
	// - 양수: 지정된 시간으로 설정
	TLSHandshakeTimeout time.Duration

	// ResponseHeaderTimeout HTTP 응답 헤더 대기 타임아웃입니다.
	// 요청 전송 후 서버로부터 응답 헤더를 받을 때까지 허용되는 최대 시간입니다.
	// 본문(Body) 데이터 수신 시간은 포함되지 않습니다.
	// - 0: 별도 제한 없음 (HTTP 요청 전체에 대한 타임아웃 설정에 따름)
	// - 양수: 지정된 시간으로 설정
	ResponseHeaderTimeout time.Duration

	// IdleConnTimeout 유휴 연결이 닫히기 전 유지되는 타임아웃입니다.
	// 연결 풀에서 사용되지 않는 연결이 닫히기 전까지 유지되는 최대 시간입니다.
	// - 0: 기본값(DefaultIdleConnTimeout=90초) 적용
	// - 양수: 지정된 시간으로 설정
	IdleConnTimeout time.Duration

	// ========================================
	// 네트워크 및 연결 풀(Connection Pool) 설정
	// ========================================

	// ProxyURL 프록시 서버 주소입니다.
	// 빈 문자열이면 기본 설정(환경 변수 HTTP_PROXY 등)을 따릅니다.
	// - 형식: "http://host:port", "https://user:pass@host:port" 등
	ProxyURL string

	// MaxIdleConns 전체 유휴(Idle) 연결의 최대 개수입니다.
	// 모든 호스트에 대해 유지할 수 있는 유휴 연결의 최대 개수를 제한합니다.
	// - 0: 무제한 (표준 라이브러리 규칙)
	// - 음수: 기본값(DefaultMaxIdleConns=100) 적용
	// - 양수: 지정된 개수로 제한
	MaxIdleConns int

	// MaxConnsPerHost 호스트(도메인)당 최대 연결 개수입니다.
	// 동일한 호스트에 대해 동시에 유지할 수 있는 최대 연결 개수를 제한합니다.
	// - 0: 무제한
	// - 음수: 무제한으로 보정 (0으로 변경됨)
	// - 양수: 지정된 개수로 제한
	MaxConnsPerHost int

	// DisableTransportCache Transport 캐시 사용 여부입니다.
	// - false (기본값/권장): 캐시 사용
	// - true: 캐시 비활성화
	DisableTransportCache bool

	// ========================================
	// HTTP 클라이언트 동작
	// ========================================

	// EnableRandomUserAgent 요청 시마다 User-Agent를 랜덤으로 선택하여 주입하는 기능을 활성화합니다.
	//
	// 설정 값:
	//   - false (기본값): 기능 비활성화 (원본 요청의 User-Agent를 그대로 사용)
	//   - true: UserAgents가 있으면 해당 목록에서 랜덤으로 선택하여 주입, 없으면 내장된 User-Agent 목록에서 랜덤으로 선택하여 주입
	EnableRandomUserAgent bool

	// UserAgents User-Agent를 랜덤으로 선택하여 주입할 때 사용할 User-Agent 문자열 목록입니다.
	//
	// 설정 값:
	//   - nil/빈 슬라이스: 내장된 User-Agent 목록에서 랜덤으로 선택하여 주입
	//   - 값 지정: 지정된 목록에서 랜덤으로 선택하여 주입
	UserAgents []string

	// MaxRedirects HTTP 클라이언트의 최대 리다이렉트(3xx) 횟수입니다.
	// - 0: 기본값 사용 (net/http 기본 정책, 통상 10회)
	// - 양수: 지정된 횟수만큼 리다이렉트 허용
	MaxRedirects int

	// ========================================
	// 재시도(Retry) 정책
	// ========================================

	// MaxRetries 최대 재시도 횟수입니다.
	//
	// 설정 규칙:
	//   - nil 또는 0 (기본값): 재시도 안 함
	//   - 양수: 실패 시(5xx 에러 또는 네트워크 오류 등) 지정된 횟수만큼 재시도
	//   - 보정: 최소값(minRetries) 미만은 최소값으로, 최대값(maxAllowedRetries) 초과는 최대값으로 보정
	MaxRetries *int

	// MinRetryDelay 재시도 대기 시간의 최소값입니다.
	//
	// 설정 규칙:
	//   - nil 또는 1초 미만: 서버 부하 방지를 위해 최소 시간(1초)으로 보정
	//   - 1초 이상: 지수 백오프(Exponential Backoff)의 시작 대기 시간으로 사용
	MinRetryDelay *time.Duration

	// MaxRetryDelay 재시도 대기 시간의 최대값입니다.
	//
	// 설정 규칙:
	//   - nil 또는 0: 기본값(defaultMaxRetryDelay) 적용
	//   - MinRetryDelay보다 작은 값: 최대 재시도 대기 시간은 최소 재시도 대기 시간보다 작을 수 없으므로 MinRetryDelay로 보정
	//   - 그 외: 재시도 대기 시간이 이 값을 초과하지 않도록 제한
	MaxRetryDelay *time.Duration

	// ========================================
	// 응답 검증 및 제한
	// ========================================

	// DisableStatusCodeCheck HTTP 상태 코드 검증을 비활성화할지 여부입니다.
	//
	// 설정 값:
	//   - false (기본값): 상태 코드 검증 수행 (200 OK 또는 AllowedStatusCodes만 허용)
	//   - true: 검증 비활성화 (모든 상태 코드 허용)
	DisableStatusCodeCheck bool

	// AllowedStatusCodes 허용할 HTTP 상태 코드 목록입니다.
	//
	// 설정 값:
	//   - nil/빈 슬라이스: 200 OK만 허용
	//   - 값 지정: 지정된 코드들만 허용
	AllowedStatusCodes []int

	// AllowedMimeTypes 응답으로 허용할 MIME 타입 목록입니다.
	//
	// 설정 값:
	//   - nil/빈 슬라이스: MIME 타입 검증 생략
	//   - 값 지정: "text/html" 같이 파라미터를 제외한 순수 MIME 타입만 허용 (대소문자 구분 안 함)
	AllowedMimeTypes []string

	// MaxBytes 응답 본문의 최대 허용 크기입니다. (단위: 바이트)
	//
	// 설정 규칙:
	//   - NoLimit(-1): 크기 제한을 적용하지 않음 (주의: 메모리 고갈 위험 있음)
	//   - 0 이하: 유효하지 않은 값으로 간주하여 기본값(defaultMaxBytes)으로 보정
	//   - 양수: 지정된 크기만큼 응답 본문의 허용 크기를 제한
	MaxBytes *int64

	// ========================================
	// 미들웨어 체인 구성
	// ========================================

	// DisableLogging HTTP 요청/응답 로깅을 비활성화할지 여부입니다.
	//
	// 설정 값:
	//   - false (기본값): 로깅을 활성화하여 URL, 상태 코드, 실행 시간 등을 기록
	//   - true: 로깅 비활성화
	DisableLogging bool
}

// ApplyDefaults Config의 설정값을 검증하고, 잘못된 값이나 미설정된 값을 안전한 기본값으로 보정합니다.
func (cfg *Config) ApplyDefaults() {
	// HTTP 요청 전체에 대한 타임아웃 검증
	// 0은 "미설정" 상태로 간주하여 기본값 적용
	// 음수는 "무한 대기"를 의미하므로 호출자가 명시적으로 설정한 경우 그대로 유지
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}

	// TLS 핸드셰이크 타임아웃 검증
	// 0은 "미설정" 상태로 간주하여 기본값 적용
	if cfg.TLSHandshakeTimeout == 0 {
		cfg.TLSHandshakeTimeout = DefaultTLSHandshakeTimeout
	}

	// 유휴 연결이 닫히기 전 유지되는 타임아웃 검증
	// 0은 "미설정" 상태로 간주하여 기본값 적용
	if cfg.IdleConnTimeout == 0 {
		cfg.IdleConnTimeout = DefaultIdleConnTimeout
	}

	// 전체 유휴(Idle) 연결의 최대 개수 검증
	// 0은 "무제한"을 의미하므로 그대로 유지
	// 음수는 설정 오류로 간주하여 기본값(DefaultMaxIdleConns)으로 보정
	if cfg.MaxIdleConns < 0 {
		cfg.MaxIdleConns = DefaultMaxIdleConns
	}

	// 호스트(도메인)당 최대 연결 개수 검증
	// 음수는 의미가 없으므로 0(무제한)으로 보정
	if cfg.MaxConnsPerHost < 0 {
		cfg.MaxConnsPerHost = 0
	}
	}

	}

	// 최대 재시도 횟수를 정규화합니다.
	normalizePtr1(&cfg.MaxRetries, normalizeMaxRetries)

	// 재시도 대기 시간을 정규화합니다.
	normalizePtrs2(&cfg.MinRetryDelay, &cfg.MaxRetryDelay, normalizeRetryDelays)

	// 응답 본문의 최대 허용 크기를 정규화합니다.
	normalizePtr1(&cfg.MaxBytes, normalizeByteLimit)
}

// New 간단한 설정값(재시도 횟수, 지연 시간, 본문 크기 제한)과 추가 옵션을 기반으로 최적화된 Fetcher 체인을 생성합니다.
//
// 이 함수는 내부적으로 Config를 생성하고 ApplyDefaults()를 호출하여 안전한 기본값으로 보정한 후,
// NewFromConfig()를 통해 최적화된 Fetcher 체인을 구성합니다.
//
// 더 많은 설정 옵션이 필요한 경우 NewFromConfig를 직접 사용하는 것을 권장합니다.
//
// 매개변수:
//   - maxRetries: 최대 재시도 횟수 (권장: 0-10, 범위 초과 시 자동 보정)
//   - retryDelay: 재시도 간 기본 대기 시간 (최소: 1초, 1초 미만이면 1초로 보정)
//   - maxBytes: 응답 본문 최대 크기 (0: 기본 10MB, -1: 무제한, 양수: 지정 크기)
//   - opts: HTTPFetcher 추가 옵션 (예: WithTimeout, WithProxy, WithMaxRedirects)
//
// 반환값:
//   - Fetcher 체인
func New(maxRetries int, retryDelay time.Duration, maxBytes int64, opts ...Option) Fetcher {
	config := Config{
		MaxRetries: maxRetries,
		RetryDelay: retryDelay,
		MaxBytes:   maxBytes,
	}
	config.ApplyDefaults()

	return NewFromConfig(config, opts...)
}

// NewFromConfig Config와 추가 옵션을 기반으로 최적화된 Fetcher 체인을 생성합니다.
//
// Fetcher 체인 구성 순서 (바깥쪽 -> 안쪽):
//
//	LoggingFetcher               (6단계) - 전체 요청/응답 과정 로깅
//	  -> UserAgentFetcher        (5단계) - User-Agent 헤더 설정
//	    -> RetryFetcher          (4단계) - 재시도 로직 (지수 백오프)
//	      -> MimeTypeFetcher     (3단계) - Content-Type 검증 (선택적)
//	        -> StatusCodeFetcher (2단계) - HTTP 상태 코드 검증
//	          -> MaxBytesFetcher (1단계) - 응답 크기 제한
//	            -> HTTPFetcher   (0단계) - 실제 HTTP 요청 수행
//
// 이러한 순서는 다음과 같은 이유로 설계되었습니다:
//   - LoggingFetcher가 가장 바깥쪽에 위치하여 재시도를 포함한 전체 과정을 로깅
//   - UserAgentFetcher가 RetryFetcher 바깥에 위치하여 재시도 시에도 동일한 UA 유지
//   - RetryFetcher가 검증 로직(StatusCodeFetcher, MimeTypeFetcher) 바깥에 위치하여 실패 시 재시도 가능
//   - 검증 로직(StatusCodeFetcher, MimeTypeFetcher)이 안쪽에 위치하여 각 시도마다 검증 수행
//
// 매개변수:
//   - cfg: Fetcher 체인 구성을 위한 설정값
//   - opts: HTTPFetcher 추가 옵션 (예: WithTimeout, WithProxy, WithMaxRedirects)
//
// 반환값:
//   - Fetcher 체인
func NewFromConfig(cfg Config, opts ...Option) Fetcher {
	cfg.ApplyDefaults()

	// ========================================
	// 0단계: 기본 옵션 및 Config 기반 옵션 통합
	// ========================================
	var mergedOpts []Option

	// HTTP 요청 전체에 대한 타임아웃 설정
	if cfg.Timeout > 0 {
		mergedOpts = append(mergedOpts, WithTimeout(cfg.Timeout))
	} else if cfg.Timeout < 0 {
		mergedOpts = append(mergedOpts, WithTimeout(0)) // 0은 무한 대기를 의미
	}

	// TLS 핸드셰이크 타임아웃 설정
	if cfg.TLSHandshakeTimeout > 0 {
		mergedOpts = append(mergedOpts, WithTLSHandshakeTimeout(cfg.TLSHandshakeTimeout))
	}

	// HTTP 응답 헤더 대기 타임아웃 설정
	if cfg.ResponseHeaderTimeout > 0 {
		mergedOpts = append(mergedOpts, WithResponseHeaderTimeout(cfg.ResponseHeaderTimeout))
	}

	// 유휴 연결이 닫히기 전 유지되는 타임아웃 설정
	if cfg.IdleConnTimeout > 0 {
		mergedOpts = append(mergedOpts, WithIdleConnTimeout(cfg.IdleConnTimeout))
	}

	// 프록시 서버 주소 설정
	if cfg.ProxyURL != "" {
		mergedOpts = append(mergedOpts, WithProxy(cfg.ProxyURL))
	}

	// 전체 유휴(Idle) 연결의 최대 개수 설정
	mergedOpts = append(mergedOpts, WithMaxIdleConns(cfg.MaxIdleConns))

	// 호스트(도메인)당 최대 연결 개수 설정
	if cfg.MaxConnsPerHost > 0 {
		mergedOpts = append(mergedOpts, WithMaxConnsPerHost(cfg.MaxConnsPerHost))
	}

	// Transport 캐시 사용 여부 설정
	if cfg.DisableTransportCache {
		mergedOpts = append(mergedOpts, WithDisableTransportCache(true))
	}

	// HTTP 클라이언트의 최대 리다이렉트(3xx) 횟수 설정
	if cfg.MaxRedirects > 0 {
		mergedOpts = append(mergedOpts, WithMaxRedirects(cfg.MaxRedirects))
	}

	// 사용자 제공 옵션을 마지막에 추가하여 Config 기반 옵션을 덮어쓸 수 있도록 함!!
	mergedOpts = append(mergedOpts, opts...)

	// ========================================
	// 기본 HTTPFetcher 생성 (체인의 가장 안쪽)
	// ========================================
	var f Fetcher = NewHTTPFetcher(mergedOpts...)

	// ========================================
	// 1단계: 응답 본문의 크기 제한 미들웨어
	// ========================================
	f = NewMaxBytesFetcher(f, *cfg.MaxBytes)

	// ========================================
	// 2단계: 상태 코드 검증 미들웨어
	// ========================================
	if !cfg.DisableStatusCodeCheck {
		if len(cfg.AllowedStatusCodes) > 0 {
			// 성공으로 간주할 상태 코드를 사용자가 명시한 경우
			f = NewStatusCodeFetcherWithOptions(f, cfg.AllowedStatusCodes...)
		} else {
			// 기본값: 200 OK만 허용
			f = NewStatusCodeFetcher(f)
		}
	}

	// ========================================
	// 3단계: MIME 타입 검증 미들웨어
	// ========================================
	if len(cfg.AllowedMimeTypes) > 0 {
		f = NewMimeTypeFetcher(f, cfg.AllowedMimeTypes, true)
	}

	// ========================================
	// 4단계: 재시도 수행 미들웨어
	// ========================================
	f = NewRetryFetcher(f, *cfg.MaxRetries, *cfg.MinRetryDelay, *cfg.MaxRetryDelay)

	// ========================================
	// 5단계: User-Agent 주입 미들웨어
	// ========================================
	// RetryFetcher 바깥에 위치하여 재시도 시에도 동일한 User-Agent를 유지합니다.
	if cfg.EnableRandomUserAgent {
		f = NewUserAgentFetcher(f, cfg.UserAgents)
	}

	// ========================================
	// 6단계: 로깅 미들웨어 (체인의 가장 바깥쪽)
	// ========================================
	// 가장 바깥쪽에 위치하여 모든 미들웨어의 동작을 포함한 전체 과정을 로깅
	if !cfg.DisableLogging {
		f = NewLoggingFetcher(f)
	}

	return f
}

// normalizePtr1 포인터 필드의 값을 안전하게 꺼내어 정규화(보정)한 뒤, 다시 포인터에 담아주는 제네릭 헬퍼 함수입니다.
//
// 주요 특징:
//   - Nil 안전성: 포인터가 nil인 경우, 해당 타입의 기본값(Zero Value)을 사용하여 패닉을 방지합니다.
//   - 값 보정: 사용자가 전달한 정규화 로직(normalizer)을 통해 값을 검증하고 올바른 범위로 맞춥니다.
//   - 불변성 보장: 원본 값을 수정하는 대신, 보정된 새로운 값의 주소를 할당합니다.
func normalizePtr1[T any](ptr **T, normalizer func(T) T) {
	var val T
	if *ptr != nil {
		val = **ptr
	}

	result := normalizer(val)

	*ptr = &result
}

// normalizePtrs2 서로 연관된 두 개의 포인터 필드 값을 함께 꺼내어, 상호 의존성을 고려해 정규화하는 제네릭 헬퍼 함수입니다.
//
// 주요 특징:
//   - 상호 보정: 두 값을 비교하여 모순되는 설정(예: 최소값이 최대값보다 큰 경우)을 안전하게 조정합니다.
//   - Nil 안전성: 포인터가 nil인 경우, 기본값(Zero Value)으로 취급하여 로직을 단순화합니다.
//   - 일관성 유지: 두 값이 항상 논리적으로 모순되지 않도록 동시에 업데이트합니다.
func normalizePtrs2[T any](ptr1 **T, ptr2 **T, normalizer func(T, T) (T, T)) {
	var val1, val2 T
	if *ptr1 != nil {
		val1 = **ptr1
	}
	if *ptr2 != nil {
		val2 = **ptr2
	}

	result1, result2 := normalizer(val1, val2)

	*ptr1 = &result1
	*ptr2 = &result2
}
