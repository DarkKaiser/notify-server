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

// MockExecutor is a shared mock implementation of Executor interface
type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) Run(req *task.RunRequest) bool {
	args := m.Called(req)
	return args.Bool(0)
}

func (m *MockExecutor) Cancel(instanceID task.InstanceID) bool {
	args := m.Called(instanceID)
	return args.Bool(0)
}
