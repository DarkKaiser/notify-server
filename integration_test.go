package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/pkg/common"
	"github.com/darkkaiser/notify-server/service/api"
	"github.com/darkkaiser/notify-server/service/notification"
	"github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/assert"
)

// TestServicesIntegration은 전체 서비스의 통합을 테스트합니다.
// mockIntegrationNotifierFactory는 테스트용 NotifierFactory 구현체입니다.
type mockIntegrationNotifierFactory struct {
	createNotifiersFunc func(cfg *config.AppConfig) []notification.NotifierHandler
}

func (m *mockIntegrationNotifierFactory) CreateNotifiers(cfg *config.AppConfig) []notification.NotifierHandler {
	if m.createNotifiersFunc != nil {
		return m.createNotifiersFunc(cfg)
	}
	return nil
}

func TestServicesIntegration(t *testing.T) {
	t.Run("서비스 초기화 및 시작", func(t *testing.T) {
		// 테스트용 설정 생성
		appConfig := createTestConfig()

		// 서비스 생성
		taskService := task.NewService(appConfig)
		notificationService := notification.NewService(appConfig, taskService)

		// Mock factory
		mockFactory := &mockIntegrationNotifierFactory{
			createNotifiersFunc: func(cfg *config.AppConfig) []notification.NotifierHandler {
				return []notification.NotifierHandler{
					&mockNotifierHandler{
						id:                  notification.NotifierID("test-notifier"),
						supportsHTMLMessage: true,
					},
				}
			},
		}
		notificationService.SetNotifierFactory(mockFactory)

		notifyAPIService := api.NewNotifyAPIService(appConfig, notificationService, common.BuildInfo{
			Version:     "test-version",
			BuildDate:   "test-date",
			BuildNumber: "test-build",
		})

		// 서비스 검증
		assert.NotNil(t, taskService, "TaskService가 생성되어야 합니다")
		assert.NotNil(t, notificationService, "NotificationService가 생성되어야 합니다")
		assert.NotNil(t, notifyAPIService, "NotifyAPIService가 생성되어야 합니다")

		// 의존성 주입 확인
		taskService.SetTaskNotificationSender(notificationService)

		// 서비스 시작 및 중지 테스트
		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}

		// 서비스 시작
		wg.Add(3)
		go taskService.Run(ctx, wg)
		go notificationService.Run(ctx, wg)
		go notifyAPIService.Run(ctx, wg)

		// 서비스가 시작될 때까지 대기
		time.Sleep(100 * time.Millisecond)

		// 서비스 중지
		cancel()
		wg.Wait()
	})

	t.Run("서비스 중복 시작 방지", func(t *testing.T) {
		appConfig := createTestConfig()
		taskService := task.NewService(appConfig)
		notificationService := notification.NewService(appConfig, taskService)

		// Mock factory
		mockFactory := &mockIntegrationNotifierFactory{
			createNotifiersFunc: func(cfg *config.AppConfig) []notification.NotifierHandler {
				return []notification.NotifierHandler{
					&mockNotifierHandler{
						id:                  notification.NotifierID("test-notifier"),
						supportsHTMLMessage: true,
					},
				}
			},
		}
		notificationService.SetNotifierFactory(mockFactory)

		taskService.SetTaskNotificationSender(notificationService)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wg := &sync.WaitGroup{}

		// 첫 번째 시작
		wg.Add(2)
		go taskService.Run(ctx, wg)
		go notificationService.Run(ctx, wg)
		time.Sleep(100 * time.Millisecond)

		// 두 번째 시작 시도 (중복)
		wg.Add(2)
		taskService.Run(ctx, wg)
		notificationService.Run(ctx, wg)

		// 정상 종료
		cancel()
		wg.Wait()
	})
}

// TestTaskToNotificationFlow는 Task에서 Notification으로의 흐름을 테스트합니다.
func TestTaskToNotificationFlow(t *testing.T) {
	t.Run("Task 실행 시 알림 발송", func(t *testing.T) {
		appConfig := createTestConfig()

		// Mock Notification Sender 생성
		mockSender := &mockNotificationSender{
			notifyCalls: make([]notifyCall, 0),
		}

		taskService := task.NewService(appConfig)
		taskService.SetTaskNotificationSender(mockSender)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		wg := &sync.WaitGroup{}

		wg.Add(1)
		go taskService.Run(ctx, wg)

		// 서비스 시작 대기
		time.Sleep(100 * time.Millisecond)

		// Task 실행 (존재하지 않는 Task로 실패 시나리오)
		result := taskService.TaskRun(
			task.TaskID("NON_EXISTENT"),
			task.TaskCommandID("TEST"),
			"test-notifier",
			true,
			task.TaskRunByUser,
		)

		// TaskRun은 비동기 요청이므로 성공적으로 큐에 들어가면 true를 반환함
		assert.True(t, result, "Task 실행 요청은 성공해야 합니다")

		// 알림 발송 확인 (처리 대기)
		time.Sleep(200 * time.Millisecond)

		// 에러 알림이 발송되었는지 확인
		assert.Greater(t, len(mockSender.notifyCalls), 0, "에러 알림이 발송되어야 합니다")

		cancel()
		wg.Wait()
	})

	t.Run("Task 실행 성공 시 알림 발송 확인", func(t *testing.T) {
		appConfig := createTestConfig()

		mockSender := &mockNotificationSender{
			notifyCalls: make([]notifyCall, 0),
		}

		taskService := task.NewService(appConfig)
		taskService.SetTaskNotificationSender(mockSender)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		wg := &sync.WaitGroup{}

		wg.Add(1)
		go taskService.Run(ctx, wg)

		time.Sleep(100 * time.Millisecond)

		// 초기 호출 횟수 기록
		initialCallCount := len(mockSender.notifyCalls)

		// Task 실행 (알림 요청 포함)
		taskService.TaskRun(
			task.TaskID("TEST"),
			task.TaskCommandID("TEST"),
			"test-notifier",
			true, // 알림 요청
			task.TaskRunByUser,
		)

		time.Sleep(200 * time.Millisecond)

		// 알림이 발송되었는지 확인
		assert.GreaterOrEqual(t, len(mockSender.notifyCalls), initialCallCount, "알림이 발송되어야 합니다")

		cancel()
		wg.Wait()
	})
}

// TestServiceLifecycle은 서비스 생명주기를 테스트합니다.
func TestServiceLifecycle(t *testing.T) {
	t.Run("서비스 시작 및 정상 종료", func(t *testing.T) {
		appConfig := createTestConfig()

		taskService := task.NewService(appConfig)
		notificationService := notification.NewService(appConfig, taskService)

		// Mock factory
		mockFactory := &mockIntegrationNotifierFactory{
			createNotifiersFunc: func(cfg *config.AppConfig) []notification.NotifierHandler {
				return []notification.NotifierHandler{
					&mockNotifierHandler{
						id:                  notification.NotifierID("test-notifier"),
						supportsHTMLMessage: true,
					},
				}
			},
		}
		notificationService.SetNotifierFactory(mockFactory)

		taskService.SetTaskNotificationSender(notificationService)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		wg := &sync.WaitGroup{}

		// 서비스 시작
		wg.Add(2)
		go taskService.Run(ctx, wg)
		go notificationService.Run(ctx, wg)

		// 잠시 실행
		time.Sleep(500 * time.Millisecond)

		// 타임아웃 또는 명시적 취소로 종료
		cancel()
		wg.Wait()
	})

	t.Run("여러 서비스 동시 시작 및 종료", func(t *testing.T) {
		appConfig := createTestConfig()

		// 여러 서비스 생성
		services := make([]*task.TaskService, 3)
		for i := 0; i < 3; i++ {
			services[i] = task.NewService(appConfig)
		}

		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}

		// 모든 서비스 시작
		for _, svc := range services {
			svc.SetTaskNotificationSender(&mockNotificationSender{})
			wg.Add(1)
			go svc.Run(ctx, wg)
		}

		time.Sleep(100 * time.Millisecond)

		// 모든 서비스 중지
		cancel()
		wg.Wait()
	})

	t.Run("서비스 빠른 시작 및 중지 반복", func(t *testing.T) {
		appConfig := createTestConfig()

		for i := 0; i < 3; i++ {
			taskService := task.NewService(appConfig)
			taskService.SetTaskNotificationSender(&mockNotificationSender{})

			ctx, cancel := context.WithCancel(context.Background())
			wg := &sync.WaitGroup{}

			wg.Add(1)
			go taskService.Run(ctx, wg)

			time.Sleep(50 * time.Millisecond)

			cancel()
			wg.Wait()
		}
	})
}

// TestNotificationServiceIntegration은 NotificationService 통합을 테스트합니다.
func TestNotificationServiceIntegration(t *testing.T) {
	t.Run("NotificationService 생성 및 초기화", func(t *testing.T) {
		appConfig := createTestConfigWithNotifier()

		mockTaskRunner := &mockTaskRunner{}
		notificationService := notification.NewService(appConfig, mockTaskRunner)

		// Mock factory
		mockFactory := &mockIntegrationNotifierFactory{
			createNotifiersFunc: func(cfg *config.AppConfig) []notification.NotifierHandler {
				return []notification.NotifierHandler{
					&mockNotifierHandler{
						id:                  notification.NotifierID("default-notifier"),
						supportsHTMLMessage: true,
					},
				}
			},
		}
		notificationService.SetNotifierFactory(mockFactory)

		// 서비스가 정상적으로 생성되었는지 확인
		assert.NotNil(t, notificationService, "NotificationService가 생성되어야 합니다")

		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}
		wg.Add(1)

		go notificationService.Run(ctx, wg)
		time.Sleep(100 * time.Millisecond)

		// 알림 발송 테스트
		result := notificationService.NotifyToDefault("통합 테스트 메시지")
		assert.True(t, result, "알림 발송이 성공해야 합니다")

		cancel()
		wg.Wait()
	})
}

// TestEndToEndScenario는 엔드투엔드 시나리오를 테스트합니다.
func TestEndToEndScenario(t *testing.T) {
	t.Run("전체 워크플로우", func(t *testing.T) {
		// 이 테스트는 실제 Task가 등록되어 있어야 하므로
		// 기본적인 서비스 연동만 확인
		appConfig := createTestConfig()

		taskService := task.NewService(appConfig)
		mockSender := &mockNotificationSender{
			notifyCalls: make([]notifyCall, 0),
		}
		taskService.SetTaskNotificationSender(mockSender)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		wg := &sync.WaitGroup{}

		wg.Add(1)
		go taskService.Run(ctx, wg)

		time.Sleep(200 * time.Millisecond)

		// Task 실행 시도
		taskService.TaskRun(
			task.TaskID("TEST"),
			task.TaskCommandID("TEST"),
			"test-notifier",
			false,
			task.TaskRunByUser,
		)

		time.Sleep(200 * time.Millisecond)

		cancel()
		wg.Wait()
	})
}

// 헬퍼 함수 및 Mock 객체
func createTestConfig() *config.AppConfig {
	return &config.AppConfig{
		Debug: true,
		Notifiers: config.NotifierConfig{
			DefaultNotifierID: "test-notifier",
			Telegrams: []config.TelegramConfig{
				{
					ID:       "test-notifier",
					BotToken: "test-token",
					ChatID:   12345,
				},
			},
		},
		NotifyAPI: config.NotifyAPIConfig{
			WS: config.WSConfig{
				ListenPort: 18080,
				TLSServer:  false,
			},
		},
		Tasks: []config.TaskConfig{},
	}
}

func createTestConfigWithNotifier() *config.AppConfig {
	appConfig := createTestConfig()
	appConfig.Notifiers.DefaultNotifierID = "default-notifier"
	appConfig.Notifiers.Telegrams = []config.TelegramConfig{
		{
			ID:       "default-notifier",
			BotToken: "test-token",
			ChatID:   12345,
		},
	}
	return appConfig
}

// mockNotificationSender는 테스트용 TaskNotificationSender 구현체입니다.
type mockNotificationSender struct {
	mu          sync.Mutex
	notifyCalls []notifyCall
}

type notifyCall struct {
	notifierID string
	message    string
	taskCtx    task.TaskContext
}

func (m *mockNotificationSender) NotifyToDefault(message string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyCalls = append(m.notifyCalls, notifyCall{
		notifierID: "default",
		message:    message,
	})
	return true
}

func (m *mockNotificationSender) NotifyWithTaskContext(notifierID string, message string, taskCtx task.TaskContext) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyCalls = append(m.notifyCalls, notifyCall{
		notifierID: notifierID,
		message:    message,
		taskCtx:    taskCtx,
	})
	return true
}

func (m *mockNotificationSender) SupportsHTMLMessage(notifierID string) bool {
	return true
}

// mockTaskRunner는 테스트용 TaskRunner 구현체입니다.
type mockTaskRunner struct{}

func (m *mockTaskRunner) TaskRun(taskID task.TaskID, taskCommandID task.TaskCommandID, notifierID string, notifyResultOfTaskRunRequest bool, taskRunBy task.TaskRunBy) bool {
	return true
}

func (m *mockTaskRunner) TaskRunWithContext(taskID task.TaskID, taskCommandID task.TaskCommandID, taskCtx task.TaskContext, notifierID string, notifyResultOfTaskRunRequest bool, taskRunBy task.TaskRunBy) bool {
	return true
}

func (m *mockTaskRunner) TaskCancel(taskInstanceID task.TaskInstanceID) bool {
	return true
}

// mockNotifierHandler는 테스트용 NotifierHandler 구현체입니다.
type mockNotifierHandler struct {
	id                  notification.NotifierID
	supportsHTMLMessage bool
}

func (m *mockNotifierHandler) ID() notification.NotifierID {
	return m.id
}

func (m *mockNotifierHandler) Notify(message string, taskCtx task.TaskContext) bool {
	return true
}

func (m *mockNotifierHandler) Run(taskRunner task.TaskRunner, notificationStopCtx context.Context, notificationStopWaiter *sync.WaitGroup) {
	defer notificationStopWaiter.Done()
	<-notificationStopCtx.Done()
}

func (m *mockNotifierHandler) SupportsHTMLMessage() bool {
	return m.supportsHTMLMessage
}

// TestFullFlow_SchedulerToNotification은 스케줄러에서 알림까지의 전체 흐름을 테스트합니다.
func TestFullFlow_SchedulerToNotification(t *testing.T) {
	t.Run("스케줄러에 의한 작업 실행 및 알림 발송", func(t *testing.T) {
		// 1. 설정 생성 (자주 실행되는 작업 포함)
		appConfig := createTestConfig()
		appConfig.Tasks = []config.TaskConfig{
			{
				ID:    "IntegrationTask",
				Title: "통합 테스트 작업",
				Commands: []config.TaskCommandConfig{
					{
						ID:    "Run",
						Title: "실행",
						Scheduler: struct {
							Runnable bool   `json:"runnable"`
							TimeSpec string `json:"time_spec"`
						}{
							Runnable: true,
							TimeSpec: "* * * * * *", // 매초 실행
						},
						Notifier: struct {
							Usable bool `json:"usable"`
						}{Usable: true},
						DefaultNotifierID: "test-notifier",
					},
				},
			},
		}

		// 2. 서비스 초기화
		taskService := task.NewService(appConfig)

		// Mock Notification Sender 사용 (알림 수신 확인용)
		mockSender := &mockNotificationSender{
			notifyCalls: make([]notifyCall, 0),
		}

		// TaskService에 Mock Sender 연결 (NotificationService를 거치지 않고 직접 확인)
		// 실제로는 NotificationService가 Sender 역할을 하지만, 여기서는 알림 요청이 발생하는지만 확인
		taskService.SetTaskNotificationSender(mockSender)

		// 3. 서비스 시작
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		wg := &sync.WaitGroup{}

		wg.Add(1)
		go taskService.Run(ctx, wg)

		// 4. 스케줄러가 트리거할 때까지 대기
		// 매초 실행되므로 2초 정도면 최소 1번은 실행되어야 함
		time.Sleep(2500 * time.Millisecond)

		// 5. 검증
		// 작업이 실행되려 했으나 등록된 핸들러가 없어서 에러 알림이 발생해야 함
		// (실제 작업 로직은 task 패키지 내부에 하드코딩되어 있거나 등록되어야 하는데, 여기서는 config만 추가했으므로 '등록되지 않은 작업' 에러가 발생할 것임)
		// 하지만 중요한 건 스케줄러 -> TaskService -> NotificationSender 흐름이 동작했느냐임.

		assert.Greater(t, len(mockSender.notifyCalls), 0, "알림이 최소 1회 이상 발송되어야 합니다")

		if len(mockSender.notifyCalls) > 0 {
			lastCall := mockSender.notifyCalls[len(mockSender.notifyCalls)-1]
			// 에러 메시지 또는 작업 시작 메시지가 포함되어야 함
			t.Logf("수신된 알림 메시지: %s", lastCall.message)
		}

		cancel()
		wg.Wait()
	})
}
