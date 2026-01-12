package v1

import (
	"net/http"
	"testing"

	apiauth "github.com/darkkaiser/notify-server/internal/service/api/auth"
	"github.com/darkkaiser/notify-server/internal/service/api/v1/handler"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Unit Tests
// =============================================================================

// TestSetupRoutes_RouteRegistration은 각 라우트가 올바른 메서드와 경로로 등록되었는지 검증합니다.
func TestSetupRoutes_RouteRegistration(t *testing.T) {
	// Setup
	e, h, auth := setupTestDependencies()

	// Execute
	SetupRoutes(e, h, auth)

	// Verify
	routes := e.Routes()

	tests := []struct {
		name        string
		method      string
		path        string
		shouldExist bool
	}{
		// 정상 등록 라우트
		{"Notifications POST 등록 확인", http.MethodPost, "/api/v1/notifications", true},
		{"Legacy Message POST 등록 확인", http.MethodPost, "/api/v1/notice/message", true},

		// 미지원 메서드 확인
		{"Notifications GET 미지원", http.MethodGet, "/api/v1/notifications", false},
		{"Notifications PUT 미지원", http.MethodPut, "/api/v1/notifications", false},
		{"Notifications DELETE 미지원", http.MethodDelete, "/api/v1/notifications", false},
		{"Notifications PATCH 미지원", http.MethodPatch, "/api/v1/notifications", false},

		// 존재하지 않는 경로 확인
		{"루트 경로 미존재", http.MethodGet, "/api/v1", false},
		{"임의 경로 미존재", http.MethodGet, "/api/v1/random", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := false
			for _, route := range routes {
				if route.Method == tt.method && route.Path == tt.path {
					found = true
					break
				}
			}
			assert.Equal(t, tt.shouldExist, found, "라우트 존재 여부가 기대값과 다릅니다: %s %s", tt.method, tt.path)
		})
	}
}

// TestSetupRoutes_HandlerName은 각 라우트에 올바른 핸들러가 할당되었는지 검증합니다.
func TestSetupRoutes_HandlerName(t *testing.T) {
	// Setup
	e, h, auth := setupTestDependencies()

	// Execute
	SetupRoutes(e, h, auth)

	// Verify
	routes := e.Routes()
	targetRoutes := []string{"/api/v1/notifications", "/api/v1/notice/message"}

	for _, path := range targetRoutes {
		found := false
		for _, route := range routes {
			if route.Path == path && route.Method == http.MethodPost {
				found = true
				// 핸들러 Function Name 검증 (패키지명 포함)
				assert.Contains(t, route.Name, "v1/handler", "올바른 핸들러 패키지가 아닙니다: %s", path)
				assert.Contains(t, route.Name, "PublishNotificationHandler", "올바른 핸들러 함수가 아닙니다: %s", path)
			}
		}
		assert.True(t, found, "라우트를 찾을 수 없습니다: %s", path)
	}
}

// TestSetupRoutes_PanicOnNilDeps는 필수 의존성이 nil일 경우 패닉 발생을 검증합니다.
func TestSetupRoutes_PanicOnNilDeps(t *testing.T) {
	e := echo.New()

	assert.Panics(t, func() {
		SetupRoutes(e, nil, nil)
	}, "nil Authenticator 전달 시 패닉이 발생해야 합니다")
}

// =============================================================================
// Helper Functions
// =============================================================================

// setupTestDependencies는 테스트에 필요한 Ech, Handler, Authenticator 인스턴스를 생성합니다.
func setupTestDependencies() (*echo.Echo, *handler.Handler, *apiauth.Authenticator) {
	e := echo.New()
	appConfig := createTestAppConfig() // integration_test.go에 정의됨 (동일 패키지)
	auth := apiauth.NewAuthenticator(appConfig)
	mockService := &mocks.MockNotificationSender{}
	h := handler.NewHandler(mockService)
	return e, h, auth
}

// findRoute는 주어진 메서드와 경로에 해당하는 라우트를 찾습니다.
func findRoute(routes []*echo.Route, method, path string) *echo.Route {
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return route
		}
	}
	return nil
}
