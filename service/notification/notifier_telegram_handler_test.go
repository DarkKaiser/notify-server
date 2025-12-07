package notification

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
)

func TestTelegramNotifier_Notify_LongMessage(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &config.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Create a long message (>3900 chars) with newlines
	longMessage := ""
	for i := 0; i < 400; i++ {
		longMessage += "0123456789\n" // 11 chars * 400 = 4400 chars
	}

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)

	// Expect two messages to be sent
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Times(2)

	mockBot.On("StopReceivingUpdates").Return()

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Send Notification
	notifier.Notify(longMessage, task.NewContext())

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
}

func TestTelegramNotifier_Notify_HTMLMessage(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &config.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	htmlMessage := "<b>Bold</b> and <i>Italic</i> text"

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)
	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		return ok && msg.ParseMode == "HTML"
	})).Return(tgbotapi.Message{}, nil)
	mockBot.On("StopReceivingUpdates").Return()

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Send HTML Notification
	notifier.Notify(htmlMessage, task.NewContext())

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
}

func TestTelegramNotifier_Notify_SendError(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &config.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, errors.New("network error"))
	mockBot.On("StopReceivingUpdates").Return()

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Send Notification (should handle error gracefully)
	notifier.Notify("Test message", task.NewContext())

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions - error is logged but doesn't affect return value
	mockBot.AssertExpectations(t)
}

func TestTelegramNotifier_Notify_WithTaskContext(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &config.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil)
	mockBot.On("StopReceivingUpdates").Return()

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Send Notification with TaskContext
	taskCtx := task.NewContext().
		WithTask(task.TaskID("TEST"), task.TaskCommandID("TEST_CMD")).
		With(task.TaskCtxKeyTitle, "Test Task")

	notifier.Notify("Test message", taskCtx)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
}

func TestTelegramNotifier_Notify_ErrorContext(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &config.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil)
	mockBot.On("StopReceivingUpdates").Return()

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Send Notification with Error Context
	taskCtx := task.NewContext().
		WithTask(task.TaskID("TEST"), task.TaskCommandID("TEST_CMD")).
		WithError()

	notifier.Notify("Error occurred", taskCtx)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
}

func TestTelegramNotifier_Notify_ElapsedTime(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &config.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)

	// Expect message with elapsed time
	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		// Check for elapsed time string "1시간 1분 1초" (3661 seconds)
		return ok && msg.ParseMode == "HTML" &&
			(strings.Contains(msg.Text, "1시간") || strings.Contains(msg.Text, "1분") || strings.Contains(msg.Text, "1초"))
	})).Return(tgbotapi.Message{}, nil)

	mockBot.On("StopReceivingUpdates").Return()

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Send Notification with Elapsed Time
	taskCtx := task.NewContext().
		With(task.TaskCtxKeyTaskInstanceID, task.TaskInstanceID("1234")).
		With(task.TaskCtxKeyElapsedTimeAfterRun, int64(3661)) // 1h 1m 1s

	notifier.Notify("Task Completed", taskCtx)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
}
