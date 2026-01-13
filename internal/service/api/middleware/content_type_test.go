package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// setupEchoForContentType 테스트용 Echo 인스턴스를 설정합니다.
func setupEchoForContentType(t *testing.T) *echo.Echo {
	t.Helper()
	e := echo.New()
	// 테스트 중 불필요한 로그 출력 방지
	applog.SetLevel(applog.FatalLevel)
	t.Cleanup(func() {
		applog.SetLevel(applog.InfoLevel)
	})
	return e
}

func TestValidateContentType(t *testing.T) {
	e := setupEchoForContentType(t)
	reqBody := strings.NewReader(`{"foo":"bar"}`)

	tests := []struct {
		name                string
		expectedContentType string
		requestContentType  string
		hasBody             bool
		expectedStatus      int
		expectedMsg         string
	}{
		// 성공 케이스
		{
			name:                "성공_정상_ContentType",
			expectedContentType: echo.MIMEApplicationJSON,
			requestContentType:  echo.MIMEApplicationJSON,
			hasBody:             true,
			expectedStatus:      http.StatusOK,
		},
		{
			name:                "성공_Charset포함_ContentType",
			expectedContentType: echo.MIMEApplicationJSON,
			requestContentType:  "application/json; charset=utf-8",
			hasBody:             true,
			expectedStatus:      http.StatusOK,
		},
		{
			name:                "성공_대소문자_혼용", // MIME 타입은 대소문자 무관하게 처리됨
			expectedContentType: echo.MIMEApplicationJSON,
			requestContentType:  "Application/JSON",
			hasBody:             true,
			expectedStatus:      http.StatusOK,
		},
		{
			name:                "성공_Body없음_검증건너뜀",
			expectedContentType: echo.MIMEApplicationJSON,
			requestContentType:  "",
			hasBody:             false,
			expectedStatus:      http.StatusOK,
		},

		// 실패 케이스
		{
			name:                "실패_ContentType_누락",
			expectedContentType: echo.MIMEApplicationJSON,
			requestContentType:  "",
			hasBody:             true,
			expectedStatus:      http.StatusUnsupportedMediaType,
			expectedMsg:         constants.ErrMsgUnsupportedMediaType,
		},
		{
			name:                "실패_잘못된_ContentType",
			expectedContentType: echo.MIMEApplicationJSON,
			requestContentType:  echo.MIMETextPlain,
			hasBody:             true,
			expectedStatus:      http.StatusUnsupportedMediaType,
			expectedMsg:         constants.ErrMsgUnsupportedMediaType,
		},
		{
			name: "실패_부분일치_주의", // "application/json"이 "application/javascript"에 포함되지 않도록 주의해야 함 (단, 현재 구현은 단순 Contains)
			// 현재 strings.Contains 로직상 "application/json"은 "application/json-patch+json" 등에 포함될 수 있음.
			// 하지만 "application/javascript"는 "application/json"을 포함하지 않으므로 실패해야 함.
			expectedContentType: echo.MIMEApplicationJSON,
			requestContentType:  "application/javascript",
			hasBody:             true,
			expectedStatus:      http.StatusUnsupportedMediaType,
			expectedMsg:         constants.ErrMsgUnsupportedMediaType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Request 생성
			var req *http.Request
			if tt.hasBody {
				req = httptest.NewRequest(http.MethodPost, "/", reqBody)
			} else {
				req = httptest.NewRequest(http.MethodGet, "/", nil)
			}

			// 헤더 설정
			if tt.requestContentType != "" {
				req.Header.Set(echo.HeaderContentType, tt.requestContentType)
			}

			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// 미들웨어 실행
			mw := ValidateContentType(tt.expectedContentType)
			h := mw(func(c echo.Context) error {
				return c.String(http.StatusOK, "OK")
			})

			err := h(c)

			// 결과 검증
			if tt.expectedStatus == http.StatusOK {
				assert.NoError(t, err)
				assert.Equal(t, http.StatusOK, rec.Code)
			} else {
				if assert.Error(t, err) {
					he, ok := err.(*echo.HTTPError)
					assert.True(t, ok, "echo.HTTPError 타입이어야 합니다")
					assert.Equal(t, tt.expectedStatus, he.Code)
					assert.Equal(t, tt.expectedMsg, he.Message)
				}
			}
		})
	}
}
