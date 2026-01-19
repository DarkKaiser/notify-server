package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/api/auth"
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

// faultyReader 읽기 작업 시 항상 에러를 반환하는 Reader입니다 for Testing.
type faultyReader struct{}

func (f *faultyReader) Read(_ []byte) (n int, err error) {
	return 0, errors.New("simulated read error")
}

// =============================================================================
// Unit Tests: Helper Functions
// =============================================================================

func Test_extractAppKey(t *testing.T) {
	tests := []struct {
		name     string
		setupReq func(req *http.Request)
		want     string
		err      error // 예상되는 에러 (없으면 nil)
	}{
		{
			name: "성공: Header 우선 (권장)",
			setupReq: func(req *http.Request) {
				req.Header.Set(headerXAppKey, "header-key")
				req.URL.RawQuery = "app_key=query-key"
			},
			want: "header-key",
		},
		{
			name: "성공: Query Parameter 폴백 (레거시)",
			setupReq: func(req *http.Request) {
				req.URL.RawQuery = "app_key=query-key"
			},
			want: "query-key",
		},
		{
			name: "실패: Key 없음",
			setupReq: func(req *http.Request) {
			},
			want: "",
			err:  ErrAppKeyRequired, // extractAppKey는 현재 string만 반환하므로 테스트 방식 유지 또는 함수 변경 필요. 현재는 빈 문자열 반환.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := setupTestContext(http.MethodGet, "/", nil)
			tt.setupReq(c.Request())

			got := extractAppKey(c)

			// extractAppKey는 에러를 직접 반환하지 않고 미들웨어에서 체크함.
			// 단위 테스트에서는 값 추출 여부만 확인.
			if tt.err != nil {
				// 미들웨어 로직 시뮬레이션: 빈 값이면 에러로 간주
				if got == "" {
					return // Pass
				}
				// t.Errorf("want error but got string: %s", got)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_extractApplicationID(t *testing.T) {
	tests := []struct {
		name     string
		setupReq func(c echo.Context)
		wantID   string
		wantErr  error // 예상되는 에러 (assert.ErrorIs 사용)
	}{
		{
			name: "성공: Header 우선 (권장)",
			setupReq: func(c echo.Context) {
				c.Request().Header.Set(headerXApplicationID, "header-id")
				c.Request().Body = io.NopCloser(strings.NewReader(`{"application_id":"body-id"}`))
			},
			wantID: "header-id",
		},
		{
			name: "성공: Body 폴백 (레거시)",
			setupReq: func(c echo.Context) {
				c.Request().Body = io.NopCloser(strings.NewReader(`{"application_id":"body-id"}`))
			},
			wantID: "body-id",
		},
		{
			name: "실패: Body 비어있음",
			setupReq: func(c echo.Context) {
				c.Request().Body = io.NopCloser(strings.NewReader(``))
			},
			wantErr: ErrEmptyBody,
		},
		{
			name: "실패: 잘못된 JSON",
			setupReq: func(c echo.Context) {
				c.Request().Body = io.NopCloser(strings.NewReader(`{invalid-json`))
			},
			wantErr: ErrInvalidJSON,
		},
		{
			name: "실패: Body 읽기 실패 (Network Error)",
			setupReq: func(c echo.Context) {
				c.Request().Body = io.NopCloser(&faultyReader{})
			},
			wantErr: ErrBodyReadFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := setupTestContext(http.MethodPost, "/", nil)
			tt.setupReq(c)

			id, err := extractApplicationID(c)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr, "예상된 에러와 일치하지 않습니다")
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
	// ErrBodyTooLarge 에러가 반환되는지 확인 (Go 1.13+ 에러 체인)
	assert.ErrorIs(t, err, ErrBodyTooLarge)
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
		expectedErr    error // 예상되는 에러 (assert.ErrorIs 사용)
	}{
		// ---------------------------------------------------------------------
		// 성공 케이스
		// ---------------------------------------------------------------------
		{
			name: "성공: Header 인증 (권장)",
			setupReq: func(req *http.Request) {
				req.Header.Set(headerXAppKey, "valid-app-key")
				req.Header.Set(headerXApplicationID, "test-app")
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "성공: Body ID + Header Key (레거시 혼합)",
			setupReq: func(req *http.Request) {
				req.Header.Set(headerXAppKey, "valid-app-key")
				req.Body = io.NopCloser(strings.NewReader(`{"application_id":"test-app"}`))
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "성공: Query Key + Header ID (레거시 혼합)",
			setupReq: func(req *http.Request) {
				req.URL.RawQuery = "app_key=valid-app-key"
				req.Header.Set(headerXApplicationID, "test-app")
			},
			expectedStatus: http.StatusOK,
		},

		// ---------------------------------------------------------------------
		// 실패 케이스 (입력 누락)
		// ---------------------------------------------------------------------
		{
			name: "실패: App Key 누락",
			setupReq: func(req *http.Request) {
				req.Header.Set(headerXApplicationID, "test-app")
			},
			expectedStatus: http.StatusBadRequest,
			expectedErr:    ErrAppKeyRequired,
		},
		{
			name: "실패: Application ID 누락 (Header & Body)",
			setupReq: func(req *http.Request) {
				req.Header.Set(headerXAppKey, "valid-app-key")
				req.Body = io.NopCloser(strings.NewReader(`{}`)) // ID 필드 없음
			},
			expectedStatus: http.StatusBadRequest,
			expectedErr:    ErrApplicationIDRequired,
		},

		// ---------------------------------------------------------------------
		// 실패 케이스 (인증 실패)
		// ---------------------------------------------------------------------
		{
			name: "실패: 잘못된 App Key",
			setupReq: func(req *http.Request) {
				req.Header.Set(headerXAppKey, "invalid-key")
				req.Header.Set(headerXApplicationID, "test-app")
			},
			expectedStatus: http.StatusUnauthorized,
			// Authenticator에서 반환하는 에러는 동적이므로 여기서는 Status만 체크하거나,
			// auth 패키지의 에러 변수를 공개하면 ErrorIs로 체크 가능.
			// 현재는 Status와 메시지 일부 체크로 만족.
			// expectedErr: auth.NewErrInvalidAppKey(...) // 이는 동적 에러라 단순 비교 불가
		},
		{
			name: "실패: 등록되지 않은 App ID",
			setupReq: func(req *http.Request) {
				req.Header.Set(headerXAppKey, "valid-app-key")
				req.Header.Set(headerXApplicationID, "unknown-app")
			},
			expectedStatus: http.StatusUnauthorized,
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

					// 예상 에러 변수가 있다면 (정적 에러인 경우) ErrorIs로 정확한 비교 수행
					if tt.expectedErr != nil {
						assert.ErrorIs(t, err, tt.expectedErr)
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
	c.Request().Header.Set(headerXAppKey, "valid-app-key")
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
	assert.PanicsWithValue(t, "Authenticator는 필수입니다", func() {
		RequireAuthentication(nil)
	})
}
