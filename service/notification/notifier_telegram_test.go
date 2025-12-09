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

// Mocks are defined in mock_test.go

func TestTelegramNotifier_Run_HelpCommand(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update, 1),
	}
	mockTaskRunner := &MockExecutor{}
	chatID := int64(12345)
	appConfig := &config.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Synchronization
	done := make(chan struct{})

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)

	// Expect Send to be called
	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		return ok && msg.ChatID == chatID && msg.Text != ""
	})).Run(func(args mock.Arguments) {
		close(done)
	}).Return(tgbotapi.Message{}, nil)

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

	// Wait for processing
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for Help command response")
	}

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
	mockTaskRunner := &MockExecutor{}
	chatID := int64(12345)
	appConfig := &config.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Synchronization
	done := make(chan struct{})

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)
	mockBot.On("StopReceivingUpdates").Return()

	// Expect TaskCancel to be called
	mockTaskRunner.On("Cancel", task.TaskInstanceID("1234")).Run(func(args mock.Arguments) {
		close(done)
	}).Return(true)

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
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for Cancel")
	}

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
	mockTaskRunner := &MockExecutor{}
	chatID := int64(12345)
	appConfig := &config.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Synchronization
	done := make(chan struct{})

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)

	// Expect a reply about unknown command
	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		return ok && msg.ChatID == chatID && msg.Text != ""
	})).Run(func(args mock.Arguments) {
		close(done)
	}).Return(tgbotapi.Message{}, nil)

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
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for Unknown command response")
	}

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
	mockTaskRunner := &MockExecutor{}
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

	// Synchronization
	done := make(chan struct{})

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)
	mockBot.On("StopReceivingUpdates").Return()

	// Expect TaskRun to be called
	mockTaskRunner.On("Run", mock.MatchedBy(func(req *task.RunRequest) bool {
		return req.TaskID == "test_task" &&
			req.TaskCommandID == "run" &&
			req.NotifierID == "test-notifier" &&
			req.NotifyOnStart == true &&
			req.RunBy == task.RunByUser
	})).Run(func(args mock.Arguments) {
		close(done)
	}).Return(true)

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
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for TaskRun")
	}

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
	mockTaskRunner.AssertExpectations(t)
}
