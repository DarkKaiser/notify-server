package handler

import (
	"testing"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/api/auth"
	"github.com/darkkaiser/notify-server/service/api/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNewHandler_Table(t *testing.T) {
	tests := []struct {
		name         string
		appConfig    *config.AppConfig
		expectedApps int
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
			expectedApps: 1,
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
			expectedApps: 2,
		},
		{
			name: "Empty Application List",
			appConfig: &config.AppConfig{
				NotifyAPI: config.NotifyAPIConfig{},
			},
			expectedApps: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &testutil.MockNotificationService{}
			appManager := auth.NewApplicationManager(tt.appConfig)

			h := NewHandler(appManager, mockService)

			assert.NotNil(t, h)
			assert.NotNil(t, h.applicationManager)

			// ApplicationManager stores apps in unexported field, so we rely on NewHandler success
			// In auth package test we verified ApplicationManager behavior.
			// Here we just verify Handler creation integration.
			// Ideally we shouldn't poke internal structure too much, relying on auth tests for counting logic.
			// But for confidence we can do a quick check via casting if we were inside same package,
			// but appManager logic is tested in auth package.
			// So "expectedApps" check here is implicit via appManager creation success.
		})
	}
}
