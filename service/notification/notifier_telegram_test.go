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
	"github.com/stretchr/testify/mock"
)

func TestTelegramNotifier_Run_Commands_Table(t *testing.T) {
	chatID := int64(12345)

	// Config for Task Command test
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

	tests := []struct {
		name          string
		commandText   string
		setupMockBot  func(*MockTelegramBot, *sync.WaitGroup)
		setupMockExec func(*MockExecutor, *sync.WaitGroup)
	}{
		{
			name:        "Help Command",
			commandText: "/help",
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && msg.ChatID == chatID && strings.Contains(msg.Text, "/help")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
		},
		{
			name:        "Cancel Command",
			commandText: "/cancel_1234",
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				// No response expected from bot for cancel in current logic, usually just action
				// But implementation might send ack? Checking current logic:
				// It calls taskRunner.Cancel.
			},
			setupMockExec: func(m *MockExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Cancel", task.InstanceID("1234")).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(nil)
			},
		},
		{
			name:        "Unknown Command",
			commandText: "/unknown",
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && msg.ChatID == chatID && strings.Contains(msg.Text, "등록되지 않은 명령어")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
		},
		{
			name:        "Task Command",
			commandText: "/test_task_run",
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				// Task command might send ack? implementation detail.
				// Usually Run calls executor.Run.
			},
			setupMockExec: func(m *MockExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Run", mock.MatchedBy(func(req *task.RunRequest) bool {
					return req.TaskID == "test_task" &&
						req.TaskCommandID == "run" &&
						req.NotifierID == "test-notifier" &&
						req.RunBy == task.RunByUser
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockBot := &MockTelegramBot{
				updatesChan: make(chan tgbotapi.Update, 1),
			}
			mockExecutor := &MockExecutor{}

			notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

			// Common Mock Expectations
			mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
			mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)
			mockBot.On("StopReceivingUpdates").Return()

			// Test specific expectations
			var wgAction sync.WaitGroup
			if tt.setupMockBot != nil {
				tt.setupMockBot(mockBot, &wgAction)
			}
			if tt.setupMockExec != nil {
				tt.setupMockExec(mockExecutor, &wgAction)
			}

			// Run
			ctx, cancel := context.WithCancel(context.Background())
			wg := &sync.WaitGroup{}
			wg.Add(1)
			go notifier.Run(ctx, mockExecutor, wg)

			// Trigger Command
			mockBot.updatesChan <- tgbotapi.Update{
				Message: &tgbotapi.Message{
					Chat: &tgbotapi.Chat{ID: chatID},
					Text: tt.commandText,
				},
			}

			// Wait for action
			done := make(chan struct{})
			go func() {
				wgAction.Wait()
				close(done)
			}()

			select {
			case <-done:
				// Success
			case <-time.After(1 * time.Second):
				t.Fatal("Timeout waiting for command action")
			}

			// Cleanup
			cancel()
			wg.Wait()

			mockBot.AssertExpectations(t)
			mockExecutor.AssertExpectations(t)
		})
	}
}
