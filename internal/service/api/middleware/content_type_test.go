package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Content-Type 검증 미들웨어 테스트
// =============================================================================

// TestValidateContentType_Table은 다양한 시나리오에 대해 미들웨어 검증 로직을 수행합니다.
//
// 검증 항목:
//   - 정상 Content-Type 요청 허용
//   - Charset 포함 등 복합 Content-Type 처리
//   - 대소문자 무시(case-insensitive) 처리
//   - 본문이 없는 경우(GET 등) 검증 생략
//   - 잘못된 Content-Type 및 누락에 대한 415 에러 반환
func TestValidateContentType_Table(t *testing.T) {
	t.Parallel()

	// 공통 테스트 데이터
	validJSON := strings.NewReader(`{"foo":"bar"}`)

	tests := []struct {
		name                string
		expectedContentType string
		requestContentType  string
		method              string
		body                *strings.Reader
		expectedStatus      int
		expectedErr         error // 예상되는 에러 (없으면 nil)
	}{
		// 성공 케이스
		{
			name:                "성공: 정상 Content-Type (JSON)",
			expectedContentType: echo.MIMEApplicationJSON,
			requestContentType:  echo.MIMEApplicationJSON,
			method:              http.MethodPost,
			body:                validJSON,
			expectedStatus:      http.StatusOK,
		},
		{
			name:                "성공: Charset 포함 (application/json; charset=utf-8)",
			expectedContentType: echo.MIMEApplicationJSON,
			requestContentType:  "application/json; charset=utf-8",
			method:              http.MethodPost,
			body:                validJSON,
			expectedStatus:      http.StatusOK,
		},
		{
			name:                "성공: 대소문자 혼용 (Application/JSON)",
			expectedContentType: echo.MIMEApplicationJSON,
			requestContentType:  "Application/JSON",
			method:              http.MethodPost,
			body:                validJSON,
			expectedStatus:      http.StatusOK,
		},
		{
			name:                "성공: Body 없음 (GET 요청) - 검증 건너뜀",
			expectedContentType: echo.MIMEApplicationJSON,
			requestContentType:  "", // Content-Type 헤더가 없어도 됨
			method:              http.MethodGet,
			body:                nil,
			expectedStatus:      http.StatusOK,
		},
		{
			name:                "성공: Empty Body (POST 요청) - 검증 건너뜀",
			expectedContentType: echo.MIMEApplicationJSON,
			requestContentType:  "", // Content-Type 헤더가 없어도 됨
			method:              http.MethodPost,
			body:                strings.NewReader(""), // Empty Body
			expectedStatus:      http.StatusOK,
		},

		// 실패 케이스
		{
			name:                "실패: Content-Type 누락 (POST + Body)",
			expectedContentType: echo.MIMEApplicationJSON,
			requestContentType:  "",
			method:              http.MethodPost,
			body:                validJSON,
			expectedStatus:      http.StatusUnsupportedMediaType,
			expectedErr:         ErrUnsupportedMediaType,
		},
		{
			name:                "실패: 잘못된 Content-Type (text/plain)",
			expectedContentType: echo.MIMEApplicationJSON,
			requestContentType:  echo.MIMETextPlain,
			method:              http.MethodPost,
			body:                validJSON,
			expectedStatus:      http.StatusUnsupportedMediaType,
			expectedErr:         ErrUnsupportedMediaType,
		},
		{
			name:                "실패: 부분 일치 주의 (javascript vs json)",
			expectedContentType: echo.MIMEApplicationJSON,
			requestContentType:  "application/javascript",
			method:              http.MethodPost,
			body:                validJSON,
			expectedStatus:      http.StatusUnsupportedMediaType,
			expectedErr:         ErrUnsupportedMediaType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt // Capture for parallel execution
			t.Parallel()

			// 1. Setup Request
			e := echo.New()
			var req *http.Request
			if tt.body != nil {
				req = httptest.NewRequest(tt.method, "/", tt.body)
			} else {
				req = httptest.NewRequest(tt.method, "/", nil)
			}

			if tt.requestContentType != "" {
				req.Header.Set(echo.HeaderContentType, tt.requestContentType)
			}

			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// 2. Execute Middleware
			middleware := ValidateContentType(tt.expectedContentType)
			h := middleware(func(c echo.Context) error {
				return c.String(http.StatusOK, "OK")
			})
			err := h(c)

			// 3. Verify Result
			if tt.expectedStatus == http.StatusOK {
				assert.NoError(t, err)
				assert.Equal(t, http.StatusOK, rec.Code)
			} else {
				require.Error(t, err)

				// assert.ErrorIs를 사용하여 정확한 에러 변수 검증
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr)
				}

				// Status Code 검증 (Echo 에러인 경우)
				var he *echo.HTTPError
				if assert.ErrorAs(t, err, &he) {
					assert.Equal(t, tt.expectedStatus, he.Code)
				}
			}
		})
	}
}

// TestValidateContentType_LogVerification은 검증 실패 시 로그가 올바르게 기록되는지 확인합니다.
//
// 주의: applog 전역 상태를 사용하므로 병렬 실행 불가 (t.Parallel 미사용)
func TestValidateContentType_LogVerification(t *testing.T) {
	// 로그 캡처 설정
	var buf bytes.Buffer
	applog.SetOutput(&buf)
	applog.SetFormatter(&applog.JSONFormatter{})

	// 명시적으로 Info 레벨로 설정하여 기본값 보장 (다른 테스트의 영향을 받지 않도록)
	// 경고 로그는 Info 레벨 이상에서 기록됨
	// applog.GetLevel() 대신 직접 Info로 복구
	applog.SetLevel(applog.InfoLevel)

	// 테스트 종료 후 복구
	t.Cleanup(func() {
		applog.SetOutput(applog.StandardLogger().Out)
		applog.SetLevel(applog.InfoLevel)
	})

	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader("{}"))
	req.Header.Set(echo.HeaderContentType, "text/xml") // 잘못된 Content-Type
	req.Header.Set("X-Real-IP", "127.0.0.1")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Execute
	middleware := ValidateContentType(echo.MIMEApplicationJSON)
	h := middleware(func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	err := h(c)
	assert.Error(t, err)
	assert.Equal(t, http.StatusUnsupportedMediaType, err.(*echo.HTTPError).Code)

	// Verify Log
	require.Greater(t, buf.Len(), 0, "로그가 기록되어야 합니다")

	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	assert.NoError(t, err)

	assert.Equal(t, "api.middleware.content_type", logEntry["component"])
	assert.Equal(t, "warning", logEntry["level"])
	assert.Equal(t, "Content-Type 검증 실패: 지원하지 않는 형식입니다", logEntry["msg"])

	// 상세 필드 검증
	assert.Equal(t, echo.MIMEApplicationJSON, logEntry["expected"])
	assert.Equal(t, "text/xml", logEntry["actual"])
	assert.Equal(t, "/api/test", logEntry["path"])
	assert.Equal(t, "POST", logEntry["method"])
	assert.Equal(t, "127.0.0.1", logEntry["remote_ip"])
}
