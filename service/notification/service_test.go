package notification

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/assert"
)

func TestNotificationService_SupportsHTMLMessage(t *testing.T) {
	t.Run("존재하는 Notifier의 HTML 지원 여부", func(t *testing.T) {
		// Mock notifier 생성
		mockNotifier := &mockNotifierHandler{
			id:                  NotifierID("test-notifier"),
			supportsHTMLMessage: true,
		}

		service := &NotificationService{
			notifierHandlers: []NotifierHandler{mockNotifier},
		}

		result := service.SupportsHTMLMessage("test-notifier")
		assert.True(t, result, "HTML 메시지를 지원해야 합니다")
	})

	t.Run("존재하지 않는 Notifier", func(t *testing.T) {
		service := &NotificationService{
			notifierHandlers: []NotifierHandler{},
		}

		result := service.SupportsHTMLMessage("non-existent")
		assert.False(t, result, "존재하지 않는 Notifier는 false를 반환해야 합니다")
	})
}

// 새로운 테스트: NewService
func TestNotificationService_NewService(t *testing.T) {
	t.Run("서비스 생성", func(t *testing.T) {
		appConfig := &config.AppConfig{}
		mockTaskRunner := &mockTaskRunner{}

		service := NewService(appConfig, mockTaskRunner)

		assert.NotNil(t, service, "서비스가 생성되어야 합니다")
		assert.Equal(t, appConfig, service.appConfig, "Config가 설정되어야 합니다")
		assert.Equal(t, mockTaskRunner, service.taskRunner, "TaskRunner가 설정되어야 합니다")
		assert.False(t, service.running, "초기에는 실행 중이 아니어야 합니다")
		assert.NotNil(t, service.notificationStopWaiter, "WaitGroup이 초기화되어야 합니다")
	})
}

// 새로운 테스트: Notify
func TestNotificationService_Notify(t *testing.T) {
	t.Run("정상적인 알림 전송", func(t *testing.T) {
		mockNotifier := &mockNotifierHandler{
			id:                  NotifierID("test-notifier"),
			supportsHTMLMessage: true,
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
			id:                  NotifierID("test-notifier"),
			supportsHTMLMessage: true,
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
			id:                  NotifierID("default-notifier"),
			supportsHTMLMessage: true,
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
			id:                  NotifierID("default-notifier"),
			supportsHTMLMessage: true,
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
			id:                  NotifierID("test-notifier"),
			supportsHTMLMessage: true,
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
			id:                  NotifierID("default-notifier"),
			supportsHTMLMessage: true,
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
			id:                  NotifierID("notifier1"),
			supportsHTMLMessage: true,
		}
		mockNotifier2 := &mockNotifierHandler{
			id:                  NotifierID("notifier2"),
			supportsHTMLMessage: false,
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
	id                  NotifierID
	supportsHTMLMessage bool
	notifyCalls         []mockNotifyCall
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

func (m *mockNotifierHandler) SupportsHTMLMessage() bool {
	return m.supportsHTMLMessage
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

// mockNotifierFactory는 테스트용 NotifierFactory 구현체입니다.
type mockNotifierFactory struct {
	createNotifiersFunc func(cfg *config.AppConfig) []NotifierHandler
}

func (m *mockNotifierFactory) CreateNotifiers(cfg *config.AppConfig) []NotifierHandler {
	if m.createNotifiersFunc != nil {
		return m.createNotifiersFunc(cfg)
	}
	return nil
}

func TestNotificationService_Run(t *testing.T) {
	t.Run("서비스 시작 및 중지", func(t *testing.T) {
		appConfig := &config.AppConfig{}
		// Setup config with default notifier
		appConfig.Notifiers.DefaultNotifierID = "default-notifier"

		mockTaskRunner := &mockTaskRunner{}
		service := NewService(appConfig, mockTaskRunner)

		// Mock factory
		mockFactory := &mockNotifierFactory{
			createNotifiersFunc: func(cfg *config.AppConfig) []NotifierHandler {
				return []NotifierHandler{
					&mockNotifierHandler{
						id:                  NotifierID("default-notifier"),
						supportsHTMLMessage: true,
					},
				}
			},
		}
		service.SetNotifierFactory(mockFactory)

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
		appConfig := &config.AppConfig{}
		appConfig.Notifiers.DefaultNotifierID = "default-notifier"

		mockTaskRunner := &mockTaskRunner{}
		service := NewService(appConfig, mockTaskRunner)

		mockFactory := &mockNotifierFactory{
			createNotifiersFunc: func(cfg *config.AppConfig) []NotifierHandler {
				return []NotifierHandler{
					&mockNotifierHandler{
						id:                  NotifierID("default-notifier"),
						supportsHTMLMessage: true,
					},
				}
			},
		}
		service.SetNotifierFactory(mockFactory)

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
		appConfig := &config.AppConfig{}
		appConfig.Notifiers.DefaultNotifierID = "notifier1"

		mockTaskRunner := &mockTaskRunner{}
		service := NewService(appConfig, mockTaskRunner)

		mockFactory := &mockNotifierFactory{
			createNotifiersFunc: func(cfg *config.AppConfig) []NotifierHandler {
				return []NotifierHandler{
					&mockNotifierHandler{
						id:                  NotifierID("notifier1"),
						supportsHTMLMessage: true,
					},
					&mockNotifierHandler{
						id:                  NotifierID("notifier2"),
						supportsHTMLMessage: true,
					},
				}
			},
		}
		service.SetNotifierFactory(mockFactory)

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

	t.Run("waitForShutdown 함수 - 정상 종료 및 리소스 정리", func(t *testing.T) {
		appConfig := &config.AppConfig{}
		appConfig.Notifiers.DefaultNotifierID = "default-notifier"

		mockTaskRunner := &mockTaskRunner{}
		service := NewService(appConfig, mockTaskRunner)

		mockFactory := &mockNotifierFactory{
			createNotifiersFunc: func(cfg *config.AppConfig) []NotifierHandler {
				return []NotifierHandler{
					&mockNotifierHandler{
						id:                  NotifierID("default-notifier"),
						supportsHTMLMessage: true,
					},
				}
			},
		}
		service.SetNotifierFactory(mockFactory)

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

		// 서비스 중지 후 알림 전송 시도 (Panic이 발생하지 않아야 함)
		assert.False(t, service.NotifyToDefault("test after shutdown"), "서비스 중지 후에는 알림 전송이 실패해야 합니다")
		assert.False(t, service.NotifyWithErrorToDefault("error after shutdown"), "서비스 중지 후에는 에러 알림 전송이 실패해야 합니다")
		assert.False(t, service.NotifyWithTaskContext("default-notifier", "msg", nil), "서비스 중지 후에는 알림 전송이 실패해야 합니다")
	})
}
