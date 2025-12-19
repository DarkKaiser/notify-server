package notification

import (
	"context"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
)

// =============================================================================
// Telegram Bot Mock
// =============================================================================

// MockTelegramBot은 telegramBotAPI 인터페이스의 Mock 구현체입니다.
//
// 이 Mock은 Telegram 봇 테스트에서 실제 Telegram API 호출 없이
// 봇 동작을 검증하는 데 사용됩니다.
//
// 주요 기능:
//   - 메시지 전송 시뮬레이션
//   - 업데이트 수신 시뮬레이션
//   - 봇 정보 조회 시뮬레이션
type MockTelegramBot struct {
	mock.Mock
	updatesChan chan tgbotapi.Update
}

// GetUpdatesChan은 Telegram 업데이트를 수신하는 채널을 반환합니다.
func (m *MockTelegramBot) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	m.Called(config)
	if m.updatesChan == nil {
		m.updatesChan = make(chan tgbotapi.Update, 100)
	}
	return m.updatesChan
}

// Send는 Telegram 메시지를 전송합니다.
func (m *MockTelegramBot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	args := m.Called(c)
	return args.Get(0).(tgbotapi.Message), args.Error(1)
}

// StopReceivingUpdates는 업데이트 수신을 중지합니다.
func (m *MockTelegramBot) StopReceivingUpdates() {
	m.Called()
}

// GetSelf는 봇 자신의 정보를 반환합니다.
func (m *MockTelegramBot) GetSelf() tgbotapi.User {
	args := m.Called()
	return args.Get(0).(tgbotapi.User)
}

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
func (m *mockNotifierHandler) Notify(taskCtx task.TaskContext, message string) bool {
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
