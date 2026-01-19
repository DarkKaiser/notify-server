package handler

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("성공: 올바른 의존성으로 핸들러 생성", func(t *testing.T) {
		t.Parallel()
		// Setup
		mockSender := mocks.NewMockNotificationSender()

		// Execute
		h := New(mockSender)

		// Verify
		require.NotNil(t, h, "생성된 핸들러는 nil이 아니어야 합니다")
		assert.Equal(t, mockSender, h.notificationSender, "주입된 NotificationSender가 일치해야 합니다")
	})

	t.Run("실패: NotificationSender가 nil인 경우 Panic", func(t *testing.T) {
		t.Parallel()
		// Verify
		assert.PanicsWithValue(t, constants.PanicMsgNotificationSenderRequired, func() {
			// Execute
			New(nil)
		})
	})
}
