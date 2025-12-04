package handler

import (
	"testing"

	"github.com/darkkaiser/notify-server/config"
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

		mockSender := &mockNotificationSender{}
		handler := NewHandler(appConfig, mockSender, "1.0.0", "2024-01-01", "100")

		assert.NotNil(t, handler, "핸들러가 생성되어야 합니다")
		assert.Equal(t, 1, len(handler.applications), "1개의 애플리케이션이 등록되어야 합니다")
		assert.Equal(t, "test-app", handler.applications["test-app"].ID, "애플리케이션 ID가 일치해야 합니다")
		assert.Equal(t, "Test Application", handler.applications["test-app"].Title, "애플리케이션 제목이 일치해야 합니다")
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

		mockSender := &mockNotificationSender{}
		handler := NewHandler(appConfig, mockSender, "1.0.0", "2024-01-01", "100")

		assert.NotNil(t, handler, "핸들러가 생성되어야 합니다")
		assert.Equal(t, 2, len(handler.applications), "2개의 애플리케이션이 등록되어야 합니다")
	})

	t.Run("빈 애플리케이션 목록", func(t *testing.T) {
		appConfig := &config.AppConfig{}

		mockSender := &mockNotificationSender{}
		handler := NewHandler(appConfig, mockSender, "1.0.0", "2024-01-01", "100")

		assert.NotNil(t, handler, "핸들러가 생성되어야 합니다")
		assert.Equal(t, 0, len(handler.applications), "애플리케이션이 없어야 합니다")
	})
}

// mockNotificationSender는 테스트용 NotificationSender 구현체입니다.
type mockNotificationSender struct {
	notifyCalls []notifyCall
}

type notifyCall struct {
	notifierID    string
	title         string
	message       string
	errorOccurred bool
}

func (m *mockNotificationSender) Notify(notifierID string, title string, message string, errorOccurred bool) bool {
	m.notifyCalls = append(m.notifyCalls, notifyCall{
		notifierID:    notifierID,
		title:         title,
		message:       message,
		errorOccurred: errorOccurred,
	})
	return true
}

func (m *mockNotificationSender) NotifyToDefault(message string) bool {
	return true
}

func (m *mockNotificationSender) NotifyWithErrorToDefault(message string) bool {
	return true
}
