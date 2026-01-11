package httputil

import (
	"net/http"

	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	"github.com/labstack/echo/v4"
)

// NewBadRequestError 400 Bad Request 에러를 생성합니다
func NewBadRequestError(message string) error {
	return echo.NewHTTPError(http.StatusBadRequest, response.ErrorResponse{
		Message: message,
	})
}

// NewUnauthorizedError 401 Unauthorized 에러를 생성합니다
func NewUnauthorizedError(message string) error {
	return echo.NewHTTPError(http.StatusUnauthorized, response.ErrorResponse{
		Message: message,
	})
}

// NewNotFoundError 404 Not Found 에러를 생성합니다
func NewNotFoundError(message string) error {
	return echo.NewHTTPError(http.StatusNotFound, response.ErrorResponse{
		Message: message,
	})
}

// NewTooManyRequestsError 429 Too Many Requests 에러를 생성합니다
func NewTooManyRequestsError(message string) error {
	return echo.NewHTTPError(http.StatusTooManyRequests, response.ErrorResponse{
		Message: message,
	})
}

// NewInternalServerError 500 Internal Server Error 에러를 생성합니다
func NewInternalServerError(message string) error {
	return echo.NewHTTPError(http.StatusInternalServerError, response.ErrorResponse{
		Message: message,
	})
}

// NewServiceUnavailableError 503 Service Unavailable 에러를 생성합니다
func NewServiceUnavailableError(message string) error {
	return echo.NewHTTPError(http.StatusServiceUnavailable, response.ErrorResponse{
		Message: message,
	})
}

// NewSuccessResponse 표준 성공 응답(200 OK)을 JSON 형식으로 반환합니다.
func NewSuccessResponse(c echo.Context) error {
	return c.JSON(http.StatusOK, response.SuccessResponse{
		ResultCode: 0,
	})
}
