package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service/api"
	"github.com/darkkaiser/notify-server/service/notification"
	"github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/assert"
)

// TestServicesIntegration은 전체 서비스의 통합을 테스트합니다.
func TestServicesIntegration(t *testing.T) {
	t.Run("서비스 초기화 및 시작", func(t *testing.T) {
		// 테스트용 설정 생성
		config := createTestConfig()

		// 서비스 생성
		taskService := task.NewService(config)
		notificationService := notification.NewService(config, taskService)

		// Mock notifier 설정
		notificationService.SetNewNotifier(func(id notification.NotifierID, botToken string, chatID int64, config *g.AppConfig) notification.NotifierHandler {
			return &mockNotifierHandler{
				id:                 id,
				supportHTMLMessage: true,
			}
		})

		notifyAPIService := api.NewNotifyAPIService(config, notificationService)

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
		config := createTestConfig()
		taskService := task.NewService(config)
		notificationService := notification.NewService(config, taskService)

		// Mock notifier 설정
		notificationService.SetNewNotifier(func(id notification.NotifierID, botToken string, chatID int64, config *g.AppConfig) notification.NotifierHandler {
			return &mockNotifierHandler{
				id:                 id,
				supportHTMLMessage: true,
			}
		})

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
		config := createTestConfig()

		// Mock Notification Sender 생성
		mockSender := &mockNotificationSender{
			notifyCalls: make([]notifyCall, 0),
		}

		taskService := task.NewService(config)
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
}

// TestServiceLifecycle은 서비스 생명주기를 테스트합니다.
func TestServiceLifecycle(t *testing.T) {
	t.Run("서비스 시작 및 정상 종료", func(t *testing.T) {
		config := createTestConfig()

		taskService := task.NewService(config)
		notificationService := notification.NewService(config, taskService)

		// Mock notifier 설정
		notificationService.SetNewNotifier(func(id notification.NotifierID, botToken string, chatID int64, config *g.AppConfig) notification.NotifierHandler {
			return &mockNotifierHandler{
				id:                 id,
				supportHTMLMessage: true,
			}
		})

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
		config := createTestConfig()

		// 여러 서비스 생성
		services := make([]*task.TaskService, 3)
		for i := 0; i < 3; i++ {
			services[i] = task.NewService(config)
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
}

// TestNotificationServiceIntegration은 NotificationService 통합을 테스트합니다.
func TestNotificationServiceIntegration(t *testing.T) {
	t.Run("NotificationService 생성 및 초기화", func(t *testing.T) {
		config := createTestConfigWithNotifier()

		mockTaskRunner := &mockTaskRunner{}
		notificationService := notification.NewService(config, mockTaskRunner)

		// Mock notifier 설정
		notificationService.SetNewNotifier(func(id notification.NotifierID, botToken string, chatID int64, config *g.AppConfig) notification.NotifierHandler {
			return &mockNotifierHandler{
				id:                 id,
				supportHTMLMessage: true,
			}
		})

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
		config := createTestConfig()

		taskService := task.NewService(config)
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
func createTestConfig() *g.AppConfig {
	return &g.AppConfig{
		Debug: true,
		Notifiers: struct {
			DefaultNotifierID string `json:"default_notifier_id"`
			Telegrams         []struct {
				ID       string `json:"id"`
				BotToken string `json:"bot_token"`
				ChatID   int64  `json:"chat_id"`
			} `json:"telegrams"`
		}{
			DefaultNotifierID: "test-notifier",
			Telegrams: []struct {
				ID       string `json:"id"`
				BotToken string `json:"bot_token"`
				ChatID   int64  `json:"chat_id"`
			}{
				{
					ID:       "test-notifier",
					BotToken: "test-token",
					ChatID:   12345,
				},
			},
		},
		NotifyAPI: struct {
			WS struct {
				TLSServer   bool   `json:"tls_server"`
				TLSCertFile string `json:"tls_cert_file"`
				TLSKeyFile  string `json:"tls_key_file"`
				ListenPort  int    `json:"listen_port"`
			} `json:"ws"`
			Applications []struct {
				ID                string `json:"id"`
				Title             string `json:"title"`
				Description       string `json:"description"`
				DefaultNotifierID string `json:"default_notifier_id"`
				AppKey            string `json:"app_key"`
			} `json:"applications"`
		}{
			WS: struct {
				TLSServer   bool   `json:"tls_server"`
				TLSCertFile string `json:"tls_cert_file"`
				TLSKeyFile  string `json:"tls_key_file"`
				ListenPort  int    `json:"listen_port"`
			}{
				ListenPort: 18080,
				TLSServer:  false,
			},
		},
		Tasks: []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Commands []struct {
				ID          string `json:"id"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Scheduler   struct {
					Runnable bool   `json:"runnable"`
					TimeSpec string `json:"time_spec"`
				} `json:"scheduler"`
				Notifier struct {
					Usable bool `json:"usable"`
				} `json:"notifier"`
				DefaultNotifierID string                 `json:"default_notifier_id"`
				Data              map[string]interface{} `json:"data"`
			} `json:"commands"`
			Data map[string]interface{} `json:"data"`
		}{},
	}
}

func createTestConfigWithNotifier() *g.AppConfig {
	config := createTestConfig()
	config.Notifiers.DefaultNotifierID = "default-notifier"
	config.Notifiers.Telegrams = []struct {
		ID       string `json:"id"`
		BotToken string `json:"bot_token"`
		ChatID   int64  `json:"chat_id"`
	}{
		{
			ID:       "default-notifier",
			BotToken: "test-token",
			ChatID:   12345,
		},
	}
	return config
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

func (m *mockNotificationSender) SupportHTMLMessage(notifierID string) bool {
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
	id                 notification.NotifierID
	supportHTMLMessage bool
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

func (m *mockNotifierHandler) SupportHTMLMessage() bool {
	return m.supportHTMLMessage
}
