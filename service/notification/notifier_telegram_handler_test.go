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
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helpers
// =============================================================================

// createLongString은 테스트용 긴 문자열을 생성합니다.
func createLongString(chunk string, repeat int) string {
	var sb strings.Builder
	for i := 0; i < repeat; i++ {
		sb.WriteString(chunk)
	}
	return sb.String()
}

// =============================================================================
// Message Sending Tests
// =============================================================================

// TestTelegramNotifier_Notify_TableDriven은 Telegram 메시지 전송을 검증합니다.
//
// 검증 항목:
//   - 긴 메시지 분할 전송 (개행 포함)
//   - 긴 메시지 분할 전송 (단일 라인)
//   - HTML 메시지 전송
//   - 네트워크 에러 처리
//   - Task Context 포함 메시지
//   - Error Context 포함 메시지
//   - 경과 시간 포함 메시지
func TestTelegramNotifier_Notify_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		message      string
		taskCtx      task.TaskContext
		setupMockBot func(*MockTelegramBot, *sync.WaitGroup)
		waitForCalls int
	}{
		{
			name:    "Long Message with Newlines",
			message: createLongString("0123456789\n", 400), // ~4400 chars
			taskCtx: task.NewTaskContext(),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(2) // Expect 2 messages
				m.On("Send", mock.Anything).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Times(2)
			},
			waitForCalls: 2,
		},
		{
			name:    "Long Single Line Message",
			message: createLongString("0123456789", 400), // 4000 chars
			taskCtx: task.NewTaskContext(),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(2)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					// Check splitting logic
					return ok && len(msg.Text) > 0 && len(msg.Text) <= 3900
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Times(2)
			},
			waitForCalls: 2,
		},
		{
			name:    "HTML Message",
			message: "<b>Bold</b> and <i>Italic</i> text",
			taskCtx: task.NewTaskContext(),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && msg.ParseMode == "HTML"
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
			waitForCalls: 1,
		},
		{
			name:    "Send Error (Network)",
			message: "Test message",
			taskCtx: task.NewTaskContext(),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Send", mock.Anything).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, errors.New("network error"))
			},
			waitForCalls: 1,
		},
		{
			name:    "With Task Context",
			message: "Test message",
			taskCtx: task.NewTaskContext().
				WithTask(task.ID("TEST"), task.CommandID("TEST_CMD")).
				WithTitle("Test Task"),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && strings.Contains(msg.Text, "Test Task")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
			waitForCalls: 1,
		},
		{
			name:    "With Error Context",
			message: "Error occurred",
			taskCtx: task.NewTaskContext().WithError(),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					_, ok := c.(tgbotapi.MessageConfig)
					return ok
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
			waitForCalls: 1,
		},
		{
			name:    "With Elapsed Time",
			message: "Task Completed",
			taskCtx: task.NewTaskContext().
				WithInstanceID(task.InstanceID("1234"), int64(3661)), // 1h 1m 1s
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok &&
						(strings.Contains(msg.Text, "1시간") ||
							strings.Contains(msg.Text, "1분") ||
							strings.Contains(msg.Text, "1초"))
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
			waitForCalls: 1,
		},
		{
			name:    "Cancelable Task (Running)",
			message: "Task Running",
			taskCtx: task.NewTaskContext().
				WithInstanceID(task.InstanceID("INST_RUN"), 0).
				WithCancelable(true),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && strings.Contains(msg.Text, "/cancel_INST_RUN")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
			waitForCalls: 1,
		},
		{
			name:    "Finished Task (Not Cancelable)",
			message: "Task Finished",
			taskCtx: task.NewTaskContext().
				WithInstanceID(task.InstanceID("INST_FIN"), 100).
				WithCancelable(false),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					// Should contain elapsed time but NOT contain cancel command
					return ok &&
						strings.Contains(msg.Text, "1분") &&
						!strings.Contains(msg.Text, "/cancel_INST_FIN")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
			waitForCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			appConfig := &config.AppConfig{}
			notifier, mockBot, _ := setupTelegramTest(t, appConfig)
			require.NotNil(t, notifier)
			require.NotNil(t, mockBot)

			// Setup expectations
			var wgSend sync.WaitGroup
			if tt.setupMockBot != nil {
				tt.setupMockBot(mockBot, &wgSend)
			}

			// Run notifier
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var wg sync.WaitGroup
			runTelegramNotifier(ctx, notifier, &wg)

			// Act
			notifier.Notify(tt.taskCtx, tt.message)

			// Wait
			waitForActionWithTimeout(t, &wgSend, 2*time.Second)

			// Cleanup
			cancel()
			wg.Wait()

			mockBot.AssertExpectations(t)
		})
	}
}
