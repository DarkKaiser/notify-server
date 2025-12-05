package server

import (
	"net/http"

	"github.com/darkkaiser/notify-server/service/api/v1/handler"
	appmiddleware "github.com/darkkaiser/notify-server/service/api/v1/middleware"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log "github.com/sirupsen/logrus"
)

// Config Router 생성 시 필요한 설정을 정의합니다.
type Config struct {
	// Debug는 Echo의 디버그 모드 활성화 여부를 설정합니다.
	Debug bool
	// AllowOrigins는 CORS에서 허용할 Origin 목록을 설정합니다.
	// 프로덕션 환경에서는 특정 도메인만 허용하도록 설정해야 합니다.
	AllowOrigins []string
}

// New 설정된 미들웨어와 라우트를 포함한 Echo 인스턴스를 생성합니다.
// 미들웨어는 다음 순서로 적용됩니다:
//  1. Recover - 패닉 복구
//  2. RequestID - 요청 ID 생성
//  3. LogrusLogger - 로깅
//  4. CORS - Cross-Origin Resource Sharing
//  5. Secure - 보안 헤더 설정
//
// 라우트 설정까지 완료된 Echo 인스턴스를 반환하므로 바로 사용 가능합니다.
func New(cfg Config, h *handler.Handler) *echo.Echo {
	e := echo.New()

	e.Debug = cfg.Debug
	e.HideBanner = true

	// echo에서 출력되는 로그를 Logrus Logger로 출력되도록 한다.
	// echo Logger의 인터페이스를 래핑한 객체를 이용하여 Logrus Logger로 보낸다.
	e.Logger = appmiddleware.Logger{Logger: log.StandardLogger()}

	// 미들웨어 적용 (권장 순서)
	e.Use(appmiddleware.LogrusRecover())                   // 1. Panic 복구
	e.Use(middleware.RequestID())                          // 2. Request ID
	e.Use(appmiddleware.LogrusLogger())                    // 3. 로깅
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{ // 4. CORS
		AllowOrigins: cfg.AllowOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))
	e.Use(middleware.Secure()) // 5. 보안 헤더

	// 라우트 설정
	SetupRoutes(e, h)

	return e
}
