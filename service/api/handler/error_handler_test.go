package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/service/api/model/response"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCustomHTTPErrorHandler_404는 404 에러 핸들러가 올바른 응답을 반환하는지 테스트합니다.
func TestCustomHTTPErrorHandler_404(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = CustomHTTPErrorHandler

	// 존재하지 않는 경로 요청을 시뮬레이션하기 위해 404 에러 직접 발생
	req := httptest.NewRequest(http.MethodGet, "/non-existent-path", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := echo.NewHTTPError(http.StatusNotFound, "Not Found")
	CustomHTTPErrorHandler(err, c)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var errorResp response.ErrorResponse
	jsonErr := json.Unmarshal(rec.Body.Bytes(), &errorResp)
	require.NoError(t, jsonErr)

	assert.Equal(t, "페이지를 찾을 수 없습니다.", errorResp.Message)
}

// TestCustomHTTPErrorHandler_Custom404는 커스텀 404 메시지가 보존되는지 테스트합니다.
func TestCustomHTTPErrorHandler_Custom404(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = CustomHTTPErrorHandler

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	customMsg := "사용자를 찾을 수 없습니다"
	err := echo.NewHTTPError(http.StatusNotFound, customMsg)
	CustomHTTPErrorHandler(err, c)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var errorResp response.ErrorResponse
	jsonErr := json.Unmarshal(rec.Body.Bytes(), &errorResp)
	require.NoError(t, jsonErr)

	assert.Equal(t, customMsg, errorResp.Message, "커스텀 404 메시지는 덮어쓰여지지 않아야 합니다")
}

// TestCustomHTTPErrorHandler_MethodNotAllowed는 405 에러 핸들러가 올바른 응답을 반환하는지 테스트합니다.
func TestCustomHTTPErrorHandler_MethodNotAllowed(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = CustomHTTPErrorHandler

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := echo.NewHTTPError(http.StatusMethodNotAllowed, "method not allowed")
	CustomHTTPErrorHandler(err, c)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)

	var errorResp response.ErrorResponse
	jsonErr := json.Unmarshal(rec.Body.Bytes(), &errorResp)
	require.NoError(t, jsonErr)

	assert.NotEmpty(t, errorResp.Message)
}

func TestCustomHTTPErrorHandler_500_Logging(t *testing.T) {
	// 로그 캡처 설정
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	defer logrus.SetOutput(logrus.StandardLogger().Out) // 복원

	e := echo.New()
	e.HTTPErrorHandler = CustomHTTPErrorHandler

	req := httptest.NewRequest(http.MethodGet, "/internal-error", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	CustomHTTPErrorHandler(err, c)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	// 로그 확인
	assert.Contains(t, buf.String(), "내부 서버 오류 발생")
	assert.Contains(t, buf.String(), "error")
	assert.Contains(t, buf.String(), "internal error")
}

func TestCustomHTTPErrorHandler_HEAD(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = CustomHTTPErrorHandler

	req := httptest.NewRequest(http.MethodHead, "/non-existent-path", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// 404 에러 발생
	err := echo.NewHTTPError(http.StatusNotFound, "Not Found")
	CustomHTTPErrorHandler(err, c)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Empty(t, rec.Body.String(), "HEAD 요청은 본문이 없어야 합니다")
}
