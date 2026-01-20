package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/api/model/domain"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupContextHelper 테스트용 Echo Context와 Recorder를 생성하는 헬퍼 함수
func setupContextHelper() (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

// TestSetApplication_GetApplication_RoundTrip은 SetApplication과 GetApplication의 상호 작용을 검증합니다.
func TestSetApplication_GetApplication_RoundTrip(t *testing.T) {
	c, _ := setupContextHelper()
	expectedApp := &domain.Application{
		ID:    "test-app",
		Title: "Test Application",
	}

	SetApplication(c, expectedApp)

	actualApp, err := GetApplication(c)
	require.NoError(t, err)
	assert.Equal(t, expectedApp, actualApp)
}

func TestGetApplication(t *testing.T) {
	tests := []struct {
		name        string
		setupCtx    func(echo.Context)
		expectedApp *domain.Application
		expectedErr error
	}{
		{
			name: "성공: 올바른 애플리케이션 정보가 있는 경우",
			setupCtx: func(c echo.Context) {
				SetApplication(c, &domain.Application{ID: "valid-app"})
			},
			expectedApp: &domain.Application{ID: "valid-app"},
			expectedErr: nil,
		},
		{
			name:        "실패: Context에 키가 없는 경우",
			setupCtx:    func(c echo.Context) {}, // 아무것도 설정하지 않음
			expectedApp: nil,
			expectedErr: ErrApplicationMissingInContext,
		},
		{
			name: "실패: 잘못된 타입이 저장된 경우 (White-box Testing)",
			setupCtx: func(c echo.Context) {
				// 내부 상수 contextKeyApplication에 직접 접근하여 잘못된 타입 주입
				c.Set(contextKeyApplication, "invalid-string-type")
			},
			expectedApp: nil,
			expectedErr: ErrApplicationTypeMismatch,
		},
		{
			name: "Edge Case: nil 포인터가 저장된 경우",
			setupCtx: func(c echo.Context) {
				var nilApp *domain.Application = nil
				SetApplication(c, nilApp)
			},
			expectedApp: nil,
			expectedErr: nil, // GetApplication은 타입이 맞으면 에러를 리턴하지 않음 (nil 리턴)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := setupContextHelper()
			tt.setupCtx(c)

			app, err := GetApplication(c)

			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
				assert.Nil(t, app)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedApp, app)
			}
		})
	}
}

func TestMustGetApplication(t *testing.T) {
	// panic 메시지 포맷 템플릿 (internal의 구현과 일치해야 함)
	// 주의: 만약 internal 구현의 메시지가 바뀌면 이 테스트도 수정해야 함
	panicMsgTemplate := "Auth: Context에서 애플리케이션 정보를 가져올 수 없습니다. 인증 미들웨어가 적용되었는지 확인해주세요. (원인: %v)"

	tests := []struct {
		name        string
		setupCtx    func(echo.Context)
		expectedApp *domain.Application
		shouldPanic bool
		panicValue  string // 예상되는 패닉 메시지
	}{
		{
			name: "성공: 올바른 애플리케이션 정보가 있는 경우",
			setupCtx: func(c echo.Context) {
				SetApplication(c, &domain.Application{ID: "must-app"})
			},
			expectedApp: &domain.Application{ID: "must-app"},
			shouldPanic: false,
		},
		{
			name:        "패닉: Context에 정보가 없는 경우",
			setupCtx:    func(c echo.Context) {},
			shouldPanic: true,
			panicValue:  fmt.Sprintf(panicMsgTemplate, ErrApplicationMissingInContext),
		},
		{
			name: "패닉: 잘못된 타입인 경우 (White-box Testing)",
			setupCtx: func(c echo.Context) {
				// 내부 상수 contextKeyApplication에 직접 접근하여 잘못된 타입 주입
				c.Set(contextKeyApplication, 12345) // int type
			},
			shouldPanic: true,
			panicValue:  fmt.Sprintf(panicMsgTemplate, ErrApplicationTypeMismatch),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := setupContextHelper()
			tt.setupCtx(c)

			if tt.shouldPanic {
				assert.PanicsWithValue(t, tt.panicValue, func() {
					MustGetApplication(c)
				})
			} else {
				assert.NotPanics(t, func() {
					app := MustGetApplication(c)
					assert.Equal(t, tt.expectedApp, app)
				})
			}
		})
	}
}
