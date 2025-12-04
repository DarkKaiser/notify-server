package handler

import (
	"net/http"

	"github.com/darkkaiser/notify-server/service/api/v1/model"
	"github.com/labstack/echo/v4"
)

// newBadRequestError 400 Bad Request 에러를 생성합니다
func newBadRequestError(message string) error {
	return echo.NewHTTPError(http.StatusBadRequest, model.ErrorResponse{
		Message: message,
	})
}

// newUnauthorizedError 401 Unauthorized 에러를 생성합니다
func newUnauthorizedError(message string) error {
	return echo.NewHTTPError(http.StatusUnauthorized, model.ErrorResponse{
		Message: message,
	})
}

// newNotFoundError 404 Not Found 에러를 생성합니다
func newNotFoundError(message string) error {
	return echo.NewHTTPError(http.StatusNotFound, model.ErrorResponse{
		Message: message,
	})
}

// newInternalServerError 500 Internal Server Error 에러를 생성합니다
func newInternalServerError(message string) error {
	return echo.NewHTTPError(http.StatusInternalServerError, model.ErrorResponse{
		Message: message,
	})
}

// newSuccessResponse 표준화된 성공 응답을 생성합니다
func newSuccessResponse(c echo.Context) error {
	return c.JSON(http.StatusOK, model.SuccessResponse{
		ResultCode: 0,
	})
}
