package middleware

import (
	"fmt"
	"runtime"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

const (
	// stackBufferSize panic 발생 시 스택 트레이스를 저장할 버퍼 크기 (4KB)
	stackBufferSize = 4 << 10
)

// PanicRecovery panic을 복구하고 로깅하는 미들웨어를 반환합니다.
//
// 이 미들웨어는 핸들러에서 발생한 panic을 복구하여 서버 다운을 방지하고,
// 스택 트레이스와 함께 에러를 로깅합니다.
func PanicRecovery() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					err, ok := r.(error)
					if !ok {
						err = apperrors.New(apperrors.Internal, fmt.Sprintf("%v", r))
					}

					// 스택 트레이스 수집
					stack := make([]byte, stackBufferSize)
					length := runtime.Stack(stack, false)

					// 로깅 필드 구성
					fields := log.Fields{
						"error": err,
						"stack": string(stack[:length]),
					}

					// Request ID가 있으면 추가
					if requestID := c.Response().Header().Get(echo.HeaderXRequestID); requestID != "" {
						fields["request_id"] = requestID
					}

					applog.WithComponentAndFields("api.middleware", fields).Error("PANIC RECOVERED")

					// Echo의 에러 핸들러로 전달
					c.Error(err)
				}
			}()
			return next(c)
		}
	}
}
