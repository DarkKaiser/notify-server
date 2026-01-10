package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeprecatedEndpoint DeprecatedEndpoint 미들웨어의 기본 동작을 검증합니다.
func TestDeprecatedEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		newEndpoint string
	}{
		{
			name:        "Standard Replacement",
			newEndpoint: "/api/v1/notifications",
		},
		{
			name:        "Different Replacement",
			newEndpoint: "/api/v2/messages",
		},
		{
			name:        "Nested Path",
			newEndpoint: "/api/v1/users/notifications",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Handler
			handler := func(c echo.Context) error {
				return c.String(http.StatusOK, "test")
			}

			// Apply middleware
			middleware := DeprecatedEndpoint(tt.newEndpoint)
			h := middleware(handler)

			// Execute
			err := h(c)

			// Verify
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)

			// Verify headers - 정확한 값 검증
			expectedWarning := "299 - \"더 이상 사용되지 않는 API입니다. " + tt.newEndpoint + "를 사용하세요\""
			assert.Equal(t, expectedWarning, rec.Header().Get(constants.HeaderWarning))
			assert.Equal(t, "true", rec.Header().Get(constants.HeaderXAPIDeprecated))
			assert.Equal(t, tt.newEndpoint, rec.Header().Get(constants.HeaderXAPIDeprecatedReplacement))
		})
	}
}

// TestDeprecatedEndpoint_HandlerError 핸들러 에러 시 미들웨어 동작을 검증합니다.
func TestDeprecatedEndpoint_HandlerError(t *testing.T) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Handler that returns error
	handler := func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusBadRequest, "test error")
	}

	// Apply middleware
	middleware := DeprecatedEndpoint("/api/v1/new")
	h := middleware(handler)

	// Execute
	err := h(c)

	// Verify
	assert.Error(t, err)

	// Headers should still be set even on error
	assert.Contains(t, rec.Header().Get(constants.HeaderWarning), "더 이상 사용되지 않는 API")
	assert.Equal(t, "true", rec.Header().Get(constants.HeaderXAPIDeprecated))
	assert.Equal(t, "/api/v1/new", rec.Header().Get(constants.HeaderXAPIDeprecatedReplacement))
}

// TestDeprecatedEndpoint_EmptyEndpoint 빈 문자열 입력 시 패닉을 검증합니다.
func TestDeprecatedEndpoint_EmptyEndpoint(t *testing.T) {
	assert.PanicsWithValue(t,
		"[DeprecatedEndpoint] 대체 엔드포인트 경로(newEndpoint)가 비어있습니다. 유효한 경로를 지정해야 합니다",
		func() {
			DeprecatedEndpoint("")
		},
		"Should panic with specific message when newEndpoint is empty")
}

// TestDeprecatedEndpoint_InvalidEndpoint 잘못된 경로 형식 입력 시 패닉을 검증합니다.
func TestDeprecatedEndpoint_InvalidEndpoint(t *testing.T) {
	tests := []struct {
		name          string
		newEndpoint   string
		expectedPanic string
	}{
		{
			name:          "No Leading Slash",
			newEndpoint:   "api/v1/notifications",
			expectedPanic: "[DeprecatedEndpoint] 대체 엔드포인트 경로(newEndpoint)는 반드시 '/'로 시작해야 합니다. 현재 값: api/v1/notifications",
		},
		{
			name:          "Relative Path",
			newEndpoint:   "../api/v1/notifications",
			expectedPanic: "[DeprecatedEndpoint] 대체 엔드포인트 경로(newEndpoint)는 반드시 '/'로 시작해야 합니다. 현재 값: ../api/v1/notifications",
		},
		{
			name:          "Just Text",
			newEndpoint:   "notifications",
			expectedPanic: "[DeprecatedEndpoint] 대체 엔드포인트 경로(newEndpoint)는 반드시 '/'로 시작해야 합니다. 현재 값: notifications",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.PanicsWithValue(t, tt.expectedPanic, func() {
				DeprecatedEndpoint(tt.newEndpoint)
			}, "Should panic with specific message")
		})
	}
}

// TestDeprecatedEndpoint_VariousHTTPMethods 다양한 HTTP 메서드로 요청 시 헤더가 올바르게 설정되는지 검증합니다.
func TestDeprecatedEndpoint_VariousHTTPMethods(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			// Setup
			e := echo.New()
			req := httptest.NewRequest(method, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Handler
			handler := func(c echo.Context) error {
				return c.String(http.StatusOK, "test")
			}

			// Apply middleware
			middleware := DeprecatedEndpoint("/api/v1/new")
			h := middleware(handler)

			// Execute
			err := h(c)

			// Verify
			assert.NoError(t, err)
			assert.Equal(t, "true", rec.Header().Get(constants.HeaderXAPIDeprecated))
			assert.Equal(t, "/api/v1/new", rec.Header().Get(constants.HeaderXAPIDeprecatedReplacement))
		})
	}
}

// TestDeprecatedEndpoint_WarningMessageFormat Warning 헤더 메시지 형식이 정확한지 검증합니다.
func TestDeprecatedEndpoint_WarningMessageFormat(t *testing.T) {
	tests := []struct {
		name            string
		newEndpoint     string
		expectedWarning string
	}{
		{
			name:            "Simple Path",
			newEndpoint:     "/api/v1/new",
			expectedWarning: "299 - \"더 이상 사용되지 않는 API입니다. /api/v1/new를 사용하세요\"",
		},
		{
			name:            "Nested Path",
			newEndpoint:     "/api/v2/users/notifications",
			expectedWarning: "299 - \"더 이상 사용되지 않는 API입니다. /api/v2/users/notifications를 사용하세요\"",
		},
		{
			name:            "Path with Hyphen",
			newEndpoint:     "/api/v1/user-notifications",
			expectedWarning: "299 - \"더 이상 사용되지 않는 API입니다. /api/v1/user-notifications를 사용하세요\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			handler := func(c echo.Context) error {
				return c.NoContent(http.StatusOK)
			}

			// Apply middleware
			middleware := DeprecatedEndpoint(tt.newEndpoint)
			h := middleware(handler)

			// Execute
			err := h(c)

			// Verify
			require.NoError(t, err)
			assert.Equal(t, tt.expectedWarning, rec.Header().Get(constants.HeaderWarning),
				"Warning message format should match RFC 7234 style")
		})
	}
}

// TestDeprecatedEndpoint_PathWithQueryParams 쿼리 파라미터가 있는 경로를 테스트합니다.
func TestDeprecatedEndpoint_PathWithQueryParams(t *testing.T) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/old/path?key=value&foo=bar", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	}

	// Apply middleware
	middleware := DeprecatedEndpoint("/api/v1/new")
	h := middleware(handler)

	// Execute
	err := h(c)

	// Verify
	assert.NoError(t, err)
	assert.Equal(t, "true", rec.Header().Get(constants.HeaderXAPIDeprecated))
	assert.Equal(t, "/api/v1/new", rec.Header().Get(constants.HeaderXAPIDeprecatedReplacement))
}

// TestDeprecatedEndpoint_SpecialCharactersInPath 특수 문자가 포함된 경로를 테스트합니다.
func TestDeprecatedEndpoint_SpecialCharactersInPath(t *testing.T) {
	tests := []struct {
		name        string
		newEndpoint string
	}{
		{
			name:        "Path with Underscore",
			newEndpoint: "/api/v1/user_notifications",
		},
		{
			name:        "Path with Hyphen",
			newEndpoint: "/api/v1/user-notifications",
		},
		{
			name:        "Path with Numbers",
			newEndpoint: "/api/v2/notifications",
		},
		{
			name:        "Path with Mixed",
			newEndpoint: "/api/v1/user-notifications_v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			handler := func(c echo.Context) error {
				return c.NoContent(http.StatusOK)
			}

			// Apply middleware
			middleware := DeprecatedEndpoint(tt.newEndpoint)
			h := middleware(handler)

			// Execute
			err := h(c)

			// Verify
			assert.NoError(t, err)
			assert.Equal(t, tt.newEndpoint, rec.Header().Get(constants.HeaderXAPIDeprecatedReplacement))
		})
	}
}

// TestDeprecatedEndpoint_ResponseBodyPreserved 응답 본문이 보존되는지 검증합니다.
func TestDeprecatedEndpoint_ResponseBodyPreserved(t *testing.T) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	expectedBody := `{"message": "test response"}`
	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, expectedBody)
	}

	// Apply middleware
	middleware := DeprecatedEndpoint("/api/v1/new")
	h := middleware(handler)

	// Execute
	err := h(c)

	// Verify
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, expectedBody, rec.Body.String(), "Response body should be preserved")
	assert.Equal(t, "true", rec.Header().Get(constants.HeaderXAPIDeprecated))
}
