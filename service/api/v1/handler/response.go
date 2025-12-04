package handler

import (
	"net/http"

	"github.com/darkkaiser/notify-server/service/api/v1/model"
	"github.com/labstack/echo/v4"
)

// newBadRequestError 400 Bad Request 에러를 생성합니다
func newBadRequestError(message string) error {
	return echo.NewHTTPError(http.StatusBadRequest, message)
}

// newUnauthorizedError 401 Unauthorized 에러를 생성합니다
func newUnauthorizedError(message string) error {
	return echo.NewHTTPError(http.StatusUnauthorized, message)
}

// newSuccessResponse 표준화된 성공 응답을 생성합니다
func newSuccessResponse(c echo.Context) error {
	return c.JSON(http.StatusOK, model.SuccessResponse{
		ResultCode: 0,
	})
}
