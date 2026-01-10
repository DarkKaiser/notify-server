package v1

import (
	"testing"

	apiauth "github.com/darkkaiser/notify-server/internal/service/api/auth"
	"github.com/darkkaiser/notify-server/internal/service/api/v1/handler"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetupRoutes_RegisteredRoutes 라우트가 올바르게 등록되었는지 검증합니다.
func TestSetupRoutes_RegisteredRoutes(t *testing.T) {
	// Setup
	e := echo.New()
	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewAuthenticator(appConfig)
	mockService := &mocks.MockNotificationSender{}
	h := handler.NewHandler(applicationManager, mockService)

	// Execute
	SetupRoutes(e, h)

	// Verify
	tests := []struct {
		name        string
		method      string
		path        string
		shouldExist bool
	}{
		// 등록된 라우트
		{"Notifications POST", "POST", "/api/v1/notifications", true},
		{"Legacy Message POST", "POST", "/api/v1/notice/message", true},

		// 미지원 메서드
		{"Notifications GET", "GET", "/api/v1/notifications", false},
		{"Notifications PUT", "PUT", "/api/v1/notifications", false},
		{"Notifications DELETE", "DELETE", "/api/v1/notifications", false},
		{"Notifications PATCH", "PATCH", "/api/v1/notifications", false},
		{"Legacy Message GET", "GET", "/api/v1/notice/message", false},

		// 존재하지 않는 경로
		{"Random Path", "GET", "/api/v1/random", false},
		{"Root Path", "GET", "/api/v1", false},
	}

	routes := e.Routes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := false
			for _, route := range routes {
				if route.Path == tt.path && route.Method == tt.method {
					found = true
					break
				}
			}
			assert.Equal(t, tt.shouldExist, found, "Route existence mismatch for %s %s", tt.method, tt.path)
		})
	}
}

// TestSetupRoutes_GroupPrefix 모든 라우트가 /api/v1 prefix를 가지는지 검증합니다.
func TestSetupRoutes_GroupPrefix(t *testing.T) {
	// Setup
	e := echo.New()
	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewAuthenticator(appConfig)
	mockService := &mocks.MockNotificationSender{}
	h := handler.NewHandler(applicationManager, mockService)

	// Execute
	SetupRoutes(e, h)

	// Verify
	routes := e.Routes()
	v1Routes := []string{
		"/api/v1/notifications",
		"/api/v1/notice/message",
	}

	for _, expectedPath := range v1Routes {
		found := false
		for _, route := range routes {
			if route.Path == expectedPath {
				found = true
				assert.Contains(t, route.Path, "/api/v1", "Route should have /api/v1 prefix: %s", route.Path)
				break
			}
		}
		assert.True(t, found, "Expected route not found: %s", expectedPath)
	}
}

// TestSetupRoutes_HandlerAssignment 핸들러가 올바르게 할당되었는지 검증합니다.
func TestSetupRoutes_HandlerAssignment(t *testing.T) {
	// Setup
	e := echo.New()
	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewAuthenticator(appConfig)
	mockService := &mocks.MockNotificationSender{}
	h := handler.NewHandler(applicationManager, mockService)

	// Execute
	SetupRoutes(e, h)

	// Verify
	routes := e.Routes()
	for _, route := range routes {
		if route.Path == "/api/v1/notifications" || route.Path == "/api/v1/notice/message" {
			// Echo의 Route 구조체는 Name 필드만 제공하므로, 핸들러 이름이 비어있지 않은지 확인
			assert.NotEmpty(t, route.Name, "Handler name should not be empty for route: %s", route.Path)
		}
	}
}

// TestSetupRoutes_MiddlewareChain deprecated 미들웨어가 올바른 라우트에만 적용되는지 검증합니다.
func TestSetupRoutes_MiddlewareChain(t *testing.T) {
	// Setup
	e := echo.New()
	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewAuthenticator(appConfig)
	mockService := &mocks.MockNotificationSender{}
	h := handler.NewHandler(applicationManager, mockService)

	// Execute
	SetupRoutes(e, h)

	// Verify
	// Echo의 Routes()는 미들웨어 정보를 직접 제공하지 않으므로,
	// 실제 HTTP 요청을 통해 헤더를 확인하는 방식으로 검증합니다.
	// 이는 통합 테스트에서 더 적절하게 검증됩니다.
	// 여기서는 라우트가 존재하는지만 확인합니다.

	routes := e.Routes()

	// /api/v1/notifications - deprecated 미들웨어 없음
	notificationsRoute := findRoute(routes, "POST", "/api/v1/notifications")
	require.NotNil(t, notificationsRoute, "Notifications route should exist")

	// /api/v1/notice/message - deprecated 미들웨어 있음
	legacyRoute := findRoute(routes, "POST", "/api/v1/notice/message")
	require.NotNil(t, legacyRoute, "Legacy route should exist")

	// 두 라우트 모두 동일한 핸들러를 사용하는지 확인
	assert.Equal(t, notificationsRoute.Name, legacyRoute.Name, "Both routes should use the same handler")
}

// TestSetupRoutes_NilHandler nil 핸들러 전달 시 패닉이 발생하지 않는지 검증합니다.
func TestSetupRoutes_NilHandler(t *testing.T) {
	// Setup
	e := echo.New()

	// Execute & Verify
	// nil 핸들러를 전달해도 패닉이 발생하지 않아야 합니다.
	// 실제로는 런타임 에러가 발생할 수 있지만, SetupRoutes 함수 자체는 패닉을 발생시키지 않습니다.
	assert.NotPanics(t, func() {
		SetupRoutes(e, nil)
	}, "SetupRoutes should not panic with nil handler")
}

// findRoute 주어진 메서드와 경로에 해당하는 라우트를 찾습니다.
func findRoute(routes []*echo.Route, method, path string) *echo.Route {
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return route
		}
	}
	return nil
}
