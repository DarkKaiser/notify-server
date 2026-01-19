package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Deprecated Endpoint 미들웨어 테스트
// =============================================================================

// TestDeprecatedEndpoint_InputValidation_Table은 미들웨어 생성 시 입력값 검증 로직을 테스트합니다.
//
// 검증 항목:
//   - 빈 경로 입력 시 패닉
//   - '/'로 시작하지 않는 경로 입력 시 패닉
//   - 패닉 메시지 정확성
func TestDeprecatedEndpoint_InputValidation_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		newEndpoint     string
		expectPanic     bool
		expectedMessage string
	}{
		{
			name:            "성공: 정상 경로",
			newEndpoint:     "/api/v1/notifications",
			expectPanic:     false,
			expectedMessage: "",
		},
		{
			name:            "실패: 빈 경로",
			newEndpoint:     "",
			expectPanic:     true,
			expectedMessage: constants.PanicMsgDeprecatedEndpointEmpty,
		},
		{
			name:            "실패: 슬래시 미포함 경로",
			newEndpoint:     "api/v1/notifications",
			expectPanic:     true,
			expectedMessage: "대체 엔드포인트 경로는 '/'로 시작해야 합니다",
		},
		{
			name:            "실패: 상대 경로",
			newEndpoint:     "../api/v1/notifications",
			expectPanic:     true,
			expectedMessage: "대체 엔드포인트 경로는 '/'로 시작해야 합니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.expectPanic {
				assert.Panics(t, func() {
					DeprecatedEndpoint(tt.newEndpoint)
				}, "잘못된 입력값에 대해 패닉이 발생해야 합니다")

				// 패닉 메시지 검증 (PanicsWithValue가 부분 일치를 지원하지 않아 recover 사용)
				defer func() {
					if r := recover(); r != nil {
						assert.Contains(t, fmt.Sprint(r), tt.expectedMessage)
					}
				}()
				DeprecatedEndpoint(tt.newEndpoint)
			} else {
				assert.NotPanics(t, func() {
					DeprecatedEndpoint(tt.newEndpoint)
				}, "정상 입력값에 대해 패닉이 발생하지 않아야 합니다")
			}
		})
	}
}

// TestDeprecatedEndpoint_Headers_Table은 응답 헤더가 올바르게 설정되는지 검증합니다.
//
// 검증 항목:
//   - Warning 헤더 (RFC 7234 형식)
//   - X-API-Deprecated 헤더
//   - X-API-Deprecated-Replacement 헤더
//   - 다양한 HTTP 메서드 호환성
func TestDeprecatedEndpoint_Headers_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		newEndpoint string
		method      string
	}{
		{
			name:        "기본 GET 요청",
			newEndpoint: "/api/v1/notifications",
			method:      http.MethodGet,
		},
		{
			name:        "POST 요청",
			newEndpoint: "/api/v2/messages",
			method:      http.MethodPost,
		},
		{
			name:        "특수 문자 포함 경로",
			newEndpoint: "/api/v1/user-notifications",
			method:      http.MethodPut,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup
			e := echo.New()
			req := httptest.NewRequest(tt.method, "/old/api", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			handler := func(c echo.Context) error {
				return c.String(http.StatusOK, "success")
			}

			// Execute
			middleware := DeprecatedEndpoint(tt.newEndpoint)
			h := middleware(handler)
			err := h(c)

			// Verify
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)

			// Warning 헤더 검증
			expectedWarning := fmt.Sprintf("299 - \"Deprecated API endpoint. Use %s instead.\"", tt.newEndpoint)
			assert.Equal(t, expectedWarning, rec.Header().Get(constants.Warning))

			// 커스텀 헤더 검증
			assert.Equal(t, "true", rec.Header().Get(constants.XAPIDeprecated))
			assert.Equal(t, tt.newEndpoint, rec.Header().Get(constants.XAPIDeprecatedReplacement))
		})
	}
}

// TestDeprecatedEndpoint_LogVerification은 구조화된 로그가 올바르게 기록되는지 검증합니다.
//
// 검증 항목:
//   - 로그 메시지 ("Deprecated API 엔드포인트 사용됨")
//   - 로그 필드 (deprecated_endpoint, replacement, method, remote_ip, user_agent)
func TestDeprecatedEndpoint_LogVerification(t *testing.T) {
	// 로그 캡처를 위해 직렬 실행
	var buf bytes.Buffer
	applog.SetOutput(&buf)
	applog.SetFormatter(&applog.JSONFormatter{})
	defer applog.SetOutput(applog.StandardLogger().Out)

	newEndpoint := "/api/v1/new"
	middleware := DeprecatedEndpoint(newEndpoint)
	handler := func(c echo.Context) error { return c.NoContent(http.StatusOK) }
	h := middleware(handler)

	// 요청 생성
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/old/api", nil)
	req.Header.Set("User-Agent", "TestClient/1.0")
	req.Header.Set("X-Real-IP", "10.0.0.1")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/old/api") // c.Path() 값이 반환되도록 경로 설정

	// 실행
	err := h(c)
	require.NoError(t, err)

	// 로그 검증
	require.Greater(t, buf.Len(), 0, "로그가 기록되어야 합니다")

	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	assert.NoError(t, err, "JSON 로그 파싱 실패")

	// 1. 공통 필드 검증
	assert.Equal(t, "api.middleware.deprecated", logEntry["component"])
	assert.Equal(t, "warning", logEntry["level"])
	assert.Equal(t, constants.LogMsgDeprecatedEndpointUsed, logEntry["msg"])

	// 2. 상세 필드 검증
	assert.Equal(t, "/old/api", logEntry["deprecated_endpoint"])
	assert.Equal(t, newEndpoint, logEntry["replacement"])
	assert.Equal(t, http.MethodGet, logEntry["method"])
	assert.Equal(t, "10.0.0.1", logEntry["remote_ip"])
	assert.Equal(t, "TestClient/1.0", logEntry["user_agent"])
}

// TestDeprecatedEndpoint_HandlerError는 핸들러 에러 발생 시에도 헤더가 유지되는지 검증합니다.
func TestDeprecatedEndpoint_HandlerError(t *testing.T) {
	t.Parallel()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// 에러를 반환하는 핸들러
	handler := func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	newEndpoint := "/api/v1/valid"
	middleware := DeprecatedEndpoint(newEndpoint)
	h := middleware(handler)

	err := h(c)
	assert.Error(t, err)

	// 에러 발생 시에도 경고 헤더는 포함되어야 함
	assert.NotEmpty(t, rec.Header().Get(constants.Warning))
	assert.Equal(t, "true", rec.Header().Get(constants.XAPIDeprecated))
	assert.Equal(t, newEndpoint, rec.Header().Get(constants.XAPIDeprecatedReplacement))
}
