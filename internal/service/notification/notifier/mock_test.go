package notifier

import (
	"context"
	"sync"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/task"
	"github.com/stretchr/testify/mock"
)

// =============================================================================
// Task Executor Mock
// =============================================================================

// MockExecutor는 task.Executor 인터페이스의 Mock 구현체입니다.
//
// 이 Mock은 Task 실행 및 취소 동작을 테스트하는 데 사용됩니다.
type MockExecutor struct {
	mock.Mock
}

// SubmitTask는 Task를 제출합니다.
func (m *MockExecutor) SubmitTask(req *task.SubmitRequest) error {
	args := m.Called(req)
	return args.Error(0)
}

// CancelTask는 실행 중인 Task를 취소합니다.
func (m *MockExecutor) CancelTask(instanceID task.InstanceID) error {
	args := m.Called(instanceID)
	return args.Error(0)
}

// =============================================================================
// Notifier Handler Mock
// =============================================================================

// mockNotifierHandler는 NotifierHandler 인터페이스의 Mock 구현체입니다.
//
// 이 Mock은 알림 전송 동작을 테스트하는 데 사용되며,
// 실제 알림 전송 없이 호출 기록을 추적합니다.
type mockNotifierHandler struct {
	id           NotifierID
	supportsHTML bool
	notifyCalls  []mockNotifyCall
	mu           sync.Mutex // notifyCalls 동시성 보호
}

// mockNotifyCall은 Notify 메서드 호출 기록을 저장합니다.
type mockNotifyCall struct {
	message string
	taskCtx task.TaskContext
}

// ID는 Notifier의 고유 식별자를 반환합니다.
func (m *mockNotifierHandler) ID() NotifierID {
	return m.id
}

// Notify는 알림 메시지를 전송하고 호출 기록을 저장합니다.
// 동시성 안전을 위해 mutex로 보호됩니다.
func (m *mockNotifierHandler) Notify(taskCtx task.TaskContext, message string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyCalls = append(m.notifyCalls, mockNotifyCall{
		message: message,
		taskCtx: taskCtx,
	})
	return true
}

// Run은 Notifier를 실행하고 context가 종료될 때까지 대기합니다.
func (m *mockNotifierHandler) Run(notificationStopCtx context.Context) {
	<-notificationStopCtx.Done()
}

// SupportsHTML은 HTML 형식 메시지 지원 여부를 반환합니다.
func (m *mockNotifierHandler) SupportsHTML() bool {
	return m.supportsHTML
}

// =============================================================================
// Notifier Factory Mock
// =============================================================================

// mockNotifierFactory는 NotifierFactory 인터페이스의 Mock 구현체입니다.
//
// 이 Mock은 Notifier 생성 로직을 테스트하는 데 사용됩니다.
type mockNotifierFactory struct {
	createNotifiersFunc func(cfg *config.AppConfig, executor task.Executor) ([]NotifierHandler, error)
}

// CreateNotifiers는 설정에 따라 Notifier 목록을 생성합니다.
func (m *mockNotifierFactory) CreateNotifiers(cfg *config.AppConfig, executor task.Executor) ([]NotifierHandler, error) {
	if m.createNotifiersFunc != nil {
		return m.createNotifiersFunc(cfg, executor)
	}
	return []NotifierHandler{}, nil
}

// RegisterProcessor는 Notifier 설정 프로세서를 등록합니다.
func (m *mockNotifierFactory) RegisterProcessor(processor NotifierConfigProcessor) {}
