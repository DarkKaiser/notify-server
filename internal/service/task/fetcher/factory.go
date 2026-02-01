package fetcher

import (
	"time"
)

// Config Fetcher 체인 구성을 위한 설정입니다.
type Config struct {
	// ========================================
	// 재시도(Retry) 설정
	// ========================================

	// MaxRetries 최대 재시도 횟수입니다.
	// HTTP 5xx 에러 또는 네트워크 오류 발생 시 재시도할 최대 횟수입니다.
	// - 범위: 0 ~ 10 (권장값)
	// - 보정: 0 미만은 0으로, 10 초과는 10으로 자동 조정됩니다.
	MaxRetries int

	// RetryDelay 재시도 대기 시간의 최소값입니다.
	// 지수 백오프(Exponential Backoff)의 시작 대기 시간으로 사용됩니다.
	// - 최소값: 1초 (1초 미만 설정 시 1초로 자동 보정)
	RetryDelay time.Duration

	// MaxRetryDelay 재시도 대기 시간의 최대값입니다.
	// 지수 백오프 적용 시 대기 시간이 이 값을 초과하지 않도록 제한합니다.
	// - 0: 미설정으로 간주하여 기본값(30초) 적용
	// - RetryDelay보다 작은 값(음수 포함): RetryDelay 값으로 자동 보정
	MaxRetryDelay time.Duration

	// ========================================
	// 응답 데이터 크기 제한
	// ========================================

	// MaxBytes 응답 본문의 읽기 허용 최대 크기(바이트)입니다.
	// 과도한 메모리 사용을 방지하기 위해 응답 크기를 제한합니다.
	// - NoLimit (-1): 크기 제한 없음 (주의: 메모리 과다 사용 위험)
	// - 0 또는 음수(NoLimit 제외): 기본값(10MB) 적용
	// - 양수: 지정된 크기로 제한
	MaxBytes int64

	// ========================================
	// 타임아웃(Timeout) 설정
	// ========================================

	// Timeout HTTP 요청 전체에 대한 타임아웃입니다.
	// 연결(Dial), 요청 전송, 응답 수신 등 전체 과정을 포함하는 시간 제한입니다.
	// - 0: 기본값(30초) 적용
	// - 음수: 타임아웃 없음(무한 대기)
	// - 양수: 지정된 시간으로 설정
	Timeout time.Duration

	// ResponseHeaderTimeout HTTP 응답 헤더 대기 타임아웃입니다.
	// 본문(Body) 데이터 수신 시간은 포함되지 않습니다.
	// - 0: 별도 제한 없음 (HTTP 요청 전체에 대한 타임아웃 설정에 따름)
	// - 양수: 지정된 시간으로 설정
	ResponseHeaderTimeout time.Duration

	// ========================================
	// 응답 검증(Validation)
	// ========================================

	// SkipStatusCodeCheck HTTP 상태 코드 검증 기능을 비활성화할지 여부입니다.
	// - false (기본값): 200 OK 또는 AllowedStatusCodes에 명시된 코드만 허용
	// - true: 검증 생략 (모든 상태 코드를 성공으로 간주)
	SkipStatusCodeCheck bool

	// AllowedStatusCodes 성공으로 간주할 HTTP 상태 코드 목록입니다.
	// SkipStatusCodeCheck가 false일 때만 유효합니다.
	// - nil/빈 슬라이스: 200 OK만 성공으로 간주
	// - 값 지정: 해당 코드들만 성공으로 간주
	AllowedStatusCodes []int

	// AllowedContentTypes 허용할 응답의 Content-Type 목록입니다.
	// - nil/빈 슬라이스: Content-Type 검증 생략
	// - 값 지정: 접두사가 일치하는 타입만 허용 (예: "application/json"은 "application/json; charset=utf-8" 허용)
	AllowedContentTypes []string

	// ========================================
	// HTTP 클라이언트 동작 옵션
	// ========================================

	// UserAgents 요청 헤더에 사용할 User-Agent 문자열 목록입니다.
	// - nil/빈 슬라이스: Go 기본 User-Agent 또는 별도 설정값 사용
	// - 값 지정: 매 요청마다 목록 중 하나를 무작위로 선택하여 사용 (Fingerprinting 회피용)
	UserAgents []string

	// MaxRedirects HTTP 클라이언트의 최대 리다이렉트(3xx) 횟수입니다.
	// - 0: 기본값 사용 (net/http 기본 정책, 통상 10회)
	// - 양수: 지정된 횟수만큼 리다이렉트 허용
	MaxRedirects int

	// ========================================
	// 네트워크 및 연결 풀(Connection Pool) 설정
	// ========================================

	// ProxyURL 프록시 서버 주소입니다.
	// - 빈 문자열: 프록시 미사용
	// - 형식: "http://host:port", "https://user:pass@host:port" 등
	ProxyURL string

	// MaxIdleConns 전체 유휴(Idle) 연결의 최대 개수입니다.
	// - 0: 무제한 (표준 라이브러리 규칙)
	// - 음수: 기본값(DefaultMaxIdleConns=100) 적용
	// - 양수: 지정된 개수로 제한
	MaxIdleConns int

	// IdleConnTimeout 유휴 연결이 닫히기 전 유지되는 최대 시간입니다.
	// - 0: 기본값(DefaultIdleConnTimeout=90초) 적용
	IdleConnTimeout time.Duration

	// TLSHandshakeTimeout TLS 핸드셰이크 타임아웃입니다.
	// - 0: 기본값(DefaultTLSHandshakeTimeout=10초) 적용
	TLSHandshakeTimeout time.Duration

	// MaxConnsPerHost 호스트(도메인)당 최대 연결 개수입니다.
	// - 0: 무제한
	// - 음수: 무제한으로 보정 (0으로 변경됨)
	// - 양수: 지정된 개수로 제한
	MaxConnsPerHost int

	// DisableTransportCache Transport 캐시 사용 여부입니다.
	// - false (기본값/권장): 캐시 사용
	// - true: 캐시 비활성화
	DisableTransportCache bool

	// ========================================
	// 기타 설정
	// ========================================

	// DisableLogging 로깅 기능 비활성화 여부입니다.
	// - false (기본값): 모든 요청/응답 생애주기 로깅
	// - true: 로깅 출력 안 함
	DisableLogging bool
}

// ApplyDefaults Config의 설정값을 검증하고, 잘못된 값이나 미설정된 값을 안전한 기본값으로 보정합니다.
func (cfg *Config) ApplyDefaults() {
	// 최대 재시도 횟수 검증
	// 음수는 의미가 없으므로 최소 재시도 횟수(0)로, 최대 재시도 횟수(10)를 초과하면 과도한 재시도로 인한 지연을 방지하기 위해 10으로 제한
	if cfg.MaxRetries < minRetries {
		cfg.MaxRetries = minRetries
	}
	if cfg.MaxRetries > maxAllowedRetries {
		cfg.MaxRetries = maxAllowedRetries
	}

	// 재시도 대기 시간의 최소값 검증
	if cfg.RetryDelay < time.Second {
		cfg.RetryDelay = 1 * time.Second // 너무 짧은 대기 시간(1초 미만)은 서버에 부담을 줄 수 있으므로 최소 1초로 설정
	}

	// 재시도 대기 시간의 최대값 검증
	// MaxRetryDelay는 지수 백오프(exponential backoff) 시 상한선 역할을 하므로 RetryDelay보다 작을 수 없음!
	if cfg.MaxRetryDelay < cfg.RetryDelay {
		if cfg.MaxRetryDelay == 0 {
			// 값을 설정하지 않은 경우 기본값(defaultMaxRetryDelay) 적용
			cfg.MaxRetryDelay = defaultMaxRetryDelay
		} else {
			// 명시적으로 설정했지만 RetryDelay보다 작은 경우, RetryDelay와 동일하게 설정
			cfg.MaxRetryDelay = cfg.RetryDelay
		}
	}

	// 응답 본문의 읽기 허용 최대 크기 검증
	// 명시적인 무제한 설정(NoLimit)을 제외하고, 0 이나 음수 값은
	// 설정 오류 또는 미설정으로 간주하여 기본값(defaultMaxBytes)으로 안전하게 보정합니다.
	if cfg.MaxBytes <= 0 && cfg.MaxBytes != NoLimit {
		cfg.MaxBytes = defaultMaxBytes
	}

	// HTTP 요청 전체에 대한 타임아웃 검증
	// 0은 "미설정" 상태로 간주하여 기본값 적용
	// 음수는 "무한 대기"를 의미하므로 호출자가 명시적으로 설정한 경우 그대로 유지
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}

	// 전체 유휴(Idle) 연결의 최대 개수 검증
	// 0은 "무제한"을 의미하므로 그대로 유지
	// 음수는 설정 오류로 간주하여 기본값(DefaultMaxIdleConns)으로 보정
	if cfg.MaxIdleConns < 0 {
		cfg.MaxIdleConns = DefaultMaxIdleConns
	}

	// 유휴 연결이 닫히기 전 유지되는 최대 시간 검증
	// 0은 "미설정" 상태로 간주하여 기본값 적용
	if cfg.IdleConnTimeout == 0 {
		cfg.IdleConnTimeout = DefaultIdleConnTimeout
	}

	// TLS 핸드셰이크 타임아웃 검증
	// 0은 "미설정" 상태로 간주하여 기본값 적용
	if cfg.TLSHandshakeTimeout == 0 {
		cfg.TLSHandshakeTimeout = DefaultTLSHandshakeTimeout
	}

	// 호스트(도메인)당 최대 연결 개수 검증
	// 음수는 의미가 없으므로 0(무제한)으로 보정
	if cfg.MaxConnsPerHost < 0 {
		cfg.MaxConnsPerHost = 0
	}
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

	// HTTP 응답 헤더 대기 타임아웃 설정
	if cfg.ResponseHeaderTimeout > 0 {
		mergedOpts = append(mergedOpts, WithResponseHeaderTimeout(cfg.ResponseHeaderTimeout))
	}

	// 프록시 서버 주소 설정
	if cfg.ProxyURL != "" {
		mergedOpts = append(mergedOpts, WithProxy(cfg.ProxyURL))
	}

	// HTTP 클라이언트의 최대 리다이렉트(3xx) 횟수 설정
	if cfg.MaxRedirects > 0 {
		mergedOpts = append(mergedOpts, WithMaxRedirects(cfg.MaxRedirects))
	}

	// 전체 유휴(Idle) 연결의 최대 개수 설정
	mergedOpts = append(mergedOpts, WithMaxIdleConns(cfg.MaxIdleConns))

	// 유휴 연결이 닫히기 전 유지되는 최대 시간 설정
	if cfg.IdleConnTimeout > 0 {
		mergedOpts = append(mergedOpts, WithIdleConnTimeout(cfg.IdleConnTimeout))
	}

	// TLS 핸드셰이크 타임아웃 설정
	if cfg.TLSHandshakeTimeout > 0 {
		mergedOpts = append(mergedOpts, WithTLSHandshakeTimeout(cfg.TLSHandshakeTimeout))
	}

	// 호스트(도메인)당 최대 연결 개수 설정
	if cfg.MaxConnsPerHost > 0 {
		mergedOpts = append(mergedOpts, WithMaxConnsPerHost(cfg.MaxConnsPerHost))
	}

	// Transport 캐시 사용 여부 설정
	if cfg.DisableTransportCache {
		mergedOpts = append(mergedOpts, WithDisableTransportCache(true))
	}

	// 사용자 제공 옵션을 마지막에 추가하여 Config 기반 옵션을 덮어쓸 수 있도록 함!!
	mergedOpts = append(mergedOpts, opts...)

	// ========================================
	// 기본 HTTPFetcher 생성 (체인의 가장 안쪽)
	// ========================================
	var f Fetcher = NewHTTPFetcher(mergedOpts...)

	// ========================================
	// 1단계: 응답 크기 제한 미들웨어
	// ========================================
	f = NewMaxBytesFetcher(f, cfg.MaxBytes)

	// ========================================
	// 2단계: 상태 코드 검사 미들웨어
	// ========================================
	if !cfg.SkipStatusCodeCheck {
		if len(cfg.AllowedStatusCodes) > 0 {
			// 성공으로 간주할 상태 코드를 사용자가 명시한 경우
			f = NewStatusCodeFetcherWithOptions(f, cfg.AllowedStatusCodes...)
		} else {
			// 기본값: 200 OK만 허용
			f = NewStatusCodeFetcher(f)
		}
	}

	// ========================================
	// 3단계: Content-Type 검사 미들웨어
	// ========================================
	// 로깅 및 재시도 로직 안쪽에 위치시켜 검사 결과를 로깅에 반영하고 재시도 가능하게 함
	if len(cfg.AllowedContentTypes) > 0 {
		f = NewMimeTypeFetcher(f, cfg.AllowedContentTypes, true)
	}

	// ========================================
	// 4단계: 재시도 미들웨어
	// ========================================
	f = NewRetryFetcher(f, cfg.MaxRetries, cfg.RetryDelay, cfg.MaxRetryDelay)

	// ========================================
	// 5단계: User-Agent 설정 미들웨어
	// ========================================
	// 요청마다 User-Agent 헤더를 설정 (여러 개 제공 시 무작위 선택)
	// RetryFetcher 바깥에 위치시켜 재시도 시에도 동일한 User-Agent 유지
	if len(cfg.UserAgents) > 0 {
		f = NewUserAgentFetcher(f, cfg.UserAgents, true)
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
