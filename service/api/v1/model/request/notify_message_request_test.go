package request

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotifyMessageRequest(t *testing.T) {
	t.Run("NotifyMessageRequest 구조체 생성", func(t *testing.T) {
		msg := &NotifyMessageRequest{
			ApplicationID: "app-123",
			Message:       "Test notification message",
			ErrorOccurred: false,
		}

		assert.Equal(t, "app-123", msg.ApplicationID, "ApplicationID가 일치해야 합니다")
		assert.Equal(t, "Test notification message", msg.Message, "Message가 일치해야 합니다")
		assert.False(t, msg.ErrorOccurred, "ErrorOccurred가 false여야 합니다")
	})

	t.Run("에러 메시지", func(t *testing.T) {
		msg := &NotifyMessageRequest{
			ApplicationID: "app-456",
			Message:       "Error occurred!",
			ErrorOccurred: true,
		}

		assert.Equal(t, "app-456", msg.ApplicationID, "ApplicationID가 일치해야 합니다")
		assert.Equal(t, "Error occurred!", msg.Message, "Message가 일치해야 합니다")
		assert.True(t, msg.ErrorOccurred, "ErrorOccurred가 true여야 합니다")
	})

	t.Run("빈 NotifyMessageRequest", func(t *testing.T) {
		msg := &NotifyMessageRequest{}

		assert.Empty(t, msg.ApplicationID, "ApplicationID가 비어있어야 합니다")
		assert.Empty(t, msg.Message, "Message가 비어있어야 합니다")
		assert.False(t, msg.ErrorOccurred, "ErrorOccurred 기본값은 false여야 합니다")
	})

	t.Run("긴 메시지", func(t *testing.T) {
		longMessage := "This is a very long message. " +
			"It contains multiple sentences and should be handled properly. " +
			"The system should be able to process messages of various lengths."

		msg := &NotifyMessageRequest{
			Message: longMessage,
		}

		assert.Equal(t, longMessage, msg.Message, "긴 메시지도 정확히 저장되어야 합니다")
		assert.Greater(t, len(msg.Message), 100, "메시지 길이가 100자를 초과해야 합니다")
	})
}
