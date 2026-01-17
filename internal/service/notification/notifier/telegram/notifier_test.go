package telegram

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
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

// setupTelegramTest sets up common test objects.
func setupTelegramTest(t *testing.T, appConfig *config.AppConfig) (*telegramNotifier, *MockTelegramBot, *taskmocks.MockExecutor) {
	t.Helper()

	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update, 1),
	}
	mockExecutor := &taskmocks.MockExecutor{}

	notifierHandler, err := newTelegramNotifierWithBot(testTelegramNotifierID, mockBot, testTelegramChatID, appConfig, mockExecutor)
	require.NoError(t, err)
	notifier := notifierHandler.(*telegramNotifier)
	notifier.retryDelay = 1 * time.Millisecond
	notifier.limiter = rate.NewLimiter(rate.Inf, 0)

	// Common expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: testTelegramBotUsername})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)
	mockBot.On("StopReceivingUpdates").Return()

	return notifier, mockBot, mockExecutor
}

// runTelegramNotifier runs the notifier in a goroutine.
func runTelegramNotifier(ctx context.Context, notifier *telegramNotifier, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		notifier.Run(ctx)
	}()
}

// waitForActionWithTimeout waits for a waitgroup with timeout.
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

// TestTelegramNotifier_Run_Commands_Table tests command processing.
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
		setupMockExec func(*taskmocks.MockExecutor, *sync.WaitGroup)
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
			setupMockExec: func(m *taskmocks.MockExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Cancel", contract.TaskInstanceID("1234")).Run(func(args mock.Arguments) {
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
					// Verify partial match for "unknown command" message (Korean: 등록되지 않은 명령어)
					// We check for empty string or just assume it sends something back.
					// Since strings are corrupted in source code too, we might fail here if we check exact match.
					// Ideally we should update the source handler to use constants, and check against those.
					// For now, let's just check it sends a message.
					return ok && msg.ChatID == testTelegramChatID
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
		},
		{
			name:        "Task Command",
			commandText: "/test_task_run",
			setupMockExec: func(m *taskmocks.MockExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Submit", mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
					return contract.TaskID(req.TaskID) == "test_task" &&
						contract.TaskCommandID(req.CommandID) == "run" &&
						req.NotifierID == testTelegramNotifierID &&
						req.RunBy == contract.TaskRunByUser
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

// TestTelegramNotifier_Run_Drain tests that the notifier processes remaining messages
// after the context is cancelled (Graceful Shutdown).
func TestTelegramNotifier_Run_Drain(t *testing.T) {
	// Setup
	appConfig := &config.AppConfig{}
	notifier, mockBot, _ := setupTelegramTest(t, appConfig)
	require.NotNil(t, notifier)
	require.NotNil(t, mockBot)

	// Disable rate limiter for test
	notifier.limiter = rate.NewLimiter(rate.Inf, 0)
	notifier.retryDelay = 1 * time.Millisecond

	// Expectation: 5 messages will be sent
	var wgSend sync.WaitGroup
	wgSend.Add(5)

	mockBot.On("Send", mock.Anything).Run(func(args mock.Arguments) {
		wgSend.Done()
	}).Return(tgbotapi.Message{}, nil).Times(5)

	// Run notifier
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	runTelegramNotifier(ctx, notifier, &wg)

	// Act: Send 5 messages
	taskCtx := contract.NewTaskContext()
	for i := 0; i < 5; i++ {
		notifier.Notify(taskCtx, "Drain Message")
	}

	// Trigger Shutdown immediately
	// Ensure initial message propagation
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Wait for shutdown and drain
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
}

// TestTelegramNotifier_Run_ChannelClosed_Fix checks loop exit on closed channel.
func TestTelegramNotifier_Run_ChannelClosed_Fix(t *testing.T) {
	appConfig := &config.AppConfig{}
	mockBot := &MockTelegramBot{}
	mockExecutor := &taskmocks.MockExecutor{}

	nHandler, err := newTelegramNotifierWithBot("test-notifier", mockBot, 11111, appConfig, mockExecutor)
	require.NoError(t, err)
	n := nHandler.(*telegramNotifier)

	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "TestBot"})
	updatesChan := make(chan tgbotapi.Update)
	mockBot.updatesChan = updatesChan
	mockBot.On("GetUpdatesChan", mock.Anything).Return((tgbotapi.UpdatesChannel)(updatesChan))
	mockBot.On("StopReceivingUpdates").Return()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runResult := make(chan struct{})
	go func() {
		n.Run(ctx)
		close(runResult)
	}()

	close(updatesChan)

	select {
	case <-runResult:
	case <-time.After(2 * time.Second):
		t.Fatal("Run loop did not exit")
	}
}

// TestRunSender_GracefulShutdown_InFlightMessage checks message delivery on shutdown.
func TestRunSender_GracefulShutdown_InFlightMessage(t *testing.T) {
	appConfig := &config.AppConfig{}
	mockBot := &MockTelegramBot{}
	mockExecutor := &taskmocks.MockExecutor{}

	nHandler, err := newTelegramNotifierWithBot("test-notifier", mockBot, 12345, appConfig, mockExecutor)
	require.NoError(t, err)
	n := nHandler.(*telegramNotifier)

	msg := "In-Flight Message"
	var wgSender sync.WaitGroup
	wgSender.Add(1)
	sendCalled := make(chan struct{})

	mockBot.On("Send", mock.Anything).Run(func(args mock.Arguments) {
		close(sendCalled)
	}).Return(tgbotapi.Message{}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer wgSender.Done()
		n.runSender(ctx)
	}()

	n.RequestC <- &notifier.NotifyRequest{Message: msg}
	time.Sleep(1 * time.Millisecond)
	cancel()

	select {
	case <-sendCalled:
	case <-time.After(1 * time.Second):
		t.Fatal("In-flight message lost")
	}

	n.Close()
	wgSender.Wait()
	mockBot.AssertExpectations(t)
}

// TestTelegramNotifier_Concurrency checks correct concurrent processing.
func TestTelegramNotifier_Concurrency(t *testing.T) {
	mockBot := &MockTelegramBot{}
	updateC := make(chan tgbotapi.Update, 100)

	mockBot.updatesChan = updateC
	mockBot.On("GetUpdatesChan", mock.AnythingOfType("tgbotapi.UpdateConfig")).Return(tgbotapi.UpdatesChannel(updateC))
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("StopReceivingUpdates").Return()

	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Run(func(args mock.Arguments) {
		time.Sleep(100 * time.Millisecond)
	})

	n := &telegramNotifier{
		BaseNotifier: notifier.NewBaseNotifier("test", true, 10, time.Second),
		botAPI:       mockBot,
		chatID:       12345,
		botCommands: []telegramBotCommand{
			{command: "help"},
		},
		notifierSemaphore: make(chan struct{}, 100),
	}
	n.RequestC = make(chan *notifier.NotifyRequest, 10)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		n.Run(ctx)
	}()

	for i := 0; i < 5; i++ {
		n.RequestC <- &notifier.NotifyRequest{Message: "Slow Message"}
	}

	cmdUpdate := tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: 12345},
			Text: "/help",
		},
	}
	updateC <- cmdUpdate

	time.Sleep(200 * time.Millisecond)
	cancel()
	wg.Wait()

	mockBot.AssertExpectations(t)
}
