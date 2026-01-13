package api

import (
	"net/http"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/httputil"
	appmiddleware "github.com/darkkaiser/notify-server/internal/service/api/middleware"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// HTTPServerConfig HTTP 서버 생성에 필요한 설정을 정의합니다.
type HTTPServerConfig struct {
	// Debug Echo 프레임워크의 디버그 모드 활성화 여부
	Debug bool

	// AllowOrigins CORS에서 허용할 Origin 목록
	// 개발 환경: ["*"] 또는 ["http://localhost:3000"]
	// 프로덕션 환경: 특정 도메인만 명시 (예: ["https://example.com"])
	AllowOrigins []string

	// RequestTimeout 각 HTTP 요청의 최대 처리 시간 (기본값: 60초)
	// 타임아웃 초과 시 컨텍스트를 취소하고 503 응답을 반환하여 리소스 고갈을 방지합니다.
	RequestTimeout time.Duration
}

// NewHTTPServer 설정된 미들웨어를 포함한 Echo 인스턴스를 생성합니다.
//
// 미들웨어는 다음 순서로 적용됩니다 (순서가 중요합니다):
//
//  1. PanicRecovery - 패닉 복구 및 로깅
//     - 핸들러에서 발생한 panic을 복구하여 서버 다운 방지
//     - 스택 트레이스와 함께 에러를 로깅
//     - 가장 먼저 적용되어야 다른 미들웨어의 panic도 복구 가능
//
//  2. RequestID - 요청 ID 생성
//     - 각 요청에 고유한 ID를 부여 (X-Request-ID 헤더)
//     - 로깅 및 디버깅 시 요청 추적에 사용
//     - 로깅 미들웨어보다 먼저 적용되어야 로그에 request_id 포함 가능
//
//  3. ServerHeader - Server 헤더 제거
//     - 응답 헤더에서 Server 필드를 삭제하여 기술 스택 노출 방지
//     - 공격자가 서버 버전을 파악하여 취약점을 악용하는 것을 어렵게 함
//     - 보안 감화를 위한 조치 (Security through Obscurity)
//
//  4. HTTPLogger - HTTP 요청/응답 로깅
//     - 모든 HTTP 요청과 응답 정보를 구조화된 로그로 기록
//     - 민감 정보(app_key, password 등)는 자동으로 마스킹
//     - 요청 처리 시간, 상태 코드, IP 주소 등 기록
//
//  5. RateLimiting - IP 기반 요청 제한
//     - IP 주소별로 초당 요청 수 제한 (기본: 20 req/s, 버스트: 40)
//     - Brute Force 공격 방어 및 서버 리소스 보호
//     - 제한 초과 시 429 Too Many Requests 응답
//     - 로깅 전에 적용하여 과도한 로그 생성 방지
//
//  6. BodyLimit - 요청 본문 크기 제한 (기본: 2MB, 초과 시 413 응답)
//     - 대용량 요청으로 인한 메모리 고갈 및 DoS 공격 방지
//
//  7. Timeout - 요청 처리 시간 제한 (기본: 60초, 초과 시 503 응답)
//     - 장시간 지연 요청의 리소스 점유 방지
//
//  8. CORS - Cross-Origin Resource Sharing
//     - 허용된 Origin에서의 크로스 도메인 요청 처리
//     - Preflight 요청(OPTIONS) 자동 응답
//     - 프로덕션 환경에서는 특정 도메인만 허용 권장
//
//  9. Secure - 보안 헤더 설정
//     - X-XSS-Protection, X-Content-Type-Options 등 보안 헤더 자동 추가
//     - XSS, 클릭재킹 등의 공격 방어
//     - 가장 마지막에 적용되어 모든 응답에 보안 헤더 추가
//
// 라우트 설정은 포함되지 않으며, 반환된 Echo 인스턴스에 별도로 설정해야 합니다.
func NewHTTPServer(cfg HTTPServerConfig) *echo.Echo {
	e := echo.New()

	e.Debug = cfg.Debug
	e.HideBanner = true

	// 보안 및 리소스 관리를 위한 HTTP 서버 타임아웃 설정
	e.Server.ReadTimeout = constants.DefaultReadTimeout             // 요청 본문 읽기 제한
	e.Server.ReadHeaderTimeout = constants.DefaultReadHeaderTimeout // 요청 헤더 읽기 제한
	e.Server.WriteTimeout = constants.DefaultWriteTimeout           // 응답 쓰기 제한
	e.Server.IdleTimeout = constants.DefaultIdleTimeout             // Keep-Alive 연결 유휴 제한

	// Echo 프레임워크의 내부 로그를 애플리케이션 로거로 통합합니다.
	// 이를 통해 모든 로그가 동일한 형식과 출력 대상을 사용하게 됩니다.
	e.Logger = appmiddleware.Logger{Logger: applog.StandardLogger()}

	// 전역 HTTP 에러 핸들러 설정
	e.HTTPErrorHandler = httputil.ErrorHandler

	// 타임아웃 미설정 시 기본값(60초)을 적용하여 무한 대기를 방지합니다.
	timeout := cfg.RequestTimeout
	if timeout == 0 {
		timeout = constants.DefaultRequestTimeout
	}

	// 미들웨어 적용 (권장 순서)

	// 1. Panic 복구
	e.Use(appmiddleware.PanicRecovery())
	// 2. Request ID
	e.Use(middleware.RequestID())
	// 3. Server 헤더 제거 (보안 강화)
	// 공격자에게 서버 스택 정보(Go/Echo 버전 등)를 노출하지 않도록 합니다.
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set(echo.HeaderServer, "")
			return next(c)
		}
	})
	// 4. HTTP 로깅 (RateLimit/Timeout 이전에 위치하여 429/503 에러도 기록)
	e.Use(appmiddleware.HTTPLogger())
	// 5. Rate Limiting
	e.Use(appmiddleware.RateLimiting(constants.DefaultRateLimitPerSecond, constants.DefaultRateLimitBurst))
	// 6. Body Limit (최대 2MB)
	e.Use(middleware.BodyLimit(constants.DefaultMaxBodySize))
	// 7. Timeout
	e.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{
		Timeout: timeout,
	}))
	// 8. CORS 설정
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: cfg.AllowOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))
	// 9. 보안 헤더 (XSS Protection 등)
	e.Use(middleware.Secure())

	return e
}
