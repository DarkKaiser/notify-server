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
)

func TestCustomHTTPErrorHandler_Table(t *testing.T) {
	// Setup Logger capture
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	defer logrus.SetOutput(logrus.StandardLogger().Out)

	tests := []struct {
		name           string
		method         string
		path           string
		err            error
		expectedStatus int
		expectLog      []string
		verifyResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "404 Not Found",
			method:         http.MethodGet,
			path:           "/non-existent",
			err:            echo.NewHTTPError(http.StatusNotFound, "Not Found"),
			expectedStatus: http.StatusNotFound,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var errorResp response.ErrorResponse
				json.Unmarshal(rec.Body.Bytes(), &errorResp)
				assert.Equal(t, "페이지를 찾을 수 없습니다.", errorResp.Message)
			},
		},
		{
			name:           "Custom 404 Message",
			method:         http.MethodGet,
			path:           "/users/123",
			err:            echo.NewHTTPError(http.StatusNotFound, "Custom 404"),
			expectedStatus: http.StatusNotFound,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var errorResp response.ErrorResponse
				json.Unmarshal(rec.Body.Bytes(), &errorResp)
				assert.Equal(t, "Custom 404", errorResp.Message)
			},
		},
		{
			name:           "405 Method Not Allowed",
			method:         http.MethodPost,
			path:           "/health",
			err:            echo.NewHTTPError(http.StatusMethodNotAllowed, "method not allowed"),
			expectedStatus: http.StatusMethodNotAllowed,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var errorResp response.ErrorResponse
				json.Unmarshal(rec.Body.Bytes(), &errorResp)
				assert.NotEmpty(t, errorResp.Message)
			},
		},
		{
			name:           "500 Internal Server Error",
			method:         http.MethodGet,
			path:           "/error",
			err:            echo.NewHTTPError(http.StatusInternalServerError, "internal error"),
			expectedStatus: http.StatusInternalServerError,
			expectLog:      []string{"내부 서버 오류 발생", "internal error"},
		},
		{
			name:           "HEAD Request 404",
			method:         http.MethodHead,
			path:           "/non-existent",
			err:            echo.NewHTTPError(http.StatusNotFound, "Not Found"),
			expectedStatus: http.StatusNotFound,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Empty(t, rec.Body.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			e := echo.New()
			e.HTTPErrorHandler = CustomHTTPErrorHandler

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			CustomHTTPErrorHandler(tt.err, c)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.verifyResponse != nil {
				tt.verifyResponse(t, rec)
			}

			if len(tt.expectLog) > 0 {
				logOutput := buf.String()
				for _, expect := range tt.expectLog {
					assert.Contains(t, logOutput, expect)
				}
			}
		})
	}
}
