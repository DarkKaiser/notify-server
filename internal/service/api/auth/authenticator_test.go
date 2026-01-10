package auth

import (
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/api/model/domain"
	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Test Helpers
// =============================================================================

// createTestAppConfig 테스트용 AppConfig를 생성합니다.
func createTestAppConfig(apps ...config.ApplicationConfig) *config.AppConfig {
	return &config.AppConfig{
		NotifyAPI: config.NotifyAPIConfig{
			Applications: apps,
		},
	}
}

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNewAuthenticator_Table(t *testing.T) {
	tests := []struct {
		name          string
		appConfig     *config.AppConfig
		expectedCount int
		verifyApps    func(*testing.T, map[string]*domain.Application)
	}{
		{
			name: "Single Application",
			appConfig: createTestAppConfig(
				config.ApplicationConfig{
					ID:                "test-app",
					Title:             "Test Application",
					Description:       "Test Description",
					DefaultNotifierID: "test-notifier",
					AppKey:            "test-key",
				},
			),
			expectedCount: 1,
			verifyApps: func(t *testing.T, apps map[string]*domain.Application) {
				app, ok := apps["test-app"]
				assert.True(t, ok)
				assert.Equal(t, "test-app", app.ID)
				assert.Equal(t, "Test Application", app.Title)
				assert.Equal(t, "Test Description", app.Description)
				assert.Equal(t, "test-notifier", app.DefaultNotifierID)
				assert.Equal(t, "test-key", app.AppKey)
			},
		},
		{
			name: "Multiple Applications",
			appConfig: createTestAppConfig(
				config.ApplicationConfig{ID: "app1", AppKey: "key1"},
				config.ApplicationConfig{ID: "app2", AppKey: "key2"},
				config.ApplicationConfig{ID: "app3", AppKey: "key3"},
			),
			expectedCount: 3,
			verifyApps: func(t *testing.T, apps map[string]*domain.Application) {
				assert.Contains(t, apps, "app1")
				assert.Contains(t, apps, "app2")
				assert.Contains(t, apps, "app3")
			},
		},
		{
			name:          "No Applications",
			appConfig:     createTestAppConfig(),
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authenticator := NewAuthenticator(tt.appConfig)

			assert.NotNil(t, authenticator)
			assert.NotNil(t, authenticator.applications)
			assert.Equal(t, tt.expectedCount, len(authenticator.applications))

			if tt.verifyApps != nil {
				tt.verifyApps(t, authenticator.applications)
			}
		})
	}
}

// =============================================================================
// Authenticate Tests
// =============================================================================

func TestAuthenticator_Authenticate_Table(t *testing.T) {
	appConfig := createTestAppConfig(
		config.ApplicationConfig{
			ID:     "test-app",
			Title:  "Test App",
			AppKey: "valid-key",
		},
	)
	authenticator := NewAuthenticator(appConfig)

	tests := []struct {
		name          string
		appID         string
		appKey        string
		expectedError bool
		checkError    func(*testing.T, error)
		checkApp      func(*testing.T, *domain.Application)
	}{
		{
			name:          "Valid Authentication",
			appID:         "test-app",
			appKey:        "valid-key",
			expectedError: false,
			checkApp: func(t *testing.T, app *domain.Application) {
				assert.Equal(t, "test-app", app.ID)
				assert.Equal(t, "Test App", app.Title)
				assert.Equal(t, "valid-key", app.AppKey)
			},
		},
		{
			name:          "Invalid App ID",
			appID:         "unknown-app",
			appKey:        "valid-key",
			expectedError: true,
			checkError: func(t *testing.T, err error) {
				httpErr, ok := err.(*echo.HTTPError)
				assert.True(t, ok, "에러는 *echo.HTTPError 타입이어야 함")
				assert.Equal(t, 401, httpErr.Code)

				// 에러 메시지 상세 검증
				errResp, ok := httpErr.Message.(response.ErrorResponse)
				assert.True(t, ok, "에러 메시지는 ErrorResponse 타입이어야 함")
				assert.Contains(t, errResp.Message, "접근이 허용되지 않은")
				assert.Contains(t, errResp.Message, "unknown-app")
			},
		},
		{
			name:          "Invalid App Key",
			appID:         "test-app",
			appKey:        "invalid-key",
			expectedError: true,
			checkError: func(t *testing.T, err error) {
				httpErr, ok := err.(*echo.HTTPError)
				assert.True(t, ok, "에러는 *echo.HTTPError 타입이어야 함")
				assert.Equal(t, 401, httpErr.Code)

				// 에러 메시지 상세 검증
				errResp, ok := httpErr.Message.(response.ErrorResponse)
				assert.True(t, ok, "에러 메시지는 ErrorResponse 타입이어야 함")
				assert.Contains(t, errResp.Message, "app_key가 유효하지 않습니다")
				assert.Contains(t, errResp.Message, "test-app")
			},
		},
		{
			name:          "Empty App Key",
			appID:         "test-app",
			appKey:        "",
			expectedError: true,
			checkError: func(t *testing.T, err error) {
				httpErr, ok := err.(*echo.HTTPError)
				assert.True(t, ok)
				assert.Equal(t, 401, httpErr.Code)
			},
		},
		{
			name:          "Empty App ID",
			appID:         "",
			appKey:        "valid-key",
			expectedError: true,
			checkError: func(t *testing.T, err error) {
				httpErr, ok := err.(*echo.HTTPError)
				assert.True(t, ok)
				assert.Equal(t, 401, httpErr.Code)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, err := authenticator.Authenticate(tt.appID, tt.appKey)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, app)
				if tt.checkError != nil {
					tt.checkError(t, err)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, app)
				if tt.checkApp != nil {
					tt.checkApp(t, app)
				}
			}
		})
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

// TestAuthenticator_ConcurrentAccess는 동시성 안전성을 검증합니다.
func TestAuthenticator_ConcurrentAccess(t *testing.T) {
	appConfig := createTestAppConfig(
		config.ApplicationConfig{
			ID:     "concurrent-app",
			Title:  "Concurrent Test App",
			AppKey: "concurrent-key",
		},
	)
	authenticator := NewAuthenticator(appConfig)

	// 동시에 100개의 고루틴에서 Authenticate 호출
	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	errors := make(chan error, goroutines)
	apps := make(chan *domain.Application, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			app, err := authenticator.Authenticate("concurrent-app", "concurrent-key")
			errors <- err
			apps <- app
		}()
	}

	wg.Wait()
	close(errors)
	close(apps)

	// 모든 호출이 성공해야 함
	successCount := 0
	for err := range errors {
		assert.NoError(t, err)
		if err == nil {
			successCount++
		}
	}

	assert.Equal(t, goroutines, successCount, "모든 고루틴에서 성공해야 함")

	// 모든 반환된 앱이 동일해야 함
	for app := range apps {
		if app != nil {
			assert.Equal(t, "concurrent-app", app.ID)
			assert.Equal(t, "Concurrent Test App", app.Title)
		}
	}
}
