package auth

import (
	"testing"

	"github.com/darkkaiser/notify-server/config"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestNewApplicationManager(t *testing.T) {
	t.Run("애플리케이션 로딩", func(t *testing.T) {
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

		manager := NewApplicationManager(appConfig)

		assert.NotNil(t, manager, "ApplicationManager가 생성되어야 합니다")
		assert.Equal(t, 1, len(manager.applications), "1개의 애플리케이션이 로드되어야 합니다")

		app, ok := manager.applications["test-app"]
		assert.True(t, ok, "test-app이 존재해야 합니다")
		assert.Equal(t, "test-app", app.ID)
		assert.Equal(t, "Test Application", app.Title)
	})

	t.Run("여러 애플리케이션 로딩", func(t *testing.T) {
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

		manager := NewApplicationManager(appConfig)

		assert.NotNil(t, manager, "ApplicationManager가 생성되어야 합니다")
		assert.Equal(t, 2, len(manager.applications), "2개의 애플리케이션이 로드되어야 합니다")
	})

	t.Run("빈 애플리케이션 목록", func(t *testing.T) {
		appConfig := &config.AppConfig{}

		manager := NewApplicationManager(appConfig)

		assert.NotNil(t, manager, "ApplicationManager가 생성되어야 합니다")
		assert.Equal(t, 0, len(manager.applications), "애플리케이션이 없어야 합니다")
	})
}

func TestApplicationManager_Authenticate(t *testing.T) {
	appConfig := &config.AppConfig{}
	appConfig.NotifyAPI.Applications = []config.ApplicationConfig{
		{
			ID:                "test-app",
			Title:             "Test App",
			Description:       "Test Application",
			DefaultNotifierID: "test-notifier",
			AppKey:            "valid-key",
		},
	}
	manager := NewApplicationManager(appConfig)

	t.Run("정상 인증", func(t *testing.T) {
		app, err := manager.Authenticate("test-app", "valid-key")

		assert.NoError(t, err, "인증이 성공해야 합니다")
		assert.NotNil(t, app, "Application 객체가 반환되어야 합니다")
		assert.Equal(t, "test-app", app.ID)
		assert.Equal(t, "Test App", app.Title)
	})

	t.Run("존재하지 않는 Application ID", func(t *testing.T) {
		app, err := manager.Authenticate("unknown-app", "valid-key")

		assert.Error(t, err, "에러가 발생해야 합니다")
		assert.Nil(t, app, "Application 객체가 nil이어야 합니다")

		httpErr, ok := err.(*echo.HTTPError)
		assert.True(t, ok, "echo.HTTPError 타입이어야 합니다")
		assert.Equal(t, 401, httpErr.Code, "401 Unauthorized여야 합니다")
	})

	t.Run("잘못된 App Key", func(t *testing.T) {
		app, err := manager.Authenticate("test-app", "invalid-key")

		assert.Error(t, err, "에러가 발생해야 합니다")
		assert.Nil(t, app, "Application 객체가 nil이어야 합니다")

		httpErr, ok := err.(*echo.HTTPError)
		assert.True(t, ok, "echo.HTTPError 타입이어야 합니다")
		assert.Equal(t, 401, httpErr.Code, "401 Unauthorized여야 합니다")
	})

	t.Run("빈 App Key", func(t *testing.T) {
		app, err := manager.Authenticate("test-app", "")

		assert.Error(t, err, "에러가 발생해야 합니다")
		assert.Nil(t, app, "Application 객체가 nil이어야 합니다")
	})
}

func TestApplicationManager_Integration(t *testing.T) {
	t.Run("Application 도메인 모델 변환 확인", func(t *testing.T) {
		appConfig := &config.AppConfig{}
		appConfig.NotifyAPI.Applications = []config.ApplicationConfig{
			{
				ID:                "integration-app",
				Title:             "Integration Test App",
				Description:       "Integration Test Description",
				DefaultNotifierID: "integration-notifier",
				AppKey:            "integration-key",
			},
		}

		manager := NewApplicationManager(appConfig)
		app, err := manager.Authenticate("integration-app", "integration-key")

		assert.NoError(t, err)
		assert.NotNil(t, app)

		// domain.Application 구조체의 모든 필드가 올바르게 매핑되었는지 확인
		assert.Equal(t, "integration-app", app.ID)
		assert.Equal(t, "Integration Test App", app.Title)
		assert.Equal(t, "Integration Test Description", app.Description)
		assert.Equal(t, "integration-notifier", app.DefaultNotifierID)
		assert.Equal(t, "integration-key", app.AppKey)
	})
}
