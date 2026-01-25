package middleware

import (
	"fmt"
	"net/http"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/api/httputil"
	"github.com/labstack/echo/v4"
)

var (
	// ErrAppKeyRequired API 호출 자격 증명인 App Key가 누락되었을 때 반환하는 에러입니다.
	// X-App-Key 헤더 또는 app_key 쿼리 파라미터를 통해 전달되어야 합니다.
	ErrAppKeyRequired = httputil.NewBadRequestError("app_key는 필수입니다 (X-App-Key 헤더 또는 app_key 쿼리 파라미터)")

	// ErrApplicationIDRequired 식별 대상인 Application ID가 요청에 포함되지 않았을 때 반환하는 에러입니다.
	// X-Application-Id 헤더 또는 요청 본문(Body)을 통해 전달되어야 합니다.
	ErrApplicationIDRequired = httputil.NewBadRequestError("application_id는 필수입니다")

	// ErrBodyTooLarge 요청 본문의 크기가 서버 허용 한도(BodyLimit)를 초과했을 때 반환하는 표준 413 에러입니다.
	ErrBodyTooLarge = echo.NewHTTPError(http.StatusRequestEntityTooLarge, "요청 본문이 너무 큽니다")

	// ErrBodyReadFailed 네트워크 문제 등으로 인해 요청 본문을 읽는 데 실패했을 때 반환하는 에러입니다.
	ErrBodyReadFailed = httputil.NewBadRequestError("요청 본문을 읽을 수 없습니다")

	// ErrEmptyBody 필수 요청 본문(Payload)이 비어있어 작업을 수행할 수 없을 때 반환하는 에러입니다.
	ErrEmptyBody = httputil.NewBadRequestError("요청 본문이 비어있습니다")

	// ErrInvalidJSON 요청 본문이 올바른 JSON 형식이 아니어서 파싱에 실패했을 때 반환하는 에러입니다.
	ErrInvalidJSON = httputil.NewBadRequestError("잘못된 JSON 형식입니다")

	// ErrRateLimitExceeded 허용된 요청 빈도를 초과한 클라이언트에게 반환할 표준 HTTP 429(Too Many Requests) 에러입니다.
	ErrRateLimitExceeded = httputil.NewTooManyRequestsError("요청이 너무 많습니다. 잠시 후 다시 시도해주세요")

	// ErrUnsupportedMediaType 클라이언트가 요청한 Content-Type을 서버가 지원하지 않을 때 반환할 표준 HTTP 415(Unsupported Media Type) 에러입니다.
	ErrUnsupportedMediaType = echo.NewHTTPError(http.StatusUnsupportedMediaType, "지원하지 않는 Content-Type 형식입니다")
)

// NewErrPanicRecovered 캡처된 패닉 값을 내부 시스템 오류로 래핑하여 새로운 에러를 생성합니다.
func NewErrPanicRecovered(r any) error {
	return apperrors.New(apperrors.Internal, fmt.Sprintf("%v", r))
}
