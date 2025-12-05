package handler

import (
	"net/http"

	"github.com/darkkaiser/notify-server/service/api/model/response"
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

// NewInternalServerError 500 Internal Server Error 에러를 생성합니다
func NewInternalServerError(message string) error {
	return echo.NewHTTPError(http.StatusInternalServerError, response.ErrorResponse{
		Message: message,
	})
}

// NewSuccessResponse 표준화된 성공 응답을 생성합니다
func NewSuccessResponse(c echo.Context) error {
	return c.JSON(http.StatusOK, response.SuccessResponse{
		ResultCode: 0,
	})
}
