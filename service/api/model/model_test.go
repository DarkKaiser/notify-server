package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllowedApplication(t *testing.T) {
	t.Run("AllowedApplication 구조체 생성", func(t *testing.T) {
		app := &AllowedApplication{
			ID:                "test-app",
			Title:             "Test Application",
			Description:       "Test Description",
			DefaultNotifierID: "test-notifier",
			AppKey:            "secret-key-123",
		}

		assert.Equal(t, "test-app", app.ID, "ID가 일치해야 합니다")
		assert.Equal(t, "Test Application", app.Title, "Title이 일치해야 합니다")
		assert.Equal(t, "Test Description", app.Description, "Description이 일치해야 합니다")
		assert.Equal(t, "test-notifier", app.DefaultNotifierID, "DefaultNotifierID가 일치해야 합니다")
		assert.Equal(t, "secret-key-123", app.AppKey, "AppKey가 일치해야 합니다")
	})

	t.Run("빈 AllowedApplication", func(t *testing.T) {
		app := &AllowedApplication{}

		assert.Empty(t, app.ID, "ID가 비어있어야 합니다")
		assert.Empty(t, app.Title, "Title이 비어있어야 합니다")
		assert.Empty(t, app.Description, "Description이 비어있어야 합니다")
		assert.Empty(t, app.DefaultNotifierID, "DefaultNotifierID가 비어있어야 합니다")
		assert.Empty(t, app.AppKey, "AppKey가 비어있어야 합니다")
	})
}

func TestNotifyMessage(t *testing.T) {
	t.Run("NotifyMessage 구조체 생성", func(t *testing.T) {
		msg := &NotifyMessage{
			ApplicationID: "app-123",
			Message:       "Test notification message",
			ErrorOccurred: false,
		}

		assert.Equal(t, "app-123", msg.ApplicationID, "ApplicationID가 일치해야 합니다")
		assert.Equal(t, "Test notification message", msg.Message, "Message가 일치해야 합니다")
		assert.False(t, msg.ErrorOccurred, "ErrorOccurred가 false여야 합니다")
	})

	t.Run("에러 메시지", func(t *testing.T) {
		msg := &NotifyMessage{
			ApplicationID: "app-456",
			Message:       "Error occurred!",
			ErrorOccurred: true,
		}

		assert.Equal(t, "app-456", msg.ApplicationID, "ApplicationID가 일치해야 합니다")
		assert.Equal(t, "Error occurred!", msg.Message, "Message가 일치해야 합니다")
		assert.True(t, msg.ErrorOccurred, "ErrorOccurred가 true여야 합니다")
	})

	t.Run("빈 NotifyMessage", func(t *testing.T) {
		msg := &NotifyMessage{}

		assert.Empty(t, msg.ApplicationID, "ApplicationID가 비어있어야 합니다")
		assert.Empty(t, msg.Message, "Message가 비어있어야 합니다")
		assert.False(t, msg.ErrorOccurred, "ErrorOccurred 기본값은 false여야 합니다")
	})

	t.Run("긴 메시지", func(t *testing.T) {
		longMessage := "This is a very long message. " +
			"It contains multiple sentences and should be handled properly. " +
			"The system should be able to process messages of various lengths."

		msg := &NotifyMessage{
			Message: longMessage,
		}

		assert.Equal(t, longMessage, msg.Message, "긴 메시지도 정확히 저장되어야 합니다")
		assert.Greater(t, len(msg.Message), 100, "메시지 길이가 100자를 초과해야 합니다")
	})
}
