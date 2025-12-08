package notification

import (
	"github.com/darkkaiser/notify-server/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
)

// MockTelegramBot is a shared mock implementation of TelegramBotAPI
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

// MockTaskRunner is a shared mock implementation of TaskRunner interface
type MockTaskRunner struct {
	mock.Mock
}

func (m *MockTaskRunner) TaskRun(taskID task.TaskID, taskCommandID task.TaskCommandID, notifierID string, manualRun bool, runType task.TaskRunBy) bool {
	args := m.Called(taskID, taskCommandID, notifierID, manualRun, runType)
	return args.Bool(0)
}

func (m *MockTaskRunner) TaskRunWithContext(taskID task.TaskID, taskCommandID task.TaskCommandID, taskCtx task.TaskContext, notifierID string, notifyResultOfTaskRunRequest bool, taskRunBy task.TaskRunBy) (succeeded bool) {
	args := m.Called(taskID, taskCommandID, taskCtx, notifierID, notifyResultOfTaskRunRequest, taskRunBy)
	return args.Bool(0)
}

func (m *MockTaskRunner) TaskCancel(taskInstanceID task.TaskInstanceID) bool {
	args := m.Called(taskInstanceID)
	return args.Bool(0)
}
