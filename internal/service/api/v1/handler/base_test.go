package handler

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		notificationSender contract.NotificationSender
		expectPanic        bool
		panicMsg           string // 패닉 발생 시 기대 메시지
	}{
		{
			name:               "성공: 올바른 의존성으로 핸들러 생성",
			notificationSender: mocks.NewMockNotificationSender(),
			expectPanic:        false,
		},
		{
			name:               "실패: NotificationSender가 nil인 경우 Panic",
			notificationSender: nil,
			expectPanic:        true,
			panicMsg:           "NotificationSender는 필수입니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.expectPanic {
				assert.PanicsWithValue(t, tt.panicMsg, func() {
					New(tt.notificationSender)
				})
			} else {
				h := New(tt.notificationSender)
				require.NotNil(t, h)
				assert.Equal(t, tt.notificationSender, h.notificationSender)
			}
		})
	}
}
