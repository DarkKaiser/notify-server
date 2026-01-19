package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/api/auth"
	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Setup Helpers
// =============================================================================

func setupTestContext(method, target string, body io.Reader) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, target, body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c, rec
}

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
	return auth.NewAuthenticator(cfg)
}

func init() {
	// 테스트 중 로그 노이즈 억제 (필요시 레벨 조정)
	applog.SetLevel(applog.FatalLevel)
}

// =============================================================================
// Unit Tests: Helper Functions
// =============================================================================

func Test_extractAppKey(t *testing.T) {
	tests := []struct {
		name     string
		setupReq func(req *http.Request)
		want     string
	}{
		{
			name: "Header 우선 (권장)",
			setupReq: func(req *http.Request) {
				req.Header.Set(constants.XAppKey, "header-key")
				req.URL.RawQuery = "app_key=query-key"
			},
			want: "header-key",
		},
		{
			name: "Query Parameter 폴백 (레거시)",
			setupReq: func(req *http.Request) {
				req.URL.RawQuery = "app_key=query-key"
			},
			want: "query-key",
		},
		{
			name: "Key 없음",
			setupReq: func(req *http.Request) {
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := setupTestContext(http.MethodGet, "/", nil)
			tt.setupReq(c.Request())

			got := extractAppKey(c)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_extractApplicationID(t *testing.T) {
	tests := []struct {
		name          string
		setupReq      func(c echo.Context)
		wantID        string
		wantErr       bool
		errCheck      func(error) bool
		bodyLimitSize string // 옵션: BodyLimit 미들웨어 테스트용
	}{
		{
			name: "Header 우선 (권장)",
			setupReq: func(c echo.Context) {
				c.Request().Header.Set(constants.XApplicationID, "header-id")
				c.Request().Body = io.NopCloser(strings.NewReader(`{"application_id":"body-id"}`))
			},
			wantID:  "header-id",
			wantErr: false,
		},
		{
			name: "Body 폴백 (레거시)",
			setupReq: func(c echo.Context) {
				c.Request().Body = io.NopCloser(strings.NewReader(`{"application_id":"body-id"}`))
			},
			wantID:  "body-id",
			wantErr: false,
		},
		{
			name: "Body 비어있음",
			setupReq: func(c echo.Context) {
				c.Request().Body = io.NopCloser(strings.NewReader(``))
			},
			wantErr: true,
			errCheck: func(err error) bool {
				he, ok := err.(*echo.HTTPError)
				if !ok || he.Code != http.StatusBadRequest {
					return false
				}
				msg := ""
				if s, ok := he.Message.(string); ok {
					msg = s
				} else if resp, ok := he.Message.(response.ErrorResponse); ok {
					msg = resp.Message
				}
				return assert.Contains(t, msg, constants.ErrMsgBadRequestEmptyBody)
			},
		},
		{
			name: "잘못된 JSON",
			setupReq: func(c echo.Context) {
				c.Request().Body = io.NopCloser(strings.NewReader(`{invalid-json`))
			},
			wantErr: true,
			errCheck: func(err error) bool {
				he, ok := err.(*echo.HTTPError)
				if !ok || he.Code != http.StatusBadRequest {
					return false
				}
				msg := ""
				if s, ok := he.Message.(string); ok {
					msg = s
				} else if resp, ok := he.Message.(response.ErrorResponse); ok {
					msg = resp.Message
				}
				return assert.Contains(t, msg, constants.ErrMsgBadRequestInvalidJSON)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := setupTestContext(http.MethodPost, "/", nil)
			tt.setupReq(c)

			id, err := extractApplicationID(c)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errCheck != nil {
					assert.True(t, tt.errCheck(err), "Error check failed: %v", err)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantID, id)
			}
		})
	}
}

// Test_extractApplicationID_BodyLimit 은 BodyLimit 초과 시 동작을 별도로 검증합니다.
// BodyLimit은 미들웨어 체인에서 동작하므로 MaxBytesReader 에러를 시뮬레이션해야 합니다.
func Test_extractApplicationID_BodyLimit(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("too-large-body"))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// 강제로 MaxBytesReader 적용 (1바이트 제한)
	c.Request().Body = http.MaxBytesReader(rec, c.Request().Body, 1)

	_, err := extractApplicationID(c)

	require.Error(t, err)
	he, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusRequestEntityTooLarge, he.Code)
}

// =============================================================================
// Integration Tests: RequireAuthentication Middleware
// =============================================================================

func TestRequireAuthentication(t *testing.T) {
	authenticator := setupAuthenticator(t)

	tests := []struct {
		name           string
		setupReq       func(req *http.Request)
		expectedStatus int
		expectedMsg    string
	}{
		// ---------------------------------------------------------------------
		// 성공 케이스
		// ---------------------------------------------------------------------
		{
			name: "성공: Header 인증 (권장)",
			setupReq: func(req *http.Request) {
				req.Header.Set(constants.XAppKey, "valid-app-key")
				req.Header.Set(constants.XApplicationID, "test-app")
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "성공: Body ID + Header Key (레거시 혼합)",
			setupReq: func(req *http.Request) {
				req.Header.Set(constants.XAppKey, "valid-app-key")
				req.Body = io.NopCloser(strings.NewReader(`{"application_id":"test-app"}`))
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "성공: Query Key + Header ID (레거시 혼합)",
			setupReq: func(req *http.Request) {
				req.URL.RawQuery = "app_key=valid-app-key"
				req.Header.Set(constants.XApplicationID, "test-app")
			},
			expectedStatus: http.StatusOK,
		},

		// ---------------------------------------------------------------------
		// 실패 케이스 (입력 누락)
		// ---------------------------------------------------------------------
		{
			name: "실패: App Key 누락",
			setupReq: func(req *http.Request) {
				req.Header.Set(constants.XApplicationID, "test-app")
			},
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    constants.ErrMsgAuthAppKeyRequired,
		},
		{
			name: "실패: Application ID 누락 (Header & Body)",
			setupReq: func(req *http.Request) {
				req.Header.Set(constants.XAppKey, "valid-app-key")
				req.Body = io.NopCloser(strings.NewReader(`{}`)) // ID 필드 없음
			},
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    constants.ErrMsgAuthApplicationIDRequired,
		},

		// ---------------------------------------------------------------------
		// 실패 케이스 (인증 실패)
		// ---------------------------------------------------------------------
		{
			name: "실패: 잘못된 App Key",
			setupReq: func(req *http.Request) {
				req.Header.Set(constants.XAppKey, "invalid-key")
				req.Header.Set(constants.XApplicationID, "test-app")
			},
			expectedStatus: http.StatusUnauthorized,
			expectedMsg:    "유효하지 않습니다", // 메시지 일부 검증
		},
		{
			name: "실패: 등록되지 않은 App ID",
			setupReq: func(req *http.Request) {
				req.Header.Set(constants.XAppKey, "valid-app-key")
				req.Header.Set(constants.XApplicationID, "unknown-app")
			},
			expectedStatus: http.StatusUnauthorized,
			expectedMsg:    "등록되지 않은 application_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			tt.setupReq(req)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// 미들웨어 실행
			mw := RequireAuthentication(authenticator)
			handler := mw(func(c echo.Context) error {
				return c.String(http.StatusOK, "success")
			})

			err := handler(c)

			if tt.expectedStatus == http.StatusOK {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, rec.Code)
			} else {
				// Echo HTTPError 검증
				var he *echo.HTTPError
				if assert.ErrorAs(t, err, &he) {
					assert.Equal(t, tt.expectedStatus, he.Code)
					if tt.expectedMsg != "" {
						// ErrorResponse 또는 string 메시지 처리
						msg := ""
						if s, ok := he.Message.(string); ok {
							msg = s
						} else if resp, ok := he.Message.(error); ok { // response.ErrorResponse가 될 수도 있음 (구조체 정의 확인 필요)
							msg = resp.Error()
						} else {
							msg = fmt.Sprintf("%v", he.Message)
						}
						assert.Contains(t, msg, tt.expectedMsg)
					}
				}
			}
		})
	}
}

// TestRequireAuthentication_BodyRestoration 은 Body 파싱 후 Body가 올바르게 복원되어
// 후속 핸들러에서 다시 읽을 수 있는지 검증합니다.
func TestRequireAuthentication_BodyRestoration(t *testing.T) {
	authenticator := setupAuthenticator(t)
	expectedBody := `{"application_id":"test-app", "payload":"data"}`

	c, _ := setupTestContext(http.MethodPost, "/", strings.NewReader(expectedBody))
	c.Request().Header.Set(constants.XAppKey, "valid-app-key")
	// Header ID 누락 -> Body 파싱 유도

	mw := RequireAuthentication(authenticator)
	handler := mw(func(c echo.Context) error {
		// 핸들러에서 Body 다시 읽기
		bodyBytes, err := io.ReadAll(c.Request().Body)
		require.NoError(t, err)
		assert.Equal(t, expectedBody, string(bodyBytes), "Body가 복원되어야 합니다")
		return c.String(http.StatusOK, "success")
	})

	err := handler(c)
	assert.NoError(t, err)
}

func TestRequireAuthentication_Panic(t *testing.T) {
	assert.PanicsWithValue(t, constants.PanicMsgAuthenticatorRequired, func() {
		RequireAuthentication(nil)
	})
}
