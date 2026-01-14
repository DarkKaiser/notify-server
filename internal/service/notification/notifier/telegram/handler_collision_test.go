package telegram

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestTelegramNotifier_Notify_Collision verifies that outgoing notifications resolve the correct title
// even when TaskID and CommandID combinations might collide in a naive string key implementation.
//
// Scenario:
// Task 1: ID="task_a", Command="run" -> Naive Key: "task_a_run"
// Task 2: ID="task", Command="a_run" -> Naive Key: "task_a_run"
//
// With the nested map fix, we expect to retrieve the correct title for each unique combination.
func TestTelegramNotifier_Notify_Collision(t *testing.T) {
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID:    "task_a",
				Title: "Task A Title",
				Commands: []config.CommandConfig{
					{
						ID:          "run",
						Title:       "Command Run Title",
						Description: "Desc 1",
						Notifier: struct {
							Usable bool `json:"usable"`
						}{Usable: true},
					},
				},
			},
			{
				ID:    "task",
				Title: "Task Title",
				Commands: []config.CommandConfig{
					{
						ID:          "a_run",
						Title:       "Command A_Run Title",
						Description: "Desc 2",
						Notifier: struct {
							Usable bool `json:"usable"`
						}{Usable: true},
					},
				},
			},
		},
		Debug: true,
	}

	tests := []struct {
		name          string
		taskID        string
		commandID     string
		expectedTitle string
	}{
		{
			name:          "Task A / Run",
			taskID:        "task_a",
			commandID:     "run",
			expectedTitle: "Task A Title &gt; Command Run Title",
		},
		{
			name:          "Task / A_Run",
			taskID:        "task",
			commandID:     "a_run",
			expectedTitle: "Task Title &gt; Command A_Run Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			notifierInstance, mockBot, mockExecutor := setupTelegramTest(t, appConfig)
			require.NotNil(t, notifierInstance)
			require.NotNil(t, mockBot)
			require.NotNil(t, mockExecutor)

			// Mock setup
			mockBot.On("GetSelf").Return(tgbotapi.User{UserName: testTelegramBotUsername}).Maybe()
			mockBot.On("GetUpdatesChan", mock.Anything).Return((tgbotapi.UpdatesChannel)(mockBot.updatesChan)).Maybe()
			mockBot.On("StopReceivingUpdates").Return().Maybe()

			var wgAction sync.WaitGroup
			wgAction.Add(1)

			// Verify that the sent message contains the expected title
			mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
				msg, ok := c.(tgbotapi.MessageConfig)
				if !ok {
					return false
				}
				// 텔레그램 메시지 내에 기대하는 제목이 포함되어 있는지 확인
				// msgContextTitle format: "<b>【 %s 】</b>\n\n%s"
				return strings.Contains(msg.Text, tt.expectedTitle)
			})).Run(func(args mock.Arguments) {
				wgAction.Done()
			}).Return(tgbotapi.Message{}, nil)

			// Run Notifier
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var wg sync.WaitGroup
			runTelegramNotifier(ctx, notifierInstance, &wg)

			// Execute Notify
			taskCtx := task.NewTaskContext().WithTask(task.ID(tt.taskID), task.CommandID(tt.commandID))
			ok := notifierInstance.Notify(taskCtx, "Test Message")
			require.True(t, ok, "Notify should be accepted")

			// Wait for Send to be called
			waitForActionWithTimeout(t, &wgAction, 3*time.Second)

			// Cleanup
			cancel()
			wg.Wait()

			mockBot.AssertExpectations(t)
		})
	}
}
