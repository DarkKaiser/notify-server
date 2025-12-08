package notification

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
)

// MockTelegramBot is a mock implementation of TelegramBot interface
type MockTelegramBot struct {
	mock.Mock
	updatesChan chan tgbotapi.Update
}

func (m *MockTelegramBot) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	m.Called(config)
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

// MockTaskRunner is a mock implementation of TaskRunner interface
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

func TestTelegramNotifier_Run_HelpCommand(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update, 1),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &config.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil) // Return value ignored as we use the channel directly
	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		return ok && msg.ChatID == chatID && msg.Text != "" // Check if help text is sent
	})).Return(tgbotapi.Message{}, nil)
	mockBot.On("StopReceivingUpdates").Return()

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Trigger Help Command
	mockBot.updatesChan <- tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: chatID},
			Text: "/help",
		},
	}

	// Wait for processing (simple sleep for test stability)
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
}

func TestTelegramNotifier_Run_CancelCommand(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update, 1),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &config.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)
	mockBot.On("StopReceivingUpdates").Return()

	// Expect TaskCancel to be called
	mockTaskRunner.On("TaskCancel", task.TaskInstanceID("1234")).Return(true)

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Trigger Cancel Command: /cancel_1234
	mockBot.updatesChan <- tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: chatID},
			Text: "/cancel_1234",
		},
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
	mockTaskRunner.AssertExpectations(t)
}

func TestTelegramNotifier_Run_UnknownCommand(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update, 1),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &config.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)

	// Expect a reply about unknown command
	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		return ok && msg.ChatID == chatID && msg.Text != "" // Check if text is sent
	})).Return(tgbotapi.Message{}, nil)

	mockBot.On("StopReceivingUpdates").Return()

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Trigger Unknown Command
	mockBot.updatesChan <- tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: chatID},
			Text: "/unknown_command",
		},
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
}

func TestTelegramNotifier_Run_TaskCommand(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update, 1),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)

	// Construct config with a task command
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID:    "test_task",
				Title: "Test Task",
				Commands: []config.TaskCommandConfig{
					{
						ID:          "run",
						Title:       "Run Task",
						Description: "Runs the test task",
						Notifier: struct {
							Usable bool `json:"usable"`
						}{Usable: true},
						DefaultNotifierID: "test-notifier",
					},
				},
			},
		},
	}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)
	mockBot.On("StopReceivingUpdates").Return()

	// Expect TaskRun to be called
	// Command format: /task_id_command_id -> /test_task_run
	mockTaskRunner.On("TaskRun", task.TaskID("test_task"), task.TaskCommandID("run"), "test-notifier", true, task.TaskRunByUser).Return(true)

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Trigger Task Command
	mockBot.updatesChan <- tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: chatID},
			Text: "/test_task_run",
		},
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
	mockTaskRunner.AssertExpectations(t)
}
