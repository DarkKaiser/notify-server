package middleware

import (
	"fmt"
	"net/http"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/api/httputil"
	"github.com/labstack/echo/v4"
)

// NewErrPanicRecovered 캡처된 패닉 값을 내부 시스템 오류로 래핑하여 새로운 에러를 생성합니다.
func NewErrPanicRecovered(r any) error {
	return apperrors.New(apperrors.Internal, fmt.Sprintf("%v", r))
}

// NewErrRateLimitExceeded 허용된 요청 빈도를 초과한 클라이언트에게 반환할 표준 HTTP 429(Too Many Requests) 에러를 생성합니다.
func NewErrRateLimitExceeded() error {
	return httputil.NewTooManyRequestsError("요청이 너무 많습니다. 잠시 후 다시 시도해주세요")
}

// NewErrUnsupportedMediaType 클라이언트가 요청한 Content-Type을 서버가 지원하지 않을 때 반환할 표준 HTTP 415(Unsupported Media Type) 에러를 생성합니다.
func NewErrUnsupportedMediaType() error {
	return echo.NewHTTPError(http.StatusUnsupportedMediaType, "지원하지 않는 Content-Type 형식입니다")
}
