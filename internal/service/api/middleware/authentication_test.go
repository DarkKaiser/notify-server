package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/api/auth"
	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/httputil"
	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Helper Functions
// =============================================================================

// setupAuthenticator 테스트를 위한 가짜 Authenticator를 생성합니다.
func setupAuthenticator(t *testing.T) *auth.Authenticator {
	t.Helper()
	cfg := &config.AppConfig{
		NotifyAPI: config.NotifyAPIConfig{
			Applications: []config.ApplicationConfig{
				{
					ID:                "test-app",
					AppKey:            "valid-app-key",
					Title:             "Test App",
					DefaultNotifierID: "telegram-bot_1",
				},
			},
		},
	}
	authenticator := auth.NewAuthenticator(cfg)
	require.NotNil(t, authenticator)
	return authenticator
}

func setupEcho(t *testing.T) *echo.Echo {
	t.Helper()
	e := echo.New()
	// 로그 레벨 조정 (테스트 중 노이즈 제거)
	applog.SetLevel(applog.FatalLevel)
	t.Cleanup(func() {
		applog.SetLevel(applog.InfoLevel)
	})
	return e
}

// =============================================================================
// Test Cases
// =============================================================================

// TestRequireAuthentication_Priority_AppKey 는 App Key 추출 우선순위를 검증합니다.
// Header(권장)가 Query Param(레거시)보다 우선해야 합니다.
func TestRequireAuthentication_Priority_AppKey(t *testing.T) {
	authenticator := setupAuthenticator(t)
	e := setupEcho(t)

	// 시나리오: Header에는 유효한 키, Query에는 잘못된 키 제공
	// Header가 우선하므로 인증 성공해야 함
	req := httptest.NewRequest(http.MethodPost, "/?app_key=invalid-key", nil)
	req.Header.Set(constants.HeaderXAppKey, "valid-app-key")
	req.Header.Set(constants.HeaderXApplicationID, "test-app")

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := RequireAuthentication(authenticator)
	h := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})

	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestRequireAuthentication_Priority_AppID 는 Application ID 추출 우선순위를 검증합니다.
// Header(권장)가 Body(레거시)보다 우선해야 합니다.
func TestRequireAuthentication_Priority_AppID(t *testing.T) {
	authenticator := setupAuthenticator(t)
	e := setupEcho(t)

	// 시나리오: Header에는 유효한 ID, Body에는 잘못된 ID 제공
	// Header가 우선하므로 인증 성공해야 함
	body := `{"application_id":"invalid-app"}` // Body ID는 무시되어야 함
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(constants.HeaderXAppKey, "valid-app-key")
	req.Header.Set(constants.HeaderXApplicationID, "test-app")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := RequireAuthentication(authenticator)
	h := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})

	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestRequireAuthentication_BodyRestoration 은 Body 파싱 후 Body가 복원되어
// 다음 핸들러에서 읽을 수 있는지 검증합니다. (Double Parsing 문제 해결 확인)
func TestRequireAuthentication_BodyRestoration(t *testing.T) {
	authenticator := setupAuthenticator(t)
	e := setupEcho(t)

	// Body를 통해 Application ID 전달 (레거시 방식)
	requestBodyContent := `{"application_id":"test-app", "data":"payload"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(requestBodyContent))
	req.Header.Set(constants.HeaderXAppKey, "valid-app-key")
	// 헤더 ID 누락 -> Body 파싱 유도

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := RequireAuthentication(authenticator)
	h := mw(func(c echo.Context) error {
		// 미들웨어 통과 후 핸들러에서 Body 다시 읽기 시도
		bodyBytes, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return err
		}
		// 원본 Body와 일치하는지 확인
		if string(bodyBytes) != requestBodyContent {
			return echo.NewHTTPError(http.StatusInternalServerError, "Body not restored")
		}
		return c.String(http.StatusOK, "success")
	})

	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestRequireAuthentication_Scenarios 는 다양한 성공/실패 시나리오를 검증합니다.
func TestRequireAuthentication_Scenarios(t *testing.T) {
	authenticator := setupAuthenticator(t)
	e := setupEcho(t)

	tests := []struct {
		name           string
		setupRequest   func(req *http.Request)
		expectedStatus int
		expectedMsg    string // 에러 메시지 일부 검증 (선택적)
	}{
		// 성공 케이스
		{
			name: "Success_HeaderAuth",
			setupRequest: func(req *http.Request) {
				req.Header.Set(constants.HeaderXAppKey, "valid-app-key")
				req.Header.Set(constants.HeaderXApplicationID, "test-app")
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Success_LegacyBodyAuth",
			setupRequest: func(req *http.Request) {
				req.Header.Set(constants.HeaderXAppKey, "valid-app-key")
				// No Header ID
				req.Body = io.NopCloser(strings.NewReader(`{"application_id":"test-app"}`))
			},
			expectedStatus: http.StatusOK,
		},

		// 실패 케이스 - 400 Bad Request
		{
			name: "Fail_MissingAppKey",
			setupRequest: func(req *http.Request) {
				req.Header.Set(constants.HeaderXApplicationID, "test-app")
			},
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    constants.ErrMsgAuthAppKeyRequired,
		},
		{
			name: "Fail_MissingAppID_HeaderAndBody",
			setupRequest: func(req *http.Request) {
				req.Header.Set(constants.HeaderXAppKey, "valid-app-key")
				req.Body = io.NopCloser(strings.NewReader(`{}`)) // Empty JSON, no ID
			},
			expectedStatus: http.StatusBadRequest, // ID 누락 (JSON 파싱 성공했으나 ID 없음)
			// 현재 구현상 JSON 언마샬 후 ID가 빈 문자열이면 ErrMsgApplicationIDRequired 반환
			expectedMsg: constants.ErrMsgAuthApplicationIDRequired,
		},
		{
			name: "Fail_EmptyBody_WhenHeaderMissing",
			setupRequest: func(req *http.Request) {
				req.Header.Set(constants.HeaderXAppKey, "valid-app-key")
				req.Body = io.NopCloser(strings.NewReader("")) // Empty Body
			},
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    constants.ErrMsgBadRequestEmptyBody,
		},
		{
			name: "Fail_InvalidJSON",
			setupRequest: func(req *http.Request) {
				req.Header.Set(constants.HeaderXAppKey, "valid-app-key")
				req.Body = io.NopCloser(strings.NewReader(`{invalid-json`))
			},
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    constants.ErrMsgBadRequestInvalidJSON,
		},

		// 실패 케이스 - 401 Unauthorized
		{
			name: "Fail_InvalidAppKey",
			setupRequest: func(req *http.Request) {
				req.Header.Set(constants.HeaderXAppKey, "invalid-key")
				req.Header.Set(constants.HeaderXApplicationID, "test-app")
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Fail_UnknownAppID",
			setupRequest: func(req *http.Request) {
				req.Header.Set(constants.HeaderXAppKey, "valid-app-key")
				req.Header.Set(constants.HeaderXApplicationID, "unknown-app")
			},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			tt.setupRequest(req)

			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			mw := RequireAuthentication(authenticator)
			h := mw(func(c echo.Context) error {
				return c.String(http.StatusOK, "success")
			})

			err := h(c)

			if tt.expectedStatus == http.StatusOK {
				assert.NoError(t, err)
				assert.Equal(t, http.StatusOK, rec.Code)
			} else {
				// 에러 핸들링
				var he *echo.HTTPError
				if assert.ErrorAs(t, err, &he) {
					assert.Equal(t, tt.expectedStatus, he.Code)
					if tt.expectedMsg != "" {
						// he.Message는 string일 수도 있고 response.ErrorResponse일 수도 있음
						if msg, ok := he.Message.(string); ok {
							assert.Contains(t, msg, tt.expectedMsg)
						} else if resp, ok := he.Message.(response.ErrorResponse); ok {
							assert.Contains(t, resp.Message, tt.expectedMsg)
						} else {
							// Fallback
							assert.Contains(t, he.Message, tt.expectedMsg)
						}
					}
				}
			}
		})
	}
}

// TestRequireAuthentication_BodyTooLarge 는 BodyLimit 초과 시 413 에러 처리를 검증합니다.
func TestRequireAuthentication_BodyTooLarge(t *testing.T) {
	authenticator := setupAuthenticator(t)
	e := setupEcho(t)
	e.HTTPErrorHandler = httputil.ErrorHandler // 커스텀 에러 핸들러 연결 (선택사항, 테스트 직접 검증 시 불필요할 수 있음)

	// BodyLimit 미들웨어 설정 (10 바이트 제한)
	e.Use(middleware.BodyLimit("10B"))

	// 10바이트 초과 데이터, Content-Length 조작으로 초기 검사 우회 -> io.ReadAll에서 에러 유발
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"id":"test-app", "data":"too-large"}`))
	req.Header.Set(constants.HeaderXAppKey, "valid-app-key")
	req.Header.Del("Content-Length") // Content-Length 기반 1차 차단 우회
	req.ContentLength = -1

	// Application ID 헤더 누락 -> Body 파싱 시도

	rec := httptest.NewRecorder()

	// 미들웨어 체인 구성
	// BodyLimit -> RequireAuthentication -> Handler
	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	}
	// 주의: BodyLimit은 전역 또는 Route 레벨로 적용됨. 여기서는 수동 체이닝
	// e.ServeHTTP를 사용하면 등록된 미들웨어가 적용됨

	// 413 에러가 RequireAuthentication 내부의 io.ReadAll에서 발생하는지 확인하기 위해
	// e.ServeHTTP 대신 직접 핸들러 체인을 구성하거나 e.POST에 등록하여 호출
	e.POST("/", handler, RequireAuthentication(authenticator))

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	// 메시지 내용 검증은 구현에 따라 다를 수 있으나, 상수값 포함 확인
	assert.Contains(t, rec.Body.String(), constants.ErrMsgRequestEntityTooLarge)
}

func TestRequireAuthentication_Panic_Nil(t *testing.T) {
	assert.PanicsWithValue(t, constants.PanicMsgAuthenticatorRequired, func() {
		RequireAuthentication(nil)
	})
}
