package auth

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

var (
	// ErrApplicationMissingInContext Context 내에서 필수 애플리케이션 정보를 조회할 수 없을 때 반환하는 에러입니다.
	ErrApplicationMissingInContext = errors.New("Context에서 애플리케이션 정보를 찾을 수 없습니다")

	// ErrApplicationTypeMismatch Context에 저장된 객체가 예상된 *domain.Application 타입이 아닐 때 반환하는 타입 단언(Type Assertion) 에러입니다.
	ErrApplicationTypeMismatch = errors.New("Context에 저장된 애플리케이션 정보의 타입이 올바르지 않습니다")
)

// NewErrInvalidApplicationID 요청된 Application ID가 시스템에 등록되어 있지 않거나 식별할 수 없을 때 반환하는 인증 에러(401 Unauthorized)를 생성합니다.
func NewErrInvalidApplicationID(id string) error {
	return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("등록되지 않은 application_id입니다 (ID: %s)", id))
}

// NewErrInvalidAppKey 제공된 App Key가 해당 Application ID의 인증 정보와 일치하지 않을 때 반환하는 인증 에러(401 Unauthorized)를 생성합니다.
func NewErrInvalidAppKey(id string) error {
	return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("app_key가 유효하지 않습니다 (application_id: %s)", id))
}
