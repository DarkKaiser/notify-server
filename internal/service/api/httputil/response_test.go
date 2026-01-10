package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestNewHTTPError_Table(t *testing.T) {
	tests := []struct {
		name           string
		ctor           func(string) error
		message        string
		expectedStatus int
		expectSubset   bool // for Unauthorized with dynamic message which might change slightly
	}{
		{
			name:           "BadRequest",
			ctor:           NewBadRequestError,
			message:        "잘못된 요청입니다",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "BadRequest Empty",
			ctor:           NewBadRequestError,
			message:        "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Unauthorized",
			ctor:           NewUnauthorizedError,
			message:        "인증이 필요합니다",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Unauthorized With AppID",
			ctor:           NewUnauthorizedError,
			message:        "접근이 허용되지 않은 application_id(test-app)입니다",
			expectedStatus: http.StatusUnauthorized,
			expectSubset:   true,
		},
		{
			name:           "NotFound",
			ctor:           NewNotFoundError,
			message:        "리소스를 찾을 수 없습니다",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "InternalServerError",
			ctor:           NewInternalServerError,
			message:        "서버 내부 오류가 발생했습니다",
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ctor(tt.message)
			assert.Error(t, err)

			httpErr, ok := err.(*echo.HTTPError)
			assert.True(t, ok)
			assert.Equal(t, tt.expectedStatus, httpErr.Code)

			errResp, ok := httpErr.Message.(response.ErrorResponse)
			assert.True(t, ok)

			if tt.expectSubset {
				assert.Contains(t, errResp.Message, "application_id")
			} else {
				assert.Equal(t, tt.message, errResp.Message)
			}
		})
	}
}

func TestNewSuccessResponse_Table(t *testing.T) {
	tests := []struct {
		name         string
		setupContext func() (echo.Context, *httptest.ResponseRecorder)
	}{
		{
			name: "Success Response",
			setupContext: func() (echo.Context, *httptest.ResponseRecorder) {
				e := echo.New()
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				rec := httptest.NewRecorder()
				return e.NewContext(req, rec), rec
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := tt.setupContext()
			err := NewSuccessResponse(c)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

			var resp response.SuccessResponse
			json.Unmarshal(rec.Body.Bytes(), &resp)
			assert.Equal(t, 0, resp.ResultCode)
		})
	}
}

func TestResponseHelpers_Integration_Table(t *testing.T) {
	// Simple integration check to ensure all helpers produce echo.HTTPError wrapping ExpectedResponse
	helpers := []struct {
		name string
		err  error
	}{
		{"BadRequest", NewBadRequestError("bad")},
		{"Unauthorized", NewUnauthorizedError("unauth")},
		{"NotFound", NewNotFoundError("404")},
		{"Internal", NewInternalServerError("500")},
	}

	for _, h := range helpers {
		t.Run(h.name, func(t *testing.T) {
			httpErr, ok := h.err.(*echo.HTTPError)
			assert.True(t, ok)
			_, ok = httpErr.Message.(response.ErrorResponse)
			assert.True(t, ok)
		})
	}
}
