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
			updatesChan: make(chan tgbotapi.Update),
		}
		// Helper to send message update
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

		mockBot.On("GetUpdatesChan", mock.Anything).Return((tgbotapi.UpdatesChannel)(mockBot.updatesChan)).Once()
		mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"}).Maybe()

		// Expect unknown command message
		mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
			msg, ok := c.(tgbotapi.MessageConfig)
			return ok && strings.Contains(msg.Text, "등록되지 않은 명령어")
		})).Return(tgbotapi.Message{}, nil).Once()

		mockBot.On("StopReceivingUpdates").Return().Once()

		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go notif.Run(&MockTaskRunner{}, ctx, wg)

		sendMessageUpdate("/unknown")
		time.Sleep(50 * time.Millisecond) // Wait for processing

		cancel()
		wg.Wait()
		mockBot.AssertExpectations(t)
	})

	t.Run("Help 명령어", func(t *testing.T) {
		mockBot := &MockTelegramBot{
			updatesChan: make(chan tgbotapi.Update),
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

		mockBot.On("GetUpdatesChan", mock.Anything).Return((tgbotapi.UpdatesChannel)(mockBot.updatesChan)).Once()
		mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"}).Maybe()

		// Expect help message
		mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
			msg, ok := c.(tgbotapi.MessageConfig)
			return ok && strings.Contains(msg.Text, "/help") && strings.Contains(msg.Text, "/task1_run")
		})).Return(tgbotapi.Message{}, nil).Once()

		mockBot.On("StopReceivingUpdates").Return().Once()

		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go notif.Run(&MockTaskRunner{}, ctx, wg)

		sendMessageUpdate("/help")
		time.Sleep(50 * time.Millisecond)

		cancel()
		wg.Wait()
		mockBot.AssertExpectations(t)
	})

	t.Run("작업 실행 명령어", func(t *testing.T) {
		mockBot := &MockTelegramBot{
			updatesChan: make(chan tgbotapi.Update),
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

		mockBot.On("GetUpdatesChan", mock.Anything).Return((tgbotapi.UpdatesChannel)(mockBot.updatesChan)).Once()
		mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"}).Maybe()
		mockBot.On("StopReceivingUpdates").Return().Once()

		// Expect NO response message for valid command (internal logic only calls TaskRunner)

		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}
		wg.Add(1)

		// Create a spy TaskRunner to verify calls
		spyTaskRunner := &MockTaskRunnerSpy{
			MockTaskRunner: MockTaskRunner{},
		}

		go notif.Run(spyTaskRunner, ctx, wg)

		sendMessageUpdate("/task1_run") // Snake case of task1 + run
		time.Sleep(50 * time.Millisecond)

		assert.Equal(t, 1, spyTaskRunner.callCount, "TaskRun should be called once")
		assert.Equal(t, task.TaskID("task1"), spyTaskRunner.lastTaskID)
		assert.Equal(t, task.TaskCommandID("run"), spyTaskRunner.lastCommandID)

		cancel()
		wg.Wait()
		mockBot.AssertExpectations(t)
	})
}

// MockTaskRunnerSpy extends MockTaskRunner to capture calls
type MockTaskRunnerSpy struct {
	MockTaskRunner
	callCount     int
	lastTaskID    task.TaskID
	lastCommandID task.TaskCommandID
}

func (m *MockTaskRunnerSpy) TaskRun(taskID task.TaskID, taskCommandID task.TaskCommandID, notifierID string, manualRun bool, runType task.TaskRunBy) bool {
	m.callCount++
	m.lastTaskID = taskID
	m.lastCommandID = taskCommandID
	return true
}

func (m *MockTaskRunnerSpy) TaskCancel(taskInstanceID task.TaskInstanceID) bool {
	return true
}
