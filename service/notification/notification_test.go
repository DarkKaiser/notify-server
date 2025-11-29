package notification

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/g"
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

	t.Run("Panic 복구 테스트", func(t *testing.T) {
		// 닫힌 채널로 panic 유발
		sendC := make(chan *notificationSendData)
		close(sendC)

		n := &notifier{
			id:                 NotifierID("test-id"),
			supportHTMLMessage: true,
			notificationSendC:  sendC,
		}

		succeeded := n.Notify("test message", nil)

		assert.False(t, succeeded, "Panic 발생 시 false를 반환해야 합니다")
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
			notifierHandlers: []NotifierHandler{mockNotifier},
		}

		result := service.SupportHTMLMessage("test-notifier")
		assert.True(t, result, "HTML 메시지를 지원해야 합니다")
	})

	t.Run("존재하지 않는 Notifier", func(t *testing.T) {
		service := &NotificationService{
			notifierHandlers: []NotifierHandler{},
		}

		result := service.SupportHTMLMessage("non-existent")
		assert.False(t, result, "존재하지 않는 Notifier는 false를 반환해야 합니다")
	})
}

// 새로운 테스트: NewService
func TestNotificationService_NewService(t *testing.T) {
	t.Run("서비스 생성", func(t *testing.T) {
		config := &g.AppConfig{}
		mockTaskRunner := &mockTaskRunner{}

		service := NewService(config, mockTaskRunner)

		assert.NotNil(t, service, "서비스가 생성되어야 합니다")
		assert.Equal(t, config, service.config, "Config가 설정되어야 합니다")
		assert.Equal(t, mockTaskRunner, service.taskRunner, "TaskRunner가 설정되어야 합니다")
		assert.False(t, service.running, "초기에는 실행 중이 아니어야 합니다")
		assert.NotNil(t, service.notificationStopWaiter, "WaitGroup이 초기화되어야 합니다")
	})
}

// 새로운 테스트: Notify
func TestNotificationService_Notify(t *testing.T) {
	t.Run("정상적인 알림 전송", func(t *testing.T) {
		mockNotifier := &mockNotifierHandler{
			id:                 NotifierID("test-notifier"),
			supportHTMLMessage: true,
		}

		service := &NotificationService{
			notifierHandlers: []NotifierHandler{mockNotifier},
			running:          true,
		}

		result := service.Notify("test-notifier", "Test Title", "Test message", false)

		assert.True(t, result, "알림 전송이 성공해야 합니다")
		assert.Equal(t, 1, len(mockNotifier.notifyCalls), "Notify가 1번 호출되어야 합니다")
		assert.Equal(t, "Test message", mockNotifier.notifyCalls[0].message, "메시지가 일치해야 합니다")
	})

	t.Run("에러 플래그와 함께 알림 전송", func(t *testing.T) {
		mockNotifier := &mockNotifierHandler{
			id:                 NotifierID("test-notifier"),
			supportHTMLMessage: true,
		}

		service := &NotificationService{
			notifierHandlers: []NotifierHandler{mockNotifier},
			running:          true,
		}

		result := service.Notify("test-notifier", "Error Title", "Error message", true)

		assert.True(t, result, "알림 전송이 성공해야 합니다")
		assert.Equal(t, 1, len(mockNotifier.notifyCalls), "Notify가 1번 호출되어야 합니다")
		// TaskContext에 에러 플래그가 설정되어야 함
		assert.NotNil(t, mockNotifier.notifyCalls[0].taskCtx, "TaskContext가 있어야 합니다")
	})
}

// 새로운 테스트: NotifyToDefault
func TestNotificationService_NotifyToDefault(t *testing.T) {
	t.Run("기본 Notifier로 알림 전송", func(t *testing.T) {
		mockNotifier := &mockNotifierHandler{
			id:                 NotifierID("default-notifier"),
			supportHTMLMessage: true,
		}

		service := &NotificationService{
			defaultNotifierHandler: mockNotifier,
			notifierHandlers:       []NotifierHandler{mockNotifier},
			running:                true,
		}

		result := service.NotifyToDefault("Default message")

		assert.True(t, result, "기본 알림 전송이 성공해야 합니다")
		assert.Equal(t, 1, len(mockNotifier.notifyCalls), "Notify가 1번 호출되어야 합니다")
		assert.Equal(t, "Default message", mockNotifier.notifyCalls[0].message, "메시지가 일치해야 합니다")
	})
}

// 새로운 테스트: NotifyWithErrorToDefault
func TestNotificationService_NotifyWithErrorToDefault(t *testing.T) {
	t.Run("기본 Notifier로 에러 알림 전송", func(t *testing.T) {
		mockNotifier := &mockNotifierHandler{
			id:                 NotifierID("default-notifier"),
			supportHTMLMessage: true,
		}

		service := &NotificationService{
			defaultNotifierHandler: mockNotifier,
			notifierHandlers:       []NotifierHandler{mockNotifier},
			running:                true,
		}

		result := service.NotifyWithErrorToDefault("Error message")

		assert.True(t, result, "에러 알림 전송이 성공해야 합니다")
		assert.Equal(t, 1, len(mockNotifier.notifyCalls), "Notify가 1번 호출되어야 합니다")
		assert.Equal(t, "Error message", mockNotifier.notifyCalls[0].message, "메시지가 일치해야 합니다")
		assert.NotNil(t, mockNotifier.notifyCalls[0].taskCtx, "TaskContext가 있어야 합니다")
	})
}

// 새로운 테스트: NotifyWithTaskContext
func TestNotificationService_NotifyWithTaskContext(t *testing.T) {
	t.Run("TaskContext와 함께 알림 전송", func(t *testing.T) {
		mockNotifier := &mockNotifierHandler{
			id:                 NotifierID("test-notifier"),
			supportHTMLMessage: true,
		}

		service := &NotificationService{
			notifierHandlers: []NotifierHandler{mockNotifier},
			running:          true,
		}

		taskCtx := task.NewContext().
			WithTask(task.TaskID("TEST"), task.TaskCommandID("TEST_CMD")).
			With(task.TaskCtxKeyTitle, "Test Task")

		result := service.NotifyWithTaskContext("test-notifier", "Test message", taskCtx)

		assert.True(t, result, "알림 전송이 성공해야 합니다")
		assert.Equal(t, 1, len(mockNotifier.notifyCalls), "Notify가 1번 호출되어야 합니다")
		assert.Equal(t, "Test message", mockNotifier.notifyCalls[0].message, "메시지가 일치해야 합니다")
		assert.Equal(t, taskCtx, mockNotifier.notifyCalls[0].taskCtx, "TaskContext가 일치해야 합니다")
	})

	t.Run("존재하지 않는 Notifier로 알림 전송", func(t *testing.T) {
		mockDefaultNotifier := &mockNotifierHandler{
			id:                 NotifierID("default-notifier"),
			supportHTMLMessage: true,
		}

		service := &NotificationService{
			defaultNotifierHandler: mockDefaultNotifier,
			notifierHandlers:       []NotifierHandler{mockDefaultNotifier},
			running:                true,
		}

		taskCtx := task.NewContext()
		result := service.NotifyWithTaskContext("non-existent", "Test message", taskCtx)

		assert.False(t, result, "존재하지 않는 Notifier로의 알림은 실패해야 합니다")
		// 기본 Notifier로 에러 메시지가 전송되어야 함
		assert.Equal(t, 1, len(mockDefaultNotifier.notifyCalls), "기본 Notifier로 에러 메시지가 전송되어야 합니다")
	})
}

// 새로운 테스트: 여러 Notifier 관리
func TestNotificationService_MultipleNotifiers(t *testing.T) {
	t.Run("여러 Notifier 중 특정 Notifier로 알림 전송", func(t *testing.T) {
		mockNotifier1 := &mockNotifierHandler{
			id:                 NotifierID("notifier1"),
			supportHTMLMessage: true,
		}
		mockNotifier2 := &mockNotifierHandler{
			id:                 NotifierID("notifier2"),
			supportHTMLMessage: false,
		}

		service := &NotificationService{
			defaultNotifierHandler: mockNotifier1,
			notifierHandlers:       []NotifierHandler{mockNotifier1, mockNotifier2},
			running:                true,
		}

		// notifier2로 알림 전송
		result := service.NotifyWithTaskContext("notifier2", "Message to notifier2", task.NewContext())

		assert.True(t, result, "알림 전송이 성공해야 합니다")
		assert.Equal(t, 0, len(mockNotifier1.notifyCalls), "notifier1은 호출되지 않아야 합니다")
		assert.Equal(t, 1, len(mockNotifier2.notifyCalls), "notifier2가 호출되어야 합니다")
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
	defer notificationStopWaiter.Done()
	<-notificationStopCtx.Done()
}

func (m *mockNotifierHandler) SupportHTMLMessage() bool {
	return m.supportHTMLMessage
}

// mockTaskRunner는 테스트용 TaskRunner 구현체입니다.
type mockTaskRunner struct{}

func (m *mockTaskRunner) TaskRun(taskID task.TaskID, taskCommandID task.TaskCommandID, notifierID string, manualRun bool, runType task.TaskRunBy) bool {
	return true
}

func (m *mockTaskRunner) TaskRunWithContext(taskID task.TaskID, taskCommandID task.TaskCommandID, taskCtx task.TaskContext, notifierID string, notifyResultOfTaskRunRequest bool, taskRunBy task.TaskRunBy) bool {
	return true
}

func (m *mockTaskRunner) TaskCancel(taskInstanceID task.TaskInstanceID) bool {
	return true
}

func TestNotificationService_Run(t *testing.T) {
	t.Run("서비스 시작 및 중지", func(t *testing.T) {
		config := &g.AppConfig{}
		// Setup config with default notifier
		config.Notifiers.DefaultNotifierID = "default-notifier"
		config.Notifiers.Telegrams = []g.TelegramConfig{
			{
				ID:       "default-notifier",
				BotToken: "test-token",
				ChatID:   12345,
			},
		}

		mockTaskRunner := &mockTaskRunner{}
		service := NewService(config, mockTaskRunner)

		// Mock createNotifier
		service.newNotifier = func(id NotifierID, botToken string, chatID int64, config *g.AppConfig) NotifierHandler {
			return &mockNotifierHandler{
				id:                 id,
				supportHTMLMessage: true,
			}
		}

		// Run service
		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}
		wg.Add(1)

		go service.Run(ctx, wg)

		// Wait a bit for service to start
		time.Sleep(100 * time.Millisecond)

		// Check if running by sending a notification
		assert.True(t, service.NotifyToDefault("test"), "서비스가 실행 중이어야 합니다")

		// Stop service
		cancel()
		wg.Wait()
	})

	t.Run("이미 실행 중인 서비스 재시작 시도", func(t *testing.T) {
		config := &g.AppConfig{}
		config.Notifiers.DefaultNotifierID = "default-notifier"
		config.Notifiers.Telegrams = []g.TelegramConfig{
			{
				ID:       "default-notifier",
				BotToken: "test-token",
				ChatID:   12345,
			},
		}

		mockTaskRunner := &mockTaskRunner{}
		service := NewService(config, mockTaskRunner)

		service.newNotifier = func(id NotifierID, botToken string, chatID int64, config *g.AppConfig) NotifierHandler {
			return &mockNotifierHandler{
				id:                 id,
				supportHTMLMessage: true,
			}
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wg := &sync.WaitGroup{}

		// 첫 번째 실행
		wg.Add(1)
		go service.Run(ctx, wg)
		time.Sleep(100 * time.Millisecond)

		// 두 번째 실행 시도 (이미 실행 중)
		wg.Add(1)
		service.Run(ctx, wg)

		// 서비스는 여전히 실행 중이어야 함
		assert.True(t, service.NotifyToDefault("test"), "서비스가 실행 중이어야 합니다")

		cancel()
		wg.Wait()
	})

	t.Run("여러 Notifier 등록", func(t *testing.T) {
		config := &g.AppConfig{}
		config.Notifiers.DefaultNotifierID = "notifier1"
		config.Notifiers.Telegrams = []g.TelegramConfig{
			{
				ID:       "notifier1",
				BotToken: "token1",
				ChatID:   11111,
			},
			{
				ID:       "notifier2",
				BotToken: "token2",
				ChatID:   22222,
			},
		}

		mockTaskRunner := &mockTaskRunner{}
		service := NewService(config, mockTaskRunner)

		service.newNotifier = func(id NotifierID, botToken string, chatID int64, config *g.AppConfig) NotifierHandler {
			return &mockNotifierHandler{
				id:                 id,
				supportHTMLMessage: true,
			}
		}

		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}
		wg.Add(1)

		go service.Run(ctx, wg)
		time.Sleep(100 * time.Millisecond)

		// 두 개의 Notifier가 등록되어야 함
		assert.Equal(t, 2, len(service.notifierHandlers), "2개의 Notifier가 등록되어야 합니다")

		cancel()
		wg.Wait()
	})

	t.Run("run0 함수 - 정상 종료 및 리소스 정리", func(t *testing.T) {
		config := &g.AppConfig{}
		config.Notifiers.DefaultNotifierID = "default-notifier"
		config.Notifiers.Telegrams = []g.TelegramConfig{
			{
				ID:       "default-notifier",
				BotToken: "test-token",
				ChatID:   12345,
			},
		}

		mockTaskRunner := &mockTaskRunner{}
		service := NewService(config, mockTaskRunner)

		service.newNotifier = func(id NotifierID, botToken string, chatID int64, config *g.AppConfig) NotifierHandler {
			return &mockNotifierHandler{
				id:                 id,
				supportHTMLMessage: true,
			}
		}

		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}
		wg.Add(1)

		go service.Run(ctx, wg)
		time.Sleep(100 * time.Millisecond)

		// 서비스가 실행 중인지 확인
		service.runningMu.Lock()
		running := service.running
		service.runningMu.Unlock()
		assert.True(t, running, "서비스가 실행 중이어야 합니다")

		// 서비스 중지
		cancel()
		wg.Wait()

		// 리소스가 정리되었는지 확인
		service.runningMu.Lock()
		assert.False(t, service.running, "서비스가 중지되어야 합니다")
		assert.Nil(t, service.taskRunner, "TaskRunner가 nil이어야 합니다")
		assert.Nil(t, service.notifierHandlers, "notifierHandlers가 nil이어야 합니다")
		assert.Nil(t, service.defaultNotifierHandler, "defaultNotifierHandler가 nil이어야 합니다")
		service.runningMu.Unlock()
	})
}
