package auth

import (
	"testing"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/api/model/domain"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestNewApplicationManager_Table(t *testing.T) {
	tests := []struct {
		name          string
		appConfig     *config.AppConfig
		expectedCount int
		verifyApps    func(*testing.T, map[string]*domain.Application)
	}{
		{
			name: "Single Application",
			appConfig: &config.AppConfig{
				NotifyAPI: config.NotifyAPIConfig{
					Applications: []config.ApplicationConfig{
						{
							ID:                "test-app",
							Title:             "Test Application",
							Description:       "Test Description",
							DefaultNotifierID: "test-notifier",
							AppKey:            "test-key",
						},
					},
				},
			},
			expectedCount: 1,
			verifyApps: func(t *testing.T, apps map[string]*domain.Application) {
				app, ok := apps["test-app"]
				assert.True(t, ok)
				assert.Equal(t, "test-app", app.ID)
				assert.Equal(t, "Test Application", app.Title)
			},
		},
		{
			name: "Multiple Applications",
			appConfig: &config.AppConfig{
				NotifyAPI: config.NotifyAPIConfig{
					Applications: []config.ApplicationConfig{
						{ID: "app1", AppKey: "key1"},
						{ID: "app2", AppKey: "key2"},
					},
				},
			},
			expectedCount: 2,
		},
		{
			name: "No Applications",
			appConfig: &config.AppConfig{
				NotifyAPI: config.NotifyAPIConfig{
					Applications: []config.ApplicationConfig{},
				},
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewApplicationManager(tt.appConfig)
			assert.NotNil(t, manager)
			assert.Equal(t, tt.expectedCount, len(manager.applications))
			if tt.verifyApps != nil {
				tt.verifyApps(t, manager.applications)
			}
		})
	}
}

func TestApplicationManager_Authenticate_Table(t *testing.T) {
	appConfig := &config.AppConfig{
		NotifyAPI: config.NotifyAPIConfig{
			Applications: []config.ApplicationConfig{
				{
					ID:     "test-app",
					Title:  "Test App",
					AppKey: "valid-key",
				},
			},
		},
	}
	manager := NewApplicationManager(appConfig)

	tests := []struct {
		name          string
		appID         string
		appKey        string
		expectedError bool
		checkError    func(*testing.T, error)
	}{
		{
			name:          "Valid Authentication",
			appID:         "test-app",
			appKey:        "valid-key",
			expectedError: false,
		},
		{
			name:          "Invalid App ID",
			appID:         "unknown-app",
			appKey:        "valid-key",
			expectedError: true,
			checkError: func(t *testing.T, err error) {
				httpErr, ok := err.(*echo.HTTPError)
				assert.True(t, ok)
				assert.Equal(t, 401, httpErr.Code)
			},
		},
		{
			name:          "Invalid App Key",
			appID:         "test-app",
			appKey:        "invalid-key",
			expectedError: true,
			checkError: func(t *testing.T, err error) {
				httpErr, ok := err.(*echo.HTTPError)
				assert.True(t, ok)
				assert.Equal(t, 401, httpErr.Code)
			},
		},
		{
			name:          "Empty App Key",
			appID:         "test-app",
			appKey:        "",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, err := manager.Authenticate(tt.appID, tt.appKey)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, app)
				if tt.checkError != nil {
					tt.checkError(t, err)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, app)
				assert.Equal(t, tt.appID, app.ID)
			}
		})
	}
}

func TestApplicationManager_Integration(t *testing.T) {
	t.Run("Application Domain Model Check", func(t *testing.T) {
		appConfig := &config.AppConfig{
			NotifyAPI: config.NotifyAPIConfig{
				Applications: []config.ApplicationConfig{
					{
						ID:                "integration-app",
						Title:             "Integration Test App",
						Description:       "Integration Test Description",
						DefaultNotifierID: "integration-notifier",
						AppKey:            "integration-key",
					},
				},
			},
		}

		manager := NewApplicationManager(appConfig)
		app, err := manager.Authenticate("integration-app", "integration-key")

		assert.NoError(t, err)
		assert.NotNil(t, app)

		assert.Equal(t, "integration-app", app.ID)
		assert.Equal(t, "Integration Test App", app.Title)
		assert.Equal(t, "Integration Test Description", app.Description)
		assert.Equal(t, "integration-notifier", app.DefaultNotifierID)
		assert.Equal(t, "integration-key", app.AppKey)
	})
}
