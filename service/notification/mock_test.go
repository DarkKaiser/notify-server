package notification

import (
	"context"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
)

// MockTelegramBot is a shared mock implementation of telegramBotAPI
type MockTelegramBot struct {
	mock.Mock
	updatesChan chan tgbotapi.Update
}

func (m *MockTelegramBot) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	m.Called(config)
	if m.updatesChan == nil {
		m.updatesChan = make(chan tgbotapi.Update, 100)
	}
	return m.updatesChan
}

func (m *MockTelegramBot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	args := m.Called(c)
	return args.Get(0).(tgbotapi.Message), args.Error(1)
}

func (m *MockTelegramBot) StopReceivingUpdates() {
	m.Called()
}

func (m *MockTelegramBot) GetSelf() tgbotapi.User {
	args := m.Called()
	return args.Get(0).(tgbotapi.User)
}

// MockExecutor is a shared mock implementation of Executor interface
type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) SubmitTask(req *task.SubmitRequest) error {
	args := m.Called(req)
	return args.Error(0)
}

func (m *MockExecutor) CancelTask(instanceID task.InstanceID) error {
	args := m.Called(instanceID)
	return args.Error(0)
}

// mockNotifierHandler is a shared mock implementation of NotifierHandler for tests
type mockNotifierHandler struct {
	id           NotifierID
	supportsHTML bool
	notifyCalls  []mockNotifyCall
}

type mockNotifyCall struct {
	message string
	taskCtx task.TaskContext
}

func (m *mockNotifierHandler) ID() NotifierID {
	return m.id
}

func (m *mockNotifierHandler) Notify(taskCtx task.TaskContext, message string) bool {
	m.notifyCalls = append(m.notifyCalls, mockNotifyCall{
		message: message,
		taskCtx: taskCtx,
	})
	return true
}

func (m *mockNotifierHandler) Run(notificationStopCtx context.Context) {
	<-notificationStopCtx.Done()
}

func (m *mockNotifierHandler) SupportsHTML() bool {
	return m.supportsHTML
}

// mockNotifierFactory is a shared mock implementation of NotifierFactory for tests
type mockNotifierFactory struct {
	createNotifiersFunc func(cfg *config.AppConfig, executor task.Executor) ([]NotifierHandler, error)
}

func (m *mockNotifierFactory) CreateNotifiers(cfg *config.AppConfig, executor task.Executor) ([]NotifierHandler, error) {
	if m.createNotifiersFunc != nil {
		return m.createNotifiersFunc(cfg, executor)
	}
	return []NotifierHandler{}, nil
}

func (m *mockNotifierFactory) RegisterProcessor(processor NotifierConfigProcessor) {}
