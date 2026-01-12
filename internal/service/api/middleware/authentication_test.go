package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/api/auth"
	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/model/domain"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Authentication 미들웨어 테스트
// =============================================================================

// 테스트용 Authenticator 생성 헬퍼
func createTestAuthenticator() *auth.Authenticator {
	cfg := &config.AppConfig{
		NotifyAPI: config.NotifyAPIConfig{
			Applications: []config.ApplicationConfig{
				{
					ID:                "test-app",
					AppKey:            "test-key-123",
					Title:             "Test Application",
					Description:       "Test application for unit tests",
					DefaultNotifierID: "test-notifier",
				},
				{
					ID:                "another-app",
					AppKey:            "another-key-456",
					Title:             "Another Application",
					Description:       "Another test application",
					DefaultNotifierID: "another-notifier",
				},
			},
		},
	}
	return auth.NewAuthenticator(cfg)
}

// 테스트용 요청 바디 생성 헬퍼
func createRequestBody(applicationID string) *bytes.Buffer {
	body := map[string]interface{}{
		"application_id": applicationID,
		"message":        "Test message",
		"error_occurred": false,
	}
	bodyBytes, _ := json.Marshal(body)
	return bytes.NewBuffer(bodyBytes)
}

// 다음 핸들러 모킹 (인증 성공 검증용)
func createAuthSuccessHandler(t *testing.T, expectedApp *domain.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Context에서 Application 추출
		app := c.Get(constants.ContextKeyApplication)
		require.NotNil(t, app, "Context에 Application 객체가 저장되어야 합니다")

		actualApp, ok := app.(*domain.Application)
		require.True(t, ok, "Context 저장 값이 *domain.Application 타입이어야 합니다")

		if expectedApp != nil {
			assert.Equal(t, expectedApp.ID, actualApp.ID)
			assert.Equal(t, expectedApp.Title, actualApp.Title)
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "success"})
	}
}

// TestRequireAuthentication_Scenarios는 다양한 인증 시나리오를 검증합니다.
func TestRequireAuthentication_Scenarios(t *testing.T) {
	t.Parallel()

	authenticator := createTestAuthenticator()
	middleware := RequireAuthentication(authenticator)
	validBody := createRequestBody("test-app")
	validBodyBytes := validBody.Bytes()

	tests := []struct {
		name           string
		appKeyHeader   string
		appKeyQuery    string
		appIDHeader    string // New: X-Application-Id 헤더 테스트용
		body           []byte
		expectedCode   int
		expectErrorStr string
	}{
		{
			name:         "성공: 헤더 인증 (권장)",
			appKeyHeader: "test-key-123",
			body:         validBodyBytes,
			expectedCode: http.StatusOK,
		},
		{
			name:         "성공: 쿼리 파라미터 인증 (레거시)",
			appKeyQuery:  "test-key-123",
			body:         validBodyBytes,
			expectedCode: http.StatusOK,
		},
		{
			name:         "성공: 헤더 우선 (잘못된 쿼리 파라미터 무시)",
			appKeyHeader: "test-key-123",
			appKeyQuery:  "wrong-key",
			body:         validBodyBytes,
			expectedCode: http.StatusOK,
		},
		{
			name:         "성공: X-Application-Id 헤더 인증 (Hybrid - Body 파싱 생략)",
			appKeyHeader: "test-key-123",
			appIDHeader:  "test-app",             // Body 파싱 없이 헤더로 인증
			body:         []byte("invalid-json"), // Body가 잘못되어도 인증은 성공해야 함
			expectedCode: http.StatusOK,
		},
		{
			name:         "성공: 헤더/Body ID 불일치 시 헤더 우선 (우선순위 검증)",
			appKeyHeader: "test-key-123",
			appIDHeader:  "test-app",                               // Header: test-app (Target)
			body:         createRequestBody("another-app").Bytes(), // Body: another-app
			expectedCode: http.StatusOK,
			// 검증 로직에서 test-app으로 인증되었는지 확인 필요 (createAuthSuccessHandler가 수행)
		},
		{
			name:         "성공: X-Application-Id 헤더가 비어있는 경우 Body 폴백",
			appKeyHeader: "test-key-123",
			appIDHeader:  "", // 헤더 있음(빈 값) -> Body 파싱 수행
			body:         validBodyBytes,
			expectedCode: http.StatusOK,
		},

		{
			name:           "실패: App Key 누락",
			body:           validBodyBytes,
			expectedCode:   http.StatusBadRequest,
			expectErrorStr: "app_key는 필수입니다",
		},
		{
			name:           "실패: 잘못된 App Key",
			appKeyHeader:   "wrong-key",
			body:           validBodyBytes,
			expectedCode:   http.StatusUnauthorized,
			expectErrorStr: "app_key가 유효하지 않습니다",
		},
		{
			name:           "실패: Request Body 누락",
			appKeyHeader:   "test-key-123",
			body:           nil,
			expectedCode:   http.StatusBadRequest,
			expectErrorStr: "요청 본문이 비어있습니다",
		},
		{
			name:           "실패: 잘못된 JSON 형식",
			appKeyHeader:   "test-key-123",
			body:           []byte("invalid-json"),
			expectedCode:   http.StatusBadRequest,
			expectErrorStr: "잘못된 JSON 형식입니다",
		},
		{
			name:           "실패: Application ID 누락",
			appKeyHeader:   "test-key-123",
			body:           []byte(`{"message": "no app id"}`),
			expectedCode:   http.StatusBadRequest,
			expectErrorStr: "application_id는 필수입니다",
		},
		{
			name:           "실패: 미등록 Application ID",
			appKeyHeader:   "test-key-123", // 키는 유효하지만 앱 ID가 다름
			body:           createRequestBody("unknown-app").Bytes(),
			expectedCode:   http.StatusUnauthorized,
			expectErrorStr: "등록되지 않은 application_id입니다",
		},
		{
			name:           "실패: 다른 앱의 App Key 사용",
			appKeyHeader:   "another-key-456", // another-app의 키
			body:           validBodyBytes,    // test-app 요청
			expectedCode:   http.StatusUnauthorized,
			expectErrorStr: "app_key가 유효하지 않습니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup
			e := echo.New()
			endpoint := "/api/v1/notifications"
			if tt.appKeyQuery != "" {
				endpoint += fmt.Sprintf("?app_key=%s", tt.appKeyQuery)
			}
			req := httptest.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(tt.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			if tt.appKeyHeader != "" {
				req.Header.Set(constants.HeaderXAppKey, tt.appKeyHeader)
			}
			if tt.appIDHeader != "" {
				req.Header.Set(constants.HeaderXApplicationID, tt.appIDHeader)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Handler
			expectedApp := &domain.Application{ID: "test-app", Title: "Test Application"}
			handler := middleware(createAuthSuccessHandler(t, expectedApp))

			// Execute
			err := handler(c)

			// Verify
			if tt.expectedCode == http.StatusOK {
				assert.NoError(t, err)
				assert.Equal(t, http.StatusOK, rec.Code)
			} else {
				require.Error(t, err)
				httpErr, ok := err.(*echo.HTTPError)
				require.True(t, ok, "HTTPError 타입이어야 합니다")
				assert.Equal(t, tt.expectedCode, httpErr.Code)
				assert.Contains(t, fmt.Sprintf("%v", httpErr.Message), tt.expectErrorStr)
			}
		})
	}
}

// TestRequireAuthentication_BodyRestoration은 미들웨어 처리 후 Request Body가 복원되는지 검증합니다.
func TestRequireAuthentication_BodyRestoration(t *testing.T) {
	t.Parallel()

	e := echo.New()
	authenticator := createTestAuthenticator()
	middleware := RequireAuthentication(authenticator)

	req := httptest.NewRequest(http.MethodPost, "/", createRequestBody("test-app"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(constants.HeaderXAppKey, "test-key-123")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	nextHandler := func(c echo.Context) error {
		// 바디 다시 읽기 시도
		var body struct {
			AppID string `json:"application_id"`
		}
		if err := c.Bind(&body); err != nil {
			return err
		}
		assert.Equal(t, "test-app", body.AppID)
		return c.String(http.StatusOK, "ok")
	}

	h := middleware(nextHandler)
	assert.NoError(t, h(c))
}

// TestRequireAuthentication_NilAuthenticator는 authenticator가 nil일 때 패닉을 검증합니다.
func TestRequireAuthentication_NilAuthenticator(t *testing.T) {
	assert.PanicsWithValue(t, "RequireAuthentication: Authenticator가 nil입니다", func() {
		RequireAuthentication(nil)
	}, "Nil authenticator는 특정 메시지와 함께 패닉을 발생시켜야 합니다")
}

// TestRequireAuthentication_LegacyLog는 쿼리 파라미터 사용 시 경고 로그가 출력되는지 검증합니다.
func TestRequireAuthentication_LegacyLog(t *testing.T) {
	// 로그 캡처를 위해 직렬 실행
	var buf bytes.Buffer
	applog.SetOutput(&buf)
	applog.SetFormatter(&applog.JSONFormatter{})
	defer applog.SetOutput(applog.StandardLogger().Out)

	authenticator := createTestAuthenticator()
	middleware := RequireAuthentication(authenticator)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/?app_key=test-key-123", createRequestBody("test-app"))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := middleware(createAuthSuccessHandler(t, nil))
	_ = h(c)

	// 로그 검증
	require.Greater(t, buf.Len(), 0, "로그가 기록되어야 합니다")

	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	assert.NoError(t, err)

	assert.Equal(t, "warning", logEntry["level"])
	assert.Contains(t, logEntry["msg"], "레거시 방식으로 App Key 전달됨")
}
