package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/service/api/v1/model"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestNewBadRequestError(t *testing.T) {
	t.Run("400 에러 생성", func(t *testing.T) {
		message := "잘못된 요청입니다"
		err := newBadRequestError(message)

		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		assert.True(t, ok, "echo.HTTPError 타입이어야 합니다")
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.Equal(t, message, httpErr.Message)
	})

	t.Run("빈 메시지로 에러 생성", func(t *testing.T) {
		err := newBadRequestError("")

		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		assert.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.Equal(t, "", httpErr.Message)
	})
}

func TestNewUnauthorizedError(t *testing.T) {
	t.Run("401 에러 생성", func(t *testing.T) {
		message := "인증이 필요합니다"
		err := newUnauthorizedError(message)

		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		assert.True(t, ok, "echo.HTTPError 타입이어야 합니다")
		assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
		assert.Equal(t, message, httpErr.Message)
	})

	t.Run("application_id를 포함한 에러 메시지", func(t *testing.T) {
		message := "접근이 허용되지 않은 application_id(test-app)입니다"
		err := newUnauthorizedError(message)

		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		assert.True(t, ok)
		assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
		assert.Contains(t, httpErr.Message, "application_id")
		assert.Contains(t, httpErr.Message, "test-app")
	})
}

func TestNewSuccessResponse(t *testing.T) {
	t.Run("성공 응답 생성", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := newSuccessResponse(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp model.SuccessResponse
		jsonErr := json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.NoError(t, jsonErr)
		assert.Equal(t, 0, resp.ResultCode)
	})

	t.Run("응답 JSON 형식 검증", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := newSuccessResponse(c)

		assert.NoError(t, err)
		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

		expectedJSON := `{"result_code":0}`
		assert.JSONEq(t, expectedJSON, rec.Body.String())
	})
}

func TestResponseHelpers_Integration(t *testing.T) {
	t.Run("여러 에러 타입 비교", func(t *testing.T) {
		badReqErr := newBadRequestError("bad request")
		unauthorizedErr := newUnauthorizedError("unauthorized")

		badReqHTTPErr, _ := badReqErr.(*echo.HTTPError)
		unauthorizedHTTPErr, _ := unauthorizedErr.(*echo.HTTPError)

		assert.NotEqual(t, badReqHTTPErr.Code, unauthorizedHTTPErr.Code)
		assert.Equal(t, http.StatusBadRequest, badReqHTTPErr.Code)
		assert.Equal(t, http.StatusUnauthorized, unauthorizedHTTPErr.Code)
	})

	t.Run("에러와 성공 응답 구분", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// 성공 응답은 에러가 아님
		successErr := newSuccessResponse(c)
		assert.NoError(t, successErr)

		// 에러 응답은 에러임
		badReqErr := newBadRequestError("error")
		assert.Error(t, badReqErr)
	})
}
