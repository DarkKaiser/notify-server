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
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Constants
// =============================================================================

const (
	testTelegramChatID      = int64(12345)
	testTelegramBotUsername = "test_bot"
	testTelegramNotifierID  = "test-notifier"
	testTelegramTimeout     = 1 * time.Second
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupTelegramTest는 Telegram 테스트를 위한 공통 설정을 생성합니다.
func setupTelegramTest(t *testing.T, appConfig *config.AppConfig) (*telegramNotifier, *MockTelegramBot, *MockExecutor) {
	t.Helper()

	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update, 1),
	}
	mockExecutor := &MockExecutor{}

	notifierHandler := newTelegramNotifierWithBot(testTelegramNotifierID, mockBot, testTelegramChatID, appConfig, mockExecutor)
	notifier := notifierHandler.(*telegramNotifier)

	// Common expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: testTelegramBotUsername})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)
	mockBot.On("StopReceivingUpdates").Return()

	return notifier, mockBot, mockExecutor
}

// runTelegramNotifier는 Telegram notifier를 고루틴에서 실행합니다.
func runTelegramNotifier(ctx context.Context, notifier *telegramNotifier, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		notifier.Run(ctx)
	}()
}

// waitForActionWithTimeout은 액션 완료를 타임아웃과 함께 대기합니다.
func waitForActionWithTimeout(t *testing.T, wg *sync.WaitGroup, timeout time.Duration) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(timeout):
		t.Fatal("Timeout waiting for action")
	}
}

// =============================================================================
// Command Processing Tests
// =============================================================================

// TestTelegramNotifier_Run_Commands_Table은 Telegram 봇의 명령어 처리를 검증합니다.
//
// 검증 항목:
//   - /help 명령어 처리
//   - /cancel 명령어 처리
//   - 알 수 없는 명령어 처리
//   - Task 실행 명령어 처리
func TestTelegramNotifier_Run_Commands_Table(t *testing.T) {
	// Config for Task Command test
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID:    "test_task",
				Title: "Test Task",
				Commands: []config.CommandConfig{
					{
						ID:          "run",
						Title:       "Run Task",
						Description: "Runs the test task",
						Notifier: struct {
							Usable bool `json:"usable"`
						}{Usable: true},
						DefaultNotifierID: testTelegramNotifierID,
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
					return ok && msg.ChatID == testTelegramChatID && strings.Contains(msg.Text, "/help")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
		},
		{
			name:        "Cancel Command",
			commandText: "/cancel_1234",
			setupMockExec: func(m *MockExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("CancelTask", task.InstanceID("1234")).Run(func(args mock.Arguments) {
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
					return ok && msg.ChatID == testTelegramChatID && strings.Contains(msg.Text, "등록되지 않은 명령어")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
		},
		{
			name:        "Task Command",
			commandText: "/test_task_run",
			setupMockExec: func(m *MockExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("SubmitTask", mock.MatchedBy(func(req *task.SubmitRequest) bool {
					return req.TaskID == "test_task" &&
						req.CommandID == "run" &&
						req.NotifierID == testTelegramNotifierID &&
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
			notifier, mockBot, mockExecutor := setupTelegramTest(t, appConfig)
			require.NotNil(t, notifier)
			require.NotNil(t, mockBot)
			require.NotNil(t, mockExecutor)

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
			defer cancel()

			var wg sync.WaitGroup
			runTelegramNotifier(ctx, notifier, &wg)

			// Trigger Command
			mockBot.updatesChan <- tgbotapi.Update{
				Message: &tgbotapi.Message{
					Chat: &tgbotapi.Chat{ID: testTelegramChatID},
					Text: tt.commandText,
				},
			}

			// Wait for action
			waitForActionWithTimeout(t, &wgAction, testTelegramTimeout)

			// Cleanup
			cancel()
			wg.Wait()

			mockBot.AssertExpectations(t)
			mockExecutor.AssertExpectations(t)
		})
	}
}
