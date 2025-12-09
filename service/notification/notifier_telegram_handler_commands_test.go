package notification

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTelegramNotifier_HandleCommand(t *testing.T) {
	// Setup
	chatID := int64(12345)

	// AppConfig with tasks for commands
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: "task1",
				Commands: []config.TaskCommandConfig{
					{
						ID:          "run",
						Title:       "Task 1 Run",
						Description: "Run Task 1",
						Notifier: struct {
							Usable bool `json:"usable"`
						}{
							Usable: true,
						},
					},
				},
			},
		},
		Debug: true,
	}

	createTestNotifier := func(mockBot *MockTelegramBot) *telegramNotifier {
		notifierObj := newTelegramNotifierWithBot(NotifierID("test-notifier"), mockBot, chatID, appConfig)
		telegramNotifierObj, ok := notifierObj.(*telegramNotifier)
		if !ok {
			t.Fatal("Failed to cast notifier to *telegramNotifier")
		}
		return telegramNotifierObj
	}

	t.Run("알 수 없는 명령어", func(t *testing.T) {
		mockBot := &MockTelegramBot{
			updatesChan: make(chan tgbotapi.Update, 1),
		}
		sendMessageUpdate := func(text string) {
			mockBot.updatesChan <- tgbotapi.Update{
				Message: &tgbotapi.Message{
					Chat: &tgbotapi.Chat{
						ID: chatID,
					},
					Text: text,
				},
			}
		}

		notif := createTestNotifier(mockBot)

		done := make(chan struct{})

		mockBot.On("GetUpdatesChan", mock.Anything).Return((tgbotapi.UpdatesChannel)(mockBot.updatesChan)).Once()
		mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"}).Maybe()

		// Expect unknown command message
		mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
			msg, ok := c.(tgbotapi.MessageConfig)
			return ok && strings.Contains(msg.Text, "등록되지 않은 명령어")
		})).Run(func(args mock.Arguments) {
			close(done)
		}).Return(tgbotapi.Message{}, nil).Once()

		mockBot.On("StopReceivingUpdates").Return().Once()

		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go notif.Run(&MockTaskRunner{}, ctx, wg)

		sendMessageUpdate("/unknown")

		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for unknown command response")
		}

		cancel()
		wg.Wait()
		mockBot.AssertExpectations(t)
	})

	t.Run("Help 명령어", func(t *testing.T) {
		mockBot := &MockTelegramBot{
			updatesChan: make(chan tgbotapi.Update, 1),
		}
		sendMessageUpdate := func(text string) {
			mockBot.updatesChan <- tgbotapi.Update{
				Message: &tgbotapi.Message{
					Chat: &tgbotapi.Chat{
						ID: chatID,
					},
					Text: text,
				},
			}
		}

		notif := createTestNotifier(mockBot)

		done := make(chan struct{})

		mockBot.On("GetUpdatesChan", mock.Anything).Return((tgbotapi.UpdatesChannel)(mockBot.updatesChan)).Once()
		mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"}).Maybe()

		// Expect help message
		mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
			msg, ok := c.(tgbotapi.MessageConfig)
			return ok && strings.Contains(msg.Text, "/help") && strings.Contains(msg.Text, "/task1_run")
		})).Run(func(args mock.Arguments) {
			close(done)
		}).Return(tgbotapi.Message{}, nil).Once()

		mockBot.On("StopReceivingUpdates").Return().Once()

		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go notif.Run(&MockTaskRunner{}, ctx, wg)

		sendMessageUpdate("/help")

		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for help command response")
		}

		cancel()
		wg.Wait()
		mockBot.AssertExpectations(t)
	})

	t.Run("작업 실행 명령어", func(t *testing.T) {
		mockBot := &MockTelegramBot{
			updatesChan: make(chan tgbotapi.Update, 1),
		}
		sendMessageUpdate := func(text string) {
			mockBot.updatesChan <- tgbotapi.Update{
				Message: &tgbotapi.Message{
					Chat: &tgbotapi.Chat{
						ID: chatID,
					},
					Text: text,
				},
			}
		}

		notif := createTestNotifier(mockBot)

		done := make(chan struct{})
		var capturedTaskID task.TaskID
		var capturedCommandID task.TaskCommandID

		mockBot.On("GetUpdatesChan", mock.Anything).Return((tgbotapi.UpdatesChannel)(mockBot.updatesChan)).Once()
		mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"}).Maybe()
		mockBot.On("StopReceivingUpdates").Return().Once()

		mockTaskRunner := &MockTaskRunner{}
		mockTaskRunner.On("TaskRun", mock.Anything).
			Run(func(args mock.Arguments) {
				data := args.Get(0).(*task.TaskRunData)
				capturedTaskID = data.TaskID
				capturedCommandID = data.TaskCommandID
				close(done)
			}).Return(true)

		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}
		wg.Add(1)

		go notif.Run(mockTaskRunner, ctx, wg)

		sendMessageUpdate("/task1_run") // Snake case of task1 + run

		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for TaskRun")
		}

		assert.Equal(t, task.TaskID("task1"), capturedTaskID)
		assert.Equal(t, task.TaskCommandID("run"), capturedCommandID)

		cancel()
		wg.Wait()
		mockBot.AssertExpectations(t)
		mockTaskRunner.AssertExpectations(t)
	})
}
