package handler

import (
	"testing"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/api/auth"
	"github.com/darkkaiser/notify-server/service/api/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNewHandler(t *testing.T) {
	t.Run("핸들러 생성", func(t *testing.T) {
		appConfig := &config.AppConfig{}
		appConfig.NotifyAPI.Applications = []config.ApplicationConfig{
			{
				ID:                "test-app",
				Title:             "Test Application",
				Description:       "Test Description",
				DefaultNotifierID: "test-notifier",
				AppKey:            "test-key",
			},
		}

		mockService := &testutil.MockNotificationService{}
		appManager := auth.NewApplicationManager(appConfig)
		handler := NewHandler(appManager, mockService)

		assert.NotNil(t, handler, "핸들러가 생성되어야 합니다")
		assert.NotNil(t, handler.applicationManager, "ApplicationManager가 설정되어야 합니다")
	})

	t.Run("여러 애플리케이션 등록", func(t *testing.T) {
		appConfig := &config.AppConfig{}
		appConfig.NotifyAPI.Applications = []config.ApplicationConfig{
			{
				ID:                "app1",
				Title:             "Application 1",
				Description:       "Description 1",
				DefaultNotifierID: "notifier1",
				AppKey:            "key1",
			},
			{
				ID:                "app2",
				Title:             "Application 2",
				Description:       "Description 2",
				DefaultNotifierID: "notifier2",
				AppKey:            "key2",
			},
		}

		mockService := &testutil.MockNotificationService{}
		appManager := auth.NewApplicationManager(appConfig)
		handler := NewHandler(appManager, mockService)

		assert.NotNil(t, handler, "핸들러가 생성되어야 합니다")
		assert.NotNil(t, handler.applicationManager, "ApplicationManager가 설정되어야 합니다")
	})

	t.Run("빈 애플리케이션 목록", func(t *testing.T) {
		appConfig := &config.AppConfig{}

		mockService := &testutil.MockNotificationService{}
		appManager := auth.NewApplicationManager(appConfig)
		handler := NewHandler(appManager, mockService)

		assert.NotNil(t, handler, "핸들러가 생성되어야 합니다")
		assert.NotNil(t, handler.applicationManager, "ApplicationManager가 설정되어야 합니다")
	})
}
