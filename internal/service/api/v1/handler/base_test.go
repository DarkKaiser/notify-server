package handler

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewHandler는 Handler 생성자 함수를 검증합니다.
func TestNewHandler(t *testing.T) {
	t.Run("성공: 올바른 의존성으로 핸들러 생성", func(t *testing.T) {
		// Setup
		mockSender := &mocks.MockNotificationSender{}

		// Execute
		h := NewHandler(mockSender)

		// Verify
		require.NotNil(t, h, "생성된 핸들러는 nil이 아니어야 합니다")
		assert.Equal(t, mockSender, h.notificationSender, "주입된 NotificationSender가 일치해야 합니다")
	})
}
