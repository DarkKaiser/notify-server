package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Deprecated Endpoint 미들웨어 테스트
// =============================================================================

// TestDeprecatedEndpoint_InputValidation 미들웨어 생성 단계의 입력값 검증을 테스트합니다.
//
// 검증 항목:
//   - 빈 경로 입력 시 패닉 발생 확인
//   - '/'로 시작하지 않는 경로 입력 시 패닉 발생 확인
//   - 패닉 메시지의 정확성
func TestDeprecatedEndpoint_InputValidation(t *testing.T) {
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
			expectedMessage: "Deprecated: 대체 엔드포인트 경로가 비어있습니다",
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
		t.Run(tt.name, func(t *testing.T) {
			tt := tt // Capture for parallel execution
			t.Parallel()

			if tt.expectPanic {
				assert.Panics(t, func() {
					DeprecatedEndpoint(tt.newEndpoint)
				}, "잘못된 입력값에 대해 패닉이 발생해야 합니다")

				// 패닉 메시지 상세 검증 (recover 활용)
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

// TestDeprecatedEndpoint_Behavior 미들웨어의 핵심 동작을 다양한 시나리오에서 검증합니다.
//
// 검증 항목:
//   - HTTP 응답 헤더 설정 (Warning, X-API-Deprecated 등)
//   - 다양한 HTTP 메서드(GET, POST 등)에 대한 지원
//   - 다음 핸들러(Next Handler) 실행 여부 및 에러 전파
func TestDeprecatedEndpoint_Behavior(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		method         string
		newEndpoint    string
		handlerError   error // 핸들러가 반환할 에러 (없으면 nil)
		expectedStatus int
	}{
		{
			name:           "성공: 기본 GET 요청",
			method:         http.MethodGet,
			newEndpoint:    "/api/v1/notifications",
			handlerError:   nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "성공: POST 요청",
			method:         http.MethodPost,
			newEndpoint:    "/api/v2/messages",
			handlerError:   nil,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "성공: 핸들러 에러 전파",
			method:         http.MethodGet,
			newEndpoint:    "/api/v1/valid",
			handlerError:   echo.NewHTTPError(http.StatusBadRequest, "bad request"),
			expectedStatus: http.StatusBadRequest, // 에러 코드가 유지되어야 함
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt // Capture for parallel execution
			t.Parallel()

			// Setup
			e := echo.New()
			req := httptest.NewRequest(tt.method, "/old/api", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			handler := func(c echo.Context) error {
				if tt.handlerError != nil {
					return tt.handlerError
				}
				return c.NoContent(tt.expectedStatus)
			}

			// Execute
			middleware := DeprecatedEndpoint(tt.newEndpoint)
			h := middleware(handler)
			err := h(c)

			// Verify Error Propagation
			if tt.handlerError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.handlerError, err, "핸들러의 에러가 전파되어야 합니다")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedStatus, rec.Code)
			}

			// Verify Headers (항상 존재해야 함)
			expectedWarning := fmt.Sprintf("299 - \"Deprecated API endpoint. Use %s instead.\"", tt.newEndpoint)
			assert.Equal(t, expectedWarning, rec.Header().Get(headerWarning))
			assert.Equal(t, "true", rec.Header().Get(headerXAPIDeprecated))
			assert.Equal(t, tt.newEndpoint, rec.Header().Get(headerXAPIDeprecatedReplacement))
		})
	}
}

// TestDeprecatedEndpoint_Concurrency 고루틴을 활용하여 동시성 상황에서의 안전성을 검증합니다.
//
// 검증 항목:
//   - 다수의 동시 요청 시 헤더 설정 경합 여부
//   - 미들웨어 인스턴스 재사용 안전성
func TestDeprecatedEndpoint_Concurrency(t *testing.T) {
	t.Parallel()

	newEndpoint := "/api/v1/safe"
	middleware := DeprecatedEndpoint(newEndpoint)
	handler := func(c echo.Context) error { return c.NoContent(http.StatusOK) }
	h := middleware(handler)
	e := echo.New()

	var wg sync.WaitGroup
	concurrency := 50

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/old", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			_ = h(c)

			// 동시 요청 상황에서도 헤더가 정확해야 함
			if rec.Header().Get(headerXAPIDeprecated) != "true" {
				t.Errorf("Concurrency Error: 헤더 설정 실패")
			}
		}()
	}
	wg.Wait()
}

// TestDeprecatedEndpoint_Logging 구조화된 로그 기록을 검증합니다.
//
// 주의: applog가 전역 상태를 사용하므로, 이 테스트는 병렬로 실행할 수 없습니다 (t.Parallel 미사용).
//
// 검증 항목:
//   - 로그 레벨 (Warning)
//   - 컴포넌트 필드
//   - 컨텍스트 필드 (deprecated_endpoint, replacement 등)
func TestDeprecatedEndpoint_Logging(t *testing.T) {
	// 로그 캡처 설정
	var buf bytes.Buffer
	applog.SetOutput(&buf)
	applog.SetFormatter(&applog.JSONFormatter{})
	// 테스트 종료 후 복구
	t.Cleanup(func() {
		applog.SetOutput(applog.StandardLogger().Out)
	})

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
	c.SetPath("/old/api")

	// 실행
	err := h(c)
	require.NoError(t, err)

	// 로그 파싱 및 검증
	require.Greater(t, buf.Len(), 0, "로그가 기록되어야 합니다")

	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	assert.NoError(t, err, "JSON 로그 파싱 실패")

	assert.Equal(t, "api.middleware.deprecated", logEntry["component"])
	assert.Equal(t, "warning", logEntry["level"])
	assert.Equal(t, "경고: Deprecated 엔드포인트가 호출되었습니다", logEntry["msg"])

	// 상세 필드 검증
	assert.Equal(t, "/old/api", logEntry["deprecated_endpoint"])
	assert.Equal(t, newEndpoint, logEntry["replacement"])
	assert.Equal(t, http.MethodGet, logEntry["method"])
	assert.Equal(t, "10.0.0.1", logEntry["remote_ip"])
	assert.Equal(t, "TestClient/1.0", logEntry["user_agent"])
}
