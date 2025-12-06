package api

import (
	"net/http"

	appmiddleware "github.com/darkkaiser/notify-server/service/api/middleware"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log "github.com/sirupsen/logrus"
)

// HTTPServerConfig 서버 생성 시 필요한 설정을 정의합니다.
type HTTPServerConfig struct {
	// Debug는 Echo의 디버그 모드 활성화 여부를 설정합니다.
	Debug bool
	// AllowOrigins는 CORS에서 허용할 Origin 목록을 설정합니다.
	// 프로덕션 환경에서는 특정 도메인만 허용하도록 설정해야 합니다.
	AllowOrigins []string
}

// NewServer 설정된 미들웨어를 포함한 Echo 인스턴스를 생성합니다.
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
//  3. HTTPLogger - HTTP 요청/응답 로깅
//     - 모든 HTTP 요청과 응답 정보를 구조화된 로그로 기록
//     - 민감 정보(app_key, password 등)는 자동으로 마스킹
//     - 요청 처리 시간, 상태 코드, IP 주소 등 기록
//
//  4. CORS - Cross-Origin Resource Sharing
//     - 허용된 Origin에서의 크로스 도메인 요청 처리
//     - Preflight 요청(OPTIONS) 자동 응답
//     - 프로덕션 환경에서는 특정 도메인만 허용 권장
//
//  5. Secure - 보안 헤더 설정
//     - X-XSS-Protection, X-Content-Type-Options 등 보안 헤더 자동 추가
//     - XSS, 클릭재킹 등의 공격 방어
//     - 가장 마지막에 적용되어 모든 응답에 보안 헤더 추가
//
// 라우트 설정은 포함되지 않으며, 반환된 Echo 인스턴스에 별도로 설정해야 합니다.
func NewHTTPServer(cfg HTTPServerConfig) *echo.Echo {
	e := echo.New()

	e.Debug = cfg.Debug
	e.HideBanner = true

	// echo에서 출력되는 로그를 Logrus Logger로 출력되도록 한다.
	// echo Logger의 인터페이스를 래핑한 객체를 이용하여 Logrus Logger로 보낸다.
	e.Logger = appmiddleware.Logger{Logger: log.StandardLogger()}

	// 미들웨어 적용 (권장 순서)
	e.Use(appmiddleware.PanicRecovery())                   // 1. Panic 복구
	e.Use(middleware.RequestID())                          // 2. Request ID
	e.Use(appmiddleware.HTTPLogger())                      // 3. HTTP 로깅
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{ // 4. CORS 설정
		AllowOrigins: cfg.AllowOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))
	e.Use(middleware.Secure()) // 5. 보안 헤더

	return e
}
