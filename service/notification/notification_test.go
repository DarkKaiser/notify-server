package notification

import (
	"context"
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/assert"
)

func TestNotifierID(t *testing.T) {
	t.Run("NotifierID 타입 테스트", func(t *testing.T) {
		id := NotifierID("test-notifier")
		assert.Equal(t, NotifierID("test-notifier"), id, "NotifierID가 일치해야 합니다")
	})
}

func TestNotifier_ID(t *testing.T) {
	t.Run("Notifier ID 반환", func(t *testing.T) {
		n := &notifier{
			id:                 NotifierID("test-id"),
			supportHTMLMessage: true,
			notificationSendC:  make(chan *notificationSendData, 1),
		}

		assert.Equal(t, NotifierID("test-id"), n.ID(), "ID가 일치해야 합니다")
	})
}

func TestNotifier_SupportHTMLMessage(t *testing.T) {
	t.Run("HTML 메시지 지원", func(t *testing.T) {
		n := &notifier{
			id:                 NotifierID("test-id"),
			supportHTMLMessage: true,
			notificationSendC:  make(chan *notificationSendData, 1),
		}

		assert.True(t, n.SupportHTMLMessage(), "HTML 메시지를 지원해야 합니다")
	})

	t.Run("HTML 메시지 미지원", func(t *testing.T) {
		n := &notifier{
			id:                 NotifierID("test-id"),
			supportHTMLMessage: false,
			notificationSendC:  make(chan *notificationSendData, 1),
		}

		assert.False(t, n.SupportHTMLMessage(), "HTML 메시지를 지원하지 않아야 합니다")
	})
}

func TestNotifier_Notify(t *testing.T) {
	t.Run("정상적인 알림 전송", func(t *testing.T) {
		sendC := make(chan *notificationSendData, 1)
		n := &notifier{
			id:                 NotifierID("test-id"),
			supportHTMLMessage: true,
			notificationSendC:  sendC,
		}

		taskCtx := task.NewContext()
		succeeded := n.Notify("test message", taskCtx)

		assert.True(t, succeeded, "알림 전송이 성공해야 합니다")

		// 채널에서 데이터 확인
		select {
		case data := <-sendC:
			assert.Equal(t, "test message", data.message, "메시지가 일치해야 합니다")
			assert.NotNil(t, data.taskCtx, "TaskContext가 nil이 아니어야 합니다")
		default:
			t.Error("채널에 데이터가 전송되지 않았습니다")
		}
	})

	t.Run("nil TaskContext로 알림 전송", func(t *testing.T) {
		sendC := make(chan *notificationSendData, 1)
		n := &notifier{
			id:                 NotifierID("test-id"),
			supportHTMLMessage: true,
			notificationSendC:  sendC,
		}

		succeeded := n.Notify("test message", nil)

		assert.True(t, succeeded, "알림 전송이 성공해야 합니다")

		// 채널에서 데이터 확인
		select {
		case data := <-sendC:
			assert.Equal(t, "test message", data.message, "메시지가 일치해야 합니다")
			assert.Nil(t, data.taskCtx, "TaskContext가 nil이어야 합니다")
		default:
			t.Error("채널에 데이터가 전송되지 않았습니다")
		}
	})
}

func TestNotificationSendData(t *testing.T) {
	t.Run("NotificationSendData 구조체 생성", func(t *testing.T) {
		taskCtx := task.NewContext()
		data := &notificationSendData{
			message: "test message",
			taskCtx: taskCtx,
		}

		assert.Equal(t, "test message", data.message, "메시지가 일치해야 합니다")
		assert.NotNil(t, data.taskCtx, "TaskContext가 nil이 아니어야 합니다")
	})
}

func TestNotificationService_SupportHTMLMessage(t *testing.T) {
	t.Run("존재하는 Notifier의 HTML 지원 여부", func(t *testing.T) {
		// Mock notifier 생성
		mockNotifier := &mockNotifierHandler{
			id:                 NotifierID("test-notifier"),
			supportHTMLMessage: true,
		}

		service := &NotificationService{
			notifierHandlers: []notifierHandler{mockNotifier},
		}

		result := service.SupportHTMLMessage("test-notifier")
		assert.True(t, result, "HTML 메시지를 지원해야 합니다")
	})

	t.Run("존재하지 않는 Notifier", func(t *testing.T) {
		service := &NotificationService{
			notifierHandlers: []notifierHandler{},
		}

		result := service.SupportHTMLMessage("non-existent")
		assert.False(t, result, "존재하지 않는 Notifier는 false를 반환해야 합니다")
	})
}

// mockNotifierHandler는 테스트용 notifierHandler 구현체입니다.
type mockNotifierHandler struct {
	id                 NotifierID
	supportHTMLMessage bool
	notifyCalls        []mockNotifyCall
}

type mockNotifyCall struct {
	message string
	taskCtx task.TaskContext
}

func (m *mockNotifierHandler) ID() NotifierID {
	return m.id
}

func (m *mockNotifierHandler) Notify(message string, taskCtx task.TaskContext) bool {
	m.notifyCalls = append(m.notifyCalls, mockNotifyCall{
		message: message,
		taskCtx: taskCtx,
	})
	return true
}

func (m *mockNotifierHandler) Run(taskRunner task.TaskRunner, notificationStopCtx context.Context, notificationStopWaiter *sync.WaitGroup) {
	// Mock implementation - do nothing
}

func (m *mockNotifierHandler) SupportHTMLMessage() bool {
	return m.supportHTMLMessage
}
